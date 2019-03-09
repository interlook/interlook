# interlook

Dynamically provision VIP, Load Balancer and DNS alias based on container platform events.

## TL;DR

Interlook has a concept of "Providers" and "Provisioners": 

Providers are container platforms, 

Provisioners are infra components like DNS server, IPAM tools and load balancers/reverse proxies.

The core receives add/delete events from the providers, injects them as a tasks workflow. 

Then it ensures the tasks are performed by the "provisioners".

Technically, providers and provisioners are all implementations of the Extension interface.

Currently supported Providers:
 * ~~Docker~~
 * ~~Docker Swarm~~
 * ~~Docker Enterprise Edition~~
 * ~~Consul Catalog~~

Currently supported Provisioners:
 * IP:
    * [ipalloc](./provisioner/ipam/ipalloc/README.md) (an embedded simple local IPAM)
    * ~~GestioIP~~
 * DNS:
    * Consul (DNS records will contain Consul specific suffix: .service._consul-domain_ )
 * Load Balancer:
    * Kemp LoadMaster
    * ~~F5 Big-IP~~ 

## Doc

[Interlook's workflow](./WORKFLOW.md)

## Authors

Boris HUISGEN <bhuisgen@hbis.fr>

Michael Champagne @mch1307
