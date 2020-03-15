# Developing an Interlook extension


## Interface

An Interlook extension must first implement the `Extension` interface:

```golang
type Extension interface {

	Start(receive <-chan service.Message, send chan<- service.Message) error
	
	Stop() error
}
```

The `Start` method will be used by the core to start the extension. The core will provide a receive and send channels for the exchanged messages.

The `Stop` method is used to shut the extension down. When invoked, it must make sure that `Start` method is stopped and return to the invoker.

## Configuration

The configuration is read at `interlook` startup. The config package must import the extension's configuration object.

`Interlook` configuration is a yaml formatted file and the Go object is config.ServerConfiguration.

Here is an example for an IPAM IPAlloc extension:

```golang
	IPAM struct {
		IPAlloc *ipalloc.Provisioner `yaml:"ipalloc,omitempty"`
	} `yaml:"ipam,omitempty"`
```

On startup, `interlook` will start all extensions that are configured in the core.workflow setup. If a configured extension fails to start, interlook will fail to start.

In order to avoid reflexion, the `core`'s `initExtensions` contains a map of extensions that needs to be enriched with new extension:

```golang
    knownExt := map[string]Extension{
        "provider.kubernetes": s.config.Provider.Kubernetes,
        "provider.swarm":      s.config.Provider.Swarm,
        "provisioner.consul":  s.config.DNS.Consul,
        "provisioner.ipalloc": s.config.IPAM.IPAlloc,
        "provisioner.f5ltm":   s.config.LB.F5LTM,
        "provisioner.kemplm":  s.config.LB.KempLM,
    }
```


## Messages

Two actions must be supported for incoming messages:

* add : when `core` sends such a message, it means the extension must create or update the existing service definition
* delete: service is being un-deployed, so the extension can delete current definition

Once processed, the message must be sent back to the core using the `send` channel. If applicable, the service definition can be modified by the extension.

In case of error, the extension must raise it through the Message.Error field.

