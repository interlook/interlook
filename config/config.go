package config

import (
	"io/ioutil"

	"github.com/bhuisgen/interlook/provider/docker"
	"github.com/bhuisgen/interlook/provider/swarm"
	yaml "gopkg.in/yaml.v2"
)

type ServerConfiguration struct {
	Core struct {
		LogLevel   string `yaml:"logLevel"`
		ListenPort int    `yaml:"listenPort"`
		LogFile    string `yaml:"logFile"`
		Workflow   string `yaml:"workflow"`
	} `yaml:"core"`
	Provider struct {
		Docker *docker.ProviderConfiguration `yaml:"docker,omitempty"`
		Swarm  *swarm.ProviderConfiguration  `yaml:"swarm,omitempty"`
		//KubernetesProviderConfiguration *kubernetes.ProviderConfiguration `yaml:"kubernetes,omitempty"`
	} `yaml:"provider"`
	//ConsulDNSConfiguration         *consul.DNSConfiguration
	//LBConfiguration                 *LBConfiguration
}

type Workflow struct {
	Sequence map[int]string
}

func ReadConfig(file string) (*ServerConfiguration, error) {
	var cfg ServerConfiguration
	// if _, err := toml.DecodeFile(file, &cfg); err != nil {
	// 	return &cfg, err
	// }
	f, err := ioutil.ReadFile(file)
	if err != nil {
		return &cfg, err
	}
	yaml.Unmarshal(f, &cfg)
	return &cfg, nil
}
