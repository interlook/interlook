// package consul allow create/update/delete of consul external service
package consul

import (
	"github.com/bhuisgen/interlook/log"
	"github.com/bhuisgen/interlook/service"
	"github.com/hashicorp/consul/api"
)

const (
	MsgAction = "extUpdate"
)

type Consul struct {
	URL      string `json:"url"`
	Token    string `json:"token,omitempty"`
	Domain   string `json:"domain,omitempty"`
	client   *api.Client
	shutdown chan bool
}

func (c *Consul) Start(receive <-chan service.Message, send chan<- service.Message) error {
	var err error
	var consulConfig api.Config
	consulConfig.Address = c.URL
	consulConfig.Token = c.Token
	c.client, err = api.NewClient(&consulConfig)
	if err != nil {
		return err
	}
	// TODO: add a simple connection to consul test
	c.shutdown = make(chan bool)
	for {
		select {
		case msg := <-receive:
			switch msg.Action {
			case "delete":
				logger.DefaultLogger().Debugf("request to delete dns for %v", msg.Service.Name)
				msg.Action = MsgAction
				if err := c.deregister(msg.Service.DNSName); err != nil {
					msg.Error = err.Error()
				}
				send <- msg

			default:
				msg.Action = MsgAction

				registration := api.CatalogRegistration{
					Node:     msg.Service.DNSName,
					Address:  msg.Service.PublicIP,
					NodeMeta: map[string]string{"external-node": "true"},
					Service: &api.AgentService{Service: msg.Service.DNSName,
						Address: msg.Service.DNSName,
						Port:    msg.Service.Port},
					Checks: api.HealthChecks{
						&api.HealthCheck{
							Status: "passing",
							Name:   "basic-tcp-check",
							Definition: api.HealthCheckDefinition{
								Interval: 10000,
							},
						},
					},
				}

				_, err = c.client.Catalog().Register(&registration, nil)
				if err != nil {
					msg.Error = err.Error()
					send <- msg
					continue
				}
				send <- msg

			}
		case <-c.shutdown:
			return nil

		}

	}
	return nil
}

func (c *Consul) Stop() {
	c.shutdown <- true
	logger.DefaultLogger().Warn("extension dns.consul down")
}

func (c *Consul) isServiceExist(name string) (bool, error) {
	consulServices, _, err := c.client.Catalog().Service(name, "", nil)
	if err != nil {
		return false, err
	}
	for _, s := range consulServices {
		if s.ServiceName == name {
			return true, nil
		}
	}
	return false, nil
}

func (c *Consul) deregister(node string) error {
	_, err := c.client.Catalog().Deregister(&api.CatalogDeregistration{Node: node}, nil)
	if err != nil {
		return err
	}
	return nil
}
