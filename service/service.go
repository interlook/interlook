package service

import "reflect"

const (
	//define message senders
	IPAMFile           = "ipam.file"
	ProviderDocker     = "provider.docker"
	ProviderSwarm      = "provider.swarm"
	ProviderKubernetes = "provider.kubernetes"
	DNSConsul          = "dns.consul"
	LBf5               = "lb.f5"
)

// Message holds config information with providers
type Message struct {
	// add update or remove
	Action string
	// will be filled by core's extensionListener
	Sender  string
	Error   string
	Service Service
}

// Service holds the containerized service
type Service struct {
	Provider string `json:"provider,omitempty"`
	Name     string `json:"name,omitempty"`
	//ID       string   `json:"internal_id,omitempty"`
	Hosts    []string `json:"hosts,omitempty"`
	Port     int      `json:"port,omitempty"`
	TLS      bool     `json:"tls,omitempty"`
	PublicIP string   `json:"public_ip,omitempty"`
	// TODO: support multiple dns names
	DNSName string `json:"dns_name,omitempty"`
	Info    string `json:"info,omitempty"`
	Error   string `json:"error,omitempty"`
}

// IsSameThan compares given service definition received from provider
// against current definition
// hosts list, port and tls
// returns a list of fields that differ
func (s *Service) IsSameThan(to Service) (bool, []string) {
	var diff []string
	if s.DNSName != to.DNSName {
		diff = append(diff, "DNSNames")
	}
	if s.Port != to.Port {
		diff = append(diff, "Port")
	}
	if s.TLS != to.TLS {
		diff = append(diff, "TLS")
	}
	if !reflect.DeepEqual(s.Hosts, to.Hosts) {
		diff = append(diff, "Hosts")
	}
	if len(diff) > 0 {
		return false, diff
	}
	return true, nil
}
