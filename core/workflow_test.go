package core

import (
	"github.com/interlook/interlook/comm"
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
	wf = initWorkflow("provider.swarm")
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
			Transition: &providerState{},
		},
		workflowStep{
			ID:         2,
			Name:       "provisioner.two",
			Transition: &provisionerState{},
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
		wantNext     transition
		wantErr      bool
	}{
		{"provider", testWF, args{
			currentStep: "undeployed",
			reverse:     false,
		}, "provisioner.two", &provisionerState{}, false},
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
		want transition
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
func Test_makeNewFlowEntry(t *testing.T) {
	tests := []struct {
		name string
		want *workflowEntry
	}{
		{"simple", &workflowEntry{
			Mutex:          sync.Mutex{},
			WorkInProgress: false,
			WIPTime:        time.Time{},
			State:          undeployedState,
			ExpectedState:  deployedState,
			Info:           "",
			Error:          "",
			TimeDetected:   time.Time{},
			LastUpdate:     time.Time{},
			Service:        comm.Service{},
			CloseTime:      time.Time{},
			transition:     &provisionerState{},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := makeNewFlowEntry(); !weIsEqual(got, tt.want) {
				t.Errorf("makeNewFlowEntry() = %v, want %v", got, tt.want)
			}
		})
	}
}
