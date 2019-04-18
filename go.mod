module github.com/bhuisgen/interlook

go 1.12

require (
	github.com/Azure/go-ansiterm v0.0.0-20170929234023-d6e3b3328b78 // indirect
	github.com/Microsoft/go-winio v0.4.12 // indirect
	github.com/Nvveen/Gotty v0.0.0-20120604004816-cd527374f1e5 // indirect
	github.com/cheapRoc/transition v0.0.0-20180824213918-9394332d32e6
	github.com/docker/distribution v2.7.1+incompatible // indirect
	github.com/docker/docker v1.13.1
	github.com/docker/go-connections v0.4.0
	github.com/docker/go-units v0.3.3 // indirect
	github.com/fatih/structs v1.1.0
	github.com/gogo/protobuf v1.2.1 // indirect
	github.com/google/go-cmp v0.2.0 // indirect
	github.com/gorilla/mux v1.7.1 // indirect
	github.com/hashicorp/consul/api v1.0.1
	github.com/opencontainers/go-digest v1.0.0-rc1 // indirect
	github.com/opencontainers/image-spec v1.0.1 // indirect
	github.com/pkg/errors v0.8.1
	github.com/sirupsen/logrus v1.4.1
	golang.org/x/net v0.0.0-20190404232315-eb5bcb51f2a3
	golang.org/x/time v0.0.0-20190308202827-9d24e82272b4 // indirect
	google.golang.org/grpc v1.19.1 // indirect
	gopkg.in/yaml.v3 v3.0.0-20190409140830-cdc409dda467
	gotest.tools v2.2.0+incompatible // indirect
)

replace github.com/docker/docker v1.13.1 => github.com/docker/engine v0.0.0-20190327083406-200b524eff60
