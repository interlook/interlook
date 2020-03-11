package swarm

import (
	"context"
	"errors"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
	"github.com/interlook/interlook/comm"
	"github.com/interlook/interlook/log"
	"net/http"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"
)

var (
	testService        swarm.Service
	msgOK              comm.Message
	targetOK           []comm.Target
	targetUpd          []comm.Target
	targetInvalid      []comm.Target
	targetHostOnly     []comm.Target
	servicePubHostOnly []servicePublishConfig
)

type fakeClient struct {
	scheme            string
	host              string
	proto             string
	addr              string
	basePath          string
	client            *http.Client
	version           string
	customHTTPHeaders map[string]string
	manualOverride    bool
	negotiateVersion  bool
	negotiated        bool
}

func newFakeProvider() Provider {
	p := Provider{
		PollInterval: 10,
		cli:          newFakeClient(),
	}

	p.init()

	return p
}

func newFakeClient() *fakeClient {
	return &fakeClient{}
}

func (f *fakeClient) ServiceList(ctx context.Context, options types.ServiceListOptions) ([]swarm.Service, error) {
	svc := options.Filters.Get("name")
	if len(svc) > 0 {
		if svc[0] == "invalid" {
			return []swarm.Service{swarm.Service{}}, errors.New("not found")
		}
	}
	return []swarm.Service{testService}, nil
}

func (f *fakeClient) TaskList(ctx context.Context, options types.TaskListOptions) ([]swarm.Task, error) {
	node1Task := swarm.Task{
		ID:   "testServiceNode1Task",
		Meta: swarm.Meta{},
		Annotations: swarm.Annotations{
			Name:   "",
			Labels: map[string]string{"interlook.port": "80", "interlook.hosts": "test.caas.csnet.me", "l7aas": "true", "interlook.ssl": "false"},
		},
		Spec:      swarm.TaskSpec{},
		ServiceID: "testService",
		Slot:      0,
		NodeID:    "node1",
		Status: swarm.TaskStatus{
			Timestamp:       time.Time{},
			State:           "running",
			Message:         "",
			Err:             "",
			ContainerStatus: nil,
			PortStatus:      swarm.PortStatus{},
		},
		DesiredState:        "running",
		NetworksAttachments: nil,
		GenericResources:    nil,
	}
	node2Task := swarm.Task{
		ID:   "testServiceNode1Task",
		Meta: swarm.Meta{},
		Annotations: swarm.Annotations{
			Name:   "",
			Labels: map[string]string{"interlook.port": "80", "interlook.hosts": "test.caas.csnet.me", "l7aas": "true", "interlook.ssl": "false"},
		},
		Spec:      swarm.TaskSpec{},
		ServiceID: "testService",
		Slot:      0,
		NodeID:    "node2",
		Status: swarm.TaskStatus{
			Timestamp:       time.Time{},
			State:           "running",
			Message:         "",
			Err:             "",
			ContainerStatus: nil,
			PortStatus:      swarm.PortStatus{},
		},
		DesiredState:        "running",
		NetworksAttachments: nil,
		GenericResources:    nil,
	}
	return []swarm.Task{node1Task, node2Task}, nil
}

func (f *fakeClient) NodeList(ctx context.Context, options types.NodeListOptions) ([]swarm.Node, error) {
	node1 := swarm.Node{
		ID:          "node1",
		Meta:        swarm.Meta{},
		Spec:        swarm.NodeSpec{},
		Description: swarm.NodeDescription{},
		Status: swarm.NodeStatus{
			State:   "ready",
			Message: "",
			Addr:    "10.32.2.2",
		},
		ManagerStatus: nil,
	}

	node2 := swarm.Node{
		ID:          "node2",
		Meta:        swarm.Meta{},
		Spec:        swarm.NodeSpec{},
		Description: swarm.NodeDescription{},
		Status: swarm.NodeStatus{
			State:   "ready",
			Message: "",
			Addr:    "10.32.2.3",
		},
		ManagerStatus: nil,
	}

	o := options.Filters.Get("id")
	var res []swarm.Node

	for _, v := range o {
		if v == "node1" {
			res = append(res, node1)
		}
		if v == "node2" {
			res = append(res, node2)
		}
	}

	return res, nil
}

