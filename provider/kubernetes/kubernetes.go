package kubernetes

import (
	"fmt"
	"github.com/interlook/interlook/comm"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"

	"k8s.io/client-go/rest"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/interlook/interlook/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	hostsLabel    = "interlook.hosts"
	portLabel     = "interlook.port"
	sslLabel      = "interlook.ssl"
	extensionName = "provider.kubernetes"
)

// Extension holds the Kubernetes provider configuration
type Extension struct {
	Name          string        `yaml:"name"`
	Endpoint      string        `yaml:"endpoint"`
	LabelSelector []string      `yaml:"listOptions"`
	TLSCa         string        `yaml:"tlsCa"`
	TLSCert       string        `yaml:"tlsCert"`
	TLSKey        string        `yaml:"tlsKey"`
	PollInterval  time.Duration `yaml:"pollInterval"`
	pollTicker    *time.Ticker
	shutdown      chan bool
	send          chan<- comm.Message
	cli           kubernetes.Interface
	waitGroup     sync.WaitGroup
	listOptions   metav1.ListOptions
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
		return
	}

	for _, svc := range sl.Items {
		if svc.Spec.Type == v1.ServiceTypeNodePort {
			msg, err := p.buildMessageFromService(&svc)
			if err != nil {
				log.Warnf("error building message for service %v %v", svc.Name, err.Error())
			}
			p.send <- msg
		}
	}
}

func (p *Extension) RefreshService(msg comm.Message) {
	var (
		res comm.Message
		err error
	)
	if svc, ok := p.getServiceByName(msg.Service.Name); !ok {
		res, err = p.buildMessageFromService(svc)
		if err != nil {
			log.Errorf("Error building delete message for %v: %v", msg.Service.Name, err.Error())
			return
		}
		if res.Service.Name == "" || len(res.Service.Targets) == 0 {
			log.Debugf("k8s service %v not found, send delete", msg.Service.Name)
			res = p.buildDeleteMessage(msg.Service.Name)
		}
	} else {
		res = p.buildDeleteMessage(msg.Service.Name)
	}

	p.send <- res
}

func (p *Extension) connect() (*kubernetes.Clientset, error) {

	config := rest.Config{
		Host: p.Endpoint,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: false,
			CertFile: p.TLSCert,
			KeyFile:  p.TLSKey,
			CAFile:   p.TLSCa,
		},
		UserAgent: "interlook",
	}
	return kubernetes.NewForConfig(&config)
}

func (p *Extension) buildMessageFromService(service *v1.Service) (msg comm.Message, err error) {
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
		if strconv.Itoa(int(port.Port)) == service.Labels[portLabel] {
			targetPort = port.NodePort
		}
	}
	if len(service.Spec.Selector) > 0 {
		var labelSelect []string
		for k, v := range service.Spec.Selector {
			labelSelect = append(labelSelect, fmt.Sprintf("%v=%v", k, v))
		}

		pods, err := p.cli.CoreV1().Pods("").List(metav1.ListOptions{LabelSelector: strings.Join(labelSelect, ",")})
		if err != nil {
			errMsg := fmt.Sprintf("error getting pods: %v", err.Error())
			log.Error(errMsg)
			return msg, errors.New(errMsg)
		}

		if len(pods.Items) == 0 {
			return msg, errors.New("no pod found for service " + service.Name)
		}

		var targets []comm.Target
		for _, pod := range pods.Items {
			targets = append(targets, comm.Target{
				Host: pod.Status.HostIP,
				Port: uint32(targetPort),
			})
			msg.Service.Targets = targets
		}
	}

	return msg, nil
}

func (p *Extension) buildDeleteMessage(svcName string) comm.Message {
	msg := comm.Message{
		Action: comm.DeleteAction,
		Service: comm.Service{
			Name: svcName,
		}}

	return msg
}

func (p *Extension) getServiceByName(svcName string) (*v1.Service, bool) {

	svc, err := p.cli.CoreV1().Services("").Get(svcName, metav1.GetOptions{})
	if err != nil {
		return nil, false
	}
	if svc.Name == "" {
		return svc, false
	}
	return svc, true
}
