package docker

import (
    "encoding/json"
    "io"
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

// Extension holds the provider ipalloc configuration
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
    close          chan bool
    receive        <-chan service.Message
    send           chan<- service.Message
}

func (p *Extension) RefreshService(serviceName string) {
    // check if service still exists and is up
    // if not send delete msg to p.send

}

// Start initialize and start sending events to core
func (p *Extension) Start(receive <-chan service.Message, send chan<- service.Message) error {
    p.close = make(chan bool)
    p.receive = receive
    p.send = send
    log.Printf("Starting %v on %v\n", p.Name, p.Endpoint)
    var msg service.Message
    msg.Action = "add" // add, remove, update, check
    msg.Service.Provider = "docker"
    msg.Service.Hosts = append(msg.Service.Hosts, "10.32.2.42", "10.32.2.46")
    msg.Service.Name = "mytest.app.csnet.me"
    msg.Service.DNSAliases = []string{"mytest.app.csnet.me", "mytest.csnet.me"}
    msg.Service.Port = 81
    msg.Service.TLS = false

    time.Sleep(2 * time.Second)
    send <- msg
    log.Debugf("##################### Add sent, will send update in 30secs")
    time.Sleep(30 * time.Second)
    msg.Service.Port = 81
    msg.Service.Hosts = make([]string, 0)
    msg.Service.Hosts = append(msg.Service.Hosts, "10.32.2.46", "10.32.2.45")
    send <- msg
    msg.Action = "delete"
    log.Debugf("##################### Will send delete in 30secs")
    time.Sleep(30 * time.Second)
    send <- msg

    for {
        select {
        case <-p.close:
            log.Debug("closed docker provider")
            return nil
        case msg := <-receive:
            log.Debugf("docker got msg", msg)
            continue
        }
    }
}

// Stop stops the provider
func (p *Extension) Stop() error {

    p.close <- true
    log.Debug("Stopping docker")
    return nil
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
    log.Info("[INFO]", "starting docker provider")

    w.closed = make(chan struct{})
    w.pollTicker = time.NewTicker(time.Duration(w.PollInterval) * time.Second)
    w.updateTicker = time.NewTicker(time.Duration(w.UpdateInterval) * time.Second)

    log.Info("[DEBUG]", "provider started")

    defer w.waitGroup.Done()

    for {
        select {
        case <-w.closed:
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

    close(w.closed)
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

    eventsFilters := filters.NewArgs()
    eventsFilters.Add("label", "interlook.host")
    eventsFilters.Add("label", "interlook.port")

    for name, values := range w.Filters {
        for _, value := range values {
            eventsFilters.Add(name, value)
        }
    }

    data, err := cli.Events(ctx, types.EventsOptions{
        Filters: eventsFilters,
    })
    if err != nil {
        log.Info("[ERROR]", err)

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
    log.Info("[DEBUG]", "container started", event.Actor.ID)

    w.containersLock.Lock()
    defer w.containersLock.Unlock()

    w.containers = append(w.containers, event.Actor.ID)
}

func (w *Provider) onContainerStop(event events.Message) {
    log.Info("[DEBUG]", "container stopped", event.Actor.ID)

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
        log.Info("[ERROR]", err)

        return
    }

    w.containersLock.RLock()

    for _, id := range w.containers {
        inspect, err := cli.ContainerInspect(ctx, id)
        if err != nil {
            log.Info("[ERROR]", err)

            return
        }

        addrs, err := net.InterfaceAddrs()
        if err != nil {
            log.Info("[ERROR]", err)

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
            log.Info("[ERROR]", "failed to get host ipam")

            return
        }

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
            w.addTarget(inspect.Config.Labels["interlook.host"], ip, port, id)
        } else {
            w.removeTarget(inspect.Config.Labels["interlook.host"], ip, port, id)
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
