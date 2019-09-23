module github.com/interlook

go 1.12

require (
	github.com/docker/docker v1.13.1
	github.com/fatih/structs v1.1.0
	github.com/hashicorp/consul/api v1.2.0
	github.com/interlook/interlook v0.0.0-20190922203222-c503f23bff0b
	github.com/morikuni/aec v0.0.0-20170113033406-39771216ff4c // indirect
	github.com/pkg/errors v0.8.1
	github.com/sirupsen/logrus v1.4.2
	golang.org/x/net v0.0.0-20190921015927-1a5e07d1ff72
	gopkg.in/yaml.v3 v3.0.0-20190905181640-827449938966
)

replace github.com/docker/docker => github.com/docker/engine v0.0.0-20190725163905-fa8dd90ceb7b
