package core

type Provider interface {
    Init()

    Start()

    Stop()
}
