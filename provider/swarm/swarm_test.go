package swarm

import (
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/interlook/interlook/comm"
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestProvider_buildDeleteMessage(t *testing.T) {
	type fields struct {
		Endpoint         string
		LabelSelector    []string
		TLSCa            string
		TLSCert          string
		TLSKey           string
		PollInterval     time.Duration
		pollTicker       *time.Ticker
		shutdown         chan bool
		send             chan<- comm.Message
		services         []string
		servicesLock     sync.RWMutex
		cli              *client.Client
		serviceFilters   filters.Args
		containerFilters filters.Args
		waitGroup        sync.WaitGroup
	}
	type args struct {
		svcName string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   comm.Message
	}{
		{"delMe", fields{}, args{svcName: "del.me"}, comm.Message{Service: comm.Service{Name: "del.me"}, Action: comm.DeleteAction}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Provider{
				Endpoint:         tt.fields.Endpoint,
				LabelSelector:    tt.fields.LabelSelector,
				TLSCa:            tt.fields.TLSCa,
				TLSCert:          tt.fields.TLSCert,
				TLSKey:           tt.fields.TLSKey,
				PollInterval:     tt.fields.PollInterval,
				pollTicker:       tt.fields.pollTicker,
				shutdown:         tt.fields.shutdown,
				send:             tt.fields.send,
				services:         tt.fields.services,
				servicesLock:     tt.fields.servicesLock,
				cli:              tt.fields.cli,
				serviceFilters:   tt.fields.serviceFilters,
				containerFilters: tt.fields.containerFilters,
				waitGroup:        tt.fields.waitGroup,
			}
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
