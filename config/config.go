package config

import (
	"io/ioutil"

	"github.com/bhuisgen/interlook/provider/docker"
	"github.com/bhuisgen/interlook/provider/kubernetes"
	"github.com/bhuisgen/interlook/provider/swarm"
	"github.com/bhuisgen/interlook/provisioner/ip/static"
	yaml "gopkg.in/yaml.v2"
)

// ServerConfiguration holds the configuration
// This includes all the providers and extensions
type ServerConfiguration struct {
	Core struct {
		LogLevel   string `yaml:"logLevel"`
		ListenPort int    `yaml:"listenPort,omitempty"`
		LogFile    string `yaml:"logFile"`
		Workflow   string `yaml:"workflow"`
	} `yaml:"core"`
	Provider struct {
		Docker     *docker.ProviderConfiguration `yaml:"docker,omitempty"`
		Swarm      *swarm.ProviderConfiguration  `yaml:"swarm,omitempty"`
		Kubernetes *kubernetes.Provider          `yaml:"kubernetes,omitempty"`
	} `yaml:"provider"`
	IP struct {
		File *static.IPProvider `yaml:"file,omitempty"`
	} `yaml:"ip,omitempty"`
	DNS struct {
	} `yaml:"dns,omitempty"`
	LoadBalancer struct {
	} `yaml:"loadbalancer,omitempty"`
}

// Workflow holds the workflow steps
type Workflow struct {
	Sequence map[int]string
}

// ReadConfig parse the configuration file
func ReadConfig(file string) (*ServerConfiguration, error) {
	var cfg ServerConfiguration
	f, err := ioutil.ReadFile(file)
	if err != nil {
		return &cfg, err
	}
	yaml.Unmarshal(f, &cfg)
	return &cfg, nil
}
