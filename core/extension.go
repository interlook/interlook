package core

import (
	"github.com/bhuisgen/interlook/service"
)

type Extension interface {
	// Run starts the provider, providing a data channel to push message
	// and an event channel to forward SIG
	Start(receive <-chan service.Message, send chan<- service.Message) error
	Stop()
}
