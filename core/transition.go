package core

import (
	"fmt"
	"github.com/interlook/interlook/comm"
	"github.com/interlook/interlook/log"
	"strings"
	"time"
)

// transitioner handles service deployment transitions
type transitioner interface {
	toNext(*workflowEntry, comm.Message)
}

// define transitions
type (
	// handle message received from provider, check if either add or delete then move to corresponding transition
	receivedFromProvider struct{}
	// handle add message from provider
	addFromProvider struct{}
	// handle delete message from provider
	deleteFromProvider struct{}
	// handle message received from provisioner extension, only update are managed
	receivedFromProvisioner struct{}
	// will close the flow
	closeState struct{}
)

func (s *receivedFromProvider) toNext(we *workflowEntry, msg comm.Message) {
	we.Lock()
	//defer we.Unlock()
	switch msg.Action {
	case comm.AddAction:
		we.LastUpdate = time.Now()
		we.ExpectedState = deployedState
		we.workflowSteps = initWorkflow(we.WfConfig)
		we.step = &addFromProvider{}
		we.setState(msg, false)

	case comm.DeleteAction:
		we.step = &deleteFromProvider{}
	default:
		logMsg := fmt.Sprintf("Unhandled action %v", msg.Action)
		we.Error = logMsg
		log.Error(logMsg)
		we.Unlock()
		return
	}
	log.Debugf("receivedFromProvider %v", msg)
	we.Unlock()
	we.step.toNext(we, msg)
}

func (s *addFromProvider) toNext(we *workflowEntry, msg comm.Message) {
	log.Debugf("ProviderAddState got msg %v", msg)
	we.Lock()
	if msg.Action != comm.AddAction || !strings.HasPrefix(msg.Sender, "provider.") {
		logMsg := fmt.Sprintf("%v action not allowed from %v in step %v %v", msg.Action, msg.Sender, we.State, we.Service.Name)
		log.Warn(logMsg)
		we.Unlock()
		return
	}

	if we.WorkInProgress && we.Error == "" {
		logMsg := fmt.Sprintf("Message ignored as service %v is still under deployment", we.Service.Name)
		log.Info(logMsg)
		we.Unlock()
		return
	}

	we.setNextStep()
	we.Unlock()
	we.step.toNext(we, msg)
}

func (s *deleteFromProvider) toNext(we *workflowEntry, msg comm.Message) {
	we.Lock()
	if msg.Action != comm.DeleteAction || !strings.HasPrefix(msg.Sender, "provider.") {
		logMsg := fmt.Sprintf("%v action not allowed in step %v %v", msg.Action, we.State, we.Service.Name)
		log.Warn(logMsg)
		we.Unlock()
		return
	}

	// if WIP, we set entry to next step before rolling back
	// so that current step is also rolled back
	if we.WorkInProgress {
		we.setNextStep()
	}

	we.ExpectedState = undeployedState
	we.setNextStep()
	// update service with entry info as provider might not have all info in case service is un-deployed (ie host, dns alias)
	msg.Service = we.Service
	we.Unlock()
	we.step.toNext(we, msg)
}

func (s *receivedFromProvisioner) toNext(we *workflowEntry, msg comm.Message) {

	log.Debugf("receivedFromProvisioner %v", msg)

	// if wip, we expect an update from extension (meaning through message)
	if we.WorkInProgress {
		log.Debug("ProvisionerState -> WIP", msg)
		if msg.Sender != we.State {
			logMsg := fmt.Sprintf("Transition from %v to %v not possible %v", we.State, msg.Sender, we.Service.Name)
			log.Warn(logMsg)
			we.Lock()
			we.Error = logMsg
			we.Unlock()
			return
		}

		if msg.Error != "" {
			we.Lock()
			we.setWIP(false)
			we.Error = msg.Error
			log.Errorf("%v entry in error %v", msg.Service.Name, msg.Error)
			we.Unlock()
			return
		}

		we.updateService(msg)
		we.Lock()
		we.setNextStep()
		we.Unlock()
		if we.workflowSteps.isLastStep(we.State, we.isReverse()) {
			we.step = &closeState{}
			we.step.toNext(we, msg)
			return
		}
		log.Debugf("ProvisionerState sending %v step %v to step", msg.Service.Name, we.State)
		we.sendToExtension()
		return
	} else {
		// Transition triggered by previous step, send to extension
		log.Debugf("Provisioner %v for entry %v", we.State, we)
		we.sendToExtension()
	}

}

func (s *closeState) toNext(we *workflowEntry, msg comm.Message) {
	log.Debugf("closeState for %v", msg.Service.Name)
	we.close(msg.Error)
}