func TestMain(m *testing.M) {
	initTestVars()
	rc := m.Run()
	os.Exit(rc)
}

// startFakeProvider returns a "running" swarm provider instance
func (p *Provider) startFakeProvider() (rec, send chan comm.Message) {
	rec = make(chan comm.Message)
	send = make(chan comm.Message)

	go func() {
		err := p.Start(rec, send)
		if err != nil {
			log.Errorf("could not start fake provider: %v", err.Error())
		}
	}()
	// sleep so that provider is ready to receive data on channel
	time.Sleep(400 * time.Millisecond)
	return rec, send
}

// initialize test variables
func initTestVars() {

	servicePubHostOnly = append(servicePubHostOnly, servicePublishConfig{
		ip:         "10.32.2.2",
		portConfig: swarm.PortConfig{},
	})
	servicePubHostOnly = append(servicePubHostOnly, servicePublishConfig{
		ip:         "10.32.2.3",
		portConfig: swarm.PortConfig{},
	})

	targetOK = append(targetOK, comm.Target{
		Host: "10.32.2.2",
		Port: 30001,
	})
	targetOK = append(targetOK, comm.Target{
		Host: "10.32.2.3",
		Port: 30001,
	})

	targetUpd = append(targetUpd, comm.Target{
		Host: "10.32.2.2",
		Port: 80,
	})
	targetUpd = append(targetUpd, comm.Target{
		Host: "10.32.2.3",
		Port: 80,
	})
	targetInvalid = append(targetInvalid, comm.Target{
		Host: "invalid",
		Port: 80,
	})

	targetHostOnly = append(targetHostOnly, comm.Target{
		Host: "10.32.2.2",
	})
	targetHostOnly = append(targetHostOnly, comm.Target{
		Host: "10.32.2.3",
	})

	msgOK = comm.Message{Service: comm.Service{
		Name:       "testService",
		DNSAliases: []string{"test.caas.csnet.me"},
		Targets:    targetOK,
		TLS:        false,
		Provider:   extensionName,
	},
		Action: comm.AddAction}

	testService = swarm.Service{
		ID:   "testService",
		Meta: swarm.Meta{},
		Spec: swarm.ServiceSpec{
			Annotations: swarm.Annotations{
				Name: "testService",
				Labels: map[string]string{
					"interlook.ssl":   "false",
					"interlook.port":  "80",
					"interlook.hosts": "test.caas.csnet.me",
					"l7aas":           "false"},
			},
			TaskTemplate:   swarm.TaskSpec{},
			Mode:           swarm.ServiceMode{},
			UpdateConfig:   nil,
			RollbackConfig: nil,
			Networks:       nil,
			EndpointSpec:   nil,
		},
		PreviousSpec: nil,
		Endpoint: swarm.Endpoint{
			Spec: swarm.EndpointSpec{
				Mode: "vip",
				Ports: []swarm.PortConfig{{
					Protocol:      "tcp",
					TargetPort:    80,
					PublishedPort: 30001,
					PublishMode:   "ingress",
				}},
			},
			Ports: []swarm.PortConfig{{
				Protocol:      "tcp",
				TargetPort:    80,
				PublishedPort: 30001,
				PublishMode:   "ingress",
			}},
			VirtualIPs: nil,
		},
		UpdateStatus: nil,
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

func TestProvider_getFilteredServicesNew(t *testing.T) {

	wantServices := []swarm.Service{testService}
	tests := []struct {
		name         string
		pr           Provider
		wantServices []swarm.Service
		wantErr      bool
	}{
		{"ok", newFakeProvider(), wantServices, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
	type args struct {
		svcName string
	}
	tests := []struct {
		name   string
		pr     Provider
		args   args
		want   swarm.Service
		wantOK bool
	}{
		{"found", newFakeProvider(), args{svcName: "test"}, testService, true},
		{"notFound", newFakeProvider(), args{svcName: "invalid"}, swarm.Service{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, _ := tt.pr.getServiceByName(tt.args.svcName); !reflect.DeepEqual(got, tt.want) {
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
		sp      Provider
		args    args
		wantIP  string
		wantErr bool
	}{
		{"10.32.2.2", newFakeProvider(), args{nodeID: "node1"}, "10.32.2.2", false},
		{"error", newFakeProvider(), args{nodeID: "error"}, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
	//var pr *Provider
	type args struct {
		svcName string
	}
	tests := []struct {
		name         string
		sp           Provider
		args         args
		wantNodeList []servicePublishConfig
		wantErr      bool
	}{
		{"ok",
			newFakeProvider(),
			args{svcName: "test"},
			servicePubHostOnly,
			false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotNodeList, err := tt.sp.getTaskPublishInfo(tt.args.svcName)
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
	//var pr *Provider
	type args struct {
		service swarm.Service
	}
	tests := []struct {
		name    string
		sp      Provider
		args    args
		want    comm.Message
		wantErr bool
	}{
		{"test", newFakeProvider(), args{service: testService}, msgOK, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
	//var pr *Provider
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
			Targets:    targetInvalid,
			TLS:        false,
			PublicIP:   "",
			DNSAliases: []string{"test.caas.csnet.me"},
		},
	}

	msgDelete := comm.Message{
		Action: comm.DeleteAction,
		Service: comm.Service{
			Name: "invalid",
			//Port: 80,
		},
	}

	type args struct {
		msg comm.Message
	}
	tests := []struct {
		name string
		sp   Provider
		args args
		want comm.Message
	}{
		{"refresh", newFakeProvider(), args{msg: msgOKRefresh}, msgOK},
		{"delete", newFakeProvider(), args{msg: msgDelete}, comm.Message{
			Action: comm.DeleteAction,
			Service: comm.Service{
				Name: "invalid",
			},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.sp.PollInterval = 10 * time.Second
			_, send = tt.sp.startFakeProvider()
			//time.Sleep(300 *time.Millisecond)
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
		//pr        *Provider
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
			Targets:    targetUpd,
			TLS:        false,
			PublicIP:   "",
			DNSAliases: []string{"test.caas.csnet.me"},
		},
	}

	type args struct {
		msg comm.Message
	}
	tests := []struct {
		name string
		sp   Provider
		args args
		want comm.Message
	}{
		{"refreshOK", newFakeProvider(), args{msg: msgOKRefresh}, msgOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.sp.PollInterval = 3 * time.Second
			tt.sp.LabelSelector = []string{"l7aas"}
			tt.sp.init()
			rec, send = tt.sp.startFakeProvider()
			//go tt.sp.RefreshService(tt.args.msg)
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
		//pr   *Provider
		send chan comm.Message
	)
	tests := []struct {
		name string
		pr   Provider
		want comm.Message
	}{
		{"poll", newFakeProvider(), msgOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.pr.PollInterval = 1 * time.Second
			_, send = tt.pr.startFakeProvider()
			got := <-send
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Msg = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProvider_setCli(t *testing.T) {
	type fields struct {
		Endpoint               string
		LabelSelector          []string
		TLSCa                  string
		TLSCert                string
		TLSKey                 string
		PollInterval           time.Duration
		DefaultPortPublishMode string
		pollTicker             *time.Ticker
		shutdown               chan bool
		send                   chan<- comm.Message
		services               []string
		servicesLock           sync.RWMutex
		cli                    dockerCliInterface
		serviceFilters         filters.Args
		containerFilters       filters.Args
		waitGroup              sync.WaitGroup
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{"testErr", fields{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Provider{
				Endpoint:               tt.fields.Endpoint,
				LabelSelector:          tt.fields.LabelSelector,
				TLSCa:                  tt.fields.TLSCa,
				TLSCert:                tt.fields.TLSCert,
				TLSKey:                 tt.fields.TLSKey,
				PollInterval:           tt.fields.PollInterval,
				DefaultPortPublishMode: tt.fields.DefaultPortPublishMode,
				pollTicker:             tt.fields.pollTicker,
				shutdown:               tt.fields.shutdown,
				send:                   tt.fields.send,
				services:               tt.fields.services,
				servicesLock:           tt.fields.servicesLock,
				cli:                    tt.fields.cli,
				serviceFilters:         tt.fields.serviceFilters,
				containerFilters:       tt.fields.containerFilters,
				waitGroup:              tt.fields.waitGroup,
			}
			if err := p.setCli(); (err != nil) != tt.wantErr {
				t.Errorf("setCli() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
