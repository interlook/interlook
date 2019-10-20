package swarm

import (
	"encoding/json"
	"strings"
	//"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
	"github.com/interlook/interlook/comm"
	"github.com/interlook/interlook/log"
	"net/http"
	"os"
	"reflect"
	"testing"
	"time"
)

var (
	testService swarm.Service
	node1       swarm.Node
	node2       swarm.Node
	msgOK       comm.Message
)

func TestMain(m *testing.M) {
	mockSwarmAPI()
	initTestVars()
	rc := m.Run()
	os.Exit(rc)
}

// startSwarmProvider returns a "running" swarm provider instance
func startSwarmProvider() (p *Provider, rec, send chan comm.Message) {
	p = &Provider{
		Endpoint:      "tcp://localhost:2377",
		LabelSelector: []string{"l7aas=true"},
		TLSCa:         "./test-files/ca.pem",
		TLSCert:       "./test-files/server.pem",
		TLSKey:        "./test-files/key.pem",
		PollInterval:  10 * time.Second,
	}
	p.init()
	rec = make(chan comm.Message)
	send = make(chan comm.Message)
	go p.Start(rec, send)

	return p, rec, send
}

// initialize test variables
func initTestVars() {

	msgOK = comm.Message{Service: comm.Service{
		Name:       "test",
		DNSAliases: []string{"test.caas.csnet.me"},
		Port:       30001,
		Hosts:      []string{"10.32.2.2", "10.32.2.3"},
		TLS:        false,
		Provider:   extensionName,
	},
		Action: comm.AddAction}

	testServiceRaw := `{
    "ID": "9mnpnzenvg8p8tdbtq4wvbkcz",
    "Version": {
        "Index": 19
    },
    "CreatedAt": "2016-06-07T21:05:51.880065305Z",
    "UpdatedAt": "2016-06-07T21:07:29.962229872Z",
    "Spec": {
        "Name": "test",
        "Labels": {
            "interlook.ssl": "false",
            "interlook.port": "80",
            "interlook.hosts": "test.caas.csnet.me",
            "l7aas": "false"
        },
        "TaskTemplate": {
            "ContainerSpec": {
                "Image": "redis"
            },
            "Resources": {
                "Limits": {},
                "Reservations": {}
            },
            "RestartPolicy": {
                "Condition": "any",
                "MaxAttempts": 0
            },
            "Placement": {},
            "ForceUpdate": 0
        },
        "Mode": {
            "Replicated": {
                "Replicas": 1
            }
        },
        "UpdateConfig": {
            "Parallelism": 1,
            "Delay": 1000000000,
            "FailureAction": "pause",
            "Monitor": 15000000000,
            "MaxFailureRatio": 0.15
        },
        "RollbackConfig": {
            "Parallelism": 1,
            "Delay": 1000000000,
            "FailureAction": "pause",
            "Monitor": 15000000000,
            "MaxFailureRatio": 0.15
        },
        "EndpointSpec": {
            "Mode": "vip",
            "Ports": [
                {
                    "Protocol": "tcp",
                    "TargetPort": 80,
                    "PublishedPort": 30001
                }
            ]
        }
    },
    "Endpoint": {
        "Spec": {
            "Mode": "vip",
            "Ports": [
                {
                    "Protocol": "tcp",
                    "TargetPort": 80,
                    "PublishedPort": 30001
                }
            ]
        },
        "Ports": [
            {
                "Protocol": "tcp",
                "TargetPort": 80,
                "PublishedPort": 30001
            }
        ],
        "VirtualIPs": [
            {
                "NetworkID": "4qvuz4ko70xaltuqbt8956gd1",
                "Addr": "10.255.0.2/16"
            },
            {
                "NetworkID": "4qvuz4ko70xaltuqbt8956gd1",
                "Addr": "10.255.0.3/16"
            }
        ]
    }
}`
	if err := json.Unmarshal([]byte(testServiceRaw), &testService); err != nil {
		log.Fatal(err)
	}
	node1Raw := `{
    "ID": "p2g1i2vehdevdpwgoqr5wkq5e",
    "Version": {
        "Index": 373531
    },
    "CreatedAt": "2016-08-18T10:44:24.496525531Z",
    "UpdatedAt": "2017-08-09T07:09:37.632105588Z",
    "Spec": {
        "Availability": "active",
        "Name": "node-name",
        "Role": "manager",
        "Labels": {
            "foo": "bar"
        }
    },
    "Description": {
        "Hostname": "bf3067039e47",
        "Platform": {
            "Architecture": "x86_64",
            "OS": "linux"
        },
        "Resources": {
            "NanoCPUs": 4000000000,
            "MemoryBytes": 8272408576,
            "GenericResources": [
                {
                    "DiscreteResourceSpec": {
                        "Kind": "SSD",
                        "Value": 3
                    }
                },
                {
                    "NamedResourceSpec": {
                        "Kind": "GPU",
                        "Value": "UUID1"
                    }
                },
                {
                    "NamedResourceSpec": {
                        "Kind": "GPU",
                        "Value": "UUID2"
                    }
                }
            ]
        },
        "Engine": {
            "EngineVersion": "19.03.0",
            "Labels": {
                "foo": "bar"
            },
            "Plugins": [
                {
                    "Type": "Log",
                    "Name": "fluentd"
                }
            ]
        },
        "TLSInfo": {
            "TrustRoot": "-----BEGIN CERTIFICATE-----\nMIIBajCCARCgAwIBAgIUbYqrLSOSQHoxD8CwG6Bi2PJi9c8wCgYIKoZIzj0EAwIw\nEzERMA8GA1UEAxMIc3dhcm0tY2EwHhcNMTcwNDI0MjE0MzAwWhcNMzcwNDE5MjE0\nMzAwWjATMREwDwYDVQQDEwhzd2FybS1jYTBZMBMGByqGSM49AgEGCCqGSM49AwEH\nA0IABJk/VyMPYdaqDXJb/VXh5n/1Yuv7iNrxV3Qb3l06XD46seovcDWs3IZNV1lf\n3Skyr0ofcchipoiHkXBODojJydSjQjBAMA4GA1UdDwEB/wQEAwIBBjAPBgNVHRMB\nAf8EBTADAQH/MB0GA1UdDgQWBBRUXxuRcnFjDfR/RIAUQab8ZV/n4jAKBggqhkjO\nPQQDAgNIADBFAiAy+JTe6Uc3KyLCMiqGl2GyWGQqQDEcO3/YG36x7om65AIhAJvz\npxv6zFeVEkAEEkqIYi0omA9+CjanB/6Bz4n1uw8H\n-----END CERTIFICATE-----\n",
            "CertIssuerSubject": "MBMxETAPBgNVBAMTCHN3YXJtLWNh",
            "CertIssuerPublicKey": "MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEmT9XIw9h1qoNclv9VeHmf/Vi6/uI2vFXdBveXTpcPjqx6i9wNazchk1XWV/dKTKvSh9xyGKmiIeRcE4OiMnJ1A=="
        }
    },
    "Status": {
        "State": "ready",
        "Message": "",
        "Addr": "10.32.2.2"
    },
    "ManagerStatus": {
        "Leader": true,
        "Reachability": "reachable",
        "Addr": "10.0.0.46:2377"
    }
}`
	node2Raw := `{
    "ID": "nybocmfjabg3wz5yx9cex9oc4",
    "Version": {
        "Index": 373531
    },
    "CreatedAt": "2016-08-18T10:44:24.496525531Z",
    "UpdatedAt": "2017-08-09T07:09:37.632105588Z",
    "Spec": {
        "Availability": "active",
        "Name": "node-name",
        "Role": "manager",
        "Labels": {
            "foo": "bar"
        }
    },
    "Description": {
        "Hostname": "bf3067039e47",
        "Platform": {
            "Architecture": "x86_64",
            "OS": "linux"
        },
        "Resources": {
            "NanoCPUs": 4000000000,
            "MemoryBytes": 8272408576,
            "GenericResources": [
                {
                    "DiscreteResourceSpec": {
                        "Kind": "SSD",
                        "Value": 3
                    }
                },
                {
                    "NamedResourceSpec": {
                        "Kind": "GPU",
                        "Value": "UUID1"
                    }
                },
                {
                    "NamedResourceSpec": {
                        "Kind": "GPU",
                        "Value": "UUID2"
                    }
                }
            ]
        },
        "Engine": {
            "EngineVersion": "19.03.0",
            "Labels": {
                "foo": "bar"
            },
            "Plugins": [
                {
                    "Type": "Log",
                    "Name": "fluentd"
                }
            ]
        },
        "TLSInfo": {
            "TrustRoot": "-----BEGIN CERTIFICATE-----\nMIIBajCCARCgAwIBAgIUbYqrLSOSQHoxD8CwG6Bi2PJi9c8wCgYIKoZIzj0EAwIw\nEzERMA8GA1UEAxMIc3dhcm0tY2EwHhcNMTcwNDI0MjE0MzAwWhcNMzcwNDE5MjE0\nMzAwWjATMREwDwYDVQQDEwhzd2FybS1jYTBZMBMGByqGSM49AgEGCCqGSM49AwEH\nA0IABJk/VyMPYdaqDXJb/VXh5n/1Yuv7iNrxV3Qb3l06XD46seovcDWs3IZNV1lf\n3Skyr0ofcchipoiHkXBODojJydSjQjBAMA4GA1UdDwEB/wQEAwIBBjAPBgNVHRMB\nAf8EBTADAQH/MB0GA1UdDgQWBBRUXxuRcnFjDfR/RIAUQab8ZV/n4jAKBggqhkjO\nPQQDAgNIADBFAiAy+JTe6Uc3KyLCMiqGl2GyWGQqQDEcO3/YG36x7om65AIhAJvz\npxv6zFeVEkAEEkqIYi0omA9+CjanB/6Bz4n1uw8H\n-----END CERTIFICATE-----\n",
            "CertIssuerSubject": "MBMxETAPBgNVBAMTCHN3YXJtLWNh",
            "CertIssuerPublicKey": "MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEmT9XIw9h1qoNclv9VeHmf/Vi6/uI2vFXdBveXTpcPjqx6i9wNazchk1XWV/dKTKvSh9xyGKmiIeRcE4OiMnJ1A=="
        }
    },
    "Status": {
        "State": "ready",
        "Message": "",
        "Addr": "10.32.2.3"
    },
    "ManagerStatus": {
        "Leader": true,
        "Reachability": "reachable",
        "Addr": "10.0.0.46:2377"
    }
}`
	if err := json.Unmarshal([]byte(node1Raw), &node1); err != nil {
		log.Fatal(err)
	}

	if err := json.Unmarshal([]byte(node2Raw), &node2); err != nil {
		log.Fatal(err)
	}
}

