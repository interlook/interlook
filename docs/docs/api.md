# APIs

Basic APIs for viewing `interlook` information

## `/health`

Returns HTTP 200

## `/workflow`

Returns JSON showing configured workflow steps

## `/services`

Retuns JSON listing the services

```json
{
    "myservice": {
        "wip_time": "0001-01-01T00:00:00Z",
        "state": "deployed",
        "expected_state": "deployed",
        "time_detected": "2019-09-27T11:32:23.098856973+02:00",
        "last_update": "2019-09-27T23:28:48.403824618+02:00",
        "service": {
            "name": "myservice",
            "hosts": [
                "10.32.2.41",
                "10.32.2.46",
                "10.32.2.42"
            ],
            "port": 30004,
            "public_ip": "10.32.30.2",
            "dns_name": [
                "myapp.mydomain.me"
            ]
        },
        "close_time": "2019-09-27T21:33:59.774608662+02:00"
    }
}
```

## `/version`

Retuns `interlook`'s version

