package core

import (
	"flag"
	"fmt"
	"github.com/bhuisgen/interlook/config"
	"github.com/bhuisgen/interlook/log"
	"github.com/bhuisgen/interlook/messaging"
	"github.com/fatih/structs"
	"github.com/pkg/errors"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"time"

	"os"
	"os/signal"
)

var (
	//	srv        server
	configFile         string
	Version            = "dev"
	workflow           workflowSteps
	coreForwardMessage chan messaging.Message
)

// holds Interlook server config
// Keeps a list of configured and started extensions
type server struct {
	config                      *config.ServerConfiguration
	apiServer                   *http.Server
	coreWG                      sync.WaitGroup
	extensionsWG                sync.WaitGroup
	signals                     chan os.Signal
	extensions                  map[string]Extension
	extensionChannels           map[string]*extensionChannels
	workflowEntries             *workflowEntries
	workflowHousekeeperTicker   *time.Ticker
	workflowHouseKeeperShutdown chan bool
	workflowHousekeeperWG       sync.WaitGroup
}

// Start initialize server and run it
func Start() {
	srv, err := initServer()
	if err != nil {
		log.Fatal(err)
	}
	srv.run()
}

// initialize the server components
func initServer() (server, error) {
	var err error
	var s server

	flag.StringVar(&configFile, "conf", "", "interlook configuration file")
	flag.Parse()

	s.config, err = config.ReadConfig(configFile)
	if err != nil {
		return s, err
	}

	// init logger
	log.Init(s.config.Core.LogFile, s.config.Core.LogLevel)
	log.Debug("logger ok")

	// init channels and maps
	s.signals = make(chan os.Signal, 1)
	s.workflowHouseKeeperShutdown = make(chan bool)
	s.workflowHousekeeperTicker = time.NewTicker(s.config.Core.WorkflowHousekeeperInterval)
	s.extensionChannels = make(map[string]*extensionChannels)
	coreForwardMessage = make(chan messaging.Message)

	// init workflow
	initWorkflow(s.config.Core.WorkflowSteps)

	// init configured extensions
	s.initExtensions()

	// init workflowEntries table
	s.workflowEntries = initWorkflowEntries(s.config.Core.WorkflowEntriesFile)
	if err := s.workflowEntries.loadFile(); err != nil {
		log.Errorf("Could not load entries from file: %v", err)
	}

	return s, nil
}

// initExtensions initializes the extensions that are configured in the workflow steps
func (s *server) initExtensions() {
	s.extensions = make(map[string]Extension)
	srvConf := structs.New(s.config)

	// get needed extensions from workflow
	for _, step := range workflow {
		ext := strings.Split(step.Name, ".")
		if len(ext) == 2 {
			extType := strings.ToLower(ext[0])
			extName := strings.ToLower(ext[1])
			// loop through struct fields. Ifs are needed due to case sensitivity
			for _, f := range srvConf.Fields() {
				if strings.ToLower(f.Name()) == extType && f.Kind() == reflect.Struct {
					for _, n := range srvConf.Field(f.Name()).Fields() {
						if strings.ToLower(n.Name()) == extName {
							s.extensions[step.Name] = n.Value().(Extension)
							log.Infof("Extension %v initialized", step)
						}
					}
				}
			}
		}
	}
}

// run starts all core components and extensions
func (s *server) run() {
	signal.Notify(s.signals, os.Interrupt)
	signal.Notify(s.signals, os.Kill)

	// run workflowHouseKeeper
	s.coreWG.Add(1)
	go s.workflowHousekeeper()

	// run messageForwarder
	go s.messageSender()

	// start all configured extensions
	// for each one, starts a dedicated listener goroutine
	for name, extension := range s.extensions {
		extensionChannels := newExtensionChannels(name)
		s.extensionChannels[name] = extensionChannels

		// starts the extension's listener
		go s.extensionListener(extensionChannels)

		curExtension := extension
		extensionChan := extensionChannels

		// launch the extension
		s.extensionsWG.Add(1)
		go func() {
			err := curExtension.Start(extensionChan.receive, extensionChan.send)
			if err != nil {
				log.Fatalf("Cannot run extension %v: %v\n", extensionChan.name, err)
			}
			log.Debugf("Extension %v stopped", extensionChan.name)
			s.extensionsWG.Done()
		}()
	}

	// run http core
	s.coreWG.Add(1)
	go s.startAPI()

	// SIGs to handle proper extensions and core components shutdown
	go func() {
		for range s.signals {
			log.Info("Stopping workflow manager")

			s.workflowHouseKeeperShutdown <- true

			for name, extension := range s.extensions {
				log.Infof("Stopping extension %v", name)
				if err := extension.Stop(); err != nil {
					log.Errorf("Error stopping extension %v:%v", name, err)
				}
			}
			s.extensionsWG.Wait()
			log.Info("All extensions are down")
			s.stopAPI()
		}
	}()

	s.coreWG.Wait()
	if err := s.workflowEntries.save(); err != nil {
		log.Error(err.Error())
	}
	log.Infof("Saved flow entries to %v", s.config.Core.WorkflowEntriesFile)
}

