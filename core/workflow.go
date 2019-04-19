package core

// TODO: Add management of flows in error
import (
	"encoding/json"
	"errors"
	"github.com/bhuisgen/interlook/log"
	"github.com/bhuisgen/interlook/messaging"
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

// workflow holds the sequence of "steps" an item must follow to be deployed or undeployed
//type workflowSteps map[int]string

type workflowSteps []workflowStep

type workflowStep struct {
	id         int
	name       string
	transition workflowState
}

// initialize the workflow from config
func initWorkflow(workflowConfig string) {

	for k, v := range strings.Split(workflowConfig, ",") {
		var transitionState workflowState

		extType := strings.Split(v, ".")[0]

		if extType == "provider" {
			transitionState = &providerState{}
		} else {
			transitionState = &provisionerState{}
		}

		workflow = append(workflow, workflowStep{
			id:         k + 1,
			name:       v,
			transition: transitionState,
		})
	}

	// add start and end steps to workflow
	workflow = append(workflow, workflowStep{
		id:         0,
		name:       undeployedState,
		transition: &closeState{},
	})

	workflow = append(workflow, workflowStep{
		id:         len(workflow),
		name:       deployedState,
		transition: &closeState{},
	})

	log.Infof("workflow initialized %v", workflow)

}

func (w workflowSteps) isLastStep(step string, reverse bool) bool {
	var lastStep string

	id := 0

	if !reverse {
		id = len(w) - 1
	}
	log.Debugf("########## isLast ID:%v", id)
	for _, step := range w {
		if step.id == id {
			lastStep = step.name
		}
	}
	log.Debugf("########## isLast lastStep %v:%v", step, lastStep)
	if step != lastStep {
		return false
	}

	return true
}

func (w workflowSteps) getTransition(step string) workflowState {

	for _, workflowStep := range w {
		if workflowStep.name == step {
			return workflowStep.transition
		}
	}
	return nil
}

// getNextStep returns the next step for a given step
// set reverse to true to get next step when undeploying a service
func (w workflowSteps) getNextStep(currentStep string, reverse bool) (nextStep string, next workflowState, err error) {
	found := false
	var stepID int

	for _, v := range w {
		if v.name == currentStep {
			found = true
			stepID = v.id
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
		if step.id == stepID {
			nextStep = step.name
			next = step.transition
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
	LastUpdate time.Time         `json:"last_update,omitempty"`
	Service    messaging.Service `json:"service,omitempty"`
	CloseTime  time.Time         `json:"close_time"`
	next       workflowState
}

func makeNewFlowEntry(msg messaging.Message) *workflowEntry {
	var ne workflowEntry
	ne.TimeDetected = time.Now()
	ne.State = msg.Sender
	ne.State = undeployedState
	ne.ExpectedState = deployedState
	ne.setNextStep()
	return &ne
}

func (we *workflowEntry) setError(err string) {
	we.Lock()
	we.Error = err
	we.Unlock()
}

func (we *workflowEntry) setWIP(wip bool) {
	we.Lock()
	if wip {
		we.WIPTime = time.Now()
	} else {
		we.WIPTime = time.Time{}
	}
	we.WorkInProgress = wip
	we.Unlock()
}

func (we *workflowEntry) setExpectedState(state string) {
	we.Lock()
	we.ExpectedState = state
	we.Unlock()
}

func (we *workflowEntry) setLastUpdate() {
	we.Lock()
	we.LastUpdate = time.Now()
	we.Unlock()
}

func (we *workflowEntry) setTransition(state string) {
	we.Lock()
	we.next = workflow.getTransition(state)
	we.Unlock()
}

// setNextStep set the next workflow step of the entry
func (we *workflowEntry) setNextStep() {

	nextStep, next, err := workflow.getNextStep(we.State, we.isReverse())
	if err != nil {
		log.Errorf("Error getting next step for %v:%v", we.State, err)
		return
	}
	log.Debugf("#### nextStep for %v is %v", we.State, nextStep)
	we.Lock()
	we.State = nextStep
	we.next = next
	we.CloseTime = time.Time{}
	we.Unlock()

}

// setState update flow entry with info from message
func (we *workflowEntry) setState(msg messaging.Message, wip bool) {
	we.Lock()
	we.State = msg.Sender
	we.WorkInProgress = wip
	we.Error = msg.Error
	we.CloseTime = time.Time{}
	we.Unlock()
}

// updateFromMsg update service with info coming from provider or ipam extensions only
func (we *workflowEntry) updateServiceFromMsg(msg messaging.Message) {
	we.Lock()
	if strings.HasPrefix(msg.Sender, "provider.") && msg.Action != messaging.DeleteAction {
		we.Service.Port = msg.Service.Port
		we.Service.Hosts = msg.Service.Hosts
		we.Service.TLS = msg.Service.TLS
		we.Service.DNSAliases = msg.Service.DNSAliases
	}

	if strings.HasPrefix(msg.Sender, "ipam.") {
		we.Service.PublicIP = msg.Service.PublicIP
	}

	we.Service.Name = msg.Service.Name
	we.Unlock()
}

func (we *workflowEntry) sendToExtension() {
	//we.setNextStep()
	we.setWIP(true)
	msg := messaging.BuildMessage(we.Service, we.isReverse())
	msg.Destination = we.State
	coreForwardMessage <- msg

}

// close closes the entry workflow
func (we *workflowEntry) close(errorMessage string) {

	we.setWIP(false)

	if errorMessage != "" {
		we.setError(errorMessage)
	}

	we.Lock()

	if we.isReverse() {
		we.State = undeployedState
	} else {
		we.State = deployedState
	}

	we.CloseTime = time.Now()

	we.Unlock()

	log.Infof("Service %v state %v", we.Service.Name, we.State)

}

// isReverse returns true if the target state is undeployed
func (we *workflowEntry) isReverse() bool {
	if we.ExpectedState == undeployedState {
		return true
	}
	return false
}

// isStateAsWanted compares current state with expected state
func (we *workflowEntry) isStateAsWanted(action string) bool {
	if we.ExpectedState == we.State &&
		((we.State == deployedState && action == messaging.AddAction) ||
			(we.State == undeployedState && action == messaging.DeleteAction)) {
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

func (we *workflowEntries) isServiceNeedUpdate(msg messaging.Message) bool {

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

func (we *workflowEntries) messageHandler(msg messaging.Message) error {

	if !we.isServiceNeedUpdate(msg) {
		log.Debugf("Service %v already in desired state\n", msg.Service.Name)
		we.Entries[msg.Service.Name].setLastUpdate()
		// add update timestamp
		return nil
	}

	we.Lock()
	defer we.Unlock()

	_, ok := we.Entries[msg.Service.Name]
	if !ok {
		log.Debugf("#### msgHandler service not found, creating it %v", msg)
		we.Entries[msg.Service.Name] = makeNewFlowEntry(msg)
	}

	entry, _ := we.Entries[msg.Service.Name]
	entry.updateServiceFromMsg(msg)
	entry.setLastUpdate()
	entry.setTransition(msg.Sender)
	go entry.next.execTransition(entry, msg)

	return nil
}

// closeEntry closes the entry workflow
func (we *workflowEntries) closeEntry(serviceName, error string, reverse bool) {

	_, ok := we.Entries[serviceName]
	if !ok {
		log.Errorf("Entry %v not found while trying to close it", serviceName)
		return
	}

	we.Entries[serviceName].Lock()
	we.Entries[serviceName].WorkInProgress = false
	we.Entries[serviceName].WIPTime = time.Time{}
	we.Entries[serviceName].CloseTime = time.Now()

	if reverse {
		we.Entries[serviceName].State = undeployedState
	} else {
		we.Entries[serviceName].State = deployedState
	}

	we.Entries[serviceName].Unlock()

	log.Infof("Service %v state %v", serviceName, we.Entries[serviceName].State)

}

// save save entries list to file
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

// loadFile load entries list from file
func (we *workflowEntries) loadFile() error {
	file, err := ioutil.ReadFile(we.DBFile)
	if err != nil {
		return err
	}

	if err = json.Unmarshal(file, &we.Entries); err != nil {
		return err
	}

	return nil
}
