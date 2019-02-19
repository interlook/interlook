package core

import (
	"os"

	"github.com/bhuisgen/interlook/config"
)

// FIXME: manager would handle the providers comm and events
// -> merge with Server?
type manager struct {
	config    config.CoreConfig
	signals   chan os.Signal
	providers map[string]*Provider
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
