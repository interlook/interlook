# KUbernetes provider

`interlook` can watch Kubernetes cluster to detect [`NodePort`](https://kubernetes.io/docs/concepts/services-networking/service/#nodeport) services that needs to be "published". 

The following labels must be setup at service level for `interlook` to detect them:

* `interlook.hosts`: comma separated list of hosts to be published
* `interlook.port`: the application's target port
* `interlook.ssl`: boolean, indicates if application is ssl exposed

Additional service label(s) can be configured to further filter the `interlook` service scan. 
This needs to be configured as `labelSelector` in the configuration.  

## Configuration

```yaml
provider:
    kubernetes:
        endpoint: https://192.168.39.89:6443
        labelSelector:
            - l7aas
        tlsCa: /path/to/kube/ca.crt
        tlsCert: /path/to/kube/client.crt
        tlsKey: /path/to/kube/client.key
        pollInterval: 15s
```
