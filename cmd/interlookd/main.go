package main

import (
    "encoding/json"
    "github.com/bhuisgen/interlook/internal/interlookd/core"
    "io/ioutil"
    "log"
    "os"
    "os/signal"
    "syscall"
)

type ApplicationConfiguration struct {
    Provider string `json:"watcher"`
}

type Application struct {
    Configuration  ApplicationConfiguration
    Server core.Server
}

var (
    Version = "0.1.0"
)

func main() {
    log.Println("[INFO]", "interlookd", Version)

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

    app.Server.Run()
}

func (app *Application) handler(sigs <-chan os.Signal) {
    sig := <-sigs

    log.Println("[INFO]", "signal", sig, "received, aborting execution")

    if sig.String() == "terminated" {
        app.Server.Exit(sig)

        os.Exit(0)
    } else {
        app.Server.Exit(sig)

        os.Exit(1)
    }
}
