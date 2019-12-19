package comm

import (
	"reflect"
)

const (
	//define message actions
	AddAction     = "add"
	UpdateAction  = "update"
	DeleteAction  = "delete"
	RefreshAction = "refresh"
)

// Message holds config information with providers
type Message struct {
	// add update or remove
	Action string
	// will be filled by core's extensionListener
	Sender      string
	Destination string
	Error       string
	Service     Service
}

// Target holds the ip and port of service backend
type Target struct {
	Host string `json:"host,omitempty"`
	Port uint32 `json:"port,omitempty"`
}

// BuildMessage returns a message built on service information
func BuildMessage(service Service, reverse bool) Message {
	var msg Message

	if reverse {
		msg.Action = DeleteAction
	} else {
		msg.Action = AddAction
	}

	msg.Service = service

	return msg
}

// Service holds the service definition
type Service struct {
	Provider   string   `json:"provider,omitempty"`
	Name       string   `json:"name,omitempty"`
	Targets    []Target `json:"targets,omitempty"`
	TLS        bool     `json:"tls,omitempty"`
	PublicIP   string   `json:"public_ip,omitempty"`
	DNSAliases []string `json:"dns_name,omitempty"`
	Info       string   `json:"info,omitempty"`
	Error      string   `json:"error,omitempty"`
}

// IsSameThan compares given service definition received from provider
// against current definition
// hosts list, port and tls
// returns a list of fields that differ
func (s *Service) IsSameThan(targetService Service) (bool, []string) {
	var diff []string

	if !reflect.DeepEqual(s.DNSAliases, targetService.DNSAliases) {
		diff = append(diff, "DNSNames")
	}

	if s.TLS != targetService.TLS {
		diff = append(diff, "TLS")
	}

	if !reflect.DeepEqual(s.Targets, targetService.Targets) {
		diff = append(diff, "Targets")
	}

	if len(diff) > 0 {
		return false, diff
	}
	return true, nil
}
