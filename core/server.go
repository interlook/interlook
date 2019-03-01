package core

// TODO: proper init from "main"
// TODO: flags and or env var. Minimum -c configfile -l loglevel
import (
	"github.com/bhuisgen/interlook/config"
	"github.com/bhuisgen/interlook/log"
	"github.com/bhuisgen/interlook/service"
	//"runtime"
	"os"
	"os/signal"
)

// holds the srv config
// Keeps a list of configured and started providers
type server struct {
	config            *config.ServerConfiguration
	sigChannel        chan os.Signal
	extensions        map[string]Extension
	extensionChannels map[string]*extensionChannels
	workflow          workflow
	flowEntries       *flowEntries
	flowChan          chan service.Message
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

var srv server

func initServer() (server, error) {
	var err error
	srv.config, err = config.ReadConfig("./share/conf/config.yml")
	if err != nil {
		return srv, err
	}
	srv.extensions = make(map[string]Extension)
	srv.extensionChannels = make(map[string]*extensionChannels)
	srv.sigChannel = make(chan os.Signal)
	srv.flowChan = make(chan service.Message)

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
	if srv.config.IPAM.File != nil {
		srv.extensions[service.IpFileExtension] = srv.config.IPAM.File
	}

	srv.workflow = initWorkflow()

	// init flowEntries table
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
	// start http server
	go s.startHTTP()
	// start goroutine that will manage workflow entries
	go s.flowControl()

	// create channel for post exit cleanup
	stopExtensions := make(chan bool)
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)
	signal.Notify(signalChan, os.Kill)

	// start all configured extensions
	for name, extension := range srv.extensions {
		logger.DefaultLogger().Printf("adding %v to extensionChannels", name)
		extensionChannels := newExtensionChannels(name)
		s.extensionChannels[name] = extensionChannels

		// start the extension's listener
		go s.extensionListener(extensionChannels)

		curExtension := extension
		extensionChan := extensionChannels

		go func() {
			err := curExtension.Start(extensionChan.receive, extensionChan.send)
			if err != nil {
				logger.DefaultLogger().Errorf("Cannot start extension %v: %v\n", name, err)
			}
		}()
	}
	// handle SIGs and extensions clean shutdown
	go func() {
		for range signalChan {
			logger.DefaultLogger().Println("Received interrupt, stopping extensions...")
			for _, extension := range s.extensions {
				extension.Stop()
			}
			stopExtensions <- true
		}
	}()
	<-stopExtensions
}

// extensionListener gets messages from extensions and send them to workflow
// tag message's sender
func (s *server) extensionListener(extension *extensionChannels) {
	logger.DefaultLogger().Debugf("core listening on % %v", extension.send, extension.name)
	for {
		newMessage := <-extension.send
		newMessage.Sender = extension.name
		// inject message/service to workflow
		logger.DefaultLogger().Debugf("received message: %v, leaving it to flow control\n", newMessage.Action)
		if err := srv.flowEntries.mergeMessage(newMessage); err != nil {
			logger.DefaultLogger().Errorf("Error %v when inserting %v to flow\n", err, newMessage.Service.Name)
		}
	}
}