// starts an http server mocking docker API
func mockSwarmAPI() {
	go http.ListenAndServe(":2376", nil)
	go http.ListenAndServeTLS(":2377", "./test-files/server.pem", "./test-files/key.pem", nil)
	time.Sleep(1 * time.Second)
	http.HandleFunc("/v1.29/services", mockServiceList)
	http.HandleFunc("/v1.29/nodes", mockNodes)
	http.HandleFunc("/v1.29/tasks", mockTasks)
}

func mockServiceList(w http.ResponseWriter, r *http.Request) {
	var rsp []byte
	if strings.Contains(r.RequestURI, "invalid") {
		l := []swarm.Service{}
		rsp, _ = json.Marshal(l)
	} else {
		l := []swarm.Service{testService}
		rsp, _ = json.Marshal(l)
	}

	w.Write(rsp)
}

func mockTasks(w http.ResponseWriter, r *http.Request) {
	rsp := `[
    {
        "ID": "7lmhlp7zgruu4lijoa6fahun3",
        "Version": {
            "Index": 128
        },
        "CreatedAt": "2019-10-13T19:16:28.857063284Z",
        "UpdatedAt": "2019-10-13T19:16:32.356747518Z",
        "Labels": {},
        "Spec": {
            "ContainerSpec": {
                "Image": "nginx:latest@sha256:aeded0f2a861747f43a01cf1018cf9efe2bdd02afd57d2b11fcc7fcadc16ccd1",
                "Labels": {
                    "com.docker.stack.namespace": "swarmnginx"
                },
                "Privileges": {
                    "CredentialSpec": null,
                    "SELinuxContext": null
                },
                "Isolation": "default"
            },
            "Resources": {},
            "RestartPolicy": {
                "Condition": "on-failure",
                "MaxAttempts": 0
            },
            "Placement": {
                "Platforms": [
                    {
                        "Architecture": "amd64",
                        "OS": "linux"
                    },
                    {
                        "OS": "linux"
                    },
                    {
                        "Architecture": "arm64",
                        "OS": "linux"
                    },
                    {
                        "Architecture": "386",
                        "OS": "linux"
                    },
                    {
                        "Architecture": "ppc64le",
                        "OS": "linux"
                    },
                    {
                        "Architecture": "s390x",
                        "OS": "linux"
                    }
                ]
            },
            "Networks": [
                {
                    "Target": "7qvqlb2to5q77iw3k9mk97o4i",
                    "Aliases": [
                        "nginx"
                    ]
                }
            ],
            "ForceUpdate": 0
        },
        "ServiceID": "kind13dc6ykcr7s5lql7wv064",
        "Slot": 2,
        "NodeID": "p2g1i2vehdevdpwgoqr5wkq5e",
        "Status": {
            "Timestamp": "2019-10-13T19:16:32.318027692Z",
            "State": "running",
            "Message": "started",
            "ContainerStatus": {
                "ContainerID": "b57c703c0f9036e1f769677dbcb2981d782edee18005cbad568d3d984a4c8ad3",
                "PID": 77098,
                "ExitCode": 0
            },
            "PortStatus": {}
        },
        "DesiredState": "running",
        "NetworksAttachments": [
            {
                "Network": {
                    "ID": "zkw3jhutz9b61bcxwpy7sy7a8",
                    "Version": {
                        "Index": 6
                    },
                    "CreatedAt": "2019-10-12T09:05:05.02856252Z",
                    "UpdatedAt": "2019-10-12T09:05:05.053529433Z",
                    "Spec": {
                        "Name": "ingress",
                        "Labels": {},
                        "DriverConfiguration": {},
                        "Ingress": true,
                        "IPAMOptions": {
                            "Driver": {},
                            "Configs": [
                                {
                                    "Subnet": "10.255.0.0/16",
                                    "Gateway": "10.255.0.1"
                                }
                            ]
                        },
                        "Scope": "swarm"
                    },
                    "DriverState": {
                        "Name": "overlay",
                        "Options": {
                            "com.docker.network.driver.overlay.vxlanid_list": "4096"
                        }
                    },
                    "IPAMOptions": {
                        "Driver": {
                            "Name": "default"
                        },
                        "Configs": [
                            {
                                "Subnet": "10.255.0.0/16",
                                "Gateway": "10.255.0.1"
                            }
                        ]
                    }
                },
                "Addresses": [
                    "10.255.0.18/16"
                ]
            },
            {
                "Network": {
                    "ID": "7qvqlb2to5q77iw3k9mk97o4i",
                    "Version": {
                        "Index": 18
                    },
                    "CreatedAt": "2019-10-12T21:07:20.665409697Z",
                    "UpdatedAt": "2019-10-12T21:07:20.666321359Z",
                    "Spec": {
                        "Name": "swarmnginx_default",
                        "Labels": {
                            "com.docker.stack.namespace": "swarmnginx"
                        },
                        "DriverConfiguration": {
                            "Name": "overlay"
                        },
                        "Scope": "swarm"
                    },
                    "DriverState": {
                        "Name": "overlay",
                        "Options": {
                            "com.docker.network.driver.overlay.vxlanid_list": "4097"
                        }
                    },
                    "IPAMOptions": {
                        "Driver": {
                            "Name": "default"
                        },
                        "Configs": [
                            {
                                "Subnet": "10.0.0.0/24",
                                "Gateway": "10.0.0.1"
                            }
                        ]
                    }
                },
                "Addresses": [
                    "10.0.0.20/24"
                ]
            }
        ]
    },
    {
        "ID": "jqfzqzz7uwo9nxazayb24ty4o",
        "Version": {
            "Index": 29
        },
        "CreatedAt": "2019-10-12T21:07:23.102122216Z",
        "UpdatedAt": "2019-10-12T21:07:56.769583051Z",
        "Labels": {},
        "Spec": {
            "ContainerSpec": {
                "Image": "nginx:latest@sha256:aeded0f2a861747f43a01cf1018cf9efe2bdd02afd57d2b11fcc7fcadc16ccd1",
                "Labels": {
                    "com.docker.stack.namespace": "swarmnginx"
                },
                "Privileges": {
                    "CredentialSpec": null,
                    "SELinuxContext": null
                },
                "Isolation": "default"
            },
            "Resources": {},
            "RestartPolicy": {
                "Condition": "on-failure",
                "MaxAttempts": 0
            },
            "Placement": {
                "Platforms": [
                    {
                        "Architecture": "amd64",
                        "OS": "linux"
                    },
                    {
                        "OS": "linux"
                    },
                    {
                        "Architecture": "arm64",
                        "OS": "linux"
                    },
                    {
                        "Architecture": "386",
                        "OS": "linux"
                    },
                    {
                        "Architecture": "ppc64le",
                        "OS": "linux"
                    },
                    {
                        "Architecture": "s390x",
                        "OS": "linux"
                    }
                ]
            },
            "Networks": [
                {
                    "Target": "7qvqlb2to5q77iw3k9mk97o4i",
                    "Aliases": [
                        "nginx"
                    ]
                }
            ],
            "ForceUpdate": 0
        },
        "ServiceID": "kind13dc6ykcr7s5lql7wv064",
        "Slot": 1,
        "NodeID": "nybocmfjabg3wz5yx9cex9oc4",
        "Status": {
            "Timestamp": "2019-10-12T21:07:56.745445979Z",
            "State": "running",
            "Message": "started",
            "ContainerStatus": {
                "ContainerID": "75eefcbfb407a7bffc0b2159d07624714bc0c7697419a4ace40122cd76e556cd",
                "PID": 43724,
                "ExitCode": 0
            },
            "PortStatus": {}
        },
        "DesiredState": "running",
        "NetworksAttachments": [
            {
                "Network": {
                    "ID": "zkw3jhutz9b61bcxwpy7sy7a8",
                    "Version": {
                        "Index": 6
                    },
                    "CreatedAt": "2019-10-12T09:05:05.02856252Z",
                    "UpdatedAt": "2019-10-12T09:05:05.053529433Z",
                    "Spec": {
                        "Name": "ingress",
                        "Labels": {},
                        "DriverConfiguration": {},
                        "Ingress": true,
                        "IPAMOptions": {
                            "Driver": {},
                            "Configs": [
                                {
                                    "Subnet": "10.255.0.0/16",
                                    "Gateway": "10.255.0.1"
                                }
                            ]
                        },
                        "Scope": "swarm"
                    },
                    "DriverState": {
                        "Name": "overlay",
                        "Options": {
                            "com.docker.network.driver.overlay.vxlanid_list": "4096"
                        }
                    },
                    "IPAMOptions": {
                        "Driver": {
                            "Name": "default"
                        },
                        "Configs": [
                            {
                                "Subnet": "10.255.0.0/16",
                                "Gateway": "10.255.0.1"
                            }
                        ]
                    }
                },
                "Addresses": [
                    "10.255.0.5/16"
                ]
            },
            {
                "Network": {
                    "ID": "7qvqlb2to5q77iw3k9mk97o4i",
                    "Version": {
                        "Index": 18
                    },
                    "CreatedAt": "2019-10-12T21:07:20.665409697Z",
                    "UpdatedAt": "2019-10-12T21:07:20.666321359Z",
                    "Spec": {
                        "Name": "swarmnginx_default",
                        "Labels": {
                            "com.docker.stack.namespace": "swarmnginx"
                        },
                        "DriverConfiguration": {
                            "Name": "overlay"
                        },
                        "Scope": "swarm"
                    },
                    "DriverState": {
                        "Name": "overlay",
                        "Options": {
                            "com.docker.network.driver.overlay.vxlanid_list": "4097"
                        }
                    },
                    "IPAMOptions": {
                        "Driver": {
                            "Name": "default"
                        },
                        "Configs": [
                            {
                                "Subnet": "10.0.0.0/24",
                                "Gateway": "10.0.0.1"
                            }
                        ]
                    }
                },
                "Addresses": [
                    "10.0.0.3/24"
                ]
            }
        ]
    }
]`
	w.Write([]byte(rsp))
}

