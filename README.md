# interlook

Dynamically provision VIP, Load Balancer and DNS alias based on container platform events.

## TL;DR

Interlook has a concept of "Providers" and "Provisioners": Providers are container platforms, Provisioners are infra components like DNS server, IPAM tools and load balancers/reverse proxies.

The core implements a basic workflow which orchestrate the tasks.

Technically, providers and provisioners are all implementations of the Extension interface.

Currently supported Providers:
 * ~~Docker~~
 * ~~Docker Swarm~~
 * ~~Docker Enterprise Edition~~

Currently supported Provisioners:
 * IP:
    * file (an embedded simple local IPAM)
 * DNS:
    * ~~Consul~~
 * Load Balancer:
    * ~~F5 Big IP~~ 
    * ~~Envoy proxy~~ 

## Doc

[Interlook's workflow](./WORKFLOW.md)

## Authors

Boris HUISGEN <bhuisgen@hbis.fr>

Michael Champagne <mch1307@gmail.com>
