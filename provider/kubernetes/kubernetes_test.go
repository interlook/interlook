package kubernetes

import (
	"fmt"
	"github.com/interlook/interlook/comm"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
			Name:   "dummyNPSvc",
			Labels: map[string]string{hostsLabel: "dummy.com", portLabel: "8080", sslLabel: "false", "l7aas": "true"},
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
			Name:   "dummyNPSvcNoPod",
			Labels: map[string]string{hostsLabel: "dummynp.com", portLabel: "8080", sslLabel: "false", "l7aas": "true"},
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
		DNSAliases: []string{"dummy.com"},
		Targets:    targetOK,
		TLS:        false,
		Provider:   extensionName,
	},
		Action: comm.AddAction}

	k8s.cli = testclient.NewSimpleClientset(&dummyNPSvc, &dummyNPSvcNoPod, dummyPod1, dummyPod2)
	return &k8s

}

// startK8sProvider returns a "running" k8s provider instance
func (p *Extension) startTestK8s() (rec, send chan comm.Message) {
	p.init()
	rec = make(chan comm.Message)
	send = make(chan comm.Message)
	go p.Start(rec, send)
	//TODO: find better way...
	time.Sleep(1 * time.Second)
	return rec, send
}

func TestExtension_getServiceByName(t *testing.T) {
	k8s := initTests()
	type args struct {
		svcName string
	}
	tests := []struct {
		name string
		k8s  *Extension
		svc  string
		want bool
	}{
		{"found", k8s, "dummyNPSvc", true},
		{"notFound", k8s, "notFound", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			pod, _ := tt.k8s.cli.CoreV1().Pods("").List(metav1.ListOptions{})
			fmt.Println(pod)
			got, ok := tt.k8s.getServiceByName(tt.svc)
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
			Action:  "refresh",
			Service: comm.Service{Name: "dummyNPSvcNoPod"},
		}}, comm.Message{}, true},
		{"refreshAdd", args{msg: comm.Message{
			Action:  "refresh",
			Service: comm.Service{Name: "dummyNPSvc"},
		}}, msgOK, false},
		{"refreshDel", args{msg: comm.Message{
			Action:  "refresh",
			Service: comm.Service{Name: "notfound"},
		}}, comm.BuildDeleteMessage("notfound"), false},
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
			Action:  "refresh",
			Service: comm.Service{Name: "dummyNPSvc"},
		}, msgOK},
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
