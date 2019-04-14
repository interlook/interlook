package docker

import (
	"github.com/bhuisgen/interlook/messaging"
	"time"

	"github.com/bhuisgen/interlook/log"
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
	close          chan bool
	receive        <-chan messaging.Message
	send           chan<- messaging.Message
}

func (p *Extension) RefreshService(serviceName string) {
	// check if service still exists and is up
	// if not send delete msg to p.send

}

// Start initialize and start sending events to core
func (p *Extension) Start(receive <-chan messaging.Message, send chan<- messaging.Message) error {
	p.close = make(chan bool)
	p.receive = receive
	p.send = send
	log.Infof("Starting %v on %v\n", p.Name, p.Endpoint)
	var msg messaging.Message
	msg.Action = "add" // add, remove, update, check
	msg.Service.Provider = "docker"
	msg.Service.Hosts = append(msg.Service.Hosts, "10.32.2.42", "10.32.2.46")
	msg.Service.Name = "mytest.app.csnet.me"
	msg.Service.DNSAliases = []string{"mytest.app.csnet.me", "mytest.csnet.me"}
	msg.Service.Port = 81
	msg.Service.TLS = false

	time.Sleep(2 * time.Second)
	send <- msg
	log.Debugf("##################### Add sent, will send update in 30secs")
	time.Sleep(30 * time.Second)
	msg.Service.Port = 81
	msg.Service.Hosts = make([]string, 0)
	msg.Service.Hosts = append(msg.Service.Hosts, "10.32.2.46", "10.32.2.45")
	send <- msg
	msg.Action = "delete"
	log.Debugf("##################### Will send delete in 30secs")
	time.Sleep(30 * time.Second)
	send <- msg

	for {
		select {
		case <-p.close:
			log.Debug("closed docker provider")
			return nil
		case msg := <-receive:
			log.Debugf("docker got msg", msg)
			continue
		}
	}
}

// Stop stops the provider
func (p *Extension) Stop() error {

	p.close <- true
	log.Debug("Stopping docker")
	return nil
}
