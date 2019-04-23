// package consul allow create/update/delete of consul external service
package consul

import (
	"github.com/bhuisgen/interlook/log"
	"github.com/bhuisgen/interlook/messaging"
	"github.com/hashicorp/consul/api"
)

type Consul struct {
	URL      string `json:"url"`
	Token    string `json:"token,omitempty"`
	Domain   string `json:"domain,omitempty"`
	client   *api.Client
	shutdown chan bool
}

func (c *Consul) init() error {
	var err error
	var consulConfig api.Config
	var cliOK bool
	c.shutdown = make(chan bool)
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
	return nil
}

func (c *Consul) Start(receive <-chan messaging.Message, send chan<- messaging.Message) error {

	if err := c.init(); err != nil {
		return err
	}

	for {
		select {
		case msg := <-receive:
			log.Debugf("dns.consul got this message %v", msg)
			switch msg.Action {
			case messaging.DeleteAction:
				log.Debugf("request to delete dns for %v", msg.Service.Name)
				msg.Action = messaging.UpdateAction
				for _, dnsAlias := range msg.Service.DNSAliases {
					if err := c.deregister(dnsAlias); err != nil {
						msg.Error = err.Error()
					}
					send <- msg
				}

			default:
				msg.Action = messaging.UpdateAction
				var servicePort int
				// FIXME: service public ports should be those used by the LB extension
				if msg.Service.TLS {
					servicePort = 443
				} else {
					servicePort = 80
				}
				for _, dnsAlias := range msg.Service.DNSAliases {
					alreadyRegistered, err := c.isServiceExist(dnsAlias)

					if err != nil {
						log.Errorf("error %v getting current dns definition for %v", err.Error(), dnsAlias)
					}

					if alreadyRegistered {
						if err := c.deregister(dnsAlias); err != nil {
							log.Errorf("Error de-registering %v: %v", dnsAlias, err.Error())
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
				}
				send <- msg
			}
		case <-c.shutdown:
			return nil
		}
	}
}

func (c *Consul) Stop() error {
	c.shutdown <- true
	log.Info("extension dns.consul down")
	return nil
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
