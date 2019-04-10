package config

// TODO: add core parameter checks and default values
import (
	"fmt"
	"github.com/bhuisgen/interlook/provisioner/dns/consul"
	"github.com/bhuisgen/interlook/provisioner/loadbalancer/f5ltm"
	"github.com/bhuisgen/interlook/provisioner/loadbalancer/kemplm"
	//"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"time"

	"github.com/bhuisgen/interlook/provider/docker"
	"github.com/bhuisgen/interlook/provider/kubernetes"
	"github.com/bhuisgen/interlook/provider/swarm"
	"github.com/bhuisgen/interlook/provisioner/ipam/ipalloc"
	"gopkg.in/yaml.v3"
)

// ServerConfiguration holds the configuration
// This includes all the providers and extensions
type ServerConfiguration struct {
	Core struct {
		LogLevel                     string        `yaml:"logLevel"`
		ListenPort                   int           `yaml:"listenPort"`
		LogFile                      string        `yaml:"logFile"`
		WorkflowSteps                string        `yaml:"workflowSteps"`
		WorkflowEntriesFile          string        `yaml:"workflowEntriesFile"`
		WorkflowControlInterval      time.Duration `yaml:"workflowControlInterval"`
		WorkflowHousekeepingInterval time.Duration `yaml:"workflowHousekeepingInterval"`
		ServiceWIPTimeout            time.Duration `yaml:"serviceWIPTimeout"`
		ServiceMaxLastUpdated        time.Duration `yaml:"serviceMaxLastUpdated"`
		CleanUndeployedServiceAfter  time.Duration `yaml:"cleanUndeployedServiceAfter"`
	} `yaml:"core"`
	Provider struct {
		Docker     *docker.Extension     `yaml:"docker"`
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
	//genReferenceConfigYAMLFile()
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

func genReferenceConfigYAMLFile() {

	cfg := ServerConfiguration{}
	cfg.Provider.Swarm = &swarm.Provider{}
	cfg.Provider.Kubernetes = &kubernetes.Extension{}
	cfg.Provider.Docker = &docker.Extension{}
	cfg.IPAM.IPAlloc = &ipalloc.IPAlloc{}
	cfg.DNS.Consul = &consul.Consul{}
	cfg.LB.KempLM = &kemplm.KempLM{}
	cfg.LB.F5LTM = &f5ltm.BigIP{}

	refFile, err := os.OpenFile("./interlook-ref-config.yml", os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		fmt.Printf("Error opening file ./interlook-ref-config.yml %v", err)
	}

	refFile.Truncate(0)
	refFile.Seek(0, 0)

	defer func() {
		if err := refFile.Close(); err != nil {
			fmt.Printf("Error closing filename %v", err)
		}
	}()

	data, _ := yaml.Marshal(cfg)
	_, _ = refFile.Write(data)

}
