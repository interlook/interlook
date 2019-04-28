# Architecture

Technically, providers and provisioners are all implementations of the Extension interface.

![](interlook-draw.png)


Currently supported Providers:
 * ~~Docker~~
 * Docker Swarm (not tested yet)
 * Docker Enterprise (Swarm)
 * ~~Consul Catalog~~

Currently supported Provisioners:
 * IP:
    * ipalloc (an embedded simple local IPAM)
    * ~~GestioIP~~
 * DNS:
    * Consul (DNS records will contain Consul specific suffix: .service._consul-domain_, use CoreDNS with rewrite)
 * Load Balancer:
    * Kemp LoadMaster
    * F5 Big-IP LTM
