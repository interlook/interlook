# Consul

Consul can be used as DNS for `interlook` discovered services.

> DNS records will contain Consul specific suffix: .service._consul-domain_, use CoreDNS with rewrite

## Configuration

```yaml
dns:
  consul:
    url: http://127.0.0.1:8500
    domain:
    token:
``
