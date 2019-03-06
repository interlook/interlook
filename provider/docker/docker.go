package docker

import (
	"encoding/json"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"github.com/bhuisgen/interlook/log"
	"github.com/bhuisgen/interlook/service"
	"github.com/docker/engine-api/client"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/events"
	"github.com/docker/engine-api/types/filters"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

// Extension holds the provider file configuration
type Extension struct {
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

// Start initialize and start sending events to core
func (p *Extension) Start(receive <-chan service.Message, send chan<- service.Message) error {
	logger.DefaultLogger().Printf("Starting %v on %v\n", p.Name, p.Endpoint)
	var msg service.Message
	msg.Action = "add" // add, remove, update, check
	msg.Service.Provider = "docker"
	msg.Service.Hosts = append(msg.Service.Hosts, "172.1.1.2")
	msg.Service.Name = "test.docker.com"
	msg.Service.DNSName = "test.docker.com"
	msg.Service.Port = 8080
	msg.Service.TLS = false

	//push <- msg

	time.Sleep(1 * time.Second)
	send <- msg

	time.Sleep(20 * time.Second)
	msg.Action = "delete"
	send <- msg

	for {
		time.Sleep(180 * time.Second)
	}
	// do stuff
	//push <- msg
	return nil

}

// Stop stops the provider
func (p *Extension) Stop() {
	logger.DefaultLogger().Printf("Stopping %v\n", p.Name)
}

type Provider struct {
	PollInterval   int
	UpdateInterval int
	Filters        map[string][]string
	closed         chan struct{}
	waitGroup      sync.WaitGroup
	pollTicker     *time.Ticker
	updateTicker   *time.Ticker
	containers     []string
	containersLock sync.RWMutex
}

func (w *Provider) Start() {
	log.Println("[INFO]", "starting docker provider")

	w.closed = make(chan struct{})
	w.pollTicker = time.NewTicker(time.Duration(w.PollInterval) * time.Second)
	w.updateTicker = time.NewTicker(time.Duration(w.UpdateInterval) * time.Second)

	log.Println("[DEBUG]", "provider started")

	defer w.waitGroup.Done()

	for {
		select {
		case <-w.closed:
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

	eventsFilters := filters.NewArgs()
	eventsFilters.Add("label", "lu.sgbt.docker.interlookd.host")
	eventsFilters.Add("label", "lu.sgbt.docker.interlookd.port")

	for name, values := range w.Filters {
		for _, value := range values {
			eventsFilters.Add(name, value)
		}
	}

	data, err := cli.Events(ctx, types.EventsOptions{
		Filters: eventsFilters,
	})
	if err != nil {
		log.Println("[ERROR]", err)

		return
	}

	dec := json.NewDecoder(data)

	for {
		var event events.Message

		err := dec.Decode(&event)
		if err != nil && err == io.EOF {
			break
		}

		if event.Type == events.ContainerEventType {
			switch event.Action {
			case "start":
				w.onContainerStart(event)

			case "die":
				w.onContainerStop(event)
			}
		}
	}
}

func (w *Provider) onContainerStart(event events.Message) {
	log.Println("[DEBUG]", "container started", event.Actor.ID)

	w.containersLock.Lock()
	defer w.containersLock.Unlock()

	w.containers = append(w.containers, event.Actor.ID)
}

func (w *Provider) onContainerStop(event events.Message) {
	log.Println("[DEBUG]", "container stopped", event.Actor.ID)

	w.containersLock.Lock()
	defer w.containersLock.Unlock()

	for index, id := range w.containers {
		if id == event.Actor.ID {
			w.containers = append(w.containers[:index], w.containers[index+1:]...)
		}
	}
}

func (w *Provider) update() {
	ctx := context.Background()

	cli, err := client.NewEnvClient()
	if err != nil {
		log.Println("[ERROR]", err)

		return
	}

	w.containersLock.RLock()

	for _, id := range w.containers {
		inspect, err := cli.ContainerInspect(ctx, id)
		if err != nil {
			log.Println("[ERROR]", err)

			return
		}

		addrs, err := net.InterfaceAddrs()
		if err != nil {
			log.Println("[ERROR]", err)

			return
		}

		var ip = ""

		for _, a := range addrs {
			if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					ip = ipnet.IP.String()

					break
				}
			}
		}
		if ip == "" {
			log.Println("[ERROR]", "failed to get host ipam")

			return
		}

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
			w.addTarget(inspect.Config.Labels["lu.sgbt.docker.interlookd.host"], ip, port, id)
		} else {
			w.removeTarget(inspect.Config.Labels["lu.sgbt.docker.interlookd.host"], ip, port, id)
		}
	}

	w.containersLock.RUnlock()
}

func (w *Provider) getHostIPAddress() (ip string, err error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}

	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String(), nil
			}
		}
	}

	return "", errors.Errorf("failed to get host IPAM address")
}

func (w *Provider) addTarget(host string, ip string, port string, description string) {
	// FIXME: update vhost
}

func (w *Provider) removeTarget(host string, ip string, port string, description string) {
	// FIXME: update vhost
}
