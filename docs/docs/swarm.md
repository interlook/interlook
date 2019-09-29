# Swarm provider

`interlook` can scan Docker Swarm cluster to detect services that needs to be "published". 

The following labels must be setup at service level for `interlook` to detect them:

* `interlook.hosts`: comma separated list of hosts to be published
* `interlook.port`: the application's target port
* `interlook.ssl`: boolean, indicates if application is ssl exposed

Additional service label(s) can be configured to further filter the `interlook` scan. 
This needs to be configured as `labelSelector` in the configuration.  

## Configuration

```yaml
provider:
  swarm:
    endpoint: tcp://ucp.csnet.me:443
    labelSelector:
      - l7aas
    tlsCa: /home/michael/dkr/bundle/interlook/ca.pem
    tlsCert: /home/michael/dkr/bundle/interlook/cert.pem
    tlsKey: /home/michael/dkr/bundle/interlook/key.pem
    pollInterval: 5s
```
