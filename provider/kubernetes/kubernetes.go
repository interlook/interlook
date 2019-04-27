package kubernetes

import (
	"github.com/interlook/interlook/messaging"
	"time"

	"github.com/interlook/interlook/log"
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

func (p *Extension) Start(receive <-chan messaging.Message, send chan<- messaging.Message) error {
	log.Infof("Starting %v on %v\n", p.Name, p.Endpoint)
	var msg messaging.Message
	msg.Action = "add"
	msg.Service.Provider = "kubernetes"
	msg.Service.Hosts = append(msg.Service.Hosts, "192.168.1.1")
	for {
		time.Sleep(3 * time.Second)
		send <- msg
	}

}

func (p *Extension) Stop() error {
	log.Infof("Stopping %v\n", p.Name)
	return nil
}
