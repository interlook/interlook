package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/bhuisgen/interlook/log"
	"io/ioutil"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/bhuisgen/interlook/service"
)

const (
	flowDeployedState   = "deployed"
	flowUndeployedState = "undeployed"
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
	// we do not send messages to providers
	if strings.Contains(next, "provider.") {
		next, _ = w.getNextStep(next, reverse)
	}
	return next, nil
}

// flowEntry represents a tracked service
type flowEntry struct {
	// current step in the workflow
	//CurrentState string `json:"current_state,omitempty"`
	// Indicates if an extension is currently working on the item
	WorkInProgress bool `json:"work_in_progress,omitempty"`
	// time the entry was set in WIP (sent to extension)
	WIPTime time.Time
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

func makeNewFlowEntry() flowEntry {
	var ne flowEntry
	ne.TimeDetected = time.Now()
	return ne
}

func (fe *flowEntry) isStateAsWanted(action string) bool {
	if fe.ExpectedState == fe.State &&
		((fe.State == flowDeployedState && action == service.MsgAddAction) ||
			(fe.State == flowUndeployedState && action == service.MsgDeleteAction)) {
		logger.DefaultLogger().Debug("service state is OK")
		return true
	}
	return false
}

// flowEntries holds the table of tracked services
type flowEntries struct {
	sync.Mutex
	M map[string]*flowEntry `json:"entries,omitempty"`
}

func newFlowEntries() *flowEntries {
	fe := new(flowEntries)
	fe.M = make(map[string]*flowEntry)
	return fe
}

// mergeMessage manages messages received from extensions
func (f *flowEntries) mergeMessage(msg service.Message) error {
	logger.DefaultLogger().Debugf("mergeMessage received %v\n", msg)
	var serviceExist, serviceUnchanged, serviceStateOK bool

	// check if we already have this service
	serviceExist = true
	curSvc, err := f.getServiceByName(msg.Service.Name)
	if err != nil {
		logger.DefaultLogger().Debugf("Service %v: %v", msg.Service.Name, err)
		serviceExist = false
	}

	// check if service needs to be updated and if current state is as expected
	if serviceExist {
		logger.DefaultLogger().Debugf("mergeMessage service %v exist\n", msg.Service.Name)
		// Check service spec has not changed
		serviceUnchanged, _ = curSvc.Service.IsSameThan(msg.Service)
		// Check current state is as requested by msg
		serviceStateOK = curSvc.isStateAsWanted(msg.Action)
	}

	// if no changes are needed on existing service, we do nothing
	if serviceUnchanged && msg.Action == service.MsgAddAction && serviceStateOK {
		logger.DefaultLogger().Debugf("Service %v already in desired state\n", msg.Service.Name)
		return nil
	}

	switch msg.Action {
	case service.MsgAddAction:
		f.Lock()
		defer f.Unlock()
		ne := makeNewFlowEntry()
		ne.Service = msg.Service
		f.M[msg.Service.Name] = &ne
		f.M[msg.Service.Name].State = msg.Sender
		f.M[msg.Service.Name].ExpectedState = flowDeployedState

		if serviceExist {
			f.M[msg.Service.Name].LastUpdate = time.Now()
		}

		f.M[msg.Service.Name].Service = service.Service{
			Name:       msg.Service.Name,
			Hosts:      msg.Service.Hosts,
			DNSAliases: msg.Service.DNSAliases,
			Port:       msg.Service.Port,
			TLS:        msg.Service.TLS,
		}

	case service.MsgUpdateFromExtension:
		logger.DefaultLogger().Debugf("Got msg from extension")
		f.Lock()
		defer f.Unlock()
		f.M[msg.Service.Name].WorkInProgress = false
		f.M[msg.Service.Name].Service = msg.Service
		if msg.Error != "" {
			f.M[msg.Service.Name].Error = msg.Error
			return nil
		}

	case service.MsgDeleteAction:
		f.Lock()
		defer f.Unlock()
		f.M[msg.Service.Name].ExpectedState = flowUndeployedState
		f.M[msg.Service.Name].LastUpdate = time.Now()

	default:
		logger.DefaultLogger().Warnf("mergeToFlow could not handle %v action\n", msg.Action)
		return errors.New("Unhandled action")
	}

	return nil
}

func (f *flowEntries) getServiceByName(name string) (*flowEntry, error) {
	res, ok := f.M[name]
	if !ok {
		return res, errors.New(fmt.Sprintf("No entry found for %v", name))
	}
	return res, nil
}

func (f *flowEntries) prepareForNextStep(entry, step string, reverse bool) {
	f.Lock()
	f.M[entry].WorkInProgress = true
	f.M[entry].WIPTime = time.Now()
	f.M[entry].State = step
	f.Unlock()
}

func (f *flowEntries) closeEntry(key string, reverse bool) {
	f.Lock()
	f.M[key].WorkInProgress = false
	f.M[key].WIPTime = time.Time{}
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
	// add run and end steps to workflow
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
