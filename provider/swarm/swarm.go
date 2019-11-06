package swarm

import (
	"fmt"
	"github.com/docker/docker/api/types/swarm"
	"github.com/interlook/interlook/comm"
	"github.com/pkg/errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/interlook/interlook/log"
	"golang.org/x/net/context"
)

const (
	hostsLabel    = "interlook.hosts"
	portLabel     = "interlook.port"
	sslLabel      = "interlook.ssl"
	extensionName = "provider.swarm"
	runningState  = "running"
)

// Provider holds the provider configuration
type Provider struct {
	Endpoint         string        `yaml:"endpoint"`
	LabelSelector    []string      `yaml:"labelSelector"`
	TLSCa            string        `yaml:"tlsCa"`
	TLSCert          string        `yaml:"tlsCert"`
	TLSKey           string        `yaml:"tlsKey"`
	PollInterval     time.Duration `yaml:"pollInterval"`
	pollTicker       *time.Ticker
	shutdown         chan bool
	send             chan<- comm.Message
	services         []string
	servicesLock     sync.RWMutex
	cli              *client.Client
	serviceFilters   filters.Args
	containerFilters filters.Args
	waitGroup        sync.WaitGroup
}

func (p *Provider) init() error {

	var err error

	p.shutdown = make(chan bool)
	p.pollTicker = time.NewTicker(p.PollInterval)

	if p.PollInterval == time.Duration(0) {
		p.PollInterval = 15 * time.Second
	}

	p.serviceFilters = filters.NewArgs()

	for _, value := range p.LabelSelector {
		p.serviceFilters.Add("label", value)
	}

	p.serviceFilters.Add("label", hostsLabel)
	p.serviceFilters.Add("label", portLabel)

	p.cli, err = client.NewClientWithOpts(client.WithTLSClientConfig(p.TLSCa, p.TLSCert, p.TLSKey),
		client.WithHost(p.Endpoint),
		// TODO: check which min docker engine api version we should support
		client.WithVersion("1.29"),
		client.WithHTTPHeaders(map[string]string{"User-Agent": "interlook"}))

	if err != nil {
		return err
	}

	return nil
}

