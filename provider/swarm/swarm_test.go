package swarm

import (
	"encoding/json"
	"strings"

	//"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"github.com/interlook/interlook/comm"
	"github.com/interlook/interlook/log"
	"net/http"
	"os"
	"reflect"
	"testing"
	"time"
)

var (
	p           *Provider
	testService swarm.Service
	node1       swarm.Node
	node2       swarm.Node
)

func TestMain(m *testing.M) {
	mockSwarmAPI()
	initTests()
	rc := m.Run()
	os.Exit(rc)
}

func initTests() {
	var err error
	p = &Provider{
		Endpoint:      "http://localhost:2376",
		LabelSelector: []string{"l7aas=true"},
		TLSCa:         "",
		TLSCert:       "",
		TLSKey:        "",
		PollInterval:  30,
	}
	_ = p.init()

	p.cli, err = client.NewClientWithOpts(client.WithHost(p.Endpoint))
	if err != nil {
		log.Fatal(err.Error())
	}
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
                    "TargetPort": 6379,
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
                    "TargetPort": 6379,
                    "PublishedPort": 30001
                }
            ]
        },
        "Ports": [
            {
                "Protocol": "tcp",
                "TargetPort": 6379,
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
    "ID": "10.32.2.2",
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
    "ID": "10.32.2.3",
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

func mockSwarmAPI() {
	go http.ListenAndServe(":2376", nil)
	time.Sleep(1 * time.Second)
	http.HandleFunc("/v1.40/services", mockServiceList)
	http.HandleFunc("/v1.40/nodes", mockNodes)
}

func mockServiceList(w http.ResponseWriter, r *http.Request) {
	l := []swarm.Service{testService}
	rsp, _ := json.Marshal(l)
	w.Write([]byte(rsp))
}

func mockNodes(w http.ResponseWriter, r *http.Request) {
	var l []swarm.Node
	if strings.Contains(r.RequestURI, "10.32.2.2") {
		l = []swarm.Node{node1}
	} else if strings.Contains(r.RequestURI, "10.32.2.3") {
		l = []swarm.Node{node2}
	} else {
		l = []swarm.Node{node1, node2}
	}
	rsp, _ := json.Marshal(l)
	w.Write([]byte(rsp))
}

func TestProvider_buildDeleteMessage(t *testing.T) {

	type args struct {
		svcName string
	}
	tests := []struct {
		name   string
		fields *Provider
		args   args
		want   comm.Message
	}{
		{"delMe", p, args{svcName: "del.me"}, comm.Message{Service: comm.Service{Name: "del.me"}, Action: comm.DeleteAction}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := p.buildDeleteMessage(tt.args.svcName); !reflect.DeepEqual(got, tt.want) {
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
	wantServices := []swarm.Service{testService}
	tests := []struct {
		name         string
		fields       *Provider
		wantServices []swarm.Service
		wantErr      bool
	}{
		{"ok", p, wantServices, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotServices, err := p.getFilteredServices()
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
	type args struct {
		svcName string
	}
	tests := []struct {
		name   string
		fields *Provider
		args   args
		want   swarm.Service
	}{
		{"found", p, args{svcName: "test"}, testService},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := p.getServiceByName(tt.args.svcName); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getServiceByName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProvider_getNodeIP(t *testing.T) {
	type args struct {
		nodeID string
	}
	tests := []struct {
		name    string
		fields  *Provider
		args    args
		wantIP  string
		wantErr bool
	}{
		{"10.32.2.2", p, args{nodeID: "10.32.2.2"}, "10.32.2.2", false},
		{"error", p, args{nodeID: "error"}, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			gotIP, err := p.getNodeIP(tt.args.nodeID)
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
