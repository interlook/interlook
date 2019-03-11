package core

import (
	"flag"
	"github.com/bhuisgen/interlook/config"
	"github.com/bhuisgen/interlook/log"
	"github.com/bhuisgen/interlook/service"
	"sync"
	"time"

	"os"
	"os/signal"
)

var (
	srv        server
	configFile string
	Version    string
)

// holds the srv config
// Keeps a list of configured and started extensions
type server struct {
	config  *config.ServerConfiguration
	signals chan os.Signal
	//    extensionShutdown chan bool
	//shutdownOK  chan bool
	coreShutdown      chan bool
	extensions        map[string]Extension
	extensionChannels map[string]*extensionChannels
	workflow          workflow
	flowEntries       *flowEntries
	flowChan          chan service.Message
	flowControlTicker *time.Ticker
	wg                sync.WaitGroup
}

// extensionChannels holds the "activated" extensions's channels
type extensionChannels struct {
	name    string
	receive chan service.Message
	send    chan service.Message
}

func newExtensionChannels(name string) *extensionChannels {
	p := new(extensionChannels)
	p.name = name
	p.send = make(chan service.Message)
	p.receive = make(chan service.Message)
	return p
}

func initServer() (server, error) {
	var err error

	flag.StringVar(&configFile, "conf", "", "Interlook configuration file")
	flag.Parse()

	//  srv.extensionShutdown = make(chan bool)
	srv.coreShutdown = make(chan bool)
	srv.signals = make(chan os.Signal, 1)

	srv.config, err = config.ReadConfig(configFile)
	if err != nil {
		return srv, err
	}
	srv.extensions = make(map[string]Extension)
	srv.extensionChannels = make(map[string]*extensionChannels)
	srv.signals = make(chan os.Signal)
	srv.flowChan = make(chan service.Message)
	srv.flowControlTicker = time.NewTicker(srv.config.Core.CheckFlowInterval)

	// add configured extensions
	if srv.config.Provider.Docker != nil {
		srv.extensions[service.ProviderDocker] = srv.config.Provider.Docker
	}
	if srv.config.Provider.Swarm != nil {
		srv.extensions[service.ProviderSwarm] = srv.config.Provider.Swarm
	}
	if srv.config.Provider.Kubernetes != nil {
		srv.extensions[service.ProviderKubernetes] = srv.config.Provider.Kubernetes
	}
	if srv.config.IPAM.IPAlloc != nil {
		srv.extensions[service.IPAMFile] = srv.config.IPAM.IPAlloc
	}
	if srv.config.DNS.Consul != nil {
		srv.extensions[service.DNSConsul] = srv.config.DNS.Consul
	}
	if srv.config.LoadBalancer.KempLM != nil {
		srv.extensions[service.LBKempLM] = srv.config.LoadBalancer.KempLM
	}

	srv.workflow = initWorkflow()

	// init flowEntries table
	srv.flowEntries = newFlowEntries()
	if err := srv.flowEntries.loadFile(srv.config.Core.FlowEntriesFile); err != nil {
		logger.DefaultLogger().Errorf("Could not load entries from file: %v", err)

	}

	return srv, nil
}

// Start initialize server and run it
func Start() {
	logger.DefaultLogger().Printf("Starting Interlook core ", Version)
	core, err := initServer()
	if err != nil {
		logger.DefaultLogger().Fatal(err)
	}
	core.run()
}

