package core

import (
	//"runtime"
	"os"
	"os/signal"
	"strings"

	"github.com/bhuisgen/interlook/config"
	"github.com/bhuisgen/interlook/log"
	"github.com/bhuisgen/interlook/service"
)

// holds the srv config
// Keeps a list of configured and started providers
type server struct {
	config              *config.ServerConfiguration
	signals             chan os.Signal
	configuredProviders []Provider
	providerChannels    map[string]*providerChannels
	workflow            workflow
	flowEntries         *flowEntries
	flowChan            chan service.Message
}

// providerChannels holds the "activated" provider's channels
type providerChannels struct {
	dataChan chan service.Message
	sigChan  chan os.Signal
}

func NewActiveProvider() *providerChannels {
	p := new(providerChannels)
	p.dataChan = make(chan service.Message)
	p.sigChan = make(chan os.Signal)
	return p
}

var srv server

func initServer() (server, error) {
	var err error
	srv.config, err = config.ReadConfig("./share/conf/config.yml")
	if err != nil {
		return srv, err
	}

	srv.providerChannels = make(map[string]*providerChannels)
	srv.signals = make(chan os.Signal)
	srv.flowChan = make(chan service.Message)

	// get configured providers
	if srv.config.Provider.Docker != nil {
		srv.configuredProviders = append(srv.configuredProviders, srv.config.Provider.Docker)
	}
	if srv.config.Provider.Swarm != nil {
		srv.configuredProviders = append(srv.configuredProviders, srv.config.Provider.Swarm)
	}
	if srv.config.Provider.Kubernetes != nil {
		srv.configuredProviders = append(srv.configuredProviders, srv.config.Provider.Swarm)
	}

	//init workflow
	srv.workflow = make(map[int]string)
	for k, v := range strings.Split(srv.config.Core.Workflow, ",") {
		srv.workflow[k+1] = v
	}
	srv.flowEntries = newFlowEntries()

	return srv, nil
}

func Start() {
	Init()
}

// Init init and start the srv server
func Init() {
	logger.DefaultLogger().Printf("Starting server")
	core, err := initServer()
	if err != nil {
		logger.DefaultLogger().Fatal(err)
	}
	core.start()
}

func (s *server) start() {
	go s.startHTTP()
	signalChan := make(chan os.Signal, 1)

	// create channel for post exit cleanup
	stopExtensions := make(chan bool)
	signal.Notify(signalChan, os.Interrupt)
	signal.Notify(signalChan, os.Kill)

	// start all configured providers
	for _, prov := range srv.configuredProviders {
		logger.DefaultLogger().Printf("adding %v to active prov", prov)
		activeProvider := NewActiveProvider()

		//srv.activeProviders[prov.Name] = providerChannels

		go s.listenProvider(activeProvider)
		curProv := prov
		provChan := activeProvider.dataChan

		go func() {
			logger.DefaultLogger().Debugf("About to start provider %v\n", curProv)
			err := curProv.Start(provChan)
			if err != nil {
				logger.DefaultLogger().Errorf("Cannot start the provider %T: %v\n", curProv, err)
			}
		}()
	}
	// handle SIGs and extensions clean shutdown
	go func() {
		for _ = range signalChan {
			logger.DefaultLogger().Println("Received interrupt, stopping extensions...")
			for _, prov := range s.configuredProviders {
				prov.Stop()
			}
			stopExtensions <- true
		}
	}()
	<-stopExtensions
}

func (s *server) listenProvider(p *providerChannels) {
	for {
		select {
		case newMessage := <-p.dataChan:
			// inject message/service to flow control
			logger.DefaultLogger().Debugf("received message: %v, leaving it to flow control\n", newMessage.Action)
			srv.flowEntries.insertToFlow(newMessage)
		case newSig := <-p.sigChan:
			logger.DefaultLogger().Warnf("provider died?", newSig.String())
		}
	}
}

func (s *server) flowControl() error {

	return nil
}
