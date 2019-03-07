// package consul allow create/update/delete of consul external service
package consul

import (
	"github.com/bhuisgen/interlook/log"
	"github.com/bhuisgen/interlook/service"
	"github.com/hashicorp/consul/api"
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
	var cliOK bool
	consulConfig.Address = c.URL
	consulConfig.Token = c.Token
	c.client, err = api.NewClient(&consulConfig)
	if err != nil {
		return err
	}
	// Check we can connect to consul
	cliOK, err = c.isServiceExist("consul")
	if !cliOK || err != nil {
		return err
	}

	c.shutdown = make(chan bool)
	for {
		select {
		case msg := <-receive:
			switch msg.Action {
			case service.MsgDeleteAction:
				logger.DefaultLogger().Debugf("request to delete dns for %v", msg.Service.Name)
				msg.Action = service.MsgUpdateFromExtension
				for _, dnsAlias := range msg.Service.DNSAliases {
					if err := c.deregister(dnsAlias); err != nil {
						msg.Error = err.Error()
					}
					send <- msg
				}

			default:
				msg.Action = service.MsgUpdateFromExtension
				var servicePort int
				if msg.Service.TLS {
					servicePort = 443
				} else {
					servicePort = 80
				}
				for _, dnsAlias := range msg.Service.DNSAliases {
					alreadyRegistered, err := c.isServiceExist(dnsAlias)
					if err != nil {
						logger.DefaultLogger().Errorf("error %v getting current dns definition for %v", err.Error(), dnsAlias)
					}
					if alreadyRegistered {
						if err := c.deregister(dnsAlias); err != nil {
							logger.DefaultLogger().Errorf("Error deregistering %v: %v", dnsAlias, err.Error())
						}
					}
					registration := api.CatalogRegistration{
						Node:     dnsAlias,
						Address:  msg.Service.PublicIP,
						NodeMeta: map[string]string{"external-node": "true"},
						Service: &api.AgentService{Service: dnsAlias,
							Address: dnsAlias,
							Port:    servicePort},
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

			}
		case <-c.shutdown:
			return nil
		}
	}
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
