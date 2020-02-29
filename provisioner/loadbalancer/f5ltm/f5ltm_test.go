package f5ltm

import (
	"errors"
	"github.com/interlook/interlook/comm"
	"github.com/interlook/interlook/log"
	"github.com/scottdware/go-bigip"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func newFakeProvider() *BigIP {
	return &BigIP{
		Endpoint:                "",
		User:                    "",
		Password:                "",
		AuthProvider:            "",
		HttpPort:                80,
		HttpsPort:               443,
		MonitorName:             "tcp",
		LoadBalancingMode:       "",
		Partition:               "interlook",
		UpdateMode:              "",
		GlobalHTTPPolicy:        "",
		GlobalSSLPolicy:         "",
		ObjectDescriptionSuffix: "(auto generated - do not edit)",
		cli:                     fakeBigIPClient{},
		shutdown:                nil,
		send:                    nil,
		wg:                      sync.WaitGroup{},
	}
}

type fakeBigIPClient struct{}

func (f fakeBigIPClient) AddPool(config *bigip.Pool) error {
	if strings.Contains(config.Name, "error") {
		return errors.New("error")
	}
	return nil
}

func (f fakeBigIPClient) AddRuleToPolicy(policyName string, rule bigip.PolicyRule) error {
	return nil
}

func (f fakeBigIPClient) AddVirtualServer(config *bigip.VirtualServer) error {
	if config.Name == "error" {
		return errors.New("could not create VS")
	}
	return nil
}

func (f fakeBigIPClient) CreateDraftFromPolicy(name string) error {
	return nil
}

func (f fakeBigIPClient) DeletePolicy(name string) error {
	return nil
}

func (f fakeBigIPClient) DeletePool(name string) error {
	if name == "~interlook~delPoolErr" {
		return errors.New("error")
	}
	return nil
}

func (f fakeBigIPClient) DeleteVirtualServer(name string) error {
	if name == "~interlook~delErr" {
		return errors.New("error")
	}
	return nil
}

func (f fakeBigIPClient) GetPolicy(name string) (*bigip.Policy, error) {
	if name == "~interlook~interlook_http_policy" {
		return &bigip.Policy{
			Name:      "interlook_http_policy",
			Partition: "interlook",
			FullPath:  "/interlook/interlook_http_policy",
			Rules: []bigip.PolicyRule{{
				Name:        "test",
				FullPath:    "test",
				Ordinal:     0,
				Description: "",
				Conditions: []bigip.PolicyRuleCondition{{
					Name:            "0",
					CaseInsensitive: true,
					Equals:          true,
					External:        true,
					Host:            true,
					HttpHost:        true,
					Index:           0,
					Present:         true,
					Remote:          true,
					Request:         true,
					Values:          []string{"test.caas.csnet.me"},
				}},
				Actions: []bigip.PolicyRuleAction{{
					Name:    "0",
					Forward: false,
					Pool:    "/interlook/test",
					Request: true,
					Select:  true,
				}},
			}},
		}, nil
	}

	if name == "~interlook~interlook_https_policy" {
		return &bigip.Policy{
			Name:      "interlook_http_policy",
			Partition: "interlook",
			FullPath:  "/interlook/interlook_https_policy",
			Rules: []bigip.PolicyRule{{
				Name:        "test",
				FullPath:    "test",
				Ordinal:     0,
				Description: "",
				Conditions: []bigip.PolicyRuleCondition{{
					Name:            "0",
					CaseInsensitive: true,
					Equals:          true,
					External:        true,
					Index:           0,
					Present:         true,
					Remote:          true,
					ServerName:      true,
					SslClientHello:  true,
					SslExtension:    true,
					Values:          []string{"test.caas.csnet.me"},
				}},
				Actions: []bigip.PolicyRuleAction{{
					Name:           "0",
					Code:           0,
					ExpirySecs:     0,
					Forward:        false,
					Length:         0,
					Offset:         0,
					Pool:           "/interlook/test",
					Port:           0,
					Select:         true,
					SslClientHello: true,
					Status:         0,
					VlanId:         0,
				}},
			}},
		}, nil

	}

	return &bigip.Policy{
		Name:      "",
		Partition: "",
		FullPath:  "",
		Controls:  nil,
		Requires:  nil,
		Strategy:  "",
		Rules:     nil,
	}, nil
}

func (f fakeBigIPClient) GetPool(name string) (*bigip.Pool, error) {
	if name == "~interlook~test" {
		return &bigip.Pool{
			Name:     "test",
			FullPath: "~interlook~test",
			Members: &[]bigip.PoolMember{{
				Name:        "10.32.2.2:30001",
				Description: "",
				FullPath:    "~interlook~10.32.2.2:30001",
				Partition:   "interlook",
				Address:     "10.32.2.2",
				Ratio:       2,
			},
				{
					Name:        "10.32.2.3:30001",
					Description: "",
					FullPath:    "~interlook~10.32.2.3:30001",
					Partition:   "interlook",
					Address:     "10.32.2.3",
					Ratio:       1,
				},
			}}, nil
	}

	return &bigip.Pool{}, nil
}

func (f fakeBigIPClient) GetVirtualServer(name string) (*bigip.VirtualServer, error) {
	if name == "~interlook~test" {
		return &bigip.VirtualServer{
			Name:        "test",
			Partition:   "interlook",
			FullPath:    "~interlook~test",
			Destination: "10.0.0.2",
		}, nil
	}

	if name == "~interlook~error" {
		return nil, errors.New("error")
	}

	return nil, nil
}

func (f fakeBigIPClient) ModifyPolicyRule(policyName, ruleName string, rule bigip.PolicyRule) error {
	return nil
}

func (f fakeBigIPClient) ModifyPool(name string, config *bigip.Pool) error {
	if name == "error" {
		return errors.New("invalid pool")
	}
	return nil
}

func (f fakeBigIPClient) ModifyVirtualServer(name string, config *bigip.VirtualServer) error {
	return nil
}

func (f fakeBigIPClient) Nodes() (*bigip.Nodes, error) {
	return &bigip.Nodes{[]bigip.Node{
		{Name: "10.32.2.2",
			Partition: "interlook",
			Address:   "10.32.2.2",
		},
		{Name: "10.32.2.3",
			Partition: "interlook",
			Address:   "10.32.2.3",
		},
		{Name: "10.32.2.4",
			Partition: "interlook",
			Address:   "10.32.2.4",
		}}}, nil

}

func (f fakeBigIPClient) PoolMembers(name string) (*bigip.PoolMembers, error) {
	if name == "~interlook~test" {
		return &bigip.PoolMembers{[]bigip.PoolMember{{
			Name:        "10.32.2.2:30001",
			Description: "",
			FullPath:    "~interlook~10.32.2.2:30001",
			Partition:   "interlook",
			Address:     "10.32.2.2",
			Ratio:       2,
		},
			{
				Name:        "10.32.2.3:30001",
				Description: "",
				FullPath:    "~interlook~10.32.2.3:30001",
				Partition:   "interlook",
				Address:     "10.32.2.3",
				Ratio:       1,
			},
		}}, nil
	}
	return &bigip.PoolMembers{}, errors.New("not found")
}

func (f fakeBigIPClient) PublishDraftPolicy(name string) error {
	return nil
}

func (f fakeBigIPClient) RefreshTokenSession(interval time.Duration) error {
	return nil
}

func (f fakeBigIPClient) RemoveRuleFromPolicy(ruleName, policyName string) error {
	return nil
}

func (f fakeBigIPClient) UpdatePoolMembers(pool string, pm *[]bigip.PoolMember) error {
	if pool == "error" {
		return errors.New("invalid pool")
	}
	return nil
}

func (p *BigIP) startFake() (rec, send chan comm.Message) {
	rec = make(chan comm.Message)
	send = make(chan comm.Message)

	go func() {
		err := p.Start(rec, send)
		if err != nil {
			log.Errorf("could not start fake provider: %v", err.Error())
		}
	}()
	time.Sleep(300 * time.Millisecond)

	return rec, send
}

func TestBigIP_StartStop(t *testing.T) {
	type args struct {
		receive <-chan comm.Message
		send    chan<- comm.Message
	}
	tests := []struct {
		name string
		f5   *BigIP
		args args
	}{
		{"StartStop",
			newFakeProvider(),
			args{
				receive: make(chan comm.Message),
				send:    make(chan comm.Message),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _ = tt.f5.startFake()
			tt.f5.Stop()
		})
	}
}

func TestBigIP_HandleVSUpdate(t *testing.T) {
	type args struct {
		msg comm.Message
	}
	tests := []struct {
		name string
		f5   *BigIP
		args args
		want comm.Message
	}{
		{"noUpdate", newFakeProvider(), args{msg: comm.Message{
			Action: "add",
			Service: comm.Service{
				Provider: "provider.kubernetes",
				Name:     "test",
				Targets: []comm.Target{{
					Host:   "10.32.2.2",
					Port:   30001,
					Weight: 2,
				},
					{
						Host:   "10.32.2.3",
						Port:   30001,
						Weight: 1,
					}},
				TLS:        false,
				PublicIP:   "10.0.0.2",
				DNSAliases: []string{"test.caas.csnet.me"},
			},
		}},
			comm.Message{
				Action: "add",
				Service: comm.Service{
					Provider: "provider.kubernetes",
					Name:     "test",
					Targets: []comm.Target{{
						Host:   "10.32.2.2",
						Port:   30001,
						Weight: 2,
					},
						{
							Host:   "10.32.2.3",
							Port:   30001,
							Weight: 1,
						}},
					TLS:        false,
					PublicIP:   "10.0.0.2",
					DNSAliases: []string{"test.caas.csnet.me"},
				},
			}},
		{"updateMembers", newFakeProvider(), args{msg: comm.Message{
			Action: "add",
			Service: comm.Service{
				Provider: "provider.kubernetes",
				Name:     "test",
				Targets: []comm.Target{{
					Host:   "10.32.2.2",
					Port:   30001,
					Weight: 2,
				},
					{
						Host:   "10.32.2.99",
						Port:   30001,
						Weight: 1,
					}},
				TLS:        false,
				PublicIP:   "10.0.0.2",
				DNSAliases: []string{"test.caas.csnet.me"},
			},
		}},
			comm.Message{
				Action: "add",
				Service: comm.Service{
					Provider: "provider.kubernetes",
					Name:     "test",
					Targets: []comm.Target{{
						Host:   "10.32.2.2",
						Port:   30001,
						Weight: 2,
					},
						{
							Host:   "10.32.2.99",
							Port:   30001,
							Weight: 1,
						}},
					TLS:        false,
					PublicIP:   "10.0.0.2",
					DNSAliases: []string{"test.caas.csnet.me"},
				},
			}},
		{"updateIP", newFakeProvider(), args{msg: comm.Message{
			Action: "add",
			Service: comm.Service{
				Provider: "provider.kubernetes",
				Name:     "test",
				Targets: []comm.Target{{
					Host:   "10.32.2.2",
					Port:   30001,
					Weight: 2,
				},
					{
						Host:   "10.32.2.3",
						Port:   30001,
						Weight: 1,
					}},
				TLS:        false,
				PublicIP:   "10.0.0.3",
				DNSAliases: []string{"test.caas.csnet.me"},
			},
		}},
			comm.Message{
				Action: "add",
				Service: comm.Service{
					Provider: "provider.kubernetes",
					Name:     "test",
					Targets: []comm.Target{{
						Host:   "10.32.2.2",
						Port:   30001,
						Weight: 2,
					},
						{
							Host:   "10.32.2.3",
							Port:   30001,
							Weight: 1,
						}},
					TLS:        false,
					PublicIP:   "10.0.0.3",
					DNSAliases: []string{"test.caas.csnet.me"},
				},
			}},
		{"error", newFakeProvider(), args{msg: comm.Message{
			Action: "add",
			Service: comm.Service{
				Provider: "provider.kubernetes",
				Name:     "error",
				Targets: []comm.Target{{
					Host:   "10.32.2.2",
					Port:   30001,
					Weight: 2,
				},
					{
						Host:   "10.32.2.3",
						Port:   30001,
						Weight: 1,
					}},
				TLS:        false,
				PublicIP:   "10.0.0.2",
				DNSAliases: []string{"test.caas.csnet.me"},
			},
		}},
			comm.Message{
				Action: "add",
				Service: comm.Service{
					Provider: "provider.kubernetes",
					Name:     "error",
					Targets: []comm.Target{{
						Host:   "10.32.2.2",
						Port:   30001,
						Weight: 2,
					},
						{
							Host:   "10.32.2.3",
							Port:   30001,
							Weight: 1,
						}},
					TLS:        false,
					PublicIP:   "10.0.0.2",
					DNSAliases: []string{"test.caas.csnet.me"},
				},
				Error: "Could not get VS error error",
			}},
		{"new", newFakeProvider(), args{msg: comm.Message{
			Action: "add",
			Service: comm.Service{
				Provider: "provider.kubernetes",
				Name:     "new",
				Targets: []comm.Target{{
					Host:   "10.32.2.2",
					Port:   30001,
					Weight: 2,
				},
					{
						Host:   "10.32.2.3",
						Port:   30001,
						Weight: 1,
					}},
				TLS:        false,
				PublicIP:   "10.0.0.2",
				DNSAliases: []string{"testnew.caas.csnet.me"},
			},
		}},
			comm.Message{
				Action: "add",
				Service: comm.Service{
					Provider: "provider.kubernetes",
					Name:     "new",
					Targets: []comm.Target{{
						Host:   "10.32.2.2",
						Port:   30001,
						Weight: 2,
					},
						{
							Host:   "10.32.2.3",
							Port:   30001,
							Weight: 1,
						}},
					TLS:        false,
					PublicIP:   "10.0.0.2",
					DNSAliases: []string{"testnew.caas.csnet.me"},
				},
			}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.f5.HandleVSUpdate(tt.args.msg); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("HandleVSUpdate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBigIP_Start(t *testing.T) {
	type fields struct {
		Endpoint                string
		User                    string
		Password                string
		AuthProvider            string
		HttpPort                int
		HttpsPort               int
		MonitorName             string
		LoadBalancingMode       string
		Partition               string
		UpdateMode              string
		GlobalHTTPPolicy        string
		GlobalSSLPolicy         string
		ObjectDescriptionSuffix string
		cli                     f5Cli
		shutdown                chan bool
		send                    chan<- comm.Message
		wg                      sync.WaitGroup
	}
	type args struct {
		receive <-chan comm.Message
		send    chan<- comm.Message
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{"errorConnect",
			fields{},
			args{
				receive: make(chan comm.Message),
				send:    make(chan comm.Message),
			},
			true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f5 := &BigIP{
				Endpoint:                tt.fields.Endpoint,
				User:                    tt.fields.User,
				Password:                tt.fields.Password,
				AuthProvider:            tt.fields.AuthProvider,
				HttpPort:                tt.fields.HttpPort,
				HttpsPort:               tt.fields.HttpsPort,
				MonitorName:             tt.fields.MonitorName,
				LoadBalancingMode:       tt.fields.LoadBalancingMode,
				Partition:               tt.fields.Partition,
				UpdateMode:              tt.fields.UpdateMode,
				GlobalHTTPPolicy:        tt.fields.GlobalHTTPPolicy,
				GlobalSSLPolicy:         tt.fields.GlobalSSLPolicy,
				ObjectDescriptionSuffix: tt.fields.ObjectDescriptionSuffix,
				cli:                     tt.fields.cli,
				shutdown:                tt.fields.shutdown,
				send:                    tt.fields.send,
				wg:                      tt.fields.wg,
			}
			//defer f5.Stop()
			if err := f5.Start(tt.args.receive, tt.args.send); (err != nil) != tt.wantErr {
				t.Errorf("Start() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBigIP_createPool(t *testing.T) {
	type fields struct {
		Endpoint                string
		User                    string
		Password                string
		AuthProvider            string
		HttpPort                int
		HttpsPort               int
		MonitorName             string
		LoadBalancingMode       string
		Partition               string
		UpdateMode              string
		GlobalHTTPPolicy        string
		GlobalSSLPolicy         string
		ObjectDescriptionSuffix string
		cli                     f5Cli
		shutdown                chan bool
		send                    chan<- comm.Message
		wg                      sync.WaitGroup
	}
	type args struct {
		msg comm.Message
	}
	tests := []struct {
		name     string
		fields   fields
		args     args
		wantPool *bigip.Pool
		wantErr  bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f5 := &BigIP{
				Endpoint:                tt.fields.Endpoint,
				User:                    tt.fields.User,
				Password:                tt.fields.Password,
				AuthProvider:            tt.fields.AuthProvider,
				HttpPort:                tt.fields.HttpPort,
				HttpsPort:               tt.fields.HttpsPort,
				MonitorName:             tt.fields.MonitorName,
				LoadBalancingMode:       tt.fields.LoadBalancingMode,
				Partition:               tt.fields.Partition,
				UpdateMode:              tt.fields.UpdateMode,
				GlobalHTTPPolicy:        tt.fields.GlobalHTTPPolicy,
				GlobalSSLPolicy:         tt.fields.GlobalSSLPolicy,
				ObjectDescriptionSuffix: tt.fields.ObjectDescriptionSuffix,
				cli:                     tt.fields.cli,
				shutdown:                tt.fields.shutdown,
				send:                    tt.fields.send,
				wg:                      tt.fields.wg,
			}
			gotPool, err := f5.createPool(tt.args.msg)
			if (err != nil) != tt.wantErr {
				t.Errorf("createPool() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotPool, tt.wantPool) {
				t.Errorf("createPool() gotPool = %v, want %v", gotPool, tt.wantPool)
			}
		})
	}
}

func TestBigIP_createVirtualServer(t *testing.T) {
	type args struct {
		msg comm.Message
	}
	tests := []struct {
		name    string
		f5      *BigIP
		args    args
		wantErr bool
	}{
		{"OK", newFakeProvider(), args{msg: comm.Message{}}, false},
		{"Error", newFakeProvider(), args{msg: comm.Message{Service: comm.Service{Name: "error"}}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.f5.createVirtualServer(tt.args.msg); (err != nil) != tt.wantErr {
				t.Errorf("createVirtualServer() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBigIP_handleDelete(t *testing.T) {
	type fields struct {
		Endpoint                string
		User                    string
		Password                string
		AuthProvider            string
		HttpPort                int
		HttpsPort               int
		MonitorName             string
		LoadBalancingMode       string
		Partition               string
		UpdateMode              string
		GlobalHTTPPolicy        string
		GlobalSSLPolicy         string
		ObjectDescriptionSuffix string
		cli                     f5Cli
		shutdown                chan bool
		send                    chan<- comm.Message
		wg                      sync.WaitGroup
	}
	type args struct {
		msg comm.Message
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   comm.Message
	}{
		{"deleteOK",
			fields{
				HttpPort:    80,
				HttpsPort:   443,
				MonitorName: "tcp",
				Partition:   "interlook",
				UpdateMode:  vsUpdateMode,
				cli:         newFakeProvider().cli,
			},
			args{msg: comm.Message{
				Action: "delete",
				Service: comm.Service{
					Provider: "",
					Name:     "",
					Targets:  nil,
				},
			}},
			comm.Message{
				Action: "update",
				Service: comm.Service{
					Provider: "",
					Name:     "",
					Targets:  nil,
				}}},
		{"GlobalPolicy",
			fields{
				HttpPort:         80,
				HttpsPort:        443,
				MonitorName:      "tcp",
				Partition:        "interlook",
				UpdateMode:       policyUpdateMode,
				GlobalHTTPPolicy: "~interlook~interlook_http_policy",
				cli:              newFakeProvider().cli,
			},
			args{msg: comm.Message{
				Action: "delete",
				Service: comm.Service{
					Provider: "",
					Name:     "",
					Targets:  nil,
				},
			}},
			comm.Message{
				Action: "delete",
				Service: comm.Service{
					Provider: "",
					Name:     "",
					Targets:  nil,
				}}},
		{"unhandled",
			fields{
				HttpPort:    80,
				HttpsPort:   443,
				MonitorName: "tcp",
				Partition:   "interlook",
				UpdateMode:  "unhandled",
				cli:         newFakeProvider().cli,
			},
			args{msg: comm.Message{
				Action: "unhandled",
				Service: comm.Service{
					Provider: "",
					Name:     "",
					Targets:  nil,
				},
			}},
			comm.Message{
				Action: "unhandled",
				Error:  "unsupported updateMode unhandled",
				Service: comm.Service{
					Provider: "",
					Name:     "",
					Targets:  nil,
				}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f5 := &BigIP{
				Endpoint:                tt.fields.Endpoint,
				User:                    tt.fields.User,
				Password:                tt.fields.Password,
				AuthProvider:            tt.fields.AuthProvider,
				HttpPort:                tt.fields.HttpPort,
				HttpsPort:               tt.fields.HttpsPort,
				MonitorName:             tt.fields.MonitorName,
				LoadBalancingMode:       tt.fields.LoadBalancingMode,
				Partition:               tt.fields.Partition,
				UpdateMode:              tt.fields.UpdateMode,
				GlobalHTTPPolicy:        tt.fields.GlobalHTTPPolicy,
				GlobalSSLPolicy:         tt.fields.GlobalSSLPolicy,
				ObjectDescriptionSuffix: tt.fields.ObjectDescriptionSuffix,
				cli:                     tt.fields.cli,
				shutdown:                tt.fields.shutdown,
				send:                    tt.fields.send,
				wg:                      tt.fields.wg,
			}
			if got := f5.handleDelete(tt.args.msg); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("handleDelete() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBigIP_handleGlobalPolicyDelete(t *testing.T) {
	type args struct {
		msg comm.Message
	}
	tests := []struct {
		name string
		f5   *BigIP
		args args
		want comm.Message
	}{
		{"Update", newFakeProvider(), args{msg: comm.Message{
			Action: "add",
			Sender: "dummy",
			Service: comm.Service{
				Provider: "kubernetes.provider",
				Name:     "test",
				Targets: []comm.Target{{
					Host:   "10.32.2.2",
					Port:   30001,
					Weight: 2,
				},
					{
						Host:   "10.32.2.99",
						Port:   30001,
						Weight: 1,
					},
				},
			},
		}},
			comm.Message{
				Action: "add",
				Sender: "dummy",
				Service: comm.Service{
					Provider: "kubernetes.provider",
					Name:     "test",
					Targets: []comm.Target{{
						Host:   "10.32.2.2",
						Port:   30001,
						Weight: 2,
					},
						{
							Host:   "10.32.2.99",
							Port:   30001,
							Weight: 1,
						},
					},
				}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.f5.handleGlobalPolicyDelete(tt.args.msg); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("handleGlobalPolicyDelete() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBigIP_handleGlobalPolicyUpdate(t *testing.T) {
	type args struct {
		msg comm.Message
	}
	tests := []struct {
		name string
		f5   *BigIP
		args args
		want comm.Message
	}{
		{"NoUpdate", newFakeProvider(), args{msg: comm.Message{
			Action: "add",
			Sender: "dummy",
			Service: comm.Service{
				Provider: "kubernetes.provider",
				Name:     "test",
				Targets: []comm.Target{{
					Host:   "10.32.2.2",
					Port:   30001,
					Weight: 2,
				},
					{
						Host:   "10.32.2.3",
						Port:   30001,
						Weight: 1,
					},
				},
			},
		}},
			comm.Message{
				Action: "add",
				Sender: "dummy",
				Service: comm.Service{
					Provider: "kubernetes.provider",
					Name:     "test",
					Targets: []comm.Target{{
						Host:   "10.32.2.2",
						Port:   30001,
						Weight: 2,
					},
						{
							Host:   "10.32.2.3",
							Port:   30001,
							Weight: 1,
						},
					},
				}}},
		{"UpsertError", newFakeProvider(), args{msg: comm.Message{
			Action: "add",
			Sender: "dummy",
			Service: comm.Service{
				Provider: "kubernetes.provider",
				Name:     "error",
				Targets: []comm.Target{{
					Host:   "10.32.2.2",
					Port:   30001,
					Weight: 2,
				},
					{
						Host:   "10.32.2.3",
						Port:   30001,
						Weight: 1,
					},
				},
			},
		}},
			comm.Message{
				Action: "add",
				Sender: "dummy",
				Error:  "Could not get members of Pool  not found",
				Service: comm.Service{
					Provider: "kubernetes.provider",
					Name:     "error",
					Targets: []comm.Target{{
						Host:   "10.32.2.2",
						Port:   30001,
						Weight: 2,
					},
						{
							Host:   "10.32.2.3",
							Port:   30001,
							Weight: 1,
						},
					},
				}}},
		{"Update", newFakeProvider(), args{msg: comm.Message{
			Action: "add",
			Sender: "dummy",
			Service: comm.Service{
				Provider: "kubernetes.provider",
				Name:     "test",
				Targets: []comm.Target{{
					Host:   "10.32.2.2",
					Port:   30001,
					Weight: 2,
				},
					{
						Host:   "10.32.2.4",
						Port:   30001,
						Weight: 1,
					},
				},
			},
		}},
			comm.Message{
				Action: "add",
				Sender: "dummy",
				Service: comm.Service{
					Provider: "kubernetes.provider",
					Name:     "test",
					Targets: []comm.Target{{
						Host:   "10.32.2.2",
						Port:   30001,
						Weight: 2,
					},
						{
							Host:   "10.32.2.4",
							Port:   30001,
							Weight: 1,
						},
					},
				}}},
		{"UpdatePolicy", &BigIP{
			HttpPort:         80,
			HttpsPort:        443,
			Partition:        "interlook",
			UpdateMode:       policyUpdateMode,
			GlobalHTTPPolicy: "interlook_http_policy",
			cli:              newFakeProvider().cli,
		}, args{msg: comm.Message{
			Action: "add",
			Sender: "dummy",
			Service: comm.Service{
				Name: "test",
				Targets: []comm.Target{{
					Host:   "10.32.2.2",
					Port:   30001,
					Weight: 2,
				},
					{
						Host:   "10.32.2.99",
						Port:   30001,
						Weight: 1,
					},
				},
				DNSAliases: []string{"update.csnet.me"},
			},
		}},
			comm.Message{
				Action: "add",
				Sender: "dummy",
				Service: comm.Service{
					Name:       "test",
					DNSAliases: []string{"update.csnet.me"},
					Targets: []comm.Target{{
						Host:   "10.32.2.2",
						Port:   30001,
						Weight: 2,
					},
						{
							Host:   "10.32.2.99",
							Port:   30001,
							Weight: 1,
						},
					},
				}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.f5.handleGlobalPolicyUpdate(tt.args.msg); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("handleGlobalPolicyUpdate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBigIP_handleUpdate(t *testing.T) {
	type args struct {
		msg comm.Message
	}
	tests := []struct {
		name string
		f5   *BigIP
		args args
		want comm.Message
	}{
		{"VSUpdate", &BigIP{
			HttpPort:          80,
			HttpsPort:         443,
			MonitorName:       "tcp",
			LoadBalancingMode: leastConnectionLBMode,
			Partition:         "interlook",
			UpdateMode:        vsUpdateMode,
			cli:               newFakeProvider().cli,
		},
			args{msg: comm.Message{
				Service: comm.Service{},
			}},
			comm.Message{
				Service: comm.Service{},
			}},
		{"Unhandled", &BigIP{
			HttpPort:          80,
			HttpsPort:         443,
			MonitorName:       "tcp",
			LoadBalancingMode: leastConnectionLBMode,
			Partition:         "interlook",
			UpdateMode:        "unsupported",
			GlobalHTTPPolicy:  "~interlook~interlook_http_policy",
			cli:               newFakeProvider().cli,
		},
			args{msg: comm.Message{
				Service: comm.Service{},
			}},
			comm.Message{
				Error:   "unsupported updateMode unsupported",
				Service: comm.Service{},
			}},
		{"GlobalPolicyUpdate", &BigIP{
			HttpPort:          80,
			HttpsPort:         443,
			MonitorName:       "tcp",
			LoadBalancingMode: leastConnectionLBMode,
			Partition:         "interlook",
			UpdateMode:        policyUpdateMode,
			GlobalHTTPPolicy:  "~interlook~interlook_http_policy",
			cli:               newFakeProvider().cli,
		},
			args{msg: comm.Message{
				Service: comm.Service{
					Name: "test",
					Targets: []comm.Target{{
						Host:   "10.32.2.2",
						Port:   30001,
						Weight: 2,
					},
						{
							Host:   "10.32.2.3",
							Port:   30001,
							Weight: 1,
						}},
				},
			}},
			comm.Message{
				Service: comm.Service{
					Name: "test",
					Targets: []comm.Target{{
						Host:   "10.32.2.2",
						Port:   30001,
						Weight: 2,
					},
						{
							Host:   "10.32.2.3",
							Port:   30001,
							Weight: 1,
						}}},
			}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _ = tt.f5.startFake()
			if got := tt.f5.handleUpdate(tt.args.msg); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("handleUpdate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBigIP_handleVSDelete(t *testing.T) {

	type args struct {
		msg comm.Message
	}
	tests := []struct {
		name string
		f5   *BigIP
		args args
		want comm.Message
	}{
		{"OK", newFakeProvider(), args{msg: comm.Message{
			Action: "delete",
			Service: comm.Service{
				Provider: "dummy",
				Name:     "test",
				Targets: []comm.Target{{
					Host:   "10.32.2.2",
					Port:   30001,
					Weight: 2,
				},
					{
						Host:   "10.32.2.3",
						Port:   30001,
						Weight: 1,
					}},
			},
		}}, comm.Message{
			Action: "update",
			Service: comm.Service{
				Provider: "dummy",
				Name:     "test",
				Targets: []comm.Target{{
					Host:   "10.32.2.2",
					Port:   30001,
					Weight: 2,
				},
					{
						Host:   "10.32.2.3",
						Port:   30001,
						Weight: 1,
					}},
				TLS:        false,
				PublicIP:   "",
				DNSAliases: nil,
			},
		}},
		{"errDeleteVS", newFakeProvider(), args{msg: comm.Message{
			Action: "delete",
			Service: comm.Service{
				Provider: "dummy",
				Name:     "delErr",
				Targets: []comm.Target{{
					Host:   "10.32.2.2",
					Port:   30001,
					Weight: 2,
				},
					{
						Host:   "10.32.2.3",
						Port:   30001,
						Weight: 1,
					}},
			},
		}}, comm.Message{
			Action: "update",
			Error:  "error",
			Service: comm.Service{
				Provider: "dummy",
				Name:     "delErr",
				Targets: []comm.Target{{
					Host:   "10.32.2.2",
					Port:   30001,
					Weight: 2,
				},
					{
						Host:   "10.32.2.3",
						Port:   30001,
						Weight: 1,
					}},
				TLS:        false,
				PublicIP:   "",
				DNSAliases: nil,
			},
		}},
		{"errDeleteVS", newFakeProvider(), args{msg: comm.Message{
			Action: "delete",
			Service: comm.Service{
				Provider: "dummy",
				Name:     "delPoolErr",
				Targets: []comm.Target{{
					Host:   "10.32.2.2",
					Port:   30001,
					Weight: 2,
				},
					{
						Host:   "10.32.2.3",
						Port:   30001,
						Weight: 1,
					}},
			},
		}}, comm.Message{
			Action: "update",
			Error:  "error",
			Service: comm.Service{
				Provider: "dummy",
				Name:     "delPoolErr",
				Targets: []comm.Target{{
					Host:   "10.32.2.2",
					Port:   30001,
					Weight: 2,
				},
					{
						Host:   "10.32.2.3",
						Port:   30001,
						Weight: 1,
					}},
				TLS:        false,
				PublicIP:   "",
				DNSAliases: nil,
			},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.f5.handleVSDelete(tt.args.msg); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("handleVSDelete() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBigIP_initialize1(t *testing.T) {
	var err error
	type fields struct {
		Endpoint                string
		User                    string
		Password                string
		AuthProvider            string
		HttpPort                int
		HttpsPort               int
		MonitorName             string
		LoadBalancingMode       string
		Partition               string
		UpdateMode              string
		GlobalHTTPPolicy        string
		GlobalSSLPolicy         string
		ObjectDescriptionSuffix string
		cli                     f5Cli
		shutdown                chan bool
		send                    chan<- comm.Message
		wg                      sync.WaitGroup
	}
	tests := []struct {
		name    string
		fields  fields
		want    BigIP
		wantErr bool
	}{
		{"Ok",
			fields{
				cli:  newFakeProvider().cli,
				User: "dummy",
			},
			BigIP{
				User:                    "dummy",
				AuthProvider:            tmosAuthProvider,
				HttpPort:                80,
				HttpsPort:               443,
				LoadBalancingMode:       leastConnectionLBMode,
				ObjectDescriptionSuffix: "",
				cli:                     newFakeProvider().cli,
				shutdown:                make(chan bool),
				wg:                      sync.WaitGroup{},
			},
			false},
		{"Error",
			fields{},
			BigIP{},
			true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f5 := &BigIP{
				Endpoint:                tt.fields.Endpoint,
				User:                    tt.fields.User,
				Password:                tt.fields.Password,
				AuthProvider:            tt.fields.AuthProvider,
				HttpPort:                tt.fields.HttpPort,
				HttpsPort:               tt.fields.HttpsPort,
				MonitorName:             tt.fields.MonitorName,
				LoadBalancingMode:       tt.fields.LoadBalancingMode,
				Partition:               tt.fields.Partition,
				UpdateMode:              tt.fields.UpdateMode,
				GlobalHTTPPolicy:        tt.fields.GlobalHTTPPolicy,
				GlobalSSLPolicy:         tt.fields.GlobalSSLPolicy,
				ObjectDescriptionSuffix: tt.fields.ObjectDescriptionSuffix,
				cli:                     tt.fields.cli,
				shutdown:                tt.fields.shutdown,
				send:                    tt.fields.send,
				wg:                      tt.fields.wg,
			}
			if err = f5.initialize(); (err != nil) != tt.wantErr {
				t.Errorf("initialize() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				if reflect.DeepEqual(f5, tt.want) {
					t.Errorf("f5 not as expected: got %v, wanted %v", f5, tt.want)
				}

			}
		})
	}
}