func mockNodes(w http.ResponseWriter, r *http.Request) {
	var l []swarm.Node
	if strings.Contains(r.RequestURI, "p2g1i2vehdevdpwgoqr5wkq5e") {
		l = []swarm.Node{node1}
	} else if strings.Contains(r.RequestURI, "nybocmfjabg3wz5yx9cex9oc4") {
		l = []swarm.Node{node2}
	} else {
		l = []swarm.Node{node1, node2}
	}
	rsp, _ := json.Marshal(l)
	w.Write([]byte(rsp))
}

func TestProvider_buildDeleteMessage(t *testing.T) {
	var pr *Provider
	type args struct {
		svcName string
	}
	tests := []struct {
		name string
		pr   *Provider
		args args
		want comm.Message
	}{
		{"delMe", pr, args{svcName: "del.me"}, comm.Message{Service: comm.Service{Name: "del.me"}, Action: comm.DeleteAction}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.pr, _, _ = startSwarmProvider()
			if got := tt.pr.buildDeleteMessage(tt.args.svcName); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildDeleteMessage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_sliceContainString(t *testing.T) {
	type args struct {
		s  string
		sl []string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{"lower", args{s: "test", sl: []string{"this", "test", "should pass"}}, true},
		{"upper", args{s: "test", sl: []string{"this", "TEST", "should pass"}}, true},
		{"false", args{s: "test", sl: []string{"this", "Tes", "should fail"}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sliceContainString(tt.args.s, tt.args.sl); got != tt.want {
				t.Errorf("sliceContainString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProvider_getFilteredServices(t *testing.T) {
	var pr *Provider
	wantServices := []swarm.Service{testService}
	tests := []struct {
		name         string
		pr           *Provider
		wantServices []swarm.Service
		wantErr      bool
	}{
		{"ok", pr, wantServices, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.pr, _, _ = startSwarmProvider()
			gotServices, err := tt.pr.getFilteredServices()
			if (err != nil) != tt.wantErr {
				t.Errorf("getFilteredServices() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotServices, tt.wantServices) {
				t.Errorf("getFilteredServices() gotServices = %v, want %v", gotServices, tt.wantServices)
			}
		})
	}
}

func TestProvider_getServiceByName(t *testing.T) {
	var pr *Provider
	type args struct {
		svcName string
	}
	tests := []struct {
		name   string
		pr     *Provider
		args   args
		want   swarm.Service
		wantOK bool
	}{
		{"found", pr, args{svcName: "test"}, testService, true},
		{"notFound", pr, args{svcName: "invalid"}, swarm.Service{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.pr, _, _ = startSwarmProvider()
			if got, _ := tt.pr.getServiceByName(tt.args.svcName); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getServiceByName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProvider_getNodeIP(t *testing.T) {
	var pr *Provider
	type args struct {
		nodeID string
	}
	tests := []struct {
		name    string
		sp      *Provider
		args    args
		wantIP  string
		wantErr bool
	}{
		{"10.32.2.2", pr, args{nodeID: "p2g1i2vehdevdpwgoqr5wkq5e"}, "10.32.2.2", false},
		{"error", pr, args{nodeID: "error"}, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.sp, _, _ = startSwarmProvider()
			gotIP, err := tt.sp.getNodeIP(tt.args.nodeID)
			if (err != nil) != tt.wantErr {
				t.Errorf("getNodeIP() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotIP != tt.wantIP {
				t.Errorf("getNodeIP() gotIP = %v, want %v", gotIP, tt.wantIP)
			}
		})
	}
}

func TestProvider_getNodesRunningService(t *testing.T) {
	var pr *Provider
	type args struct {
		svcName string
	}
	tests := []struct {
		name         string
		sp           *Provider
		args         args
		wantNodeList []string
		wantErr      bool
	}{
		{"ok", pr, args{svcName: "test"}, []string{"p2g1i2vehdevdpwgoqr5wkq5e", "nybocmfjabg3wz5yx9cex9oc4"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.sp, _, _ = startSwarmProvider()
			gotNodeList, err := tt.sp.getNodesRunningService(tt.args.svcName)
			if (err != nil) != tt.wantErr {
				t.Errorf("getNodesRunningService() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotNodeList, tt.wantNodeList) {
				t.Errorf("getNodesRunningService() gotNodeList = %v, want %v", gotNodeList, tt.wantNodeList)
			}
		})
	}
}

func TestProvider_buildMessageFromService(t *testing.T) {
	var pr *Provider
	type args struct {
		service swarm.Service
	}
	tests := []struct {
		name    string
		sp      *Provider
		args    args
		want    comm.Message
		wantErr bool
	}{
		{"test", pr, args{service: testService}, msgOK, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.sp, _, _ = startSwarmProvider()
			got, err := tt.sp.buildMessageFromService(tt.args.service)
			if (err != nil) != tt.wantErr {
				t.Errorf("buildMessageFromService() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildMessageFromService() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProvider_RefreshService(t *testing.T) {
	var pr *Provider
	var (
		send chan comm.Message
	)
	msgOKRefresh := comm.Message{
		Action:      comm.RefreshAction,
		Sender:      extensionName,
		Destination: "",
		Error:       "",
		Service: comm.Service{
			Provider:   extensionName,
			Name:       "test",
			Hosts:      []string{"10.32.2.2", "10.32.2.3"},
			Port:       80,
			TLS:        false,
			PublicIP:   "",
			DNSAliases: []string{"test.caas.csnet.me"},
			Info:       "",
			Error:      "",
		},
	}

	msgDelete := comm.Message{
		Action: comm.DeleteAction,
		Service: comm.Service{
			Name: "invalid",
			Port: 80,
		},
	}

	type args struct {
		msg comm.Message
	}
	tests := []struct {
		name string
		sp   *Provider
		args args
		want comm.Message
	}{
		{"refresh", pr, args{msg: msgOKRefresh}, msgOK},
		{"delete", pr, args{msg: msgDelete}, comm.Message{
			Action: comm.DeleteAction,
			Service: comm.Service{
				Name: "invalid",
			},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.sp, _, send = startSwarmProvider()
			go tt.sp.RefreshService(tt.args.msg)
			got := <-send
			go tt.sp.Stop()
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Msg = %v, want %v", got, tt.want)
			}
		})
	}
}

//add test for refresh msg in provider.start
func TestProvider_SendRefreshRequest(t *testing.T) {
	var (
		pr        *Provider
		rec, send chan comm.Message
	)
	msgOKRefresh := comm.Message{
		Action:      comm.RefreshAction,
		Sender:      extensionName,
		Destination: "",
		Error:       "",
		Service: comm.Service{
			Provider:   extensionName,
			Name:       "test",
			Hosts:      []string{"10.32.2.2", "10.32.2.3"},
			Port:       80,
			TLS:        false,
			PublicIP:   "",
			DNSAliases: []string{"test.caas.csnet.me"},
			Info:       "",
			Error:      "",
		},
	}

	type args struct {
		msg comm.Message
	}
	tests := []struct {
		name string
		sp   *Provider
		args args
		want comm.Message
	}{
		{"refresh", pr, args{msg: msgOKRefresh}, msgOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.sp, rec, send = startSwarmProvider()
			//go tt.pr.RefreshService(tt.args.msg)
			rec <- tt.args.msg
			got := <-send
			go tt.sp.Stop()
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Msg = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProvider_poll(t *testing.T) {
	var (
		pr   *Provider
		send chan comm.Message
	)
	tests := []struct {
		name string
		pr   *Provider
		want comm.Message
	}{
		{"poll", pr, msgOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.pr, _, send = startSwarmProvider()
			go tt.pr.poll()
			got := <-send
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Msg = %v, want %v", got, tt.want)
			}
		})
	}
}
