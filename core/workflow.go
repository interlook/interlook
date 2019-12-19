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

// workflow holds the sequence of "steps" an item must follow to be deployed or un-deployed

type workflowSteps []workflowStep

type workflowStep struct {
	ID         int
	Name       string
	Transition transition
}

// initialize the workflow from config
func initWorkflow(workflowConfig string) {

	for k, v := range strings.Split(workflowConfig, ",") {
		var transitionState transition

		extType := strings.Split(v, ".")[0]

		if extType == "provider" {
			transitionState = &providerState{}
		} else {
			transitionState = &provisionerState{}
		}

		workflow = append(workflow, workflowStep{
			ID:         k + 1,
			Name:       v,
			Transition: transitionState,
		})
	}

	// add start and end steps to workflow
	workflow = append(workflow, workflowStep{
		ID:         0,
		Name:       undeployedState,
		Transition: &closeState{},
	})

	workflow = append(workflow, workflowStep{
		ID:         len(workflow),
		Name:       deployedState,
		Transition: &closeState{},
	})

	log.Infof("workflow initialized %v", workflow)

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
func (w workflowSteps) getTransition(step string) transition {

	for _, workflowStep := range w {
		if workflowStep.Name == step {
			return workflowStep.Transition
		}
	}
	return nil
}

// getNextStep returns the transition step for a given step
// set reverse to true to get transition step when undeploying a service
func (w workflowSteps) getNextStep(currentStep string, reverse bool) (nextStep string, next transition, err error) {
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

// workflowEntry represents a tracked service
type workflowEntry struct {
	sync.Mutex
	// Indicates if an extension is currently working on the item
	WorkInProgress bool `json:"work_in_progress,omitempty"`
	// time the entry was set in WIP (sent to extension)
	WIPTime time.Time `json:"wip_time"`
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
	LastUpdate time.Time    `json:"last_update,omitempty"`
	Service    comm.Service `json:"service,omitempty"`
	CloseTime  time.Time    `json:"close_time"`
	transition transition
}

func makeNewFlowEntry() *workflowEntry {
	var ne workflowEntry
	ne.TimeDetected = time.Now()
	ne.State = undeployedState
	ne.ExpectedState = deployedState
	ne.setNextStep()
	return &ne
}

func (e *workflowEntry) setError(err string) {
	e.Lock()
	e.Error = err
	e.Unlock()
}

func (e *workflowEntry) setWIP(wip bool) {
	e.Lock()
	if wip {
		e.WIPTime = time.Now()
	} else {
		e.WIPTime = time.Time{}
	}
	e.WorkInProgress = wip
	e.Unlock()
}

func (e *workflowEntry) setTargetState(state string) {
	e.Lock()
	e.ExpectedState = state
	e.Unlock()
}

// setLastUpdate to now()
func (e *workflowEntry) setLastUpdate() {
	e.Lock()
	e.LastUpdate = time.Now()
	e.Unlock()
}

// setTransition based on given state
func (e *workflowEntry) setTransition(state string) {
	e.Lock()
	e.transition = workflow.getTransition(state)
	e.Unlock()
}

// setNextStep in the workflow
func (e *workflowEntry) setNextStep() {

	nextStep, next, err := workflow.getNextStep(e.State, e.isReverse())
	if err != nil {
		log.Errorf("Error getting transition step for %v:%v", e.State, err)
		return
	}

	log.Debugf("#### nextStep for %v is %v", e.State, nextStep)
	e.Lock()
	e.State = nextStep
	e.transition = next
	e.WorkInProgress = false
	e.WIPTime = time.Time{}
	e.CloseTime = time.Time{}
	e.Unlock()

}

// setState update flow entry with info from message
func (e *workflowEntry) setState(msg comm.Message, wip bool) {
	e.Lock()
	e.State = msg.Sender
	e.WorkInProgress = wip
	e.Error = msg.Error
	e.CloseTime = time.Time{}
	e.Unlock()
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

	e.Service.Name = msg.Service.Name
	e.Unlock()
}

func (e *workflowEntry) sendToExtension() {
	//e.setNextStep()
	e.setWIP(true)
	msg := comm.BuildMessage(e.Service, e.isReverse())
	msg.Destination = e.State
	msgToExtension <- msg

}

// close closes the entry workflow
func (e *workflowEntry) close(errorMessage string) {

	e.setWIP(false)

	if errorMessage != "" {
		e.setError(errorMessage)
	}

	e.Lock()

	if e.isReverse() {
		e.State = undeployedState
	} else {
		e.State = deployedState
	}

	e.CloseTime = time.Now()

	e.Unlock()

	log.Infof("Service %v state %v", e.Service.Name, e.State)

}

// isReverse returns true if the target state is undeployed
func (e *workflowEntry) isReverse() bool {
	if e.ExpectedState == undeployedState {
		return true
	}
	return false
}

// isStateAsWanted compares current state with expected state
func (e *workflowEntry) isStateAsWanted(action string) bool {
	if e.ExpectedState == e.State &&
		((e.State == deployedState && action == comm.AddAction) ||
			(e.State == undeployedState && action == comm.DeleteAction)) {
		log.Debug("service state is OK")
		return true
	}

	return false
}

// workflowEntries holds the table of tracked services
type workflowEntries struct {
	sync.Mutex
	Entries map[string]*workflowEntry `json:"entries,omitempty"`
	DBFile  string
}

func initWorkflowEntries(dbFile string) *workflowEntries {
	fe := new(workflowEntries)
	fe.Entries = make(map[string]*workflowEntry)
	fe.DBFile = dbFile

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
func (we *workflowEntries) mergeMessage(msg comm.Message) error {

	if !we.serviceNeedUpdate(msg) {
		log.Debugf("Service %v already in desired state\n", msg.Service.Name)
		we.Entries[msg.Service.Name].setLastUpdate()
		return nil
	}

	we.Lock()
	defer we.Unlock()

	_, ok := we.Entries[msg.Service.Name]
	if !ok {
		log.Debugf("Service not found, creating it %v", msg)
		we.Entries[msg.Service.Name] = makeNewFlowEntry()
	}

	entry, _ := we.Entries[msg.Service.Name]
	entry.updateService(msg)
	entry.setTransition(msg.Sender)

	go entry.transition.execute(entry, msg)

	return nil
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
func (we *workflowEntries) load() error {
	file, err := ioutil.ReadFile(we.DBFile)
	if err != nil {
		return err
	}

	if err = json.Unmarshal(file, &we.Entries); err != nil {
		return err
	}

	return nil
}
