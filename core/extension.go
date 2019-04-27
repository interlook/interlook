package core

import "github.com/interlook/interlook/messaging"

// Extension describe basic extension behaviour
type Extension interface {
	// Run starts the provider, providing a data channel to push message
	// and an event channel to forward SIG
	Start(receive <-chan messaging.Message, send chan<- messaging.Message) error
	Stop() error
}

type Provider interface {
	Extension
	RefreshService(msg messaging.Message)
}
