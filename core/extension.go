package core

import "github.com/interlook/interlook/comm"

// Extension describe extension basic behaviour
type Extension interface {
	// Run starts the extension, providing a data channel to push message
	// and an event channel to forward SIG
	Start(receive <-chan comm.Message, send chan<- comm.Message) error
	Stop() error
}

// Provider adds the RefreshService on top of the extension interface
// allowing the core to request a "refresh" of a given service definition/state
type Provider interface {
	Extension
	RefreshService(msg comm.Message)
}
