package core

import (
	"github.com/bhuisgen/interlook/service"
)

type Provider interface {

	// not really needed?
	//Init() error

	// Push allows the provider to push events that will trigger DNS and LB configurations
	// through channel
	//Push(configurationChan chan<- service.Message) error

	// Run starts the provider, providing a data channel to push message
	// and an event channel to forward SIG
	Start(push chan service.Message) error

	// Start and stop could be replaced with Run having os signals channels?
	//Start()

	Stop()
}
