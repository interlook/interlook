package core

import (
	"encoding/json"
	"errors"
	"github.com/interlook/interlook/comm"
	"github.com/interlook/interlook/log"
	"io/ioutil"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	deployedState   = "deployed"
	undeployedState = "undeployed"
)

// workflowStep defines a step in the workflow
type workflowStep struct {
	ID         int
	Name       string
	Transition transitioner
}

type workflowSteps []workflowStep

/*type wfEntryHandler interface {
    Lock()
    Unlock()
}*/

// workflowEntry represents a tracked service
type workflowEntry struct {
	sync.Mutex
	// Indicates if an extension is currently working on the item
	WorkInProgress bool `json:"work_in_progress,omitempty"`
	// time the entry was set in WIP (sent to extension)
	WIPTime time.Time `json:"wip_time"`
	// Current step as defined in the workflow config
	State string `json:"step,omitempty"`
	// Desired service step (deployed or undeployed)
	ExpectedState string `json:"expected_state,omitempty"`
	// Additional info
	Info string `json:"info,omitempty"`
	// Last encountered Error
	Error string `json:"error,omitempty"`
	// First time the service was pushed by the provider
	TimeDetected time.Time `json:"time_detected,omitempty"`
	// Last time provider pushed an updated definition of the service
	LastUpdate time.Time    `json:"last_update,omitempty"`
	Service    comm.Service `json:"service,omitempty"`
	CloseTime  time.Time    `json:"close_time"`
	// direction in the workflow
	Reverse bool `json:"reverse"`
	// steps the item must follow in the workflow
	workflowSteps workflowSteps
	// current transition step
	step        transitioner
	toExtension chan comm.Message
	// Stores workflow config string
	WfConfig string `json:"workflow"`
}

// initialize the workflow from config
func initWorkflow(workflowConfig string) workflowSteps {
	var wf workflowSteps
	for k, v := range strings.Split(workflowConfig, ",") {
		var transitionState transitioner

		extType := strings.Split(v, ".")[0]

		if extType == "provider" {
			transitionState = &receivedFromProvider{}
		} else {
			transitionState = &receivedFromProvisioner{}
		}

		wf = append(wf, workflowStep{
			ID:         k + 1,
			Name:       v,
			Transition: transitionState,
		})
	}

	// add start and end steps to workflow
	wf = append(wf, workflowStep{
		ID:         0,
		Name:       undeployedState,
		Transition: &closeState{},
	})

	wf = append(wf, workflowStep{
		ID:         len(wf),
		Name:       deployedState,
		Transition: &closeState{},
	})

	log.Debugf("workflow initialized %v", wf)
	return wf

}

func (w workflowSteps) isLastStep(step string, reverse bool) bool {
	var lastStep string

	id := 0

	if !reverse {
		id = len(w) - 1
	}

	for _, step := range w {
		if step.ID == id {
			lastStep = step.Name
		}
	}

	if step != lastStep {
		return false
	}

	return true
}

// getTransition for the given step
func (w workflowSteps) getTransition(step string) transitioner {

	for _, workflowStep := range w {
		if workflowStep.Name == step {
			return workflowStep.Transition
		}
	}
	return nil
}

// getNextStep returns the next step for a given step
// set reverse to true to get next step when undeploying a service
func (w workflowSteps) getNextStep(currentStep string, reverse bool) (nextStep string, next transitioner, err error) {
	found := false
	var stepID int

	for _, v := range w {
		if v.Name == currentStep {
			found = true
			stepID = v.ID
		}
	}

	if !found {
		return nextStep, next, errors.New("could not find currentStep in workflow")
	}

	if reverse {
		stepID = stepID - 1
	} else {
		stepID = stepID + 1
	}
	found = false
	for _, step := range w {
		if step.ID == stepID {
			nextStep = step.Name
			next = step.Transition
			found = true
		}
	}

	if !found {
		return nextStep, next, errors.New("could not find nextStep currentStep in workflow")
	}

	// we do not send messages to providers
	if strings.HasPrefix(nextStep, "provider.") {
		nextStep, next, _ = w.getNextStep(nextStep, reverse)
	}

	return nextStep, next, nil
}

func (e *workflowEntry) setWIP(wip bool) {

	if wip {
		e.WIPTime = time.Now()
	} else {
		e.WIPTime = time.Time{}
	}
	e.WorkInProgress = wip

}

// setNextStep in the workflow
func (e *workflowEntry) setNextStep() {

	nextStep, next, err := e.workflowSteps.getNextStep(e.State, e.isReverse())
	if err != nil {
		log.Errorf("Error getting next step for %v:%v", e.State, err)
		return
	}

	log.Debugf("#### nextStep for %v is %v", e.State, nextStep)

	e.State = nextStep
	e.step = next
	e.WorkInProgress = false
	e.WIPTime = time.Time{}
	e.CloseTime = time.Time{}

}

// setState update flow entry with message information
func (e *workflowEntry) setState(msg comm.Message, wip bool) {
	e.State = msg.Sender
	e.WorkInProgress = wip
	e.Error = msg.Error
	e.CloseTime = time.Time{}
}

