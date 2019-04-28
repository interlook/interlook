# Configuration

Interlook uses a yaml formatted configuration file. 

The file contains different sections:

* core:

Configure interlook's core module
 
```yaml
core:
  logLevel: DEBUG
  listenPort: 8080
  logFile : stdout
  # workflowSteps: comma separated succession of extenstions
  workflowSteps: provider.swarm,ipam.ipalloc,lb.f5ltm
  # where the workflow entries are saved
  workflowEntriesFile: ./share/flowentries.db
  # how often should the workflow controller run
  workflowActivityLauncherInterval: 3s
  # how often should the workflow housekeeper run
  workflowHousekeeperInterval: 60s
  # close the entry in error if work in progress for longer than
  serviceWIPTimeout: 90s
  # remove entries that have been closed for time
  cleanUndeployedServiceAfter: 10m
  # trigger a refresh request to provider if service has not been updated since
  serviceMaxLastUpdated: 90s
``` 

The other sections configure the `provider` and the `provisioners`. Each component has its own configuration. Refer to the `extension` implementation in the component package. For example for `ipalloc` refer to the package in provisioner\ipam\ipalloc.

* `provider`

```yaml
provider:
  swarm:
    endpoint: tcp://ucp.csnet.me:443
    labelSelector:
      - l7aas
    tlsCa: /path/to/ca.pem
    tlsCert: /path/to/cert.pem
    tlsKey: /path/to/key.pem
    pollInterval: 5s
```

* `ipam`

```yaml
ipam:
  ipalloc:
    network_cidr: 192.168.99.0/24
    db_file: ./share/conf/allocated.db
```


* `dns`
```yaml
dns:
  consul:
    url: http://127.0.0.1:8500
    domain:
    token:
```

* `lb`

```yaml
lb:
  kemplm:
    endpoint: https://192.168.99.2
    username: api
    password: apiPassw0rd
    httpPort:
    httpsPort:
```