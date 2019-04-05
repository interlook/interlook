package swarm

import (
    "sync"
    "time"

    "github.com/bhuisgen/interlook/log"
    "github.com/bhuisgen/interlook/service"
    "github.com/docker/engine-api/client"
    "github.com/docker/engine-api/types"
    "github.com/docker/engine-api/types/filters"
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
}

//type services map[string][]string

func (p *Provider) Start(receive <-chan service.Message, send chan<- service.Message) error {

    p.shutdown = make(chan bool)
    p.pollTicker = time.NewTicker(p.WatchInterval)
    p.updateTicker = time.NewTicker(p.UpdateInterval)
    p.send = send
    p.waitGroup.Add(1)

    for {
        select {
        case <-p.shutdown:
            p.waitGroup.Done()

            return

        case <-p.pollTicker.C:
            log.Info("[DEBUG]", "new poll task")
            p.poll()

        case <-p.updateTicker.C:
            log.Info("[DEBUG]", "new update task")
            p.update()
        }
    }
}
    var msg service.Message
    msg.Action = "add" // add, remove, update, check
    msg.Service.Provider = "swarm"
    msg.Service.Hosts = append(msg.Service.Hosts, "192.168.1.1")
    msg.Service.Name = "test.swarm.com"
    msg.Service.DNSAliases = []string{"test.swarm.com"}
    msg.Service.Port = 8080
    msg.Service.TLS = true
    for {
        time.Sleep(10 * time.Second)
        send <- msg
    }

}

func (p *Provider) Stop() error {
    log.Info("Stopping Swarm provider")
    p.shutdown <- true
    return nil
}

// FIXME: will Init function initialize the Provider (from Provider)?
type Provider0 struct {
    WatchInterval  int
    UpdateInterval int
    Filters        map[string][]string
    shutdown       chan struct{}
    waitGroup      sync.WaitGroup
    pollTicker     *time.Ticker
    updateTicker   *time.Ticker
    services       []string
    servicesLock   sync.RWMutex
}

// FIXME: temp renamed for interface
func (w *Provider) StartO() {
    w.shutdown = make(chan struct{})
    w.pollTicker = time.NewTicker(time.Duration(w.WatchInterval) * time.Second)
    w.updateTicker = time.NewTicker(time.Duration(w.UpdateInterval) * time.Second)

    log.Info("[DEBUG]", "provider started")

    for {
        select {
        case <-w.shutdown:
            w.waitGroup.Done()

            return

        case <-w.pollTicker.C:
            log.Info("[DEBUG]", "new poll task")
            w.poll()

        case <-w.updateTicker.C:
            log.Info("[DEBUG]", "new update task")
            w.update()
        }
    }
}

func (w *Provider) Stop() {
    log.Info("[DEBUG]", "watcher stop request received")

    close(w.shutdown)
    w.waitGroup.Wait()

    log.Info("[DEBUG]", "watcher stopped")
}

func (w *Provider) poll() {
    ctx := context.Background()

    cli, err := client.NewEnvClient()
    if err != nil {
        log.Info("[ERROR]", err)

        return
    }

    servicesFilters := filters.NewArgs()
    servicesFilters.Add("label", "interlook.host")
    servicesFilters.Add("label", "interlook.port")

    for name, values := range w.Filters {
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

    w.servicesLock.Lock()
    defer w.servicesLock.Unlock()

    w.services = services
}

func (w *Provider) update() {
    ctx := context.Background()

    cli, err := client.NewEnvClient()
    if err != nil {
        log.Info("[ERROR]", err)

        return
    }

    w.servicesLock.RLock()

    for _, id := range w.services {
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
                w.addTarget(service.Spec.Labels["interlook.host"], ip, port, id)
            } else {
                w.removeTarget(service.Spec.Labels["interlook.host"], ip, port, id)
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
