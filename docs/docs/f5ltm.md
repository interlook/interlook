# F5 BigIP

`interlook` can configure the f5 BigIP system for routing the traffic to the containerized application.

Two modes (`updateMode`) are supported:

* `vs`: a virtual server is maintained for each service
* `policy`: a policy, attached to a global VS (`golbalVS`) is maintained for each service
    

## Configuration

```yaml
  f5ltm:
    httpEndpoint: https://10.32.20.100
    username: api
    password: restaccess
    authProvider: tmos
    authToken:
    httpPort: 80
    httpsPort: 443
    monitorName: tcp
    tcpProfile:
    partition: interlook
    loadBalancingMode: least-connections-member
    # interlook has 2 ways to update BigIP:
        # "vs": a virtual server is created for each service
        # "policy": a policy is added to a global vs for ech service
    updateMode: vs
    globalVS:
```
