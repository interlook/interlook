package f5ltm

import (
	"github.com/interlook/interlook/comm"
	"github.com/scottdware/go-bigip"
	"reflect"
	"testing"
)

func TestBigIP_getNodeByAddress(t *testing.T) {
	type args struct {
		address string
	}
	tests := []struct {
		name  string
		f5    *BigIP
		args  args
		want  bigip.Node
		want1 bool
	}{
		{"node1",
			newFakeProvider(),
			args{address: "10.32.2.2"},
			bigip.Node{Name: "10.32.2.2",
				Partition: "interlook",
				Address:   "10.32.2.2",
			},
			true},
		{"ko",
			newFakeProvider(),
			args{address: "10.32.2.300"},
			bigip.Node{},
			false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := tt.f5.getNodeByAddress(tt.args.address)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getNodeByAddress() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("getNodeByAddress() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestBigIP_buildPoolMembersFromMessage(t *testing.T) {
	type args struct {
		msg comm.Message
	}
	tests := []struct {
		name string
		f5   *BigIP
		args args
		want bigip.PoolMembers
	}{
		{"existingNode",
			newFakeProvider(),
			args{msg: comm.Message{Service: comm.Service{
				Name:       "test",
				DNSAliases: []string{"test.caas.csnet.me"},
				Targets: []comm.Target{
					{
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
				TLS: true,
			},
			}}, bigip.PoolMembers{
				PoolMembers: []bigip.PoolMember{
					{
						Name:        "10.32.2.2:30001",
						Description: "Pool Member for test (auto generated - do not edit)",
						Partition:   "interlook",
						Address:     "10.32.2.2",
						Ratio:       2,
						Monitor:     "tcp",
					},
					{
						Name:        "10.32.2.3:30001",
						Description: "Pool Member for test (auto generated - do not edit)",
						Partition:   "interlook",
						Address:     "10.32.2.3",
						Ratio:       1,
						Monitor:     "tcp",
					},
				}}},
		{"newNode", newFakeProvider(), args{msg: comm.Message{Service: comm.Service{
			Name:       "test",
			DNSAliases: []string{"test.caas.csnet.me"},
			Targets: []comm.Target{
				{
					Host:   "10.32.2.50",
					Port:   30001,
					Weight: 2,
				},
				{
					Host:   "10.32.2.51",
					Port:   30001,
					Weight: 1,
				},
			},
			TLS: true,
		}}}, bigip.PoolMembers{
			PoolMembers: []bigip.PoolMember{
				{
					Name:        "10.32.2.50:30001",
					Address:     "10.32.2.50",
					Partition:   "interlook",
					Monitor:     "tcp",
					Description: "Pool Member for test (auto generated - do not edit)",
					Ratio:       2,
				},
				{
					Name:        "10.32.2.51:30001",
					Address:     "10.32.2.51",
					Partition:   "interlook",
					Monitor:     "tcp",
					Description: "Pool Member for test (auto generated - do not edit)",
					Ratio:       1,
				}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			//time.Sleep(5*time.Second)
			if got := tt.f5.buildPoolMembersFromMessage(tt.args.msg); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildPoolMembersFromMessage() got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBigIP_buildPolicyRuleFromMsg(t *testing.T) {
	type args struct {
		msg comm.Message
	}
	tests := []struct {
		name string
		f5   *BigIP
		msg  comm.Message
		want bigip.PolicyRule
	}{
		{"HTTP",
			newFakeProvider(),
			comm.Message{Service: comm.Service{
				Name:       "test",
				DNSAliases: []string{"test.caas.csnet.me"},
				Targets: []comm.Target{
					{
						Host:   "10.32.2.2",
						Port:   30001,
						Weight: 2,
					},
					{
						Host:   "10.32.2.3",
						Port:   30001,
						Weight: 1,
					},
				}}},
			bigip.PolicyRule{
				Name:        "test",
				Description: "ingress rule for test (auto generated - do not edit)",
				Conditions: []bigip.PolicyRuleCondition{{
					Name:            "0",
					CaseInsensitive: true,
					Host:            true,
					HttpHost:        true,
					Request:         true,
					Values:          []string{"test.caas.csnet.me"},
				}},
				Actions: []bigip.PolicyRuleAction{{
					Name:    "0",
					Forward: true,
					Pool:    "/interlook/test",
					Request: true,
				}},
			}},
		{"TLS",
			newFakeProvider(),
			comm.Message{Service: comm.Service{
				Name:       "test",
				DNSAliases: []string{"test.caas.csnet.me"},
				TLS:        true,
				Targets: []comm.Target{
					{
						Host:   "10.32.2.2",
						Port:   30001,
						Weight: 2,
					},
					{
						Host:   "10.32.2.3",
						Port:   30001,
						Weight: 1,
					},
				}}},
			bigip.PolicyRule{
				Name:        "test",
				Description: "ingress rule for test (auto generated - do not edit)",
				Conditions: []bigip.PolicyRuleCondition{{
					Name:            "0",
					CaseInsensitive: true,
					Present:         true,
					ServerName:      true,
					SslClientHello:  true,
					SslExtension:    true,
					Values:          []string{"test.caas.csnet.me"},
				}},
				Actions: []bigip.PolicyRuleAction{{
					Name:           "0",
					Forward:        true,
					Pool:           "/interlook/test",
					SslClientHello: true,
				}},
			}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.f5.buildPolicyRuleFromMsg(tt.msg); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildPolicyRuleFromMsg() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBigIP_policyNeedsUpdate(t *testing.T) {
	type args struct {
		name string
		msg  comm.Message
	}
	tests := []struct {
		name                string
		f5                  *BigIP
		args                args
		wantUpdateNeeded    bool
		wantPolicyRuleExist bool
		wantErr             bool
	}{
		{"httpNeedsUpdate", newFakeProvider(), args{
			name: "~interlook~interlook_http_policy",
			msg: comm.Message{Service: comm.Service{
				Name:       "test",
				DNSAliases: []string{"testko.caas.csnet.me"},
				Targets: []comm.Target{
					{
						Host: "10.32.2.2",
						Port: 30001,
					},
					{
						Host: "10.32.2.99",
						Port: 30001,
					},
				},
				TLS: false,
			}},
		}, true, true, false},
		{"httpNoUpdate", newFakeProvider(), args{
			name: "~interlook~interlook_http_policy",
			msg: comm.Message{Service: comm.Service{
				Name:       "test",
				DNSAliases: []string{"test.caas.csnet.me"},
				Targets: []comm.Target{
					{
						Host: "10.32.2.2",
						Port: 30001,
					},
					{
						Host: "10.32.2.3",
						Port: 30001,
					},
				},
				TLS: false,
			}},
		}, false, true, false},
		{"httpsNeedsUpdate", newFakeProvider(), args{
			name: "~interlook~interlook_https_policy",
			msg: comm.Message{Service: comm.Service{
				Name:       "test",
				DNSAliases: []string{"testko.caas.csnet.me"},
				Targets: []comm.Target{
					{
						Host: "10.32.2.2",
						Port: 30001,
					},
					{
						Host: "10.32.2.3",
						Port: 30001,
					},
				},
				TLS: true,
			}},
		}, true, true, false},
		{"httpsNoUpdate", newFakeProvider(), args{
			name: "~interlook~interlook_https_policy",
			msg: comm.Message{Service: comm.Service{
				Name:       "test",
				DNSAliases: []string{"test.caas.csnet.me"},
				Targets: []comm.Target{
					{
						Host: "10.32.2.2",
						Port: 30001,
					},
					{
						Host: "10.32.2.3",
						Port: 30001,
					},
				},
				TLS: true,
			}},
		}, false, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotUpdateNeeded, gotPolicyRuleExist, err := tt.f5.policyNeedsUpdate(tt.args.name, tt.args.msg)
			if (err != nil) != tt.wantErr {
				t.Errorf("policyNeedsUpdate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotUpdateNeeded != tt.wantUpdateNeeded {
				t.Errorf("policyNeedsUpdate() gotUpdateNeeded = %v, want %v", gotUpdateNeeded, tt.wantUpdateNeeded)
			}
			if gotPolicyRuleExist != tt.wantPolicyRuleExist {
				t.Errorf("policyNeedsUpdate() gotPolicyRuleExist = %v, want %v", gotPolicyRuleExist, tt.wantPolicyRuleExist)
			}
		})
	}
}

func TestBigIP_poolMembersNeedsUpdate(t *testing.T) {
	type args struct {
		pool *bigip.Pool
		msg  comm.Message
	}
	tests := []struct {
		name    string
		f5      *BigIP
		args    args
		want    bool
		wantErr bool
	}{
		{"needUpdate", newFakeProvider(), args{&bigip.Pool{FullPath: "~interlook~test"},
			comm.Message{Service: comm.Service{
				Name: "test",
				Targets: []comm.Target{
					{
						Host: "10.32.2.2",
						Port: 30001,
					},
					{
						Host: "10.32.2.99",
						Port: 30001,
					},
				},
				DNSAliases: []string{"test.caas.csnet.me"},
				TLS:        false,
			}}}, true, false,
		},
		{"noUpdate", newFakeProvider(), args{&bigip.Pool{FullPath: "~interlook~test"},
			comm.Message{Service: comm.Service{
				Name: "test",
				Targets: []comm.Target{
					{
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
				DNSAliases: []string{"test.caas.csnet.me"},

				TLS: false,
			}}}, false, false,
		},
		{"error", newFakeProvider(), args{&bigip.Pool{FullPath: "~interlook~notfound"},
			comm.Message{Service: comm.Service{
				Name: "notfound",
				Targets: []comm.Target{
					{
						Host: "10.32.2.2",
						Port: 30001,
					},
					{
						Host: "10.32.2.3",
						Port: 30001,
					},
				},
				DNSAliases: []string{"test.caas.csnet.me"},

				TLS: false,
			}}}, false, true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.f5.poolMembersNeedsUpdate(tt.args.pool, tt.args.msg)
			if (err != nil) != tt.wantErr {
				t.Errorf("poolMembersNeedsUpdate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("poolMembersNeedsUpdate() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBigIP_newPoolFromService(t *testing.T) {
	type args struct {
		msg comm.Message
	}
	tests := []struct {
		name string
		f5   *BigIP
		args args
		want *bigip.Pool
	}{
		{"test", newFakeProvider(), args{comm.Message{Service: comm.Service{
			Name:       "test",
			DNSAliases: []string{"test.caas.csnet.me"},
			Targets: []comm.Target{
				{
					Host: "10.32.2.2",
					Port: 30001,
				},
				{
					Host: "10.32.2.3",
					Port: 30001,
				},
			},
			TLS: false,
		}}},
			&bigip.Pool{
				Name:        "test",
				Description: "Pool for test " + defaultDescriptionSuffix,
				Partition:   "interlook",
				//LoadBalancingMode: f5.LoadBalancingMode,
				Monitor: "tcp",
			}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			if got := tt.f5.newPoolFromService(tt.args.msg); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("newPoolFromService() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBigIP_getGlobalPolicyInfo(t *testing.T) {

	type args struct {
		tls bool
	}
	tests := []struct {
		name         string
		f5           *BigIP
		args         args
		wantName     string
		wantFullName string
		wantPath     string
	}{
		{"tls", newFakeProvider(), args{tls: true}, "interlook_https_policy", "~interlook~Drafts~interlook_https_policy", "/interlook/Drafts/interlook_https_policy"},
		{"http", newFakeProvider(), args{tls: false}, "interlook_http_policy", "~interlook~Drafts~interlook_http_policy", "/interlook/Drafts/interlook_http_policy"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.f5.GlobalHTTPPolicy = "interlook_http_policy"
			tt.f5.GlobalSSLPolicy = "interlook_https_policy"
			gotName, gotFullName, gotPath := tt.f5.getGlobalPolicyInfo(tt.args.tls)
			if gotName != tt.wantName {
				t.Errorf("getGlobalPolicyInfo() gotName = %v, want %v", gotName, tt.wantName)
			}
			if gotFullName != tt.wantFullName {
				t.Errorf("getGlobalPolicyInfo() gotFullName = %v, want %v", gotFullName, tt.wantFullName)
			}
			if gotPath != tt.wantPath {
				t.Errorf("getGlobalPolicyInfo() gotPath = %v, want %v", gotPath, tt.wantPath)
			}
		})
	}
}

func TestBigIP_getLBPort(t *testing.T) {

	type args struct {
		msg comm.Message
	}
	tests := []struct {
		name string
		f5   *BigIP
		args args
		want int
	}{
		{"http", newFakeProvider(), args{msg: comm.Message{Service: comm.Service{
			Name:       "test",
			DNSAliases: []string{"test.caas.csnet.me"},
			Targets: []comm.Target{
				{
					Host: "10.32.2.2",
					Port: 30001,
				},
				{
					Host: "10.32.2.3",
					Port: 30001,
				},
			},
			TLS: false,
		}}}, 80},
		{"tls", newFakeProvider(), args{msg: comm.Message{Service: comm.Service{
			Name:       "test",
			DNSAliases: []string{"test.caas.csnet.me"},
			Targets: []comm.Target{
				{
					Host: "10.32.2.2",
					Port: 30001,
				},
				{
					Host: "10.32.2.3",
					Port: 30001,
				},
			},
			TLS: true,
		}}}, 443},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.f5.getLBPort(tt.args.msg); got != tt.want {
				t.Errorf("getLBPort() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBigIP_upsertPool(t *testing.T) {
	type args struct {
		msg comm.Message
	}
	tests := []struct {
		name    string
		f5      *BigIP
		args    args
		wantErr bool
	}{
		{"updatePool", newFakeProvider(), args{msg: comm.Message{Service: comm.Service{
			Name:       "test",
			DNSAliases: []string{"testko.caas.csnet.me"},
			Targets: []comm.Target{
				{
					Host: "10.32.2.2",
					Port: 30001,
				},
				{
					Host: "10.32.2.3",
					Port: 30001,
				},
			},
			TLS: false,
		}}}, false},
		{"createPool", newFakeProvider(), args{msg: comm.Message{Service: comm.Service{
			Name:       "test2",
			DNSAliases: []string{"testko.caas.csnet.me"},
			Targets: []comm.Target{
				{
					Host: "10.32.2.2",
					Port: 30001,
				},
				{
					Host: "10.32.2.99",
					Port: 30001,
				},
			},
			TLS: false,
		}}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.f5.upsertPool(tt.args.msg); (err != nil) != tt.wantErr {
				t.Errorf("upsertPool() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
