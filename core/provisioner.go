package core

import "github.com/bhuisgen/interlook/service"

// Provisioner is the global interface describing behaviour of
// all extensions (IP provider, DNS and loafbalancer)
type Provisioner interface {
	ServiceExist(*service.Message) (bool, error)
	AddService(*service.Message) error
	RemoveService(*service.Message) error
}
