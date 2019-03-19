# WORKFLOW

This doc describes the interlook workflow by explaining a basic example.

`interlook` creates a listener for each configured extension. 
This listener will receive all messages/events coming from a given extension.

Those events, together with their state will be stored in an entry list.

In order to bring the services to the desired state (deployed or undeployed), `interlook` polls the entry list at an interval defined in the config (`core.checkFlowInterval`). 
It then sends an event to the corresponding extension.

The extension is handling the required action and sending back the status to the core listener.    

## Example

We have a provider (docker) that publishes services on given host(s) / port

When we get a new service definition from docker, we want to get an IP from our IPAM component (local file)

In the yml config file, our workflow definition will be:

`provider.docker,ipam.file`

The internal flow/message exchange will be like this:

### 1. Provider to workflow

Docker provider pushes a newly published service.
 
The listener gets it and inject it to the workflow

```
|Provider|  ->      Listener       ->       |workflow|  
                    state:provider.docker
                    wip:false
```

### 2. The flowControl detects the new entry

flowControl check current state against the expected state. 

If it does not match, sets the next step/extension, changes the status to "wip" and sends the message to the next extension (IPAM) 

```
|workflow|  ->      flowControl     ->      |IPAM|
                    state:ipam.file
                    wip:true
```

### 3. The IPAM extension sends back a message
 
IPAM extension does it's job and gets an IP for us. Then it sends back the message

```
|IPAM|      ->      Listener     ->      |workflow|
state:ipam.file     state:ipam.file
wip:true            wip:false
```

### 4. Closing the flow

In our example the IPAM extension is the last step of the workflow. 

The flow control module detects that "ipam.file" is the last step in our workflow. 

As we have reached the final step, it updates the entry's state to deployed, closing the flow.

```
|workflow|
state:deployed
wip:false
```
