package kubernetes

import (
	"time"

	"github.com/bhuisgen/interlook/log"
	"github.com/bhuisgen/interlook/service"
)

// Extension holds the provider ipalloc configuration
type Extension struct {
	Name           string   `yaml:"name"`
	Endpoint       string   `yaml:"endpoint"`
	LabelSelector  []string `yaml:"labelSelector"`
	TLSCa          string   `yaml:"tlsCa"`
	TLSCert        string   `yaml:"tlsCert"`
	TLSKey         string   `yaml:"tlsKey"`
	Watch          bool     `yaml:"watch"`
	WatchInterval  string   `yaml:"watchInterval"`
	UpdateInterval string   `yaml:"updateInterval"`
}

func (p *Extension) Start(receive <-chan service.Message, send chan<- service.Message) error {
	logger.DefaultLogger().Printf("Starting %v on %v\n", p.Name, p.Endpoint)
	var msg service.Message
	msg.Action = "add"
	msg.Service.Provider = "kubernetes"
	msg.Service.Hosts = append(msg.Service.Hosts, "192.168.1.1")
	for {
		time.Sleep(3 * time.Second)
		send <- msg
	}
	logger.DefaultLogger().Println("exiting")
	// do stuff
	//send <- msg
	return nil
}

func (p *Extension) Stop() error {
	logger.DefaultLogger().Printf("Stopping %v\n", p.Name)
	return nil
}
