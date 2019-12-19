package f5ltm

import (
	"github.com/interlook/interlook/comm"
	"github.com/scottdware/go-bigip"
	"os"
	"reflect"
	"testing"
)

var (
	f5              BigIP
	testPool        bigip.Pool
	msgOK           comm.Message
	msgUpdate       comm.Message
	msgNew          comm.Message
	msgTLSOK        comm.Message
	msgTLSUpdate    comm.Message
	msgExistingNode comm.Message
	msgNewNodes     comm.Message
	pr              bigip.PolicyRule
	prSSL           bigip.PolicyRule
	prCondition     bigip.PolicyRuleCondition
	prSSLCondition  bigip.PolicyRuleCondition
	prAction        bigip.PolicyRuleAction
	prSSLAction     bigip.PolicyRuleAction
	pmExisting      bigip.PoolMember
	pmNew1          bigip.PoolMember
	pmNew2          bigip.PoolMember
)

func TestMain(m *testing.M) {
	initTests()
	rc := m.Run()
	os.Exit(rc)
}

func TestBigIP_buildPolicyRuleFromMsg(t *testing.T) {

	type args struct {
		msg comm.Message
	}
	tests := []struct {
		name   string
		fields *BigIP
		args   args
		want   bigip.PolicyRule
	}{
		{"http", &f5, args{msg: msgOK}, pr},
		{"tls", &f5, args{msg: msgTLSOK}, prSSL},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := f5.buildPolicyRuleFromMsg(tt.args.msg); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildPolicyRuleFromMsg() = %v, want %v", got, tt.want)
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
		{"needUpdate", &f5, args{&bigip.Pool{FullPath: "~interlook~test"},
			comm.Message{Service: comm.Service{
				Name:       "test",
				Targets:    targetUpdate,
				DNSAliases: []string{"test.caas.csnet.me"},
				TLS:        false,
			}}}, true, false,
		},
		{"noUpdate", &f5, args{&bigip.Pool{FullPath: "~interlook~test"},
			comm.Message{Service: comm.Service{
				Name:       "test",
				Targets:    targetOK,
				DNSAliases: []string{"test.caas.csnet.me"},

				TLS: false,
			}}}, false, false,
		},
		{"error", &f5, args{&bigip.Pool{FullPath: "~interlook~notfound"},
			comm.Message{Service: comm.Service{
				Name:       "test",
				Targets:    targetOK,
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

func TestBigIP_getGlobalPolicyInfo(t *testing.T) {

	type args struct {
		tls bool
	}
	tests := []struct {
		name         string
		fields       *BigIP
		args         args
		wantName     string
		wantFullName string
		wantPath     string
	}{
		{"tls", &f5, args{tls: true}, "interlook_https_policy", "~interlook~Drafts~interlook_https_policy", "/interlook/Drafts/interlook_https_policy"},
		{"http", &f5, args{tls: false}, "interlook_http_policy", "~interlook~Drafts~interlook_http_policy", "/interlook/Drafts/interlook_http_policy"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			gotName, gotFullName, gotPath := f5.getGlobalPolicyInfo(tt.args.tls)
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
		name   string
		fields *BigIP
		args   args
		want   int
	}{
		{"http", &f5, args{msg: msgOK}, 80},
		{"tls", &f5, args{msg: msgTLSOK}, 443},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := f5.getLBPort(tt.args.msg); got != tt.want {
				t.Errorf("getLBPort() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBigIP_newPoolFromService(t *testing.T) {
	type args struct {
		msg comm.Message
	}
	tests := []struct {
		name   string
		fields *BigIP
		args   args
		want   *bigip.Pool
	}{
		{"test", &f5, args{msg: msgOK}, &testPool},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			if got := f5.newPoolFromService(tt.args.msg); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("newPoolFromService() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBigIP_getNodeByAddress(t *testing.T) {

	type args struct {
		address string
	}
	tests := []struct {
		name   string
		fields *BigIP
		args   args
		want   bigip.Node
		want1  bool
	}{
		{"test", &f5, args{address: "10.32.2.2"}, bigip.Node{
			Name:            "10.32.2.2",
			Partition:       "interlook",
			FullPath:        "/interlook/10.32.2.2",
			Generation:      569,
			Address:         "10.32.2.2",
			ConnectionLimit: 0,
			DynamicRatio:    1,
			Logging:         "disabled",
			Monitor:         "default",
			RateLimit:       "disabled",
			Ratio:           1,
			Session:         "user-enabled",
			State:           "unchecked",
			FQDN: struct {
				AddressFamily string `json:"addressFamily,omitempty"`
				AutoPopulate  string `json:"autopopulate,omitempty"`
				DownInterval  int    `json:"downInterval,omitempty"`
				Interval      string `json:"interval,omitempty"`
				Name          string `json:"tmName,omitempty"`
			}{
				AddressFamily: "ipv4",
				AutoPopulate:  "disabled",
				DownInterval:  5,
				Interval:      "3600",
				Name:          "",
			},
		},
			true,
		},
		{"ko", &f5, args{address: "10.32.2.300"}, bigip.Node{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := f5.getNodeByAddress(tt.args.address)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getNodeByAddress() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("getNodeByAddress() got1 = %v, want %v", got1, tt.want1)
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
		fields              *BigIP
		args                args
		wantUpdateNeeded    bool
		wantPolicyRuleExist bool
		wantErr             bool
	}{
		{"httpNeedsUpdate", &f5, args{
			name: "~interlook~interlook_http_policy",
			msg:  msgUpdate,
		}, true, true, false},
		{"httpNoUpdate", &f5, args{
			name: "~interlook~interlook_http_policy",
			msg:  msgOK,
		}, false, true, false},
		{"httpsNeedsUpdate", &f5, args{
			name: "~interlook~interlook_https_policy",
			msg:  msgTLSUpdate,
		}, true, true, false},
		{"httpsNoUpdate", &f5, args{
			name: "~interlook~interlook_https_policy",
			msg:  msgTLSOK,
		}, false, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotUpdateNeeded, gotPolicyRuleExist, err := f5.policyNeedsUpdate(tt.args.name, tt.args.msg)
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

func TestBigIP_buildPoolMembersFromMessage(t *testing.T) {

	type args struct {
		msg comm.Message
	}
	tests := []struct {
		name   string
		fields *BigIP
		args   args
		want   bigip.PoolMembers
	}{
		{"existingNode", &f5, args{msg: msgExistingNode}, bigip.PoolMembers{
			PoolMembers: []bigip.PoolMember{
				pmExisting}}},
		{"newNode", &f5, args{msg: msgNewNodes}, bigip.PoolMembers{
			PoolMembers: []bigip.PoolMember{
				pmNew1, pmNew2}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			//time.Sleep(5*time.Second)
			if got := f5.buildPoolMembersFromMessage(tt.args.msg); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildPoolMembersFromMessage() = %v, want %v", got, tt.want)
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
		fields  *BigIP
		args    args
		wantErr bool
	}{
		//{"updatePool", &f5, args{msg: msgUpdate}, false},
		{"createPool", &f5, args{msg: msgNew}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := f5.upsertPool(tt.args.msg); (err != nil) != tt.wantErr {
				t.Errorf("upsertPool() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
