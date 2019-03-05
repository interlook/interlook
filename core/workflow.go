package core

import (
	"encoding/json"
	"errors"
	"github.com/bhuisgen/interlook/log"
	"io/ioutil"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/bhuisgen/interlook/service"
)

// TODO: move msg related status to service pkg.
const (
	flowDeployedState      = "deployed"
	flowUndeployedState    = "undeployed"
	flowDeployState        = "deploy"
	flowUndeployState      = "undeploy"
	msgAddAction           = "add"
	msgUpdateAction        = "update"
	msgDeleteAction        = "delete"
	msgUpdateFromExtension = "extUpdate"
)

// workflow holds the sequence of "steps" an item must follow to be deployed or undeployed
type workflow map[int]string

// getNextStep returns the next step for a given step
// set reverse to true to get next step when undeploying a service
func (w workflow) getNextStep(step string, reverse bool) (next string, err error) {
	found := false
	var index int
	for k, v := range w {
		if v == step {
			found = true
			index = k
		}
	}
	if !found {
		return next, errors.New("could not find step in workflow")
	}
	if reverse {
		index = index - 1
	} else {
		index = index + 1
	}

	ok := false
	if next, ok = w[index]; !ok {
		return "", errors.New("could not find next step in workflow")
	}
	if strings.Contains(next, "provider.") {
		logger.DefaultLogger().Debugf("##### next:%v", next)
		next, _ = w.getNextStep(next, reverse)
		logger.DefaultLogger().Debugf("##### sub next:%v", next)
	}
	return next, nil
}

// flowentry represents a tracked service
type flowentry struct {
	// current step in the workflow
	//CurrentState string `json:"current_state,omitempty"`
	// Indicates if an extension is currently working on the item
	WorkInProgress bool `json:"work_in_progress,omitempty"`
	// Current state of the item
	State string `json:"state,omitempty"`
	// Desired service state (deployed or undeployed)
	ExpectedState string `json:"expected_state,omitempty"`
	// Additional info
	Info string `json:"info,omitempty"`
	// Last encountered Error
	Error string `json:"error,omitempty"`
	// First time the service was pushed by the provider
	TimeDetected time.Time `json:"time_detected,omitempty"`
	// Last time provider pushed an updated definition of the service
	LastUpdate time.Time       `json:"last_update,omitempty"`
	Service    service.Service `json:"service,omitempty"`
}

func makeNewFlowEntry() flowentry {
	var ne flowentry
	ne.TimeDetected = time.Now()
	return ne
}

func (we *flowentry) isStateAsWanted(action string) bool {
	if we.ExpectedState == we.State &&
		((we.State == flowDeployedState && action == msgAddAction) ||
			(we.State == flowUndeployedState && action == msgDeleteAction)) {
		logger.DefaultLogger().Debug("service state is OK")
		return true
	}
	return false
}

// flowEntries holds the table of tracked services
type flowEntries struct {
	sync.Mutex
	M map[string]*flowentry `json:"entries,omitempty"`
}

func newFlowEntries() *flowEntries {
	fe := new(flowEntries)
	fe.M = make(map[string]*flowentry)
	return fe
}

func (f *flowEntries) getServiceByName(name string) (*flowentry, error) {
	res, ok := f.M[name]
	if !ok {
		return res, errors.New("No entry found")
	}
	return res, nil
}

func (f *flowEntries) prepareForNextStep(entry, step string, reverse bool) {
	f.Lock()
	f.M[entry].WorkInProgress = true
	f.M[entry].State = step
	f.Unlock()
}

func (f *flowEntries) closeEntry(key string, reverse bool) {
	f.Lock()
	f.M[key].WorkInProgress = false
	if reverse {
		f.M[key].State = flowUndeployedState
	} else {
		f.M[key].State = flowDeployedState
	}
	f.Unlock()
}

func (f *flowEntries) save(file string) error {
	dbFile, err := os.OpenFile(file, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer dbFile.Close()

	data, err := json.Marshal(f.M)
	{
		if err != nil {
			return err
		}
	}
	_, err = dbFile.Write(data)
	if err != nil {
		return err
	}
	dbFile.Sync()

	return nil
}

func (f *flowEntries) loadFile(file string) error {
	dbFile, err := ioutil.ReadFile(file)
	if err != nil {
		return err
	}
	if err = json.Unmarshal(dbFile, &f.M); err != nil {
		return err
	}
	return nil
}

// initialize the workflow from config
func initWorkflow() workflow {
	var wf workflow
	wf = make(map[int]string)
	for k, v := range strings.Split(srv.config.Core.Workflow, ",") {
		wf[k+1] = v
	}
	// add start and end steps to workflow
	wf[0] = flowUndeployedState
	wf[len(wf)] = flowDeployedState
	return wf
}

func (w workflow) isLastStep(step string, reverse bool) bool {
	var lastStep string
	if !reverse {
		lastStep = w[len(w)-1]
	} else {
		lastStep = w[0]
	}
	if step != lastStep {
		return false
	}
	return true
}
