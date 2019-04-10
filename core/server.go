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
	configFile string
	//Version    string
)

// TODO: add deletion of closed, undeployed workflowEntries (runner + time param?)

// holds Interlook server config
// Keeps a list of configured and started extensions
type server struct {
	config                      *config.ServerConfiguration
	signals                     chan os.Signal
	workflowControllerShutdown  chan bool
	workflowHouseKeeperShutdown chan bool
	extensions                  map[string]Extension
	extensionChannels           map[string]*extensionChannels
	workflow                    workflow
	workflowEntries             *workflowEntries
	workflowControlTicker       *time.Ticker
	workflowHousekeeperTicker   *time.Ticker
	coreWG                      sync.WaitGroup // waitgroup for core processes sync
	extensionsWG                sync.WaitGroup // waitgroup for extensions sync
	apiServer                   *http.Server
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
	s.workflowControllerShutdown = make(chan bool)
	s.workflowHouseKeeperShutdown = make(chan bool)
	s.workflowControlTicker = time.NewTicker(s.config.Core.WorkflowControlInterval)
	s.workflowHousekeeperTicker = time.NewTicker(s.config.Core.WorkflowHousekeepingInterval)
	s.extensionChannels = make(map[string]*extensionChannels)

	// init workflow
	s.workflow = s.initWorkflow()

	// init configured extensions
	s.initExtensions()

	// init workflowEntries table
	s.workflowEntries = initWorkflowEntries(s.config.Core.WorkflowEntriesFile)
	if err := s.workflowEntries.loadFile(); err != nil {
		log.Errorf("Could not load entries from file: %v", err)
	}

	return s, nil
}

// initialize the workflow from config
func (s *server) initWorkflow() workflow {
	var wf workflow
	wf = make(map[int]string)

	for k, v := range strings.Split(s.config.Core.WorkflowSteps, ",") {
		wf[k+1] = v
	}

	// add run and end steps to workflow
	// useful if we want to use real transitions later
	wf[0] = undeployedState
	wf[len(wf)] = deployedState

	return wf
}

