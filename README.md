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
