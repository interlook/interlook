package core

import (
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/bhuisgen/interlook/config"
	"github.com/bhuisgen/interlook/service"
)

// FIXME: manager would handle the providers comm and events
// -> merge with Server?
type manager struct {
	config    *config.ServerConfiguration
	signals   chan os.Signal
	providers map[string]*activeProvider
}

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
	//core := new(manager)
	core.config, err = config.ReadConfig("./share/conf/config.toml")
	if err != nil {
		return core, err
	}
	core.providers = make(map[string]*activeProvider)
	core.signals = make(chan os.Signal)
	return core, nil
}

func Start() {
	core, err := makeManager()
	if err != nil {
		log.Fatal(err)
	}
	core.start()
}

func (m *manager) start() {
	// create channel for post exit cleanup
	signalChan := make(chan os.Signal, 1)
	stopExtensions := make(chan bool)
	signal.Notify(signalChan, os.Interrupt)
	signal.Notify(signalChan, os.Kill)

	if core.config.Docker.Watch {
		activeProvider := makeActiveProvider()
		log.Println("init")
		log.Println(core.config.Docker.Endpoint)
		core.providers[core.config.Docker.Name] = activeProvider
		go activeProvider.listen()
		go core.config.Docker.Run(activeProvider.dataChan, activeProvider.sigChan)
	}
	go func() {
		for sig := range signalChan {
			fmt.Println("Received interrupt, stopping extensions...")
			for _, v := range m.providers {
				v.sigChan <- sig
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

type Server struct {
	loadBalancer LoadBalancer
	provider     Provider
	resolver     Resolver
}

func (server *Server) Init() {
	// inject config
}

func (server *Server) Run() {
	//switch app.Configuration.Provider {
	//case "docker":
	//    app.startDocker()
	//case "swarm":
	//    app.startSwarm()
	//}
}

func (server *Server) Exit(sig os.Signal) {
}

//
//func (app *Application) startDocker() {
//    log.Println("[INFO]", "starting docker watcher")
//
//    app.dockerProvider = docker.Provider{
//        PollInterval:   15,
//        UpdateInterval: 30,
//        Filters: map[string][] string{
//            "label": {"lu.sgbt.docker.interlook"},
//        },
//    }
//
//    app.dockerProvider.Start()
//}
//
//func (app *Application) startSwarm() {
//    log.Println("[INFO]", "starting swarm watcher")
//
//    app.swarmProvider = swarm.Provider{
//        PollInterval:   15,
//        UpdateInterval: 30,
//        Filters: map[string][] string{
//            "label": {"lu.sgbt.docker.interlook"},
//        },
//    }
//
//    app.swarmProvider.Start()
//}
