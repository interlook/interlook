package core

import (
	"github.com/interlook/interlook/comm"
	"log"
	"sync"
	"testing"
)

func Test_receivedFromProvider_toNext_AddActionClose(t *testing.T) {
	ch := make(chan comm.Message)
	type args struct {
		we  *workflowEntry
		msg comm.Message
	}
	tests := []struct {
		name string
		args args
	}{
		{"AddActionToClose",
			args{
				we: &workflowEntry{
					Mutex:         sync.Mutex{},
					ExpectedState: "",
					workflowSteps: initWorkflow("provider.kubernetes"),
					toExtension:   ch,
					WfConfig:      "provider.kubernetes",
					step:          &receivedFromProvider{},
					State:         undeployedState,
				},
				msg: comm.Message{
					Action: comm.AddAction,
					Sender: "provider.kubernetes",
					Service: comm.Service{
						Provider:  "provider.kubernetes",
						Name:      "test",
						Namespace: "default",
						Targets:   nil,
						TLS:       false,
					},
				},
			}},
		{"DeleteAction",
			args{
				we: &workflowEntry{
					Mutex:         sync.Mutex{},
					ExpectedState: "",
					workflowSteps: initWorkflow("provider.kubernetes"),
					toExtension:   ch,
					WfConfig:      "provider.kubernetes",
					step:          &receivedFromProvider{},
					State:         deployedState,
				},
				msg: comm.Message{
					Action: comm.DeleteAction,
					Sender: "provider.kubernetes",
					Service: comm.Service{
						Provider:  "provider.kubernetes",
						Name:      "test",
						Namespace: "default",
						Targets:   nil,
						TLS:       false,
					},
				},
			}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.args.we.step.toNext(tt.args.we, tt.args.msg)
			if tt.args.we.CloseTime.IsZero() {
				t.Errorf("got %v", tt.args.we.CloseTime)
			}
		})
	}
}

func Test_receivedFromProvider_toNext_UnhandledAction(t *testing.T) {
	ch := make(chan comm.Message)
	type args struct {
		we  *workflowEntry
		msg comm.Message
	}
	tests := []struct {
		name string
		args args
	}{
		{"UnhandledAction",
			args{
				we: &workflowEntry{
					Mutex:         sync.Mutex{},
					ExpectedState: "",
					workflowSteps: initWorkflow("provider.kubernetes"),
					toExtension:   ch,
					WfConfig:      "provider.kubernetes",
					step:          &receivedFromProvider{},
					State:         undeployedState,
				},
				msg: comm.Message{
					Action: "unhandledAction",
					Sender: "provider.kubernetes",
					Service: comm.Service{
						Provider:  "provider.kubernetes",
						Name:      "test",
						Namespace: "default",
						Targets:   nil,
						TLS:       false,
					},
				},
			}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.args.we.step.toNext(tt.args.we, tt.args.msg)
			if tt.args.we.Error != "Unhandled action unhandledAction" {
				t.Errorf("got %v", tt.args.we)
			}
		})
	}
}

func Test_addFromProvider_toNext(t *testing.T) {
	type args struct {
		we  *workflowEntry
		msg comm.Message
	}
	tests := []struct {
		name string
		args args
	}{
		{"AddActionErrorNotAllowed",
			args{
				we: &workflowEntry{
					Mutex:         sync.Mutex{},
					ExpectedState: deployedState,
					workflowSteps: initWorkflow("provider.kubernetes"),
					WfConfig:      "provider.kubernetes",
					step:          &receivedFromProvider{},
					State:         deployedState,
				},
				msg: comm.Message{
					Action: "wrong",
					Sender: "provider.kubernetes",
					Service: comm.Service{
						Provider:  "provider.kubernetes",
						Name:      "test",
						Namespace: "default",
						Targets:   nil,
						TLS:       false,
					},
				},
			}},
		{"AddActionErrorUnderDeployment",
			args{
				we: &workflowEntry{
					Mutex:          sync.Mutex{},
					ExpectedState:  deployedState,
					workflowSteps:  initWorkflow("provider.kubernetes"),
					WfConfig:       "provider.kubernetes",
					step:           &receivedFromProvider{},
					State:          deployedState,
					WorkInProgress: true,
				},
				msg: comm.Message{
					Action: comm.AddAction,
					Sender: "provider.kubernetes",
					Service: comm.Service{
						Provider:  "provider.kubernetes",
						Name:      "test",
						Namespace: "default",
						Targets:   nil,
						TLS:       false,
					},
				},
			}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.args.we.step = &addFromProvider{}
			tt.args.we.step.toNext(tt.args.we, tt.args.msg)
			if tt.args.we.State != deployedState || !tt.args.we.LastUpdate.IsZero() {
				t.Errorf("got %v ,want %v", tt.args.we.State, deployedState)
			}
		})
	}
}

