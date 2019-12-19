package comm

import (
	"os"
	"reflect"
	"testing"
)

var (
	svc             Service
	revMsg          Message
	fwdMsg          Message
	diffSvc         Service
	target8080      []Target
	target8081      []Target
	targetMulti8081 []Target
)

func TestMain(m *testing.M) {
	initTests()
	rc := m.Run()
	os.Exit(rc)
}

func initTests() {
	target8080 = append(target8080, Target{
		Host: "10.32.2.2",
		Port: 8080,
	})

	target8081 = append(target8081, Target{
		Host: "10.32.2.2",
		Port: 8081,
	})

	targetMulti8081 = append(targetMulti8081, Target{
		Host: "10.32.2.2",
		Port: 8081,
	})
	targetMulti8081 = append(targetMulti8081, Target{
		Host: "10.32.2.3",
		Port: 8081,
	})

	svc = Service{
		Provider:   "provider.swarm",
		Name:       "test",
		Targets:    target8080,
		TLS:        false,
		PublicIP:   "10.10.10.10",
		DNSAliases: []string{"www.test.dom"},
		Info:       "",
		Error:      "",
	}

	diffSvc = Service{
		Provider:   "provider.swarm",
		Name:       "test",
		Targets:    target8080,
		TLS:        false,
		PublicIP:   "10.10.10.10",
		DNSAliases: []string{"www.test.dom"},
		Info:       "",
		Error:      "",
	}

	fwdMsg = Message{
		Action:      AddAction,
		Sender:      "",
		Destination: "",
		Error:       "",
		Service:     svc,
	}

	revMsg = Message{
		Action:      DeleteAction,
		Sender:      "",
		Destination: "",
		Error:       "",
		Service:     svc,
	}
}

func TestBuildMessage(t *testing.T) {
	type args struct {
		service Service
		reverse bool
	}
	tests := []struct {
		name string
		args args
		want Message
	}{
		{"testFWD", args{service: svc, reverse: false}, fwdMsg},
		{"testREV", args{service: svc, reverse: true}, revMsg},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := BuildMessage(tt.args.service, tt.args.reverse); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("BuildMessage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestService_IsSameThan(t *testing.T) {

	type args struct {
		targetService Service
	}
	tests := []struct {
		name   string
		svc    Service
		target Service
		want   bool
		want1  []string
	}{
		{"isSame", svc, svc, true, nil},
		{"dnsDif", svc, Service{
			Provider:   "provider.swarm",
			Name:       "test",
			Targets:    target8080,
			TLS:        false,
			PublicIP:   "10.10.10.10",
			DNSAliases: []string{"www.test1.dom"},
			Info:       "",
			Error:      "",
		}, false, []string{"DNSNames"}},
		{"TLS", svc, Service{
			Provider:   "provider.swarm",
			Name:       "test",
			Targets:    targetMulti8081,
			TLS:        true,
			PublicIP:   "10.10.10.10",
			DNSAliases: []string{"www.test.dom"},
			Info:       "",
			Error:      "",
		}, false, []string{"TLS", "Targets"}},
		{"Hosts", svc, Service{
			Provider:   "provider.swarm",
			Name:       "test",
			Targets:    targetMulti8081,
			TLS:        false,
			PublicIP:   "10.10.10.10",
			DNSAliases: []string{"www.test.dom"},
			Info:       "",
			Error:      "",
		}, false, []string{"Targets"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			got, got1 := tt.svc.IsSameThan(tt.target)
			if got != tt.want {
				t.Errorf("IsSameThan() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("IsSameThan() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}
