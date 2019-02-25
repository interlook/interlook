package core

import (
	"os"
	"os/signal"
	"strings"

	"github.com/bhuisgen/interlook/config"
	"github.com/bhuisgen/interlook/log"
	"github.com/bhuisgen/interlook/service"
)

// holds the core config
// Keeps a list of configured and started providers
type manager struct {
	config              *config.ServerConfiguration
	signals             chan os.Signal
	configuredProviders []Provider
	activeProviders     map[string]*activeProvider
	workflow            map[int]string
}

// activeProvider holds the "activated" provider's channels
type activeProvider struct {
	dataChan chan service.Message
	sigChan  chan os.Signal
}

func makeActiveProvider() *activeProvider {
	p := new(activeProvider)
	p.dataChan = make(chan service.Message)
	p.sigChan = make(chan os.Signal)
	return p
}

var core manager

func makeManager() (manager, error) {
	var err error
	core.config, err = config.ReadConfig("./share/conf/config.yml")
	if err != nil {
		return core, err
	}

	core.activeProviders = make(map[string]*activeProvider)
	core.signals = make(chan os.Signal)
	logger.DefaultLogger().Println(core.config.Provider.Docker)
	logger.DefaultLogger().Println(core.config.Provider.Swarm)
	// get configured providers
	if core.config.Provider.Docker != nil {
		core.configuredProviders = append(core.configuredProviders, core.config.Provider.Docker)
	}
	if core.config.Provider.Swarm != nil {
		core.configuredProviders = append(core.configuredProviders, core.config.Provider.Swarm)
	}

	tmp := strings.Split(core.config.Core.Workflow, ",")
	core.workflow = make(map[int]string)
	for k, v := range tmp {
		core.workflow[k+1] = v
	}
	logger.DefaultLogger().Print(core.workflow)
	return core, nil
}

func Start() {
	Init()
}

// Init init and start the core server
func Init() {
	logger.DefaultLogger().Printf("Starting server")
	core, err := makeManager()
	if err != nil {
		logger.DefaultLogger().Fatal(err)
	}
	core.start()
}

func (m *manager) start() {
	signalChan := make(chan os.Signal, 1)

	// create channel for post exit cleanup
	stopExtensions := make(chan bool)
	signal.Notify(signalChan, os.Interrupt)
	signal.Notify(signalChan, os.Kill)

	// start all configured providers
	for _, prov := range core.configuredProviders {
		logger.DefaultLogger().Printf("adding %v to active prov", prov)
		activeProvider := makeActiveProvider()

		//core.activeProviders[prov.Name] = activeProvider

		go m.handle(activeProvider)
		curProv := prov
		provChan := activeProvider.dataChan

		go func() {
			logger.DefaultLogger().Printf("About to start provider %v\n", curProv)
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
			for _, prov := range m.configuredProviders {
				prov.Stop()
			}
			stopExtensions <- true
		}
	}()
	<-stopExtensions
}

func (m *manager) handle(p *activeProvider) {
	logger.DefaultLogger().Println("starting manager")
	for {
		select {
		case newService := <-p.dataChan:
			newService.Service.CurrentStep = 1
			logger.DefaultLogger().Printf("received message: %v, routing to next step %v %v \n", newService.Action)
		case newSig := <-p.sigChan:
			logger.DefaultLogger().Printf("provider died?", newSig.String())
		}
	}
}
