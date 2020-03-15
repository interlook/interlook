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
	Cli           kubernetes.Interface
	pollTicker    *time.Ticker
	shutdown      chan bool
	send          chan<- comm.Message
	waitGroup     sync.WaitGroup
	listOptions   metav1.ListOptions
}

func (p *Extension) init() {

	p.shutdown = make(chan bool)

	if p.PollInterval == time.Duration(0) {
		p.PollInterval = 15 * time.Second
	}

	p.pollTicker = time.NewTicker(p.PollInterval)

	p.listOptions.LabelSelector = hostsLabel + "," + portLabel
	if len(p.LabelSelector) > 0 {
		p.listOptions.LabelSelector = p.listOptions.LabelSelector + "," + strings.Join(p.LabelSelector, ",")
	}
	log.Debugf("label selector: %v", p.listOptions.LabelSelector)
}

// Start the kubernetes provider
func (p *Extension) Start(receive <-chan comm.Message, send chan<- comm.Message) error {
	log.Infof("Starting %v on %v\n", p.Name, p.Endpoint)
	var err error
	p.send = send

	p.init()

	if p.Cli == nil {
		p.Cli, err = p.connect()
		if err != nil {
			return err
		}
	}

	_, err = p.Cli.Discovery().ServerVersion()
	if err != nil {
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

// Stop the kubernetes provider
func (p *Extension) Stop() error {
	log.Debug("Stopping Kubernetes provider")
	p.shutdown <- true
	p.waitGroup.Wait()

	return nil
}

func (p *Extension) listServices() (sl *v1.ServiceList, err error) {
	return p.Cli.CoreV1().Services("").List(p.listOptions)
}

func (p *Extension) poll() {

	sl, err := p.listServices()
	if err != nil {
		log.Error(err.Error())
		return
	}

	for _, svc := range sl.Items {
		if svc.Spec.Type == v1.ServiceTypeNodePort {
			msg, err := p.buildMessageFromService(&svc)
			if err != nil {
				log.Warnf("error building message for service %v %v", svc.Name+"@"+svc.Namespace, err.Error())
			}
			p.send <- msg
		}
	}
}

// RefreshService sends an updated state for a given service
func (p *Extension) RefreshService(msg comm.Message) {
	var (
		res comm.Message
		err error
	)

	if svc, ok := p.getServiceByName(msg.Service.Name, msg.Service.Namespace); ok {
		res, err = p.buildMessageFromService(svc)
		if err != nil {
			errMsg := fmt.Sprintf("Error building message for %v: %v", msg.Service.Name, err.Error())
			log.Errorf(errMsg)
			res.Error = errMsg
		}

	} else {
		log.Infof("k8s service %v not found, send delete", msg.Service.Name)
		res = msg
		res.Action = comm.DeleteAction
		//comm.BuildDeleteMessage(msg.Service.Name)
	}

	p.send <- res
}

func (p *Extension) connect() (kubernetes.Interface, error) {

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
			Namespace:  service.Namespace,
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

		pods, err := p.Cli.CoreV1().Pods("").List(metav1.ListOptions{LabelSelector: strings.Join(labelSelect, ",")})
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

func (p *Extension) getServiceByName(svcName, namespace string) (*v1.Service, bool) {

	svc, err := p.Cli.CoreV1().Services(namespace).Get(svcName, metav1.GetOptions{})
	if err != nil {
		return nil, false
	}

	return svc, true
}
