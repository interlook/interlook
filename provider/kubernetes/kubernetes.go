package kubernetes

import (
	"github.com/docker/docker/api/types/filters"
	"github.com/interlook/interlook/comm"
	v1 "k8s.io/api/core/v1"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/interlook/interlook/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	hostsLabel    = "interlook.hosts"
	portLabel     = "interlook.port"
	sslLabel      = "interlook.ssl"
	extensionName = "provider.kubernetes"
	runningState  = "running"
)

// Extension holds the Kubernetes provider configuration
type Extension struct {
	Name             string        `yaml:"name"`
	Endpoint         string        `yaml:"endpoint"`
	LabelSelector    []string      `yaml:"listOptions"`
	KubeconfigFile   string        `yaml:"kubeconfigFile"`
	TLSCa            string        `yaml:"tlsCa"`
	TLSCert          string        `yaml:"tlsCert"`
	TLSKey           string        `yaml:"tlsKey"`
	PollInterval     time.Duration `yaml:"pollInterval"`
	pollTicker       *time.Ticker
	shutdown         chan bool
	send             chan<- comm.Message
	services         []string
	servicesLock     sync.RWMutex
	cli              *kubernetes.Clientset
	serviceFilters   filters.Args
	containerFilters filters.Args
	waitGroup        sync.WaitGroup
	listOptions      metav1.ListOptions
}

func (p *Extension) init() error {

	var err error

	p.shutdown = make(chan bool)
	p.pollTicker = time.NewTicker(p.PollInterval)

	if p.PollInterval == time.Duration(0) {
		p.PollInterval = 15 * time.Second
	}
	p.listOptions.LabelSelector = hostsLabel + "," + portLabel
	if len(p.LabelSelector) > 0 {
		p.listOptions.LabelSelector = p.listOptions.LabelSelector + "," + strings.Join(p.LabelSelector, ",")
	}
	log.Debugf("label selector: %v", p.listOptions.LabelSelector)
	p.cli, err = p.connect()
	if err != nil {
		return err
	}

	return nil

}

func (p *Extension) Start(receive <-chan comm.Message, send chan<- comm.Message) error {
	log.Infof("Starting %v on %v\n", p.Name, p.Endpoint)
	p.send = send

	if err := p.init(); err != nil {
		return err
	}

	p.waitGroup.Add(1)
	for {
		select {
		case <-p.shutdown:
			p.waitGroup.Done()

			return nil

		case <-p.pollTicker.C:
			log.Debug("New poll launched")
			p.poll()

		case msg := <-receive:
			log.Debugf("Received message from core: %v on %v", msg.Action, msg.Service.Name)
			switch msg.Action {
			case comm.RefreshAction:
				log.Debugf("Request to refresh service %v", msg.Service.Name)
				p.RefreshService(msg)
			default:
				log.Warnf("Unhandled action requested: %v", msg.Action)
			}
		}
	}

}

func (p *Extension) Stop() error {
	log.Debug("Stopping Swarm provider")
	p.shutdown <- true
	p.waitGroup.Wait()

	return nil
}

func (p *Extension) poll() {

	sl, err := p.cli.CoreV1().Services("").List(p.listOptions)
	if err != nil {
		log.Error(err.Error())
	}
	for _, svc := range sl.Items {
		if svc.Spec.Type == v1.ServiceTypeNodePort {
			msg, err := p.buildMessageFromService(svc)
			if err != nil {
				log.Warnf("error building message for service %v %v", svc.Name, err.Error())
			}

			p.send <- msg
		}
	}

}

func (p *Extension) RefreshService(msg comm.Message) {}

func (p *Extension) connect() (*kubernetes.Clientset, error) {

	config, err := clientcmd.BuildConfigFromFlags("", p.KubeconfigFile)
	if err != nil {
		panic(err.Error())
	}

	return kubernetes.NewForConfig(config)

}

func (p *Extension) buildMessageFromService(service v1.Service) (msg comm.Message, err error) {
	var targetPort int32
	tlsService, _ := strconv.ParseBool(service.Labels[sslLabel])

	msg = comm.Message{
		Action: comm.AddAction,
		Service: comm.Service{
			Name:       service.Name,
			Provider:   extensionName,
			DNSAliases: strings.Split(service.Labels[hostsLabel], ","),
			TLS:        tlsService,
		}}

	for _, port := range service.Spec.Ports {
		if port.TargetPort.StrVal == service.Labels[portLabel] {
			targetPort = port.NodePort
		}
	}

	var targets []comm.Target
	targets = append(targets, comm.Target{
		Host: "kwrk1.csnet.me",
		Port: uint32(targetPort),
	})
	msg.Service.Targets = targets

	return msg, nil
}
