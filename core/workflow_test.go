package core

import (
	"github.com/interlook/interlook/comm"
	"github.com/interlook/interlook/log"
	"reflect"
	"sync"
	"testing"
	"time"
)

var (
	testWF workflowSteps
	wf     workflowSteps
)

func init() {
	wf = initWorkflow("provider.kubernetes")
	testWF = workflowSteps{workflowStep{
		ID:         0,
		Name:       "undeployed",
		Transition: &closeState{},
	},
		workflowStep{
			ID:         3,
			Name:       "deployed",
			Transition: &closeState{},
		},
		workflowStep{
			ID:         1,
			Name:       "provider.one",
			Transition: &receivedFromProvider{},
		},
		workflowStep{
			ID:         2,
			Name:       "provisioner.two",
			Transition: &receivedFromProvisioner{},
		}}
}

func Test_workflowSteps_getNextStep(t *testing.T) {
	type args struct {
		currentStep string
		reverse     bool
	}
	tests := []struct {
		name         string
		w            workflowSteps
		args         args
		wantNextStep string
		wantNext     transitioner
		wantErr      bool
	}{
		{"provider", testWF, args{
			currentStep: "undeployed",
			reverse:     false,
		}, "provisioner.two", &receivedFromProvisioner{}, false},
		{"provisioner", testWF, args{
			currentStep: "provisioner.two",
			reverse:     false,
		}, "deployed", &closeState{}, false},
		{"provisionerReverse", testWF, args{
			currentStep: "provisioner.two",
			reverse:     true,
		}, "undeployed", &closeState{}, false},
		{"currentStepNotFound", testWF, args{
			currentStep: "provisioner.one",
			reverse:     true,
		}, "", nil, true},
		{"nextStepNotFound", testWF, args{
			currentStep: "deployed",
			reverse:     false,
		}, "", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotNextStep, gotNext, err := tt.w.getNextStep(tt.args.currentStep, tt.args.reverse)
			if (err != nil) != tt.wantErr {
				t.Errorf("getNextStep() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotNextStep != tt.wantNextStep {
				t.Errorf("getNextStep() gotNextStep = %v, want %v", gotNextStep, tt.wantNextStep)
			}
			if !reflect.DeepEqual(gotNext, tt.wantNext) {
				t.Errorf("getNextStep() gotNext = %v, want %v", gotNext, tt.wantNext)
			}
		})
	}
}

func Test_workflowSteps_getTransition(t *testing.T) {
	type args struct {
		step string
	}
	tests := []struct {
		name string
		w    workflowSteps
		args args
		want transitioner
	}{
		{"close", testWF, args{step: "deployed"}, &closeState{}},
		{"close", testWF, args{step: "invalidStep"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.w.getTransition(tt.args.step); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getTransition() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_workflowSteps_isLastStep(t *testing.T) {
	type args struct {
		step    string
		reverse bool
	}
	tests := []struct {
		name string
		w    workflowSteps
		args args
		want bool
	}{
		{"isLaststep", testWF, args{
			step:    "deployed",
			reverse: false,
		}, true},
		{"isLaststepReverse", testWF, args{
			step:    "undeployed",
			reverse: true,
		}, true},
		{"isNotLaststep", testWF, args{
			step:    "provider.one",
			reverse: false,
		}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.w.isLastStep(tt.args.step, tt.args.reverse); got != tt.want {
				t.Errorf("isLastStep() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_initWorkflow(t *testing.T) {
	type args struct {
		workflowConfig string
	}
	tests := []struct {
		name string
		args args
		want workflowSteps
	}{
		{"init", args{workflowConfig: "provider.one,provisioner.two"}, testWF},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := initWorkflow(tt.args.workflowConfig)
			if len(got) != len(tt.want) {
				t.Errorf("initWorkflow() = %v, want %v", got, tt.want)
			}
			found := 0
			for _, g := range got {
				for _, w := range tt.want {
					if w.Name == g.Name {
						if g.ID == w.ID {
							found++
						}
					}
				}
			}
			if found != len(tt.want) {
				t.Error("initWorkflow() workflowSteps do not match")
			}
		})
	}
}

func weIsEqual(a *workflowEntry, b *workflowEntry) bool {
	if a.WorkInProgress != b.WorkInProgress {
		return false
	}
	if !reflect.DeepEqual(a.Service, b.Service) {
		return false
	}
	if a.Error != b.Error {
		return false
	}
	if a.ExpectedState != b.ExpectedState {
		return false
	}
	if a.Info != b.Info {
		return false
	}
	if a.State != b.State {
		return false
	}
	return true
}

func Test_workflowEntry_setWIP(t *testing.T) {
	type fields struct {
		Mutex          sync.Mutex
		WorkInProgress bool
		WIPTime        time.Time
		State          string
		ExpectedState  string
		Info           string
		Error          string
		TimeDetected   time.Time
		LastUpdate     time.Time
		Service        comm.Service
		CloseTime      time.Time
		transition     transitioner
	}
	type args struct {
		wip bool
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantWIP bool
	}{
		{"WIPTrue",
			fields{},
			args{wip: true},
			true},
		{"WIPFalsee",
			fields{},
			args{wip: false},
			false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &workflowEntry{
				Mutex:          tt.fields.Mutex,
				WorkInProgress: tt.fields.WorkInProgress,
				WIPTime:        tt.fields.WIPTime,
				State:          tt.fields.State,
				ExpectedState:  tt.fields.ExpectedState,
				Info:           tt.fields.Info,
				Error:          tt.fields.Error,
				TimeDetected:   tt.fields.TimeDetected,
				LastUpdate:     tt.fields.LastUpdate,
				Service:        tt.fields.Service,
				CloseTime:      tt.fields.CloseTime,
				step:           tt.fields.transition,
			}
			e.setWIP(tt.args.wip)
			if e.WorkInProgress != tt.args.wip {
				t.Errorf("Expected %v, got %v", tt.args.wip, e.WorkInProgress)
			}
		})
	}
}

func Test_workflowEntry_updateService(t *testing.T) {
	type fields struct {
		Mutex          sync.Mutex
		WorkInProgress bool
		WIPTime        time.Time
		State          string
		ExpectedState  string
		Info           string
		Error          string
		TimeDetected   time.Time
		LastUpdate     time.Time
		Service        comm.Service
		CloseTime      time.Time
		transition     transitioner
		Steps          workflowSteps
		Reverse        bool
	}
	type args struct {
		msg comm.Message
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{"UpdateFromProvider",
			fields{
				Mutex: sync.Mutex{},
				Service: comm.Service{
					Provider: "provider.dummy",
					Name:     "dummy",
					Targets: []comm.Target{{
						Host:   "dummy",
						Port:   80,
						Weight: 1,
					}},
					TLS:      false,
					PublicIP: "10.10.10.10",
				},
			},
			args{msg: comm.Message{
				Action: "add",
				Sender: "provider.dummy",
				Service: comm.Service{
					Provider: "provider.dummy",
					Name:     "dummy",
					Targets: []comm.Target{{
						Host:   "dummy",
						Port:   80,
						Weight: 1,
					}},
					PublicIP:   "10.10.10.10",
					DNSAliases: []string{"dummy.example.com"},
				},
			}},
		},
		{"UpdateFromIpam",
			fields{
				Mutex: sync.Mutex{},
				Service: comm.Service{
					Provider: "ipam.dummy",
					Name:     "dummy",
				},
			},
			args{msg: comm.Message{
				Action: "add",
				Sender: "ipam.dummy",
				Service: comm.Service{
					Provider: "provider.dummy",
					Name:     "dummy",
					PublicIP: "10.10.10.10",
				},
			}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &workflowEntry{
				Mutex:          tt.fields.Mutex,
				WorkInProgress: tt.fields.WorkInProgress,
				WIPTime:        tt.fields.WIPTime,
				State:          tt.fields.State,
				ExpectedState:  tt.fields.ExpectedState,
				Info:           tt.fields.Info,
				Error:          tt.fields.Error,
				TimeDetected:   tt.fields.TimeDetected,
				LastUpdate:     tt.fields.LastUpdate,
				Service:        tt.fields.Service,
				CloseTime:      tt.fields.CloseTime,
				step:           tt.fields.transition,
				workflowSteps:  tt.fields.Steps,
				Reverse:        tt.fields.Reverse,
			}
			e.updateService(tt.args.msg)
			if e.Service.Name != tt.args.msg.Service.Name || !reflect.DeepEqual(e.Service.Targets, tt.args.msg.Service.Targets) || e.Service.TLS != tt.args.msg.Service.TLS {
				t.Errorf("service not updated as expected got %v, want %v", e.Service, tt.args.msg.Service)
			}
		})
	}
}

func Test_workflowEntry_close(t *testing.T) {
	type fields struct {
		Mutex          sync.Mutex
		WorkInProgress bool
		WIPTime        time.Time
		State          string
		ExpectedState  string
		Info           string
		Error          string
		TimeDetected   time.Time
		LastUpdate     time.Time
		Service        comm.Service
		CloseTime      time.Time
		transition     transitioner
		Steps          workflowSteps
		Reverse        bool
	}
	type args struct {
		errorMessage string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{"CloseOK",
			fields{
				Mutex:         sync.Mutex{},
				ExpectedState: deployedState,
				Steps:         initWorkflow("provider.kubernetes"),
			},
			args{}},
		{"CloseReverse",
			fields{
				Mutex:         sync.Mutex{},
				ExpectedState: undeployedState,
				Reverse:       true,
				Steps:         initWorkflow("provider.kubernetes"),
			},
			args{}},
		{"CloseError",
			fields{
				Mutex:         sync.Mutex{},
				ExpectedState: deployedState,
				Steps:         initWorkflow("provider.kubernetes"),
			},
			args{errorMessage: "error"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &workflowEntry{
				Mutex:          tt.fields.Mutex,
				WorkInProgress: tt.fields.WorkInProgress,
				WIPTime:        tt.fields.WIPTime,
				State:          tt.fields.State,
				ExpectedState:  tt.fields.ExpectedState,
				Info:           tt.fields.Info,
				Error:          tt.fields.Error,
				TimeDetected:   tt.fields.TimeDetected,
				LastUpdate:     tt.fields.LastUpdate,
				Service:        tt.fields.Service,
				CloseTime:      tt.fields.CloseTime,
				step:           tt.fields.transition,
				workflowSteps:  tt.fields.Steps,
				Reverse:        tt.fields.Reverse,
			}
			e.close(tt.args.errorMessage)
			if e.State != e.ExpectedState || e.Error != tt.args.errorMessage {
				t.Error("workflow entry has unexpected properties")
			}
		})
	}
}

func Test_workflowEntry_isStateAsWanted(t *testing.T) {
	type fields struct {
		Mutex          sync.Mutex
		WorkInProgress bool
		WIPTime        time.Time
		State          string
		ExpectedState  string
		Info           string
		Error          string
		TimeDetected   time.Time
		LastUpdate     time.Time
		Service        comm.Service
		CloseTime      time.Time
		transition     transitioner
		Steps          workflowSteps
		Reverse        bool
	}
	type args struct {
		action string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{"True",
			fields{
				State:         deployedState,
				ExpectedState: deployedState,
			},
			args{action: comm.AddAction},
			true},
		{"False",
			fields{
				State:         undeployedState,
				ExpectedState: deployedState,
			},
			args{action: comm.AddAction},
			false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &workflowEntry{
				Mutex:          tt.fields.Mutex,
				WorkInProgress: tt.fields.WorkInProgress,
				WIPTime:        tt.fields.WIPTime,
				State:          tt.fields.State,
				ExpectedState:  tt.fields.ExpectedState,
				Info:           tt.fields.Info,
				Error:          tt.fields.Error,
				TimeDetected:   tt.fields.TimeDetected,
				LastUpdate:     tt.fields.LastUpdate,
				Service:        tt.fields.Service,
				CloseTime:      tt.fields.CloseTime,
				step:           tt.fields.transition,
				workflowSteps:  tt.fields.Steps,
				Reverse:        tt.fields.Reverse,
			}
			if got := e.isStateAsWanted(tt.args.action); got != tt.want {
				t.Errorf("isStateAsWanted() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_workflowEntry_setNextState(t *testing.T) {
	type fields struct {
		Mutex          sync.Mutex
		WorkInProgress bool
		WIPTime        time.Time
		State          string
		ExpectedState  string
		Info           string
		Error          string
		TimeDetected   time.Time
		LastUpdate     time.Time
		Service        comm.Service
		CloseTime      time.Time
		Reverse        bool
		workflowSteps  workflowSteps
		step           transitioner
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{"ok",
			fields{
				Mutex:         sync.Mutex{},
				State:         undeployedState,
				ExpectedState: deployedState,
				workflowSteps: initWorkflow("provider.kubernetes"),
				step:          &receivedFromProvider{},
			},
			deployedState},
		{"ko",
			fields{
				Mutex:         sync.Mutex{},
				State:         "notFound",
				ExpectedState: deployedState,
				workflowSteps: initWorkflow("provider.kubernetes"),
				step:          nil,
			},
			"notFound"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &workflowEntry{
				Mutex:          tt.fields.Mutex,
				WorkInProgress: tt.fields.WorkInProgress,
				WIPTime:        tt.fields.WIPTime,
				State:          tt.fields.State,
				ExpectedState:  tt.fields.ExpectedState,
				Info:           tt.fields.Info,
				Error:          tt.fields.Error,
				TimeDetected:   tt.fields.TimeDetected,
				LastUpdate:     tt.fields.LastUpdate,
				Service:        tt.fields.Service,
				CloseTime:      tt.fields.CloseTime,
				Reverse:        tt.fields.Reverse,
				workflowSteps:  tt.fields.workflowSteps,
				step:           tt.fields.step,
			}
			e.setNextStep()
			if !reflect.DeepEqual(e.State, tt.want) {
				t.Errorf("got %v, expected %v", e.State, tt.want)
			}
		})
	}
}

func Test_workflowEntry_setState(t *testing.T) {
	type fields struct {
		Mutex          sync.Mutex
		WorkInProgress bool
		WIPTime        time.Time
		State          string
		ExpectedState  string
		Info           string
		Error          string
		TimeDetected   time.Time
		LastUpdate     time.Time
		Service        comm.Service
		CloseTime      time.Time
		Reverse        bool
		workflowSteps  workflowSteps
		step           transitioner
	}
	type args struct {
		msg comm.Message
		wip bool
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{"ok",
			fields{
				Mutex: sync.Mutex{},
			},
			args{
				msg: comm.Message{
					Sender:  "dummy",
					Error:   "",
					Service: comm.Service{},
				},
				wip: true,
			}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &workflowEntry{
				Mutex:          tt.fields.Mutex,
				WorkInProgress: tt.fields.WorkInProgress,
				WIPTime:        tt.fields.WIPTime,
				State:          tt.fields.State,
				ExpectedState:  tt.fields.ExpectedState,
				Info:           tt.fields.Info,
				Error:          tt.fields.Error,
				TimeDetected:   tt.fields.TimeDetected,
				LastUpdate:     tt.fields.LastUpdate,
				Service:        tt.fields.Service,
				CloseTime:      tt.fields.CloseTime,
				Reverse:        tt.fields.Reverse,
				workflowSteps:  tt.fields.workflowSteps,
				step:           tt.fields.step,
			}
			e.setState(tt.args.msg, true)
			if !reflect.DeepEqual(e.State, tt.args.msg.Sender) {
				t.Errorf("State: got %v, want %v", e.State, tt.args.msg.Sender)
			}
			if !reflect.DeepEqual(e.WorkInProgress, tt.args.wip) {
				t.Errorf("WorkInProgress: got %v, want %v", e.WorkInProgress, tt.args.wip)
			}
			if !reflect.DeepEqual(e.Error, tt.args.msg.Error) {
				t.Errorf("WorkInProgress: got %v, want %v", e.Error, tt.args.msg.Error)
			}
		})
	}
}

func Test_workflowEntry_sendToExtension(t *testing.T) {
	type fields struct {
		Mutex          sync.Mutex
		WorkInProgress bool
		WIPTime        time.Time
		State          string
		ExpectedState  string
		Info           string
		Error          string
		TimeDetected   time.Time
		LastUpdate     time.Time
		Service        comm.Service
		CloseTime      time.Time
		Reverse        bool
		workflowSteps  workflowSteps
		step           transitioner
		toExtension    chan comm.Message
	}
	tests := []struct {
		name   string
		fields fields
		toExt  chan comm.Message
		want   comm.Message
	}{
		{"ok",
			fields{
				Mutex:          sync.Mutex{},
				WorkInProgress: false,
				WIPTime:        time.Time{},
				Service: comm.Service{
					Provider:  "provider.kubernetes",
					Name:      "dummy",
					Namespace: "default",
				},
			},
			make(chan comm.Message),
			comm.BuildMessage(comm.Service{
				Provider:  "provider.kubernetes",
				Name:      "dummy",
				Namespace: "default",
			}, false),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &workflowEntry{
				Mutex:          tt.fields.Mutex,
				WorkInProgress: tt.fields.WorkInProgress,
				WIPTime:        tt.fields.WIPTime,
				State:          tt.fields.State,
				ExpectedState:  tt.fields.ExpectedState,
				Info:           tt.fields.Info,
				Error:          tt.fields.Error,
				TimeDetected:   tt.fields.TimeDetected,
				LastUpdate:     tt.fields.LastUpdate,
				Service:        tt.fields.Service,
				CloseTime:      tt.fields.CloseTime,
				Reverse:        tt.fields.Reverse,
				workflowSteps:  tt.fields.workflowSteps,
				step:           tt.fields.step,
				toExtension:    tt.toExt,
			}
			go e.sendToExtension()
			got := <-tt.toExt
			if !reflect.DeepEqual(got, comm.BuildMessage(e.Service, e.isReverse())) {
				t.Errorf("got %v, want %v", got, comm.BuildMessage(e.Service, e.isReverse()))
			}
		})
	}
}

func Test_initWorkflowEntries(t *testing.T) {
	ch := make(chan comm.Message)
	type args struct {
		dbFile       string
		workflowConf string
	}
	tests := []struct {
		name string
		args args
		want *workflowEntries
	}{
		{"OK",
			args{dbFile: "dummy.db",
				workflowConf: "provider.kubernetes"},
			&workflowEntries{
				Mutex:          sync.Mutex{},
				Entries:        make(map[string]*workflowEntry),
				DBFile:         "dummy.db",
				workflowConfig: "provider.kubernetes",
				commChan:       ch,
			}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := newWorkflowEntries(tt.args.dbFile, tt.args.workflowConf, ch); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("newWorkflowEntries() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_workflowEntries_serviceNeedUpdate(t *testing.T) {
	type fields struct {
		Mutex   sync.Mutex
		Entries map[string]*workflowEntry
		DBFile  string
	}
	type args struct {
		msg comm.Message
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{"NeedUpdate",
			fields{
				Mutex: sync.Mutex{},
				Entries: map[string]*workflowEntry{"dummy": &workflowEntry{
					Mutex:          sync.Mutex{},
					WorkInProgress: false,
					WIPTime:        time.Time{},
					State:          deployedState,
					ExpectedState:  deployedState,
					Service: comm.Service{
						Provider:  "provider.kubernetes",
						Name:      "dummy",
						Namespace: "default",
					},
					CloseTime: time.Time{},

					WfConfig: "provider.kubernetes",
				}},
				DBFile: "dummy.db",
			},
			args{msg: comm.Message{
				Action: comm.UpdateAction,
				Sender: "provider.kubernetes",
				Service: comm.Service{
					Provider:  "provider.kubernetes",
					Name:      "dummy",
					Namespace: "default",
					Targets: []comm.Target{{
						Host:   "10.32.2.2",
						Port:   8080,
						Weight: 1,
					}},
					TLS:        false,
					PublicIP:   "",
					DNSAliases: nil,
				},
			}},
			true},
		{"NeedUpdateState",
			fields{
				Mutex: sync.Mutex{},
				Entries: map[string]*workflowEntry{"test/default": &workflowEntry{
					Mutex:          sync.Mutex{},
					WorkInProgress: false,
					WIPTime:        time.Time{},
					State:          undeployedState,
					ExpectedState:  deployedState,
					Service: comm.Service{
						Provider:  "provider.kubernetes",
						Name:      "test",
						Namespace: "default",
						Targets: []comm.Target{{
							Host:   "10.32.2.2",
							Port:   32000,
							Weight: 2,
						},
							{
								Host:   "10.32.2.3",
								Port:   32000,
								Weight: 1,
							}},
						DNSAliases: []string{"test.dummy.com"},
					},
					CloseTime: time.Time{},

					WfConfig: "provider.kubernetes",
				}},
				DBFile: "dummy.db",
			},
			args{msg: comm.Message{
				Action: comm.UpdateAction,
				Sender: "provider.kubernetes",
				Service: comm.Service{
					Provider:  "provider.kubernetes",
					Name:      "test",
					Namespace: "default",
					Targets: []comm.Target{{
						Host:   "10.32.2.2",
						Port:   32000,
						Weight: 2,
					},
						{
							Host:   "10.32.2.3",
							Port:   32000,
							Weight: 1,
						}},
					DNSAliases: []string{"test.dummy.com"},
				},
			}},
			true},
		{"NeedUpdateNotFound",
			fields{
				Mutex: sync.Mutex{},
				Entries: map[string]*workflowEntry{"dummy/default": &workflowEntry{
					Mutex:          sync.Mutex{},
					WorkInProgress: false,
					WIPTime:        time.Time{},
					State:          deployedState,
					ExpectedState:  deployedState,
					Service: comm.Service{
						Provider:  "provider.kubernetes",
						Name:      "dummy",
						Namespace: "default",
					},
					CloseTime: time.Time{},

					WfConfig: "provider.kubernetes",
				}},
				DBFile: "dummy.db",
			},
			args{msg: comm.Message{
				Action: comm.UpdateAction,
				Sender: "provider.kubernetes",
				Service: comm.Service{
					Provider:  "provider.kubernetes",
					Name:      "differs",
					Namespace: "default",
					Targets: []comm.Target{{
						Host:   "10.32.2.2",
						Port:   8080,
						Weight: 1,
					}},
					TLS:        false,
					PublicIP:   "",
					DNSAliases: nil,
				},
			}},
			true},
		{"NoUpdate",
			fields{
				Mutex: sync.Mutex{},
				Entries: map[string]*workflowEntry{"dummy/default": &workflowEntry{
					Mutex:          sync.Mutex{},
					WorkInProgress: false,
					WIPTime:        time.Time{},
					State:          deployedState,
					ExpectedState:  deployedState,
					Service: comm.Service{
						Provider:  "provider.kubernetes",
						Name:      "dummy",
						Namespace: "default",
						Targets: []comm.Target{{
							Host:   "10.32.2.2",
							Port:   8080,
							Weight: 1,
						}},
					},
				}},
				DBFile: "dummy.db",
			},
			args{msg: comm.Message{
				Action: comm.AddAction,
				Service: comm.Service{
					Provider:  "provider.kubernetes",
					Name:      "dummy",
					Namespace: "default",
					Targets: []comm.Target{{
						Host:   "10.32.2.2",
						Port:   8080,
						Weight: 1,
					}},
				},
			}},
			false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			we := &workflowEntries{
				Mutex:   tt.fields.Mutex,
				Entries: tt.fields.Entries,
				DBFile:  tt.fields.DBFile,
			}
			if got := we.serviceNeedUpdate(tt.args.msg); got != tt.want {
				t.Errorf("serviceNeedUpdate() = %v, want %v", got, tt.want)
			}
		})
	}
}

// helper to check entry has been updated
// and if service is same
func entryIsOk(e1, e2 *workflowEntry) bool {
	if time.Since(e1.LastUpdate) > 1*time.Second {
		return false
	}
	isSame, dif := e1.Service.IsSameThan(e2.Service)
	if !isSame {
		log.Warn(dif)
		return false
	}
	return true
}

func Test_workflowEntries_mergeMessage(t *testing.T) {
	ch := make(chan comm.Message)
	type fields struct {
		Mutex          sync.Mutex
		Entries        map[string]*workflowEntry
		DBFile         string
		WorkflowConfig string
		CommChan       chan comm.Message
	}
	type args struct {
		msg comm.Message
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   workflowEntries
	}{
		{"NoUpdate",
			fields{
				Mutex: sync.Mutex{},
				Entries: map[string]*workflowEntry{"dummy/default": &workflowEntry{
					Mutex:          sync.Mutex{},
					WorkInProgress: false,
					WIPTime:        time.Time{},
					State:          deployedState,
					ExpectedState:  deployedState,
					Service: comm.Service{
						Provider:  "provider.kubernetes",
						Name:      "dummy",
						Namespace: "default",
					},
					CloseTime: time.Time{},
					WfConfig:  "provider.kubernetes",
				}},
				DBFile: "dummy.db",
			},
			args{msg: comm.Message{
				Action: comm.AddAction,
				Sender: "provider.kubernetes",
				Service: comm.Service{
					Provider:  "provider.kubernetes",
					Name:      "dummy",
					Namespace: "default",
				},
			}},
			workflowEntries{
				Mutex: sync.Mutex{},
				Entries: map[string]*workflowEntry{"dummy/default": &workflowEntry{
					Mutex:          sync.Mutex{},
					WorkInProgress: false,
					WIPTime:        time.Time{},
					State:          deployedState,
					ExpectedState:  deployedState,
					Info:           "",
					Error:          "",
					TimeDetected:   time.Time{},
					LastUpdate:     time.Time{},
					Service: comm.Service{
						Provider:  "provider.kubernetes",
						Name:      "dummy",
						Namespace: "default",
					},
					CloseTime:     time.Time{},
					Reverse:       false,
					workflowSteps: nil,
					step:          nil,
					toExtension:   nil,
					WfConfig:      "provider.kubernetes",
				}},
				DBFile: "",
			},
		},
		{"Update",
			fields{
				Mutex: sync.Mutex{},
				Entries: map[string]*workflowEntry{"dummy/default": &workflowEntry{
					Mutex:          sync.Mutex{},
					WorkInProgress: false,
					WIPTime:        time.Time{},
					State:          undeployedState,
					ExpectedState:  deployedState,
					Service: comm.Service{
						Provider:  "provider.kubernetes",
						Name:      "dummy",
						Namespace: "default",
					},
					CloseTime:     time.Time{},
					WfConfig:      "provider.kubernetes",
					workflowSteps: initWorkflow("provider.kubernetes"),
				}},
				DBFile:         "dummy.db",
				WorkflowConfig: "provider.kubernetes",
			},
			args{msg: comm.Message{
				Action: comm.AddAction,
				Sender: "provider.kubernetes",
				Service: comm.Service{
					Provider:  "provider.kubernetes",
					Name:      "dummy",
					Namespace: "default",
				},
			}},
			workflowEntries{
				Mutex: sync.Mutex{},
				Entries: map[string]*workflowEntry{"dummy/default": &workflowEntry{
					Mutex:          sync.Mutex{},
					WorkInProgress: false,
					WIPTime:        time.Time{},
					State:          "",
					ExpectedState:  deployedState,
					Info:           "",
					Error:          "",
					TimeDetected:   time.Time{},
					LastUpdate:     time.Time{},
					Service: comm.Service{
						Provider:  "provider.kubernetes",
						Name:      "dummy",
						Namespace: "default",
					},
					CloseTime:     time.Time{},
					Reverse:       false,
					workflowSteps: nil,
					step:          nil,
					toExtension:   nil,
					WfConfig:      "provider.kubernetes",
				}},
				DBFile:         "",
				workflowConfig: "provider.kubernetes",
			},
		},
		{"UpdateNew",
			fields{
				Mutex:          sync.Mutex{},
				Entries:        map[string]*workflowEntry{"test/default": &workflowEntry{}},
				DBFile:         "dummy.db",
				WorkflowConfig: "provider.kubernetes",
				CommChan:       ch,
			},
			args{msg: comm.Message{
				Action: comm.AddAction,
				Sender: "provider.kubernetes",
				Service: comm.Service{
					Provider:  "provider.kubernetes",
					Name:      "dummy",
					Namespace: "default",
				},
			}},
			workflowEntries{
				Mutex: sync.Mutex{},
				Entries: map[string]*workflowEntry{"dummy/default": &workflowEntry{
					Mutex:          sync.Mutex{},
					WorkInProgress: false,
					WIPTime:        time.Time{},
					State:          "",
					ExpectedState:  deployedState,
					Info:           "",
					Error:          "",
					TimeDetected:   time.Time{},
					LastUpdate:     time.Time{},
					Service: comm.Service{
						Provider:  "provider.kubernetes",
						Name:      "dummy",
						Namespace: "default",
					},
					CloseTime:     time.Time{},
					Reverse:       false,
					workflowSteps: nil,
					step:          nil,
					toExtension:   nil,
					WfConfig:      "provider.kubernetes",
				}},
				DBFile:         "",
				workflowConfig: "provider.kubernetes",
				commChan:       ch,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			we := &workflowEntries{
				Mutex:          tt.fields.Mutex,
				Entries:        tt.fields.Entries,
				DBFile:         tt.fields.DBFile,
				workflowConfig: tt.fields.WorkflowConfig,
				commChan:       ch,
			}
			we.mergeMessage(tt.args.msg)
			//time.Sleep(1100*time.Millisecond)
			if !entryIsOk(we.Entries[tt.args.msg.GetServiceID()], tt.want.Entries[tt.args.msg.GetServiceID()]) {
				t.Errorf("mergeMessage() got = %v, want %v", we.Entries[tt.args.msg.Service.Name], tt.want.Entries[tt.args.msg.Service.Name])
			}
		})
	}
}
