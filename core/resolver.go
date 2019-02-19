package core

type Resolver interface {
    Init()

    Start()

    Stop()

    AddEntry(host string, ip string)

    RemoveEntry(host string, ip string)
}

