package core

type LoadBalancer interface {
    Init()

    Start()

    Stop()

    AddTarget(host string, ip string, port int)

    RemoveTarget(host string, ip string, port int)
}
