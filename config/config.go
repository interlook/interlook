package config

import (
	"io/ioutil"
	"time"

	"github.com/bhuisgen/interlook/provider/docker"
	"github.com/bhuisgen/interlook/provider/kubernetes"
	"github.com/bhuisgen/interlook/provider/swarm"
	"github.com/bhuisgen/interlook/provisioner/ipam/file"
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
		FlowEntriesFile   string        `yaml:"flowEntriesFile""`
	} `yaml:"core"`
	Provider struct {
		Docker     *docker.Extension     `yaml:"docker,omitempty"`
		Swarm      *swarm.Extension      `yaml:"swarm,omitempty"`
		Kubernetes *kubernetes.Extension `yaml:"kubernetes,omitempty"`
	} `yaml:"provider"`
	IPAM struct {
		File *file.Extension `yaml:"file,omitempty"`
	} `yaml:"ipam,omitempty"`
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
