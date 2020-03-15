package kubernetes

import (
	"github.com/interlook/interlook/comm"
	"github.com/interlook/interlook/log"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	testclient "k8s.io/client-go/kubernetes/fake"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"
)

var (
	dummyNPSvc, dummyNPSvcNoPod v1.Service
	msgOK                       comm.Message
	dummyPod1, dummyPod2        *v1.Pod
)

func TestMain(m *testing.M) {
	initTests()
	rc := m.Run()
	os.Exit(rc)
}

func initTests() *Extension {
	k8s := Extension{
		Name:        "dummy",
		waitGroup:   sync.WaitGroup{},
		listOptions: metav1.ListOptions{},
	}
	podSelector := map[string]string{"app": "dummy"}
	podSelectorNoPod := map[string]string{"app": "notfound"}

	dummyNPSvc = v1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dummyNPSvc",
			Namespace: "default",
			Labels:    map[string]string{hostsLabel: "dummy.com", portLabel: "8080", sslLabel: "false", "l7aas": "true"},
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{{Protocol: "TCP",
				Port:     8080,
				NodePort: 32200}},
			Type:     "NodePort",
			Selector: podSelector,
		},
	}

	dummyNPSvcNoPod = v1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dummyNPSvcNoPod",
			Namespace: "default",
			Labels:    map[string]string{hostsLabel: "dummynp.com", portLabel: "8080", sslLabel: "false", "l7aas": "true"},
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{{Protocol: "TCP",
				Port:     8080,
				NodePort: 32200}},
			Type:     "NodePort",
			Selector: podSelectorNoPod,
		},
	}

	dummyPod1 = &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dummypod-1",
			Namespace: "default",
			Labels:    podSelector,
		},
		Spec: v1.PodSpec{
			NodeName: "node1",
			Hostname: "node1",
		},
		Status: v1.PodStatus{
			HostIP: "10.32.2.1",
		},
	}

	dummyPod2 = &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dummypod-2",
			Namespace: "default",
			Labels:    podSelector,
		},
		Spec: v1.PodSpec{
			NodeName: "node2",
			Hostname: "node2",
		},
		Status: v1.PodStatus{
			HostIP: "10.32.2.2",
		},
	}
	targetOK := []comm.Target{{
		Host: "10.32.2.1",
		Port: 32200,
	},
		{
			Host: "10.32.2.2",
			Port: 32200,
		}}

	msgOK = comm.Message{Service: comm.Service{
		Name:       "dummyNPSvc",
		Namespace:  "default",
		DNSAliases: []string{"dummy.com"},
		Targets:    targetOK,
		TLS:        false,
		Provider:   extensionName,
	},
		Action: comm.AddAction}

	k8s.Cli = testclient.NewSimpleClientset(&dummyNPSvc, &dummyNPSvcNoPod, dummyPod1, dummyPod2)
	return &k8s

}

// startK8sProvider returns a "running" k8s provider instance
func (p *Extension) startTestK8s() (rec, send chan comm.Message) {
	p.init()
	rec = make(chan comm.Message)
	send = make(chan comm.Message)

	go func() {
		err := p.Start(rec, send)
		if err != nil {
			log.Errorf("could not start fake provider: %v", err.Error())
		}
	}()
	// sleep so that provider is ready to receive data on channel
	time.Sleep(300 * time.Millisecond)
	return rec, send
}

