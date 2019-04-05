package swarm

import (
	"sync"
	"time"

	"github.com/bhuisgen/interlook/log"
	"github.com/bhuisgen/interlook/service"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"golang.org/x/net/context"
)

// Provider holds the provider configuration
type Provider struct {
	Name           string        `yaml:"name"`
	Endpoint       string        `yaml:"endpoint"`
	LabelSelector  []string      `yaml:"labelSelector"`
	TLSCa          string        `yaml:"tlsCa"`
	TLSCert        string        `yaml:"tlsCert"`
	TLSKey         string        `yaml:"tlsKey"`
	WatchInterval  time.Duration `yaml:"watchInterval"`
	UpdateInterval time.Duration `yaml:"updateInterval"`
	pollTicker     *time.Ticker
	updateTicker   *time.Ticker
	shutdown       chan bool
	waitGroup      sync.WaitGroup
	send           chan<- service.Message
	Filters        map[string][]string
	services       []string
	servicesLock   sync.RWMutex
	cli            *client.Client
}

//type services map[string][]string
func (p *Provider) initDockerCli() error {

	p.cli = client.NewClientWithOpts(client.WithHTTPHeaders())
	return nil
}

func (p *Provider) Start(receive <-chan service.Message, send chan<- service.Message) error {

	p.shutdown = make(chan bool)
	p.pollTicker = time.NewTicker(p.WatchInterval)
	p.updateTicker = time.NewTicker(p.UpdateInterval)
	p.send = send
	p.waitGroup.Add(1)

	if err := p.initDockerCli(); err != nil {
		return err
	}

	for {
		select {
		case <-p.shutdown:
			p.waitGroup.Done()

			return nil

		case <-p.pollTicker.C:
			log.Info("[DEBUG]", "new poll task")
			p.poll()

		case <-p.updateTicker.C:
			log.Info("[DEBUG]", "new update task")
			p.update()
		}
	}
}

func (p *Provider) Stop() error {
	log.Info("Stopping Swarm provider")
	p.shutdown <- true
	p.waitGroup.Wait()
	return nil
}

// FIXME: will Init function initialize the Provider (from Provider)?
type Provider0 struct {
	WatchInterval  int
	UpdateInterval int

	shutdown     chan struct{}
	waitGroup    sync.WaitGroup
	pollTicker   *time.Ticker
	updateTicker *time.Ticker
	services     []string
	servicesLock sync.RWMutex
}

func (p *Provider) poll() {
	ctx := context.Background()

	cli, err := client.NewEnvClient()
	if err != nil {
		log.Info("[ERROR]", err)

		return
	}

	servicesFilters := filters.NewArgs()
	servicesFilters.Add("label", "interlook.host")
	servicesFilters.Add("label", "interlook.port")

	for name, values := range p.Filters {
		for _, value := range values {
			servicesFilters.Add(name, value)
		}
	}

	data, err := cli.ServiceList(ctx, types.ServiceListOptions{
		Filter: servicesFilters,
	})
	if err != nil {
		log.Info("[ERROR]", "failed to list services", err)

		return
	}

	var services []string

	for _, service := range data {
		services = append(services, service.ID)
	}

	p.servicesLock.Lock()
	defer p.servicesLock.Unlock()

	p.services = services
}

func (p *Provider) update() {
	ctx := context.Background()

	cli, err := client.NewEnvClient()
	if err != nil {
		log.Info("[ERROR]", err)

		return
	}

	p.servicesLock.RLock()

	for _, id := range p.services {
		service, _, err := cli.ServiceInspectWithRaw(ctx, id)
		if err != nil {
			log.Info("[ERROR]", err)

			return
		}

		tasksFilters := filters.NewArgs()
		tasksFilters.Add("service", id)

		tasks, err := cli.TaskList(ctx, types.TaskListOptions{
			Filter: tasksFilters,
		})
		if err != nil {
			log.Info("[ERROR]", err)

			return
		}

		for _, task := range tasks {
			id := task.Status.ContainerStatus.ContainerID

			inspect, err := cli.ContainerInspect(ctx, id)
			if err != nil {
				log.Info("[ERROR]", err)

				return
			}

			node, data, err := cli.NodeInspectWithRaw(ctx, task.NodeID)
			if err != nil {
				log.Info("[ERROR]", err)

				return
			}
			ip := "1.2.3.4" // FIXME: no addr field ?
			log.Info(node.ID)
			log.Info(data)

			var port string

			if inspect.HostConfig.NetworkMode.IsHost() {
				port = inspect.Config.Labels["interlook.port"]
			} else {
				exposed := false

				for portMapping := range inspect.NetworkSettings.Ports {
					if portMapping.Port() == port {
						for _, target := range inspect.NetworkSettings.Ports[portMapping] {
							if target.HostIP == "0.0.0.0" {
								port = target.HostPort

								exposed = true
							}
						}
					}
				}

				if exposed == false {
					continue
				}
			}

			if inspect.State.Running && inspect.State.Health.Status != "healthy" {
				p.addTarget(service.Spec.Labels["interlook.host"], ip, port, id)
			} else {
				p.removeTarget(service.Spec.Labels["interlook.host"], ip, port, id)
			}
		}
	}

	p.servicesLock.RUnlock()
}

func (p *Provider) addTarget(host string, ip string, port string, description string) {
	// FIXME: update vhost
}

func (p *Provider) removeTarget(host string, ip string, port string, description string) {
	// FIXME: update vhost
}
