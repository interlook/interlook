package main

import (
    "encoding/json"
    "github.com/bhuisgen/interlook/internal/interlookd/docker"
    "github.com/bhuisgen/interlook/internal/interlookd/swarm"
    "io/ioutil"
    "log"
    "os"
    "os/signal"
    "syscall"
)

type ApplicationConfiguration struct {
    Watcher string `json:"watcher"`
}

type Application struct {
    Configuration ApplicationConfiguration
    dockerWatcher docker.Watcher
    swarmWatcher  swarm.Watcher
}

var (
    Version   = "0.1.0"
)

func main() {
    log.Println("[INFO]", "interlockd", Version)

    app := Application{}

    if err := app.readConfiguration(&app.Configuration); err != nil {
        log.Println("[ERROR]", err)

        os.Exit(2)

        return
    }

    app.Run()
}

func (app *Application) readConfiguration(config *ApplicationConfiguration) (err error) {
    file, err := os.Open("interlookd.json")
    if err != nil {
        return err
    }

    defer file.Close()

    byteValue, _ := ioutil.ReadAll(file)

    err = json.Unmarshal(byteValue, config)
    if err != nil {
        return err
    }

    return nil
}

func (app *Application) Run() {
    ch := make(chan os.Signal)
    signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
    go app.handler(ch)

    switch app.Configuration.Watcher {
    case "docker":
        app.startDocker()
    case "swarm":
        app.startSwarm()
    }
}

func (app *Application) startDocker() {
    log.Println("[INFO]", "starting docker watcher")

    app.dockerWatcher = docker.Watcher{
        PollInterval:   15,
        UpdateInterval: 30,
        Filters: map[string][] string{
            "label": {"lu.sgbt.docker.interlook"},
        },
    }

    app.dockerWatcher.Start()
}

func (app *Application) startSwarm() {
    log.Println("[INFO]", "starting swarm watcher")

    app.swarmWatcher = swarm.Watcher{
        PollInterval:   15,
        UpdateInterval: 30,
        Filters: map[string][] string{
            "label": {"lu.sgbt.docker.interlook"},
        },
    }

    app.swarmWatcher.Start()
}

func (app *Application) handler(sigs <-chan os.Signal) {
    sig := <-sigs

    log.Println("[INFO]", "signal", sig, "received, aborting execution")

    switch app.Configuration.Watcher {
    case "docker":
        app.dockerWatcher.Stop()

    case "swarm":
        app.swarmWatcher.Stop()
    }

    if sig.String() == "terminated" {
        os.Exit(0)
    } else {
        os.Exit(1)
    }
}
