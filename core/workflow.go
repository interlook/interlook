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
	// current step in the workflow
	//CurrentState string `json:"current_state,omitempty"`
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

// messageHandler manages messages received from extensions
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

	// if no changes are needed on existing service, we do nothing
	if serviceUnchanged && msg.Action == messaging.AddAction && serviceStateOK {
		log.Debugf("Service %v already in desired state\n", msg.Service.Name)
		return nil
	}

	switch msg.Action {
	case messaging.AddAction, messaging.UpdateAction:
		we.Lock()
		defer we.Unlock()

		if !serviceExist {
			ne := makeNewFlowEntry()
			we.Entries[msg.Service.Name] = &ne
		}
		// only provider can change desired state
		if strings.Contains(msg.Sender, "provider.") {
			we.Entries[msg.Service.Name].ExpectedState = deployedState
		}

		we.Entries[msg.Service.Name].State = msg.Sender
		we.Entries[msg.Service.Name].Service = msg.Service
		we.Entries[msg.Service.Name].WorkInProgress = false
		we.Entries[msg.Service.Name].Error = msg.Error
		if err := we.save(); err != nil {
			log.Errorf("Error saving flow entries")
		}

	case messaging.DeleteAction:
		we.Lock()
		defer we.Unlock()
		we.Entries[msg.Service.Name].ExpectedState = undeployedState
		we.Entries[msg.Service.Name].LastUpdate = time.Now()
		if err := we.save(); err != nil {
			log.Errorf("Error saving flow entries")
		}

	default:
		log.Warnf("messageHandler could not handle %v action\n", msg.Action)
		return errors.New("unhandled action")
	}

	return nil
}

func (we *workflowEntries) getServiceByName(name string) (*workflowEntry, error) {
	res, ok := we.Entries[name]
	if !ok {
		return res, errors.New(fmt.Sprintf("No entry found for %v", name))
	}

	return res, nil
}

func (we *workflowEntries) prepareForNextStep(entry, step string, reverse bool) {
	we.Lock()
	we.Entries[entry].WorkInProgress = true
	we.Entries[entry].WIPTime = time.Now()
	we.Entries[entry].State = step
	if err := we.save(); err != nil {
		log.Errorf("Error saving flow entries")
	}
	we.Unlock()
}

func (we *workflowEntries) closeEntry(serviceName string, reverse bool) {
	we.Lock()
	we.Entries[serviceName].WorkInProgress = false
	we.Entries[serviceName].WIPTime = time.Time{}
	we.Entries[serviceName].CloseTime = time.Now()

	if reverse {
		we.Entries[serviceName].State = undeployedState
	} else {
		we.Entries[serviceName].State = deployedState
	}
	if err := we.save(); err != nil {
		log.Errorf("Error saving flow entries")
	}
	we.Unlock()
}

func (we *workflowEntries) save() error {
	dbFile, err := os.OpenFile(we.DBFile, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	dbFile.Truncate(0)
	dbFile.Seek(0, 0)
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
		log.Errorf("Error synching dbfile %v", err)
	}

	return nil
}

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
