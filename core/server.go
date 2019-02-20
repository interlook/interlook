package core

import (
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/bhuisgen/interlook/config"
	"github.com/bhuisgen/interlook/service"
)

// holds the core config
// Keeps a list of configured and started providers
type manager struct {
	config              *config.ServerConfiguration
	signals             chan os.Signal
	configuredProviders []Provider
	activeProviders     map[string]*activeProvider
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
	core.config, err = config.ReadConfig("./share/conf/config.toml")
	if err != nil {
		return core, err
	}

	core.activeProviders = make(map[string]*activeProvider)
	core.signals = make(chan os.Signal)

	// get configured providers
	if core.config.Docker != nil {
		core.configuredProviders = append(core.configuredProviders, core.config.Docker)
	}
	if core.config.Swarm != nil {
		core.configuredProviders = append(core.configuredProviders, core.config.Swarm)
	}

	return core, nil
}

func Start() {
	Init()
}

// Init init and start the core server
func Init() {
	core, err := makeManager()
	if err != nil {
		log.Fatal(err)
	}
	core.start()
}

func (m *manager) start() {
	signalChan := make(chan os.Signal, 1)

	// create channel for post exit cleanup
	stopExtensions := make(chan bool)
	signal.Notify(signalChan, os.Interrupt)
	signal.Notify(signalChan, os.Kill)

	for _, prov := range core.configuredProviders {
		activeProvider := makeActiveProvider()
		core.activeProviders[core.config.Docker.Name] = activeProvider

		go activeProvider.listen()

		//go core.config.Docker.Run(activeProvider.dataChan, activeProvider.sigChan)
		go func() {
			err := prov.Run(activeProvider.dataChan, activeProvider.sigChan)
			if err != nil {
				fmt.Printf("Cannot start the provider %T: %v", prov, err)
			}
		}()
	}

	go func() {
		for sig := range signalChan {
			fmt.Println("Received interrupt, stopping extensions...")
			for _, prov := range m.activeProviders {
				prov.sigChan <- sig
			}
			stopExtensions <- true
		}
	}()
	<-stopExtensions
}

func (p *activeProvider) listen() {
	for {
		select {
		case newService := <-p.dataChan:
			fmt.Printf("received message: %v \n", newService.Action)
		case newSig := <-p.sigChan:
			fmt.Println("provider died?", newSig.String())
		}
	}
}
