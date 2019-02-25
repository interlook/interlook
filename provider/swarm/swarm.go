package swarm

import (
	"log"
	"sync"
	"time"

	"github.com/bhuisgen/interlook/log"
	"github.com/bhuisgen/interlook/service"
	"github.com/docker/engine-api/client"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/filters"
	"golang.org/x/net/context"
)

// ProviderConfiguration holds the provider static configuration
type ProviderConfiguration struct {
	Name           string   `yaml:"name"`
	Endpoint       string   `yaml:"endpoint"`
	LabelSelector  []string `yaml:"labelSelector"`
	TLSCa          string   `yaml:"tlsCa"`
	TLSCert        string   `yaml:"tlsCert"`
	TLSKey         string   `yaml:"tlsKey"`
	Watch          bool     `yaml:"watch"`
	WatchInterval  string   `yaml:"watchInterval"`
	UpdateInterval string   `yaml:"updateInterval"`
}

func (p *ProviderConfiguration) Start(push chan service.Message) error {
	logger.DefaultLogger().Printf("Starting %v on %v\n", p.Name, p.Endpoint)
	var msg service.Message
	msg.Action = "swarm"
	msg.Provider = "swarm"
	msg.Service.Hosts = append(msg.Service.Hosts, "192.168.1.1")
	for {
		time.Sleep(5 * time.Second)
		push <- msg
	}
	logger.DefaultLogger().Println("exiting")
	// do stuff
	//push <- msg
	return nil
}

func (p *ProviderConfiguration) Stop() {
	logger.DefaultLogger().Printf("Stopping %v on %v\n", p.Name, p.Endpoint)
}

// FIXME: will Init function initialize the Provider (from ProviderConfiguration)?
type Provider struct {
	PollInterval   int
	UpdateInterval int
	Filters        map[string][]string
	closed         chan struct{}
	waitGroup      sync.WaitGroup
	pollTicker     *time.Ticker
	updateTicker   *time.Ticker
	services       []string
	servicesLock   sync.RWMutex
}

// FIXME: temp renamed for interface
func (w *Provider) StartO() {
	w.closed = make(chan struct{})
	w.pollTicker = time.NewTicker(time.Duration(w.PollInterval) * time.Second)
	w.updateTicker = time.NewTicker(time.Duration(w.UpdateInterval) * time.Second)

	log.Println("[DEBUG]", "provider started")

	for {
		select {
		case <-w.closed:
			w.waitGroup.Done()

			return

		case <-w.pollTicker.C:
			log.Println("[DEBUG]", "new poll task")
			w.poll()

		case <-w.updateTicker.C:
			log.Println("[DEBUG]", "new update task")
			w.update()
		}
	}
}

func (w *Provider) Stop() {
	log.Println("[DEBUG]", "watcher stop request received")

	close(w.closed)
	w.waitGroup.Wait()

	log.Println("[DEBUG]", "watcher stopped")
}

func (w *Provider) poll() {
	ctx := context.Background()

	cli, err := client.NewEnvClient()
	if err != nil {
		log.Println("[ERROR]", err)

		return
	}

	servicesFilters := filters.NewArgs()
	servicesFilters.Add("label", "lu.sgbt.docker.interlookd.host")
	servicesFilters.Add("label", "lu.sgbt.docker.interlookd.port")

	for name, values := range w.Filters {
		for _, value := range values {
			servicesFilters.Add(name, value)
		}
	}

	data, err := cli.ServiceList(ctx, types.ServiceListOptions{
		Filter: servicesFilters,
	})
	if err != nil {
		log.Println("[ERROR]", "failed to list services", err)

		return
	}

	var services []string

	for _, service := range data {
		services = append(services, service.ID)
	}

	w.servicesLock.Lock()
	defer w.servicesLock.Unlock()

	w.services = services
}

func (w *Provider) update() {
	ctx := context.Background()

	cli, err := client.NewEnvClient()
	if err != nil {
		log.Println("[ERROR]", err)

		return
	}

	w.servicesLock.RLock()

	for _, id := range w.services {
		service, _, err := cli.ServiceInspectWithRaw(ctx, id)
		if err != nil {
			log.Println("[ERROR]", err)

			return
		}

		tasksFilters := filters.NewArgs()
		tasksFilters.Add("service", id)

		tasks, err := cli.TaskList(ctx, types.TaskListOptions{
			Filter: tasksFilters,
		})
		if err != nil {
			log.Println("[ERROR]", err)

			return
		}

		for _, task := range tasks {
			id := task.Status.ContainerStatus.ContainerID

			inspect, err := cli.ContainerInspect(ctx, id)
			if err != nil {
				log.Println("[ERROR]", err)

				return
			}

			node, data, err := cli.NodeInspectWithRaw(ctx, task.NodeID)
			if err != nil {
				log.Println("[ERROR]", err)

				return
			}
			ip := "1.2.3.4" // FIXME: no addr field ?
			log.Println(node.ID)
			log.Println(data)

			var port string

			if inspect.HostConfig.NetworkMode.IsHost() {
				port = inspect.Config.Labels["lu.sgbt.docker.interlookd.port"]
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
				w.addTarget(service.Spec.Labels["lu.sgbt.docker.interlookd.host"], ip, port, id)
			} else {
				w.removeTarget(service.Spec.Labels["lu.sgbt.docker.interlookd.host"], ip, port, id)
			}
		}
	}

	w.servicesLock.RUnlock()
}

func (w *Provider) addTarget(host string, ip string, port string, description string) {
	// FIXME: update vhost
}

func (w *Provider) removeTarget(host string, ip string, port string, description string) {
	// FIXME: update vhost
}