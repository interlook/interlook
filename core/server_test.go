package core

import (
	testclient "k8s.io/client-go/kubernetes/fake"
	"os"
	"sync"
	"testing"
	"time"
)

func TestMain(m *testing.M) {

	initTmpDir()
	rc := m.Run()
	cleanup()
	os.Exit(rc)
}

func initTmpDir() {
	_ = os.Mkdir("_tmp", os.ModeDir)

}

func cleanup() {
	_ = os.RemoveAll("_tmp")
}

func Test_initServer(t *testing.T) {
	type args struct {
		configFile string
	}
	tests := []struct {
		name       string
		args       args
		wantDbFile string
		wantErr    bool
	}{
		{"ok",
			args{configFile: "./test-files/conf-ok.yml"},
			"./_tmp/flowentries.db",
			false},
		{"ko",
			args{configFile: "./_tmp/no-config.yml"},
			"",
			true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := initServer(tt.args.configFile)
			if (err != nil) != tt.wantErr {
				t.Errorf("initServer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				if got.workflowEntries.DBFile != tt.wantDbFile {
					t.Errorf("initServer() got = %v, want %v", got.workflowEntries.DBFile, tt.wantDbFile)
				}
			}
		})
	}
}

func Test_initExtensions(t *testing.T) {
	type args struct {
		configFile string
	}
	tests := []struct {
		name          string
		args          args
		wantExtension string
		wantErr       bool
	}{
		{"ok",
			args{configFile: "./test-files/conf-ok.yml"},
			"provider.kubernetes",
			false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := initServer(tt.args.configFile)
			s.config.Provider.Kubernetes.Cli = testclient.NewSimpleClientset()
			if (err != nil) != tt.wantErr {
				t.Errorf("initServer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				s.initExtensions()
				ok := false
				for n, _ := range s.extensions {
					if n == tt.wantExtension {
						ok = true
					}
				}
				if !ok {
					t.Errorf("initServer() count not find initialized extenstion %v", tt.wantExtension)
				}
			}
		})
	}
}

func Test_server_housekeeperDelete(t *testing.T) {
	tests := []struct {
		name     string
		confFile string
		interval time.Duration
		entries  map[string]*workflowEntry
	}{
		{"delete",
			"./test-files/conf-noprovider.yml",
			10 * time.Millisecond,
			map[string]*workflowEntry{"test": {
				Mutex:          sync.Mutex{},
				WorkInProgress: false,
				WIPTime:        time.Time{},
				State:          undeployedState,
				ExpectedState:  undeployedState,
				CloseTime:      time.Now().Add(-300 * time.Second),
			}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, _ := initServer(tt.confFile)
			s.initExtensions()
			for n, e := range tt.entries {
				s.workflowEntries.Entries[n] = e
			}
			go s.run()
			time.Sleep(200 * time.Millisecond)
			s.housekeeperShutdown <- true
			if len(s.workflowEntries.Entries) != 0 {
				t.Errorf("expected empty entries list")
			}
		})
	}
}

func Test_server_housekeeperClose(t *testing.T) {
	tests := []struct {
		name     string
		confFile string
		interval time.Duration
		entries  map[string]*workflowEntry
	}{
		{"Close",
			"./test-files/conf-ok.yml",
			10 * time.Millisecond,
			map[string]*workflowEntry{"test": {
				Mutex:          sync.Mutex{},
				WorkInProgress: true,
				WIPTime:        time.Now().Add(-10 * time.Second),
				State:          deployedState,
				ExpectedState:  undeployedState,
				CloseTime:      time.Now().Add(-300 * time.Second),
				LastUpdate:     time.Now(),
			}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, _ := initServer(tt.confFile)
			s.config.Provider.Kubernetes.Cli = testclient.NewSimpleClientset()
			s.initExtensions()
			for n, e := range tt.entries {
				s.workflowEntries.Entries[n] = e
			}
			go s.run()
			time.Sleep(200 * time.Millisecond)
			s.signals <- os.Kill
			time.Sleep(200 * time.Millisecond)
			if len(s.workflowEntries.Entries) == 0 {
				t.Errorf("expected empty entries list")
			}
			if s.workflowEntries.Entries["test"].State != undeployedState {
				t.Errorf("expected close, got %v", s.workflowEntries.Entries["test"].State)
			}
		})
	}
}

func Test_server_refreshService(t *testing.T) {
	tests := []struct {
		name     string
		confFile string
		interval time.Duration
		entries  map[string]*workflowEntry
	}{
		{"Close",
			"./test-files/conf-ok.yml",
			10 * time.Millisecond,
			map[string]*workflowEntry{"test": {
				Mutex:          sync.Mutex{},
				WorkInProgress: false,
				WIPTime:        time.Time{},
				State:          deployedState,
				ExpectedState:  deployedState,
				CloseTime:      time.Now().Add(-300 * time.Second),
				LastUpdate:     time.Now().Add(-300 * time.Second),
			}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, _ := initServer(tt.confFile)
			s.config.Provider.Kubernetes.Cli = testclient.NewSimpleClientset()
			s.initExtensions()
			for n, e := range tt.entries {
				s.workflowEntries.Entries[n] = e
			}
			go s.run()
			time.Sleep(200 * time.Millisecond)
			s.signals <- os.Kill
			time.Sleep(200 * time.Millisecond)
			if len(s.workflowEntries.Entries) == 0 {
				t.Errorf("expected empty entries list")
			}
			if s.workflowEntries.Entries["test"].State != deployedState {
				t.Errorf("expected close, got %v", s.workflowEntries.Entries["test"].State)
			}
		})
	}
}
