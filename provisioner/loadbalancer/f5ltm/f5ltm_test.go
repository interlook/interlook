package f5ltm

import (
	"github.com/interlook/interlook/comm"
	"github.com/scottdware/go-bigip"
	"sync"
	"testing"
)

func TestBigIP_updatePoolMembers(t *testing.T) {
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
		cli                     *bigip.BigIP
		shutdown                chan bool
		send                    chan<- comm.Message
		wg                      sync.WaitGroup
	}
	type args struct {
		pool *bigip.Pool
		msg  comm.Message
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
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
			if err := f5.updatePoolMembers(tt.args.pool, tt.args.msg); (err != nil) != tt.wantErr {
				t.Errorf("updatePoolMembers() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