func Test_receivedFromProvisioner_toNext_Error(t *testing.T) {
	type args struct {
		we  *workflowEntry
		msg comm.Message
	}
	tests := []struct {
		name string
		args args
	}{
		{"AddActionErrorNotPossible",
			args{
				we: &workflowEntry{
					Mutex:          sync.Mutex{},
					ExpectedState:  deployedState,
					workflowSteps:  initWorkflow("provider.kubernetes"),
					WfConfig:       "provider.kubernetes",
					State:          deployedState,
					WorkInProgress: true,
				},
				msg: comm.Message{
					Action: "wrong",
					Sender: "provider.kubernetes",
					Service: comm.Service{
						Provider:  "provider.kubernetes",
						Name:      "test",
						Namespace: "default",
						Targets:   nil,
						TLS:       false,
					},
				},
			}},
		{"AddActionErrorMsgError",
			args{
				we: &workflowEntry{
					Mutex:          sync.Mutex{},
					ExpectedState:  deployedState,
					workflowSteps:  initWorkflow("provider.kubernetes"),
					WfConfig:       "provider.kubernetes",
					State:          "provider.kubernetes",
					WorkInProgress: true,
				},
				msg: comm.Message{
					Action: comm.AddAction,
					Sender: "provider.kubernetes",
					Error:  "error",
					Service: comm.Service{
						Provider:  "provider.kubernetes",
						Name:      "test",
						Namespace: "default",
						Targets:   nil,
						TLS:       false,
					},
				},
			}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.args.we.step = &receivedFromProvisioner{}
			tt.args.we.step.toNext(tt.args.we, tt.args.msg)
			if len(tt.args.we.Error) == 0 {
				t.Errorf("got %v", tt.args.we.Error)
			}
		})
	}
}

func Test_receivedFromProvisioner_toNext(t *testing.T) {
	ch := make(chan comm.Message)
	type args struct {
		we  *workflowEntry
		msg comm.Message
	}
	tests := []struct {
		name string
		args args
	}{
		{"AddActionCloseWip",
			args{
				we: &workflowEntry{
					Mutex:          sync.Mutex{},
					ExpectedState:  deployedState,
					workflowSteps:  initWorkflow("provider.kubernetes"),
					WfConfig:       "provider.kubernetes",
					State:          "provider.kubernetes",
					WorkInProgress: true,
					toExtension:    ch,
				},
				msg: comm.Message{
					Action: comm.AddAction,
					Sender: "provider.kubernetes",
					Service: comm.Service{
						Provider:  "provider.kubernetes",
						Name:      "test",
						Namespace: "default",
						Targets:   nil,
						TLS:       false,
					},
				},
			}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.args.we.step = &receivedFromProvisioner{}
			go listenChan(tt.args.we.toExtension)
			tt.args.we.step.toNext(tt.args.we, tt.args.msg)
			if tt.args.we.CloseTime.IsZero() {
				t.Errorf("got %v", tt.args.we.Error)
			}
		})
	}
}

func listenChan(c chan comm.Message) {
	m := <-c
	log.Print(m)
}