func (s *server) run() {
	signal.Notify(s.signals, os.Interrupt)
	signal.Notify(s.signals, os.Kill)

	// run flowControl
	s.wg.Add(1)
	go s.runFlowControl()

	// start all configured extensions
	// for each one, starts a dedicated listener goroutine
	for name, extension := range srv.extensions {
		logger.DefaultLogger().Printf("adding %v to extensionChannels", name)
		extensionChannels := newExtensionChannels(name)
		s.extensionChannels[name] = extensionChannels

		// run the extension's listener
		go s.extensionListener(extensionChannels)

		curExtension := extension
		extensionChan := extensionChannels
		s.wg.Add(1)
		go func() {
			err := curExtension.Start(extensionChan.receive, extensionChan.send)
			if err != nil {
				logger.DefaultLogger().Fatalf("Cannot run extension %v: %v\n", extensionChan.name, err)
			}
			logger.DefaultLogger().Debugf("###Extension %v stopped", extensionChan.name)
			s.wg.Done()
		}()
	}

	// run http core
	go s.startHTTP()

	// handle SIGs and extensions clean extensionShutdown
	go func() {
		for range s.signals {
			for name, extension := range s.extensions {
				logger.DefaultLogger().Warnf("Stopping extension %v", name)
				extension.Stop()
			}
			logger.DefaultLogger().Debug("send true to coreShutdown")
			s.coreShutdown <- true

		}
	}()
	logger.DefaultLogger().Debug("waiting for wg")
	s.wg.Wait()
	logger.DefaultLogger().Debug("wg ok")
	//<-s.coreShutdown
}

// extensionListener gets messages from extensions and send them to workflow
// fill out message's Sender
func (s *server) extensionListener(extension *extensionChannels) {
	logger.DefaultLogger().Debugf("Listening for %v messages", extension.name)
	for {
		newMessage := <-extension.send
		newMessage.Sender = extension.name
		// inject message/service to workflow
		logger.DefaultLogger().Debugf("Received message from %v, sending to flow control\n", extension.name)
		if err := srv.flowEntries.mergeMessage(newMessage); err != nil {
			logger.DefaultLogger().Errorf("Error %v when inserting %v to flow\n", err, newMessage.Service.Name)
		}
	}
}

func (s *server) runFlowControl() {
	for {
		select {
		case <-s.coreShutdown:
			logger.DefaultLogger().Warn("Stopping FlowControl")
			if err := s.flowEntries.save(s.config.Core.FlowEntriesFile); err != nil {
				logger.DefaultLogger().Error(err.Error())
			}
			s.wg.Done()
			return

		case <-s.flowControlTicker.C:
			logger.DefaultLogger().Debug("Running flowControl")
			s.flowControl()
		}
	}
}

// flowControl compares provided service with existing one
// triggers required action(s) to bring service state
// to desired state (deployed or undeployed)
func (s *server) flowControl() {
	for k, v := range s.flowEntries.M {
		if v.State != v.ExpectedState && !v.WorkInProgress {
			var msg service.Message

			if v.Error != "" {
				logger.DefaultLogger().Warnf("Service %v is in error %v", v.Service.Name, v.Error)
				continue
			}

			logger.DefaultLogger().Debugf("flowControl: Service %v current state differs from target state", k)
			reverse := false
			if v.ExpectedState == flowUndeployedState {
				reverse = true
			}

			nextStep, err := s.workflow.getNextStep(v.State, reverse)
			if err != nil {
				logger.DefaultLogger().Errorf("Could not find next step for %v %v\n", k, err)
				continue
			}
			logger.DefaultLogger().Debugf("next step: %v", nextStep)

			if s.workflow.isLastStep(nextStep, reverse) {
				logger.DefaultLogger().Debugf("Closing flow entry %v (state: %v)\n", k, nextStep)
				s.flowEntries.closeEntry(k, reverse)
				continue
			}

			s.flowEntries.prepareForNextStep(k, nextStep, reverse)

			if reverse {
				msg.Action = service.MsgDeleteAction
			} else {
				msg.Action = service.MsgAddAction
			}
			msg.Service = v.Service
			// get the extension channel to write message to
			ext, ok := s.extensionChannels[nextStep]
			if !ok {
				logger.DefaultLogger().Errorf("flowControl could not find channel for ext %v\n", nextStep)
				continue
			}
			ext.receive <- msg
		}
	}
}
