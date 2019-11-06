package config

import (
	"github.com/interlook/interlook/provisioner/dns/consul"
	"github.com/interlook/interlook/provisioner/loadbalancer/f5ltm"
	"github.com/interlook/interlook/provisioner/loadbalancer/kemplm"
	"io/ioutil"
	"time"

	"github.com/interlook/interlook/provider/kubernetes"
	"github.com/interlook/interlook/provider/swarm"
	"github.com/interlook/interlook/provisioner/ipam/ipalloc"
	"gopkg.in/yaml.v3"
)

// ServerConfiguration holds the configuration
// This includes all the providers and extensions
type ServerConfiguration struct {
	Core struct {
		LogLevel                         string        `yaml:"logLevel"`
		ListenPort                       int           `yaml:"listenPort"`
		LogFile                          string        `yaml:"logFile"`
		WorkflowSteps                    string        `yaml:"workflowSteps"`
		WorkflowEntriesFile              string        `yaml:"workflowEntriesFile"`
		WorkflowActivityLauncherInterval time.Duration `yaml:"workflowActivityLauncherInterval"`
		WorkflowHousekeeperInterval      time.Duration `yaml:"workflowHousekeeperInterval"`
		ServiceWIPTimeout                time.Duration `yaml:"serviceWIPTimeout"`
		ServiceMaxLastUpdated            time.Duration `yaml:"serviceMaxLastUpdated"`
		CleanUndeployedServiceAfter      time.Duration `yaml:"cleanUndeployedServiceAfter"`
	} `yaml:"core"`
	Provider struct {
		Swarm      *swarm.Provider       `yaml:"swarm"`
		Kubernetes *kubernetes.Extension `yaml:"kubernetes"`
	} `yaml:"provider"`
	IPAM struct {
		IPAlloc *ipalloc.IPAlloc `yaml:"ipalloc"`
	} `yaml:"ipam"`
	DNS struct {
		Consul *consul.Consul `yaml:"consul"`
	} `yaml:"dns"`
	LB struct {
		KempLM *kemplm.KempLM `yaml:"kemplm"`
		F5LTM  *f5ltm.BigIP   `yaml:"f5ltm"`
	} `yaml:"lb"`
}

// ReadConfig parse the configuration
func ReadConfig(filename string) (*ServerConfiguration, error) {
	var cfg ServerConfiguration
	file, err := ioutil.ReadFile(filename)
	if err != nil {
		return &cfg, err
	}
	err = yaml.Unmarshal(file, &cfg)
	if err != nil {
		return &cfg, err
	}
	return &cfg, nil
}
