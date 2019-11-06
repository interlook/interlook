![Travis (.org)](https://img.shields.io/travis/interlook/interlook)
[![Coverage Status](https://coveralls.io/repos/github/interlook/interlook/badge.svg?branch=master)](https://coveralls.io/github/interlook/interlook?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/interlook/interlook)](https://goreportcard.com/report/github.com/interlook/interlook)

# interlook

Dynamically provision (V)IP, Load Balancer configuration and DNS alias for services deployed on containers platforms.

## TL;DR

Interlook has a concept of "Providers" and "Provisioners", orchestrated by the "core".

Providers are connected to the containers platform and are responsible for detecting service deployment or deletion.

Provisioners are responsible for interacting with infra components like DNS server, IPAM tools and load balancers/reverse proxies.

The core receives add/delete events from the providers, injects them as workflow entry with a target state ("deployed" or "undeployed"). 

Then it ensures the relevant tasks are performed by the different "provisioners" (as defined in the workflow) to bring the service to the desired state.


## Documentation

[Interlook's documentation](https://interlook.github.io)


## Authors

Boris HUISGEN <bhuisgen@hbis.fr>

Michael CHAMPAGNE <mch1307@gmail.com> 

## Contributing

[Contributing guide](.github/CONTRIBUTING.md)

## Build

``bash
VERSION=$(git log -n1 --pretty="format:%d" | sed "s/, /\n/g" | grep tag: | sed "s/tag: \|)//g") && \
VERSION=$VERSION-$(git log -1 --pretty=format:%h) && \
CGO_ENABLED="0" GOARCH="amd64" GOOS="linux" go build -a -installsuffix cgo -o interlook -ldflags="-s -w -X github.com/interlook/interlook/core=$VERSION"
```