func (p *Provider) Start(receive <-chan comm.Message, send chan<- comm.Message) error {

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

func (p *Provider) Stop() error {
	log.Debug("Stopping Swarm provider")
	p.shutdown <- true
	p.waitGroup.Wait()

	return nil
}

// poll get the services to be deployed
// list docker services with filters (interlook.hosts and interlook.port labels)
// for each, inspect the container(s) to get IPs and ports
// finally send the info to the core
func (p *Provider) poll() {

	log.Debugf("looking for services with filters %v", p.serviceFilters)

	data, err := p.getFilteredServices()
	if err != nil {
		log.Errorf("Querying services %v", err.Error())
		return
	}

	for _, service := range data {
		log.Debugf("Swarm service: %v", service)
		msg, err := p.buildMessageFromService(service)
		log.Debugf("swarm message %v", msg)
		if err != nil {
			log.Debugf("Error building message for service %v %v", service.Spec.Name, err.Error())
			continue
		}

		if len(msg.Service.Hosts) == 0 {
			log.Warnf("No host found for service %v", service.Spec.Name)
			continue
		}

		log.Debugf("%v sent msg %v", extensionName, msg)
		p.send <- msg
	}
}

func (p *Provider) RefreshService(msg comm.Message) {
	var (
		newMsg comm.Message
		err    error
	)

	if service, ok := p.getServiceByName(msg.Service.Name); ok {
		newMsg, err = p.buildMessageFromService(service)

		if err != nil {
			log.Errorf("Error building message for %v: %v", msg.Service.Name, err)
		}

		if newMsg.Service.Name == "" || len(newMsg.Service.Hosts) == 0 {
			log.Debugf("Swarm service %v not found, send delete", msg.Service.Name)
			newMsg = p.buildDeleteMessage(msg.Service.Name)
		}
	} else if !ok {
		newMsg = p.buildDeleteMessage(msg.Service.Name)
	}

	p.send <- newMsg

	return
}

func (p *Provider) getFilteredServices() (services []swarm.Service, err error) {
	ctx := context.Background()

	data, err := p.cli.ServiceList(ctx, types.ServiceListOptions{
		Filters: p.serviceFilters,
	})
	if err != nil {
		log.Errorf("Querying services %v", err.Error())
		return data, err
	}

	return data, nil
}

func (p *Provider) getServiceByName(svcName string) (swarm.Service, bool) {

	ctx := context.Background()

	p.serviceFilters.Add("name", svcName)
	services, err := p.cli.ServiceList(ctx, types.ServiceListOptions{
		Filters: p.serviceFilters,
	})
	p.serviceFilters.Del("name", svcName)

	if err != nil {
		log.Errorf("Error getting service %v : %v", svcName, err)
		return swarm.Service{}, false
	}

	if len(services) == 0 {
		return swarm.Service{}, false
	}
	return services[0], true
}

func (p *Provider) getNodesRunningService(svcName string) (nodeList []string, err error) {

	ctx := context.Background()

	var f types.TaskListOptions

	f.Filters = filters.NewArgs()
	f.Filters.Add("desired-state", runningState)
	f.Filters.Add("service", svcName)

	tasks, err := p.cli.TaskList(ctx, f)
	if err != nil {
		return nodeList, err
	}
	for _, task := range tasks {
		if task.Status.State == runningState {
			if !sliceContainString(task.NodeID, nodeList) {
				nodeList = append(nodeList, task.NodeID)
			}
		}
	}
	return nodeList, nil

}

func sliceContainString(s string, sl []string) bool {
	for _, x := range sl {
		if strings.ToUpper(x) == strings.ToUpper(s) {
			return true
		}
	}
	return false

}

func (p *Provider) getNodeIP(nodeID string) (IP string, err error) {

	ctx := context.Background()

	var f types.NodeListOptions

	f.Filters = filters.NewArgs()
	f.Filters.Add("id", nodeID)

	node, err := p.cli.NodeList(ctx, f)
	if err != nil {
		return "", err
	}

	if len(node) != 1 {
		return "", errors.New(fmt.Sprintf("Could not get node %v's IP", nodeID))
	}

	return node[0].Status.Addr, nil
}

func (p *Provider) buildMessageFromService(service swarm.Service) (comm.Message, error) {

	tlsService, _ := strconv.ParseBool(service.Spec.Labels[sslLabel])

	msg := comm.Message{
		Action: comm.AddAction,
		Service: comm.Service{
			Name:       service.Spec.Name,
			Provider:   extensionName,
			DNSAliases: strings.Split(service.Spec.Labels[hostsLabel], ","),
			TLS:        tlsService,
		}}

	targetPort, err := strconv.Atoi(service.Spec.Labels[portLabel])
	if err != nil {
		return msg, errors.New(fmt.Sprintf("Error converting %v to int (%v). Is %v correctly specified?", service.Spec.Labels[portLabel], err.Error(), portLabel))
	}

	// get ports published through service.Endpoint.Ports
	ports := service.Endpoint.Ports
	if ports == nil {
		return msg, errors.New("service has no published port")
	}

	for _, port := range ports {
		if int(port.TargetPort) == targetPort {
			log.Debugf("PublishedPort: %v through %v", port.PublishedPort, port.PublishMode)
			msg.Service.Port = int(port.PublishedPort)
		}
	}

	// get hosts running service container
	nodes, err := p.getNodesRunningService(service.Spec.Name)
	if err != nil {
		return msg, err
	}
	for _, node := range nodes {
		addr, err := p.getNodeIP(node)
		if err != nil {
			log.Warnf("Error getting node %v info %v", node, err.Error())
		}
		msg.Service.Hosts = append(msg.Service.Hosts, addr)
	}

	return msg, nil
}

func (p *Provider) buildDeleteMessage(svcName string) comm.Message {
	msg := comm.Message{
		Action: comm.DeleteAction,
		Service: comm.Service{
			Name: svcName,
		}}

	return msg
}
