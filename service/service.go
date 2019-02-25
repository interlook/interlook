package service

import "reflect"

// Message holds config information with providers
type Message struct {
	Action string // add update remove
	// FIXME: who will handle update = remove and add?
	Service Service
}

// Service holds the containerized service
type Service struct {
	Provider    string   `json:"provider,omitempty"`
	ServiceName string   `json:"service_name,omitempty"`
	ID          string   `json:"internal_id,omitempty"`
	Hosts       []string `json:"hosts,omitempty"`
	Port        int      `json:"port,omitempty"`
	TLS         bool     `json:"tls,omitempty"`
	PublicIP    string   `json:"public_ip,omitempty"`
	DNSName     string   `json:"dns_name,omitempty"`
	Info        string   `json:"info,omitempty"`
}

// IsSameThan compares given service definition against existing one
// hosts list, dns names, port and tls
func (s *Service) IsSameThan(to Service) bool {
	if s.DNSName != to.DNSName || s.Port != to.Port || s.TLS != to.TLS || !reflect.DeepEqual(s.Hosts, to.Hosts) {
		return false
	}

	return true
}
