module github.com/interlook/interlook

go 1.12

require (
	github.com/Azure/go-ansiterm v0.0.0-20170929234023-d6e3b3328b78 // indirect
	github.com/Microsoft/go-winio v0.4.12 // indirect
	github.com/docker/distribution v2.7.1+incompatible // indirect
	github.com/docker/docker v1.13.1
	github.com/docker/go-connections v0.4.0 // indirect
	github.com/docker/go-units v0.4.0 // indirect
	github.com/fatih/structs v1.1.0
	github.com/google/go-cmp v0.5.2
	github.com/gorilla/mux v1.7.1 // indirect
	github.com/hashicorp/consul/api v1.2.0
	github.com/morikuni/aec v0.0.0-20170113033406-39771216ff4c // indirect
	github.com/opencontainers/go-digest v1.0.0-rc1 // indirect
	github.com/opencontainers/image-spec v1.0.1 // indirect
	github.com/pkg/errors v0.9.1
	github.com/scottdware/go-bigip v0.0.0-00010101000000-000000000000
	github.com/sirupsen/logrus v1.4.2
	golang.org/x/net v0.0.0-20201110031124-69a78807bb2b
	gopkg.in/yaml.v3 v3.0.0-20200313102051-9f266ea9e77c
	gotest.tools v2.2.0+incompatible // indirect
	k8s.io/api v0.20.0
	k8s.io/apimachinery v0.20.0
	k8s.io/client-go v0.20.0
)

replace (
	github.com/docker/docker => github.com/docker/engine v0.0.0-20190725163905-fa8dd90ceb7b
	github.com/scottdware/go-bigip => github.com/mch1307/go-bigip v0.0.0-20191206213651-f622de5c0149
	gotest.tools => github.com/gotestyourself/gotest.tools v2.2.0+incompatible
)
