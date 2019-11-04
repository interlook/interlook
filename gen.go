// +build ignore

package main

import (
	"fmt"
	"github.com/interlook/interlook/config"
	"github.com/interlook/interlook/log"
	"github.com/interlook/interlook/provider/kubernetes"
	"github.com/interlook/interlook/provider/swarm"
	"github.com/interlook/interlook/provisioner/dns/consul"
	"github.com/interlook/interlook/provisioner/ipam/ipalloc"
	"github.com/interlook/interlook/provisioner/loadbalancer/f5ltm"
	"github.com/interlook/interlook/provisioner/loadbalancer/kemplm"
	"gopkg.in/yaml.v3"
	"os"
)

const (
	refConfigFile = "./docs/interlook.yml"
)

func main() {

	cfg := config.ServerConfiguration{}
	cfg.Provider.Swarm = &swarm.Provider{}
	cfg.Provider.Kubernetes = &kubernetes.Extension{}
	cfg.IPAM.IPAlloc = &ipalloc.IPAlloc{}
	cfg.DNS.Consul = &consul.Consul{}
	cfg.LB.KempLM = &kemplm.KempLM{}
	cfg.LB.F5LTM = &f5ltm.BigIP{}

	refFile, err := os.OpenFile(refConfigFile, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		fmt.Printf("Error opening file %v %v", refConfigFile, err)
	}

	if err := refFile.Truncate(0); err != nil {
		log.Error(err)
	}
	_, err = refFile.Seek(0, 0)
	if err != nil {
		log.Error(err)
	}

	defer func() {
		if err := refFile.Close(); err != nil {
			fmt.Printf("Error closing filename %v", err)
		}
	}()

	data, _ := yaml.Marshal(cfg)
	_, _ = refFile.Write(data)

}
