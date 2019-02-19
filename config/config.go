package config

import (
	"github.com/BurntSushi/toml"
	"github.com/bhuisgen/interlook/provider/docker"
	"github.com/bhuisgen/interlook/provider/swarm"
)

type ServerConfiguration struct {
	Core   *CoreConfig                   `toml:"core,omitempty"`
	Docker *docker.ProviderConfiguration `toml:"docker,omitempty"`
	Swarm  *swarm.ProviderConfiguration  `toml:"swarm,omitempty"`
	//KubernetesProviderConfiguration *kubernetes.ProviderConfiguration `toml:"kubernetes,omitempty"`
	//ConsulDNSConfiguration         *consul.DNSConfiguration
	//LBConfiguration                 *LBConfiguration
}

type CoreConfig struct {
	ListenPort    string `toml:"listenPort"`
	DefaultDomain string `toml:"defaultDomain"`
	LogLevel      string `toml:"logLevel"`
	LogFile       string `toml:"logFile"`
}

func ReadConfig(file string) (*ServerConfiguration, error) {
	var cfg ServerConfiguration
	if _, err := toml.DecodeFile(file, &cfg); err != nil {
		return &cfg, err
	}
	return &cfg, nil
}