func TestExtension_getServiceByName(t *testing.T) {
	k8s := initTests()
	type args struct {
		svcName string
	}
	tests := []struct {
		name      string
		k8s       *Extension
		svc       string
		namespace string
		want      bool
	}{
		{"found", k8s, "dummyNPSvc", "default", true},
		{"notFound", k8s, "notFound", "default", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := tt.k8s.getServiceByName(tt.svc, tt.namespace)
			if ok != tt.want {
				t.Errorf("getServiceByName() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtension_buildMessageFromService(t *testing.T) {
	k8s := initTests()
	type args struct {
		service *v1.Service
	}
	tests := []struct {
		name    string
		k8s     *Extension
		args    args
		wantMsg comm.Message
		wantErr bool
	}{
		{"basic", k8s, args{&dummyNPSvc}, comm.Message{
			Action: "add",
			Sender: "",
			Service: comm.Service{
				Provider:  "provider.kubernetes",
				Name:      "dummyNPSvc",
				Namespace: "default",
				Targets: []comm.Target{{
					Host:   "10.32.2.1",
					Port:   32200,
					Weight: 0,
				},
					{
						Host:   "10.32.2.2",
						Port:   32200,
						Weight: 0,
					},
				},
				TLS:        false,
				PublicIP:   "",
				DNSAliases: []string{"dummy.com"},
			},
		}, false},
		{"fail", k8s, args{&dummyNPSvcNoPod}, comm.Message{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMsg, err := tt.k8s.buildMessageFromService(tt.args.service)
			if (err != nil) != tt.wantErr {
				t.Errorf("buildMessageFromService() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(gotMsg, tt.wantMsg) && err == nil {
				t.Errorf("buildMessageFromService() gotMsg = %v, want %v", gotMsg, tt.wantMsg)
			}
		})
	}
}

func TestExtension_RefreshService(t *testing.T) {

	type args struct {
		msg comm.Message
	}
	tests := []struct {
		name    string
		args    args
		want    comm.Message
		wantErr bool
	}{
		{"refreshErr", args{msg: comm.Message{
			Action: "refresh",
			Service: comm.Service{Name: "dummyNPSvcNoPod",
				Namespace: "default"},
		}}, comm.Message{
			Action:      "refresh",
			Sender:      "",
			Destination: "",
			Error:       "",
			Service: comm.Service{
				Provider:   "kubernetes.provider",
				Name:       "dummyNPSvcNoPod",
				Namespace:  "default",
				Targets:    nil,
				TLS:        false,
				PublicIP:   "",
				DNSAliases: nil,
			},
		}, true},
		{"refreshAdd", args{msg: comm.Message{
			Action:      "refresh",
			Sender:      "provider.kubernetes",
			Destination: "",
			Error:       "",
			Service: comm.Service{Name: "dummyNPSvc",
				Namespace: "default"},
		}}, msgOK, false},
		{"refreshDel", args{msg: comm.Message{
			Action: "refresh",
			Service: comm.Service{
				Provider:   "kubernetes.provider",
				Name:       "notfound",
				Namespace:  "notfound",
				Targets:    nil,
				TLS:        false,
				PublicIP:   "",
				DNSAliases: nil,
			},
		}}, comm.Message{
			Action: "delete",
			Service: comm.Service{
				Provider:  "kubernetes.provider",
				Name:      "notfound",
				Namespace: "notfound",
			},
		}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := initTests()
			_, send := p.startTestK8s()
			go p.RefreshService(tt.args.msg)
			got := <-send
			go p.Stop()
			if len(got.Error) > 0 && !tt.wantErr {
				t.Errorf("Got unexpected error")
			}
			if !reflect.DeepEqual(got, tt.want) && !tt.wantErr {
				t.Errorf("Msg = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtension_SendRefreshRequest(t *testing.T) {

	tests := []struct {
		name string
		msg  comm.Message
		want comm.Message
	}{
		{"refresh", comm.Message{
			Action: "refresh",
			Service: comm.Service{Name: "dummyNPSvc",
				Namespace: "default"},
		}, comm.Message{
			Action: comm.AddAction,
			Service: comm.Service{
				Provider:  "provider.kubernetes",
				Name:      "dummyNPSvc",
				Namespace: "default",
				Targets: []comm.Target{{
					Host:   "10.32.2.1",
					Port:   32200,
					Weight: 0,
				},
					{
						Host:   "10.32.2.2",
						Port:   32200,
						Weight: 0,
					}},
				TLS:        false,
				DNSAliases: []string{"dummy.com"},
			},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := initTests()
			p.LabelSelector = []string{"l7aas"}
			rec, send := p.startTestK8s()
			rec <- tt.msg
			got := <-send
			go p.Stop()
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Msg = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtension_Poll(t *testing.T) {

	tests := []struct {
		name    string
		want    comm.Message
		wantErr bool
	}{
		{"refresh", msgOK, false},
		{"error", comm.Message{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := initTests()
			p.LabelSelector = []string{"l7aas"}
			p.PollInterval = 500
			_, send := p.startTestK8s()
			got := <-send
			go p.Stop()
			if len(got.Error) > 0 && !tt.wantErr {
				t.Errorf("Got unexpected error")
			}
			if !reflect.DeepEqual(got, tt.want) && !tt.wantErr {
				t.Errorf("Msg = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtension_Start(t *testing.T) {
	type fields struct {
		Name          string
		Endpoint      string
		LabelSelector []string
		TLSCa         string
		TLSCert       string
		TLSKey        string
		PollInterval  time.Duration
		pollTicker    *time.Ticker
		shutdown      chan bool
		send          chan<- comm.Message
		cli           kubernetes.Interface
		waitGroup     sync.WaitGroup
		listOptions   metav1.ListOptions
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
		{"connectErr", fields{}, args{
			receive: make(chan comm.Message),
			send:    make(chan comm.Message),
		}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Extension{}
			if err := p.Start(tt.args.receive, tt.args.send); (err != nil) != tt.wantErr {
				t.Errorf("Start() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
