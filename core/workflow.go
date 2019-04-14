package core

// TODO: Add management of flows in error
import (
	"encoding/json"
	"errors"
	"fmt"
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
type workflow map[int]string

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

// getNextStep returns the next step for a given step
// set reverse to true to get next step when undeploying a service
func (w workflow) getNextStep(currentStep string, reverse bool) (nextStep string, err error) {
	found := false
	var index int

	for k, v := range w {
		if v == currentStep {
			found = true
			index = k
		}
	}

	if !found {
		return nextStep, errors.New("could not find currentStep in workflow")
	}

	if reverse {
		index = index - 1
	} else {
		index = index + 1
	}

	ok := false
	if nextStep, ok = w[index]; !ok {
		return "", errors.New("could not find nextStep currentStep in workflow")
	}

	// we do not send messages to providers
	if strings.Contains(nextStep, "provider.") {
		nextStep, _ = w.getNextStep(nextStep, reverse)
	}

	return nextStep, nil
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
}

func makeNewFlowEntry() workflowEntry {
	var ne workflowEntry
	ne.TimeDetected = time.Now()

	return ne
}

// isReverse returns true if the target state is undeployed
func (we *workflowEntry) isReverse() bool {
	if we.ExpectedState == undeployedState {
		return true
	}
	return false
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

// updateState update flow entry with info from message
func (we *workflowEntry) updateState(msg messaging.Message, wip bool) {
	we.Lock()
	we.State = msg.Sender
	we.WorkInProgress = wip
	we.Error = msg.Error
	we.CloseTime = time.Time{}
	we.Unlock()
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

// messageHandler merge messages received from extensions to core's internal entries list
func (we *workflowEntries) messageHandler(msg messaging.Message) error {
	log.Debugf("messageHandler received %v\n", msg)
	var serviceExist, serviceUnchanged, serviceStateOK bool

	// check if we already have this service
	serviceExist = true
	curSvc, err := we.getServiceByName(msg.Service.Name)
	if err != nil {
		log.Debugf("Service %v: %v", msg.Service.Name, err)
		serviceExist = false
	}

	// check if service needs to be updated and if current state is as expected
	if serviceExist {
		log.Debugf("messageHandler service %v exist\n", msg.Service.Name)
		// Check service spec has not changed
		serviceUnchanged, _ = curSvc.Service.IsSameThan(msg.Service)
		// Check current state is as requested by msg
		serviceStateOK = curSvc.isStateAsWanted(msg.Action)
	}

	// if no changes are needed on existing service, we do nothing but update LastUpdate
	if serviceUnchanged && msg.Action == messaging.AddAction && serviceStateOK {
		log.Debugf("Service %v already in desired state\n", msg.Service.Name)
		we.Entries[curSvc.Service.Name].setLastUpdate()

		return nil
	}

	switch msg.Action {
	case messaging.AddAction, messaging.UpdateAction:

		if !serviceExist {
			log.Infof("Registering new service entry %v", msg.Service.Name)
			ne := makeNewFlowEntry()
			we.Entries[msg.Service.Name] = &ne
		}

		// only provider can change desired state
		if strings.Contains(msg.Sender, "provider.") {

			if serviceExist && we.Entries[msg.Service.Name].CloseTime.IsZero() && we.Entries[msg.Service.Name].Error == "" {
				log.Infof("%v action from %v ignored as service %v is still under deployment", msg.Action, msg.Sender, msg.Service.Name)
				return nil
			}
			we.Entries[msg.Service.Name].setExpectedState(deployedState)
		}

		we.Entries[msg.Service.Name].updateState(msg, false)
		we.Entries[msg.Service.Name].Lock()
		we.Entries[msg.Service.Name].Service.UpdateFromMsg(msg)
		we.Entries[msg.Service.Name].Unlock()

		if serviceExist {
			log.Infof("Service %v state %v", msg.Service.Name, we.Entries[msg.Service.Name].State)
		}

	case messaging.DeleteAction:
		_, ok := we.Entries[msg.Service.Name]
		if !ok {
			log.Warnf("No entry found for service %v", msg.Service.Name)
			return nil
		}
		log.Infof("Request to un-deploy service %v", msg.Service.Name)
		we.Entries[msg.Service.Name].setExpectedState(undeployedState)
		we.Entries[msg.Service.Name].setLastUpdate()

	default:
		log.Warnf("messageHandler could not handle %v action\n", msg.Action)
		return errors.New("unhandled action")
	}

	return nil
}

// getServiceByName return the current entry for a given service
func (we *workflowEntries) getServiceByName(name string) (*workflowEntry, error) {
	res, ok := we.Entries[name]
	if !ok {
		return res, errors.New(fmt.Sprintf("No entry found for %v", name))
	}

	return res, nil
}

// setNextStep set the next workflow step of the entry
func (we *workflowEntries) setNextStep(entry, step string, reverse bool) {
	_, ok := we.Entries[entry]
	if !ok {
		log.Errorf("Entry %v not found while trying to delete it", entry)
		return
	}

	we.Entries[entry].Lock()
	we.Entries[entry].WorkInProgress = true
	we.Entries[entry].WIPTime = time.Now()
	we.Entries[entry].State = step
	we.Entries[entry].Unlock()

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
