package kubernetes

import (
	"fmt"
	"github.com/interlook/interlook/comm"
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
	k8s        Extension
	dummyNPSvc v1.Service
)

func TestMain(m *testing.M) {
	initTests()
	startK8sProvider()
	rc := m.Run()
	os.Exit(rc)
}

func initTests() {
	k8s = Extension{
		Name:          "dummy",
		Endpoint:      "",
		LabelSelector: nil,
		TLSCa:         "",
		TLSCert:       "",
		TLSKey:        "",
		PollInterval:  5,
		pollTicker:    nil,
		shutdown:      nil,
		send:          nil,
		cli:           nil,
		waitGroup:     sync.WaitGroup{},
		listOptions:   metav1.ListOptions{},
	}
	podSelector := map[string]string{"app": "dummy"}

	dummyNPSvc = v1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   "dummyNPSvc",
			Labels: map[string]string{hostsLabel: "dummy.com", portLabel: "8080", sslLabel: "false"},
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{{Protocol: "TCP",
				Port:     8080,
				NodePort: 32200}},
			Type:     "NodePort",
			Selector: podSelector,
		},
	}

	dummyPod1 := &v1.Pod{
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

	dummyPod2 := &v1.Pod{
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

	k8s.cli = testclient.NewSimpleClientset(&dummyNPSvc, dummyPod1, dummyPod2)

}

// startK8sProvider returns a "running" k8s provider instance
func startK8sProvider() (p *Extension, rec, send chan comm.Message) {
	p = &Extension{
		cli:          k8s.cli,
		PollInterval: 10 * time.Second,
	}

	p.init()
	rec = make(chan comm.Message)
	send = make(chan comm.Message)
	go p.Start(rec, send)

	return p, rec, send
}

func TestExtension_getServiceByName(t *testing.T) {
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
		svcName string
	}
	tests := []struct {
		name   string
		fields fields
		svc    string
		want   bool
	}{
		{"found", fields{
			cli:         k8s.cli,
			waitGroup:   sync.WaitGroup{},
			listOptions: metav1.ListOptions{},
		}, "dummyNPSvc", true},
		{"notFound", fields{
			cli:         k8s.cli,
			waitGroup:   sync.WaitGroup{},
			listOptions: metav1.ListOptions{},
		}, "notFound", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Extension{
				Name:          tt.fields.Name,
				Endpoint:      tt.fields.Endpoint,
				LabelSelector: tt.fields.LabelSelector,
				TLSCa:         tt.fields.TLSCa,
				TLSCert:       tt.fields.TLSCert,
				TLSKey:        tt.fields.TLSKey,
				PollInterval:  tt.fields.PollInterval,
				pollTicker:    tt.fields.pollTicker,
				shutdown:      tt.fields.shutdown,
				send:          tt.fields.send,
				cli:           tt.fields.cli,
				waitGroup:     tt.fields.waitGroup,
				listOptions:   tt.fields.listOptions,
			}
			pod, _ := p.cli.CoreV1().Pods("").List(metav1.ListOptions{})
			fmt.Println(pod)
			got, ok := p.getServiceByName(tt.svc)
			if ok != tt.want {
				t.Errorf("getServiceByName() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtension_buildMessageFromService(t *testing.T) {

	type args struct {
		service *v1.Service
	}
	tests := []struct {
		name    string
		k8s     Extension
		args    args
		wantMsg comm.Message
		wantErr bool
	}{
		{"basic", k8s, args{&dummyNPSvc}, comm.Message{
			Action: "add",
			Sender: "",
			Service: comm.Service{
				Provider: "provider.kubernetes",
				Name:     "dummyNPSvc",
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
				Info:       "",
				Error:      "",
			},
		}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMsg, err := tt.k8s.buildMessageFromService(tt.args.service)
			if (err != nil) != tt.wantErr {
				t.Errorf("buildMessageFromService() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotMsg, tt.wantMsg) {
				t.Errorf("buildMessageFromService() gotMsg = %v, want %v", gotMsg, tt.wantMsg)
			}
		})
	}
}
