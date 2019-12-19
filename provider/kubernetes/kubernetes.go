package kubernetes

import (
	"github.com/interlook/interlook/comm"
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

func (p *Extension) Start(receive <-chan comm.Message, send chan<- comm.Message) error {
	log.Infof("Starting %v on %v\n", p.Name, p.Endpoint)
	var msg comm.Message
	msg.Action = "add"
	msg.Service.Provider = "kubernetes"
	msg.Service.Targets = append(msg.Service.Targets, comm.Target{
		Host: "10.32.2.2",
		Port: 8080,
	})
	for {
		time.Sleep(3 * time.Second)
		send <- msg
	}

}

func (p *Extension) Stop() error {
	log.Infof("Stopping %v\n", p.Name)
	return nil
}
