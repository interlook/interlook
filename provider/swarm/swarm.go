package swarm

import (
	"github.com/bhuisgen/interlook/messaging"
	"github.com/docker/docker/api/types/swarm"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bhuisgen/interlook/log"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"golang.org/x/net/context"
)

const (
	hostsLabel    = "interlook.hosts"
	portLabel     = "interlook.port"
	sslLabel      = "interlook.ssl"
	extensionName = "provider.swarm"
)

// Provider holds the provider configuration
type Provider struct {
	Name             string        `yaml:"name"`
	Endpoint         string        `yaml:"endpoint"`
	LabelSelector    []string      `yaml:"labelSelector"`
	TLSCa            string        `yaml:"tlsCa"`
	TLSCert          string        `yaml:"tlsCert"`
	TLSKey           string        `yaml:"tlsKey"`
	PollInterval     time.Duration `yaml:"pollInterval"`
	pollTicker       *time.Ticker
	shutdown         chan bool
	send             chan<- messaging.Message
	services         []string
	servicesLock     sync.RWMutex
	cli              *client.Client
	serviceFilters   filters.Args
	containerFilters filters.Args
	waitGroup        sync.WaitGroup
}

//type services map[string][]string
func (p *Provider) initDockerCli() error {

	var err error

	p.cli, err = client.NewClientWithOpts(client.WithTLSClientConfig(p.TLSCa, p.TLSCert, p.TLSKey),
		client.WithHost(p.Endpoint),
		client.WithVersion("1.29"),
		client.WithHTTPHeaders(map[string]string{"User-Agent": "interlook"}))

	if err != nil {
		return err
	}

	p.serviceFilters = filters.NewArgs()

	for _, value := range p.LabelSelector {
		p.serviceFilters.Add("label", value)
	}

	p.serviceFilters.Add("label", hostsLabel)
	p.serviceFilters.Add("label", portLabel)

	return nil
}

func (p *Provider) Start(receive <-chan messaging.Message, send chan<- messaging.Message) error {

	p.shutdown = make(chan bool)
	p.pollTicker = time.NewTicker(p.PollInterval)
	p.send = send

	if err := p.initDockerCli(); err != nil {
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
			case messaging.RefreshAction:
				log.Debugf("Request to refresh service %v", msg.Service.Name)
			default:
				log.Debugf("Unhandled action requested: %v", msg.Action)
			}
		}
	}
}

func (p *Provider) Stop() error {
	log.Info("Stopping Swarm provider")
	p.shutdown <- true
	p.waitGroup.Wait()

	return nil
}

// poll get the services to be deployed
// builds the interlook service definition
// list docker services with filters
// for each, get the containers
// finally send the info to the core
func (p *Provider) poll() {

	log.Debugf("looking for services with filters %v", p.serviceFilters)

	data, err := p.getFilteredServices()
	if err != nil {
		log.Errorf("Querying services %v", err.Error())
		return
	}

	for _, service := range data {

		if service.Endpoint.Spec.Mode != swarm.ResolutionModeVIP {
			log.Warnf("unsupported endpoint mode for service %v", service.Spec.Name)
			return
		}

		msg, err := p.buildMessageFromService(service)
		log.Debugf("swarm message %v", msg)
		if err != nil {
			log.Debugf("Error building message for service %v %v", service.Spec.Name, err.Error())
			return
		}

		if len(msg.Service.Hosts) == 0 {
			log.Debugf("No host found for service %v", service.Spec.Name)
			return
		}

		msg.Action = messaging.AddAction
		log.Debugf("%v sent msg %v", extensionName, msg)
		p.send <- msg
	}
}

func (p *Provider) refreshService(msg messaging.Message) {

	service := p.getServiceByName(msg.Service.Name)

	newMsg, err := p.buildMessageFromService(service)
	if err != nil {
		log.Errorf("Error building message for %v: %v", msg.Service.Name, err)
	}

	if newMsg.Service.Name == "" || len(newMsg.Service.Hosts) == 0 {
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

func (p *Provider) getServiceByName(svcName string) swarm.Service {

	ctx := context.Background()

	filter := filters.NewArgs()
	filter.Add("name", svcName)

	services, err := p.cli.ServiceList(ctx, types.ServiceListOptions{
		Filters: filter,
	})
	if err != nil {
		log.Errorf("Error getting service %v : %v", svcName, err)
		return swarm.Service{}
	}

	return services[0]
}

func (p *Provider) getContainersByService(svcName string) ([]types.Container, error) {

	ctx := context.Background()

	ctFilter := filters.NewArgs()
	ctFilter.Add("label", "com.docker.swarm.service.name="+svcName)

	ctList, err := p.cli.ContainerList(ctx, types.ContainerListOptions{
		Filters: ctFilter,
		All:     false})
	if err != nil {
		return ctList, err
	}

	return ctList, nil
}

func (p *Provider) buildMessageFromService(service swarm.Service) (messaging.Message, error) {

	ctx := context.Background()

	tlsService, _ := strconv.ParseBool(service.Spec.Labels[sslLabel])

	port, err := strconv.Atoi(service.Spec.Labels[portLabel])
	if err != nil {
		log.Error(err)
	}
	svcTargetPort := uint32(port)

	msg := messaging.Message{
		Service: messaging.Service{
			Name:       service.Spec.Name,
			Provider:   extensionName,
			DNSAliases: strings.Split(service.Spec.Labels[hostsLabel], ","),
			TLS:        tlsService,
		}}

	for _, svcPort := range service.Endpoint.Ports {

		if svcPort.TargetPort == svcTargetPort && svcPort.PublishedPort != 0 {
			msg.Service.Port = int(svcPort.PublishedPort)

			if svcPort.PublishMode == swarm.PortConfigPublishModeHost {
				log.Debugf("############ %v", svcPort)
				containers, err := p.getContainersByService(service.Spec.Name)
				if err != nil {
					return msg, err
				}
				for _, container := range containers {
					for _, portSpec := range container.Ports {
						if portSpec.PrivatePort == uint16(svcTargetPort) &&
							portSpec.IP != "" &&
							portSpec.Type == "tcp" &&
							portSpec.PublicPort == uint16(svcPort.PublishedPort) {
							log.Debugf("####### SWARM ######## adding %v for %v", portSpec.IP, service.Spec.Name)
							msg.Service.Hosts = append(msg.Service.Hosts, portSpec.IP)
						}
					}
				}
			}

			if svcPort.PublishMode == swarm.PortConfigPublishModeIngress {
				log.Debugf("############ %v", svcPort)
				containers, err := p.getContainersByService(service.Spec.Name)
				if err != nil {
					return msg, err
				}
				for _, container := range containers {
					containerDetails, err := p.cli.ContainerInspect(ctx, container.ID)
					if err != nil {
						log.Error(err)
						continue
					}
					log.Debugf("####### SWARM ######## adding %v for %v", "containerDetails", service.Spec.Name)
					msg.Service.Hosts = append(msg.Service.Hosts, containerDetails.Node.IPAddress)
				}
			}
		}
	}
	return msg, nil
}

func (p *Provider) buildDeleteMessage(svcName string) messaging.Message {
	msg := messaging.Message{
		Action: messaging.DeleteAction,
		Service: messaging.Service{
			Name: svcName,
		}}

	return msg
}
