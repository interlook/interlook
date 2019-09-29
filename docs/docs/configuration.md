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

The other config sections configure the `provider` and the `provisioner(s)`. 

Each component has its own config section. Refer to each extension's doc for configuration reference.