// updateService from given message
// only provider and ipam can update service definition
func (e *workflowEntry) updateService(msg comm.Message) {
	e.Lock()
	if strings.HasPrefix(msg.Sender, "provider.") && msg.Action != comm.DeleteAction {
		e.Service.Targets = msg.Service.Targets
		e.Service.TLS = msg.Service.TLS
		e.Service.DNSAliases = msg.Service.DNSAliases
	}

	if strings.HasPrefix(msg.Sender, "ipam.") {
		e.Service.PublicIP = msg.Service.PublicIP
	}
	e.LastUpdate = time.Now()
	e.Service.Name = msg.Service.Name
	e.Service.Namespace = msg.Service.Namespace
	e.Unlock()
}

// sendToExtension. core.messageSender will forward to the right extension
func (e *workflowEntry) sendToExtension() {
	//e.setNextStep()
	e.setWIP(true)
	msg := comm.BuildMessage(e.Service, e.isReverse())
	msg.Destination = e.State
	e.toExtension <- msg
}

// close closes the entry workflow
func (e *workflowEntry) close(errorMessage string) {
	e.Lock()
	defer e.Unlock()

	e.setWIP(false)

	if errorMessage != "" {
		e.Error = errorMessage
	}

	if e.isReverse() {
		e.State = undeployedState
	} else {
		e.State = deployedState
	}

	e.CloseTime = time.Now()

	log.Infof("Service %v step %v", e.Service.Name, e.State)

}

// isReverse returns true if the target step is undeployed
func (e *workflowEntry) isReverse() bool {
	if e.ExpectedState == undeployedState {
		return true
	}
	return false
}

// isStateAsWanted compares current step with expected step
func (e *workflowEntry) isStateAsWanted(action string) bool {
	if e.ExpectedState == e.State &&
		((e.State == deployedState && action == comm.AddAction) ||
			(e.State == undeployedState && action == comm.DeleteAction)) {
		log.Debug("service step is OK")
		return true
	}

	return false
}

// workflowEntries holds the table of tracked services
type workflowEntries struct {
	sync.Mutex
	Entries        map[string]*workflowEntry `json:"entries,omitempty"`
	DBFile         string
	workflowConfig string
	commChan       chan comm.Message
}

func newWorkflowEntries(dbFile, workflowConfig string, commChan chan comm.Message) *workflowEntries {
	fe := new(workflowEntries)
	fe.Entries = make(map[string]*workflowEntry)
	fe.DBFile = dbFile
	fe.workflowConfig = workflowConfig
	fe.commChan = commChan
	return fe
}

func (we *workflowEntries) serviceNeedUpdate(msg comm.Message) bool {
	curSvc, ok := we.Entries[msg.Service.Name]
	if !ok {
		log.Debugf("Service %v not found ", msg.Service.Name)
		return true
	}

	serviceIsSame, _ := curSvc.Service.IsSameThan(msg.Service)
	if !serviceIsSame {
		return true
	}

	serviceStateOK := curSvc.isStateAsWanted(msg.Action)
	if !serviceStateOK {
		return true
	}

	return false
}

// mergeMessage by inserting/merging it to the workflow entries list
func (we *workflowEntries) mergeMessage(msg comm.Message) {

	if !we.serviceNeedUpdate(msg) {
		log.Debugf("Service %v already in desired step\n", msg.Service.Name)
		we.Entries[msg.Service.Name].LastUpdate = time.Now()
		return
	}

	we.Lock()
	defer we.Unlock()

	_, ok := we.Entries[msg.Service.Name]
	if !ok {
		log.Debugf("Service not found, creating it %v", msg)
		we.addEntry(msg.Service.Name)
	}

	//entry, _ := we.Entries[msg.Service.Name]
	we.Entries[msg.Service.Name].updateService(msg)
	we.Entries[msg.Service.Name].step = we.Entries[msg.Service.Name].workflowSteps.getTransition(msg.Sender)

	go we.Entries[msg.Service.Name].step.toNext(we.Entries[msg.Service.Name], msg)

	return
}

func (we *workflowEntries) addEntry(name string) {
	var e workflowEntry

	e.WfConfig = we.workflowConfig
	e.workflowSteps = initWorkflow(e.WfConfig)
	e.toExtension = we.commChan
	e.TimeDetected = time.Now()
	e.State = undeployedState
	e.ExpectedState = deployedState
	e.setNextStep()

	we.Entries[name] = &e
}

// save entries list to file
func (we *workflowEntries) save() error {

	dbFile, err := os.OpenFile(we.DBFile, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return err
	}

	if err := dbFile.Truncate(0); err != nil {
		log.Error(err)
	}

	_, err = dbFile.Seek(0, 0)
	if err != nil {
		log.Error(err)
	}

	defer func() {
		if err := dbFile.Close(); err != nil {
			log.Errorf("Error closing filename %v", err)
		}
	}()

	data, err := json.Marshal(we.Entries)
	if err != nil {
		return err
	}

	_, err = dbFile.Write(data)
	if err != nil {
		return err
	}

	if err := dbFile.Sync(); err != nil {
		log.Errorf("Error syncing db file %v", err)
	}

	return nil
}

// load entries list from file
func (we *workflowEntries) load(extChan chan comm.Message) error {
	file, err := ioutil.ReadFile(we.DBFile)
	if err != nil {
		return err
	}

	if err = json.Unmarshal(file, &we.Entries); err != nil {
		return err
	}

	for _, e := range we.Entries {
		e.workflowSteps = initWorkflow(e.WfConfig)
		e.toExtension = extChan
		if e.State == deployedState || e.State == undeployedState {
			e.step = &closeState{}
		}
	}

	return nil
}
