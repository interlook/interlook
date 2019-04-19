package core

import (
	"fmt"
	"github.com/bhuisgen/interlook/log"
	"github.com/bhuisgen/interlook/messaging"
	"strings"
)

type workflowState interface {
	execTransition(*workflowEntry, messaging.Message)
}

// provider -> add or delete -> next: extension, send Message: true
// extension: update(always), check WIP / err -> do next, sendmessage

type providerState struct{}

func (s *providerState) execTransition(we *workflowEntry, msg messaging.Message) {
	reverse := false
	switch msg.Action {
	case messaging.AddAction:
		we.setLastUpdate()
		we.next = &providerAddState{}
		we.setState(msg, reverse)
	case messaging.DeleteAction:
		we.next = &providerDeleteState{}
	default:
		logMsg := fmt.Sprintf("Unhandled action %v", msg.Action)
		log.Error(logMsg)
	}
	log.Debugf("ProviderState got msg %v", msg)

	we.next.execTransition(we, msg)
}

type providerAddState struct{}

func (s *providerAddState) execTransition(we *workflowEntry, msg messaging.Message) {
	log.Debugf("ProviderAddState got msg %v", msg)
	if msg.Action != messaging.AddAction {
		logMsg := fmt.Sprintf("%v action not allowed in state %v %v", msg.Action, we.State, we.Service.Name)
		log.Warn(logMsg)
		return
	}

	if we.CloseTime.IsZero() && we.Error == "" && !strings.HasPrefix(we.State, "provider.") {
		logMsg := fmt.Sprintf("Message ignored as service %v is still under deployment", we.Service.Name)
		log.Info(logMsg)
		return
	}

	we.setNextStep()
	//we.setWIP(true)
	we.Lock()
	we.next = &provisionerState{}
	we.Unlock()

	we.next.execTransition(we, msg)

}

type providerDeleteState struct{}

func (s *providerDeleteState) execTransition(we *workflowEntry, msg messaging.Message) {
	if msg.Action != messaging.DeleteAction {
		logMsg := fmt.Sprintf("%v action not allowed in state %v %v", msg.Action, we.State, we.Service.Name)
		log.Warn(logMsg)
		return
	}

	if !we.WorkInProgress {
		we.setExpectedState(undeployedState)
		we.setNextStep()
		// update service with entry info as provider might not have all info anymore (ie host, dns alias)
		msg.Service = we.Service
		we.next.execTransition(we, msg)
	}
}

type provisionerState struct{}

func (s *provisionerState) execTransition(we *workflowEntry, msg messaging.Message) {
	log.Debugf("ProvisionerState got msg %v", msg)
	// if wip, we expect an update from extension (meaning through message)
	// else we need to send to next step
	if we.WorkInProgress {
		log.Debug("ProvisionerState -> WIP", msg)
		if msg.Sender != we.State {
			logMsg := fmt.Sprintf("Transition from %v to %v not possible %v", we.State, msg.Sender, we.Service.Name)
			log.Warn(logMsg)
			msg.Error = logMsg
			return
		}

		if msg.Error != "" {
			we.setWIP(false)
			we.setError(msg.Error)
			log.Errorf("%v entry in error %%", msg.Service.Name, msg.Error)
			return
		}

		we.updateServiceFromMsg(msg)
		we.setNextStep()
		log.Debugf("########################## state after setNext %v", we.State)
		if workflow.isLastStep(we.State, we.isReverse()) {
			we.next = &closeState{}
			we.next.execTransition(we, msg)
			return
		}
		log.Debugf("ProvisionerState sending %v state %v to next", msg.Service.Name, we.State)
		we.sendToExtension()

		return
	} else {
		// transition triggered by previous state
		log.Debugf("Provisioner %v for entry %v", we.State, we)
		if msg.Error != "" {
			// handle flow in error
			return
		}

		we.setWIP(true)
		//we.updateServiceFromMsg(msg)
		we.sendToExtension()
	}

}

type closeState struct{}

func (s *closeState) execTransition(we *workflowEntry, msg messaging.Message) {
	log.Debugf("closeState for %v", msg.Service.Name)
	we.close(msg.Error)
}
