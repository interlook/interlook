package config

import (
	"github.com/bhuisgen/interlook/provisioner/dns/consul"
	"github.com/bhuisgen/interlook/provisioner/loadbalancer/kemplm"
	"io/ioutil"
	"time"

	"github.com/bhuisgen/interlook/provider/docker"
	"github.com/bhuisgen/interlook/provider/kubernetes"
	"github.com/bhuisgen/interlook/provider/swarm"
	"github.com/bhuisgen/interlook/provisioner/ipam/ipalloc"
	"gopkg.in/yaml.v2"
)

// ServerConfiguration holds the configuration
// This includes all the providers and extensions
type ServerConfiguration struct {
	Core struct {
		LogLevel          string        `yaml:"logLevel"`
		ListenPort        int           `yaml:"listenPort,omitempty"`
		LogFile           string        `yaml:"logFile"`
		Workflow          string        `yaml:"workflow"`
		CheckFlowInterval time.Duration `yaml:"checkFlowInterval"`
		FlowEntriesFile   string        `yaml:"flowEntriesFile"`
	} `yaml:"core"`
	Provider struct {
		Docker     *docker.Extension     `yaml:"docker,omitempty"`
		Swarm      *swarm.Extension      `yaml:"swarm,omitempty"`
		Kubernetes *kubernetes.Extension `yaml:"kubernetes,omitempty"`
	} `yaml:"provider"`
	IPAM struct {
		IPAlloc *ipalloc.Extension `yaml:"ipalloc,omitempty"`
	} `yaml:"ipam,omitempty"`
	DNS struct {
		Consul *consul.Consul `yaml:"consul,omitempty"`
	} `yaml:"dns,omitempty"`
	LoadBalancer struct {
		KempLM *kemplm.KempLM `yaml:"kemplm,omitempty"`
	} `yaml:"loadbalancer,omitempty"`
}

// ReadConfig parse the configuration
func ReadConfig(file string) (*ServerConfiguration, error) {
	var cfg ServerConfiguration
	f, err := ioutil.ReadFile(file)
	if err != nil {
		return &cfg, err
	}
	yaml.Unmarshal(f, &cfg)
	return &cfg, nil
}
