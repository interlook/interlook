package core

import (
	"flag"
	"fmt"
	"github.com/fatih/structs"
	"github.com/interlook/interlook/comm"
	"github.com/interlook/interlook/config"
	"github.com/interlook/interlook/log"
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
	configFile string
	Version    = "dev"
	wfConfig   string
)

// holds Interlook server config
// Keeps a list of configured and started extensions
type server struct {
	config              *config.ServerConfiguration
	apiServer           *http.Server
	coreWG              sync.WaitGroup
	extensionsWG        sync.WaitGroup
	signals             chan os.Signal
	extensions          map[string]Extension
	extensionChannels   map[string]*extensionChannels
	workflowEntries     *workflowEntries
	housekeeperTicker   *time.Ticker
	housekeeperShutdown chan bool
	housekeeperWG       sync.WaitGroup
	msgToExtension      chan comm.Message
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
	s.housekeeperShutdown = make(chan bool)
	s.housekeeperTicker = time.NewTicker(s.config.Core.WorkflowHousekeeperInterval)
	s.extensionChannels = make(map[string]*extensionChannels)
	s.msgToExtension = make(chan comm.Message)

	// init workflow
	wfConfig = s.config.Core.WorkflowSteps
	//workflow = initWorkflow(s.config.Core.workflowSteps)

	// init configured extensions
	s.initExtensions()

	// init workflowEntries table
	s.workflowEntries = newWorkflowEntries(s.config.Core.WorkflowEntriesFile, s.config.Core.WorkflowSteps, s.msgToExtension)
	if err := s.workflowEntries.load(s.msgToExtension); err != nil {
		log.Errorf("Could not load entries from file: %v", err)
	}

	return s, nil
}

// initExtensions initializes the extensions that are configured in the workflow steps
func (s *server) initExtensions() {
	s.extensions = make(map[string]Extension)
	srvConf := structs.New(s.config)

	// get needed extensions from workflow
	for _, step := range initWorkflow(wfConfig) {
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
	go s.housekeeper()

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

			s.housekeeperShutdown <- true
			log.Info("Sent stop signal to housekeeper")
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
		newMessage.SetTargetWeight()

		log.Debugf("Received message from %v, sending to message handler", extension.name)

		// inject message to workflow
		s.workflowEntries.mergeMessage(newMessage)
		s.coreWG.Done()
	}
}

// housekeeper manage the workflow entries list
func (s *server) housekeeper() {
	for {
		select {
		case <-s.housekeeperShutdown:
			log.Info("Stopping housekeeper")
			s.housekeeperTicker.Stop()
			s.coreWG.Done()
			return

		case <-s.housekeeperTicker.C:
			s.housekeeperWG.Add(1)
			log.Debug("Running housekeeper")
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
					err := s.refreshService(entry.Service.Name, entry.Service.Namespace)
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
			s.housekeeperWG.Done()
		}
		s.workflowEntries.Unlock()
	}
}

// refreshService request information/state of a given service to the provider
func (s *server) refreshService(serviceName, namespace string) error {
	log.Infof("Sending refresh request for %v", serviceName)
	for name, extension := range s.extensions {

		_, ok := extension.(Provider)
		if ok {
			msg := comm.Message{
				Action: comm.RefreshAction,
				Service: comm.Service{
					Name:      serviceName,
					Namespace: namespace,
				},
			}
			log.Debugf("Sending refresh request to %v", name)
			s.sendMessageToExtension(msg, name)
			return nil
		}
	}
	return errors.New("Could not send refresh message to provider")
}

// messageSender listen for messages in a go routine
func (s *server) messageSender() {
	for {
		msg := <-s.msgToExtension
		log.Debugf("Forwarding msg %v", msg)
		s.sendMessageToExtension(msg, msg.Destination)
	}
}

func (s *server) sendMessageToExtension(msg comm.Message, extensionName string) {
	// get the extension channel to write message to
	ext, ok := s.extensionChannels[extensionName]
	if !ok {
		log.Errorf("Could not find channel for ext %v\n", extensionName)
		return
	}

	ext.receive <- msg
}

// extensionChannels holds the activated extensions channels
type extensionChannels struct {
	name    string
	receive chan comm.Message
	send    chan comm.Message
}

func newExtensionChannels(name string) *extensionChannels {
	p := new(extensionChannels)
	p.name = name
	p.send = make(chan comm.Message)
	p.receive = make(chan comm.Message)

	return p
}