// initExtensions initializes the extensions that are configured in the workflow steps
func (s *server) initExtensions() {
	s.extensions = make(map[string]Extension)
	srvConf := structs.New(s.config)

	// get needed extensions from workflow
	for _, step := range s.workflow {
		ext := strings.Split(step, ".")
		if len(ext) == 2 {
			extType := strings.ToLower(ext[0])
			extName := strings.ToLower(ext[1])
			// loop through struct fields. Ifs are needed due to case sensitivity
			for _, f := range srvConf.Fields() {
				if strings.ToLower(f.Name()) == extType && f.Kind() == reflect.Struct {
					for _, n := range srvConf.Field(f.Name()).Fields() {
						if strings.ToLower(n.Name()) == extName {
							s.extensions[step] = n.Value().(Extension)
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

	// run workflowControl
	s.coreWG.Add(1)
	go s.workflowControlRunner()

	// run workflowHouseKeeper
	s.coreWG.Add(1)
	go s.workflowHousekeeper()

	// start all configured extensions
	// for each one, starts a dedicated listener goroutine
	for name, extension := range s.extensions {
		log.Printf("adding %v to extensionChannels", name)
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
			for name, extension := range s.extensions {
				log.Infof("Stopping extension %v", name)
				if err := extension.Stop(); err != nil {
					log.Errorf("Error stopping extension %v:%v", name, err)
				}
			}
			s.extensionsWG.Wait()
			s.stopAPI()
			s.workflowHouseKeeperShutdown <- true
			s.workflowControllerShutdown <- true
		}
	}()

	s.coreWG.Wait()
}

// extensionListener gets messages from extensions and send them to workflow
// tag messages with sender
// no need to handle shutdown as corresponding extensions will do
func (s *server) extensionListener(extension *extensionChannels) {
	log.Debugf("Listening for %v messages", extension.name)

	for {
		newMessage := <-extension.send
		// tag the message with it's sender
		newMessage.Sender = extension.name

		log.Debugf("Received message from %v, sending to flow control\n", extension.name)

		// inject message to workflow
		if err := s.workflowEntries.messageHandler(newMessage); err != nil {
			log.Errorf("Error %v when inserting %v to flow\n", err, newMessage.Service.Name)
		}
	}
}

// workflowControlRunner runs workflowControl every x seconds
func (s *server) workflowControlRunner() {
	for {
		select {
		case <-s.workflowControllerShutdown:
			log.Info("Stopping FlowControl")
			if err := s.workflowEntries.save(); err != nil {
				log.Error(err.Error())
			}
			log.Infof("Saved flow entries to %v", s.config.Core.WorkflowEntriesFile)
			s.coreWG.Done()
			return
		case <-s.workflowControlTicker.C:
			log.Debug("Running workflowControl")
			s.workflowControl()
		}
	}
}

// workflowControl compares received service definition with existing one
// triggers required action(s) to bring service state
// to desired state (deployed or undeployed)
func (s *server) workflowControl() {
	for k, v := range s.workflowEntries.Entries {
		if v.State != v.ExpectedState && !v.WorkInProgress {
			var msg messaging.Message

			if v.Error != "" && v.CloseTime.IsZero() {
				log.Warnf("Service %v is in error %v", v.Service.Name, v.Error)
				continue
			}

			log.Debugf("workflowControl: Service %v current state differs from target state", k)
			reverse := false
			if v.ExpectedState == undeployedState {
				reverse = true
			}

			nextStep, err := s.workflow.getNextStep(v.State, reverse)
			if err != nil {
				log.Errorf("Could not find next step for %v %v\n", k, err)
				continue
			}
			log.Debugf("next step: %v", nextStep)

			if s.workflow.isLastStep(nextStep, reverse) {
				log.Debugf("Closing flow entry %v (state: %v)\n", k, nextStep)
				s.workflowEntries.closeEntry(k, reverse)
				continue
			}

			s.workflowEntries.prepareForNextStep(k, nextStep, reverse)

			if reverse {
				msg.Action = messaging.DeleteAction
			} else {
				msg.Action = messaging.AddAction
			}

			msg.Service = v.Service
			// get the extension channel to write message to
			ext, ok := s.extensionChannels[nextStep]
			if !ok {
				log.Errorf("workflowControl could not find channel for ext %v\n", nextStep)
				continue
			}

			ext.receive <- msg
		}
	}
}

func (s *server) workflowHousekeeper() {
	for {
		select {
		case <-s.workflowHouseKeeperShutdown:
			log.Info("Stopping workflowHousekeeper")
			s.coreWG.Done()
			return

		case <-s.workflowHousekeeperTicker.C:
			log.Info("Running workflowHousekeeper")
			s.workflowEntries.Lock()
			for k, v := range s.workflowEntries.Entries {
				if v.State == v.ExpectedState && !v.WorkInProgress {
					// remove old closed entry
					if v.State == undeployedState && time.Now().Sub(v.CloseTime) > s.config.Core.CleanUndeployedServiceAfter {
						delete(s.workflowEntries.Entries, k)
					}
					// ask refresh to provider
					if time.Now().Sub(v.LastUpdate) > s.config.Core.ServiceMaxLastUpdated {
						err := s.refreshService(v.Service.Name)
						if err != nil {
							log.Errorf("Error sending service refresh to provider %v", err)
						}
					}
				}

				// closing of WIP timed out
				if v.WorkInProgress && time.Now().Sub(v.WIPTime) > s.config.Core.ServiceWIPTimeout {
					errorMsg := fmt.Sprintf("Closed due to ServiceWIPTimeout reached at step %v. Err: %v", v.State, v.Error)
					v.CloseTime = time.Now()
					v.WorkInProgress = false
					v.WIPTime = time.Time{}
					v.Error = errorMsg
					v.State = v.ExpectedState

					log.Warn(errorMsg)
				}
				// add closing of in error flows
			}
		}
		s.workflowEntries.Unlock()
	}
}

func (s *server) refreshService(serviceName string) error {
	log.Debugf("Sending refresh request for %v", serviceName)
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
			s.extensionChannels[name].receive <- msg
			return nil
		}
	}
	return errors.New("Could not send refresh message to provider")
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
