# ipalloc

IPAlloc is a simple IP allocator.

It allocates "free" IP address from the configured subnet and keeps track of them is a JSON file.

## config

```yaml
  ipalloc:
    network_cidr: 192.168.99.0/24
    db_file: ./share/allocated.db
```