// extensionListener gets messages from extensions and send them to workflow
// tag messages with sender
// no need to handle shutdown as corresponding extensions will do
func (s *server) extensionListener(extension *extensionChannels) {
	log.Infof("extensionListener for %v started", extension.name)

	for {
		newMessage := <-extension.send
		s.coreWG.Add(1)
		// tag the message with it's sender
		newMessage.Sender = extension.name

		log.Debugf("Received message from %v, sending to message handler", extension.name)

		// inject message to workflow
		if err := s.workflowEntries.messageHandler(newMessage); err != nil {
			log.Errorf("Error %v when inserting %v to flow\n", err, newMessage.Service.Name)
		}
		s.coreWG.Done()
	}
}

// workflowHousekeeper
func (s *server) workflowHousekeeper() {
	for {
		select {
		case <-s.workflowHouseKeeperShutdown:
			log.Info("Stopping workflowHousekeeper")
			s.coreWG.Done()
			return

		case <-s.workflowHousekeeperTicker.C:
			s.workflowHousekeeperWG.Add(1)
			log.Debug("Running workflowHousekeeper")
			s.workflowEntries.Lock()
			for k, entry := range s.workflowEntries.Entries {
				if entry.State == entry.ExpectedState && !entry.WorkInProgress {
					// remove old closed entry
					if entry.State == undeployedState && time.Now().Sub(entry.CloseTime) > s.config.Core.CleanUndeployedServiceAfter {
						delete(s.workflowEntries.Entries, k)
					}
				}
				// ask refresh to provider
				if time.Now().Sub(entry.LastUpdate) > s.config.Core.ServiceMaxLastUpdated && entry.State == deployedState {
					err := s.refreshService(entry.Service.Name)
					if err != nil {
						log.Errorf("Error sending service refresh to provider %entry", err)
					}
				}
				// closing of WIP timed out
				if entry.WorkInProgress && time.Now().Sub(entry.WIPTime) > s.config.Core.ServiceWIPTimeout {
					errorMsg := fmt.Sprintf("Closed due to ServiceWIPTimeout reached at step %v. Err: %v", entry.State, entry.Error)
					entry.close(errorMsg)
					log.Warn(errorMsg)
				}
				// add closing of in error flows
			}
			s.workflowHousekeeperWG.Done()
		}
		s.workflowEntries.Unlock()
	}
}

func (s *server) refreshService(serviceName string) error {
	log.Infof("Sending refresh request for %v", serviceName)
	for name, extension := range s.extensions {

		_, ok := extension.(Provider)
		if ok {
			msg := messaging.Message{
				Action: messaging.RefreshAction,
				Service: messaging.Service{
					Name: serviceName,
				},
			}
			log.Debugf("Sending refresh request to %v", name)
			s.sendMessageToExtension(msg, name)
			return nil
		}
	}
	return errors.New("Could not send refresh message to provider")
}

func (s *server) messageSender() {
	for {
		msg := <-coreForwardMessage
		log.Debugf("Forwarding msg %v", msg)
		s.sendMessageToExtension(msg, msg.Destination)
	}
}

func (s *server) sendMessageToExtension(msg messaging.Message, extensionName string) {
	// get the extension channel to write message to
	ext, ok := s.extensionChannels[extensionName]
	if !ok {
		log.Errorf("Could not find channel for ext %v\n", extensionName)
		return
	}

	ext.receive <- msg
}

// extensionChannels holds the "activated" extensions's channels
type extensionChannels struct {
	name    string
	receive chan messaging.Message
	send    chan messaging.Message
}

func newExtensionChannels(name string) *extensionChannels {
	p := new(extensionChannels)
	p.name = name
	p.send = make(chan messaging.Message)
	p.receive = make(chan messaging.Message)

	return p
}
