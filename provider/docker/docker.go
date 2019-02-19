package docker

import (
    "encoding/json"
    "github.com/docker/engine-api/client"
    "github.com/docker/engine-api/types"
    "github.com/docker/engine-api/types/events"
    "github.com/docker/engine-api/types/filters"
    "github.com/pkg/errors"
    "golang.org/x/net/context"
    "io"
    "log"
    "net"
    "sync"
    "time"
)

// ProviderConfiguration holds the provider static configuration
type ProviderConfiguration struct {
    Name           string   `toml:"name"`
    Endpoint       string   `toml:"endpoint"`
    LabelSelector  []string `toml:"labelSelector"`
    TLSCa          string   `toml:"tlsCa"`
    TLSCert        string   `toml:"tlsCert"`
    TLSKey         string   `toml:"tlsKey"`
    Watch          bool     `toml:"watch"`
    WatchInterval  string   `toml:"watchInterval"`
    UpdateInterval string   `toml:"updateInterval"`
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
            log.Println("[ERROR]", "failed to get host ip")

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

    return "", errors.Errorf("failed to get host IP address")
}

func (w *Provider) addTarget(host string, ip string, port string, description string) {
    // FIXME: update vhost
}

func (w *Provider) removeTarget(host string, ip string, port string, description string) {
    // FIXME: update vhost
}
