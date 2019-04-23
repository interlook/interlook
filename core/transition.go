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

type providerState struct{}

func (s *providerState) execTransition(we *workflowEntry, msg messaging.Message) {

	switch msg.Action {
	case messaging.AddAction:
		we.setLastUpdate()
		we.setTargetState(deployedState)
		we.Lock()
		we.next = &providerAddState{}
		we.Unlock()
		we.setState(msg, false)
	case messaging.DeleteAction:
		we.next = &providerDeleteState{}
	default:
		logMsg := fmt.Sprintf("Unhandled action %v", msg.Action)
		we.setError(logMsg)
		log.Error(logMsg)
		return
	}
	log.Debugf("ProviderState got msg %v", msg)
	we.next.execTransition(we, msg)
}

type providerAddState struct{}

func (s *providerAddState) execTransition(we *workflowEntry, msg messaging.Message) {
	log.Debugf("ProviderAddState got msg %v", msg)
	if msg.Action != messaging.AddAction || !strings.HasPrefix(msg.Sender, "provider.") {
		logMsg := fmt.Sprintf("%v action not allowed from %v in state %v %v", msg.Action, msg.Sender, we.State, we.Service.Name)
		log.Warn(logMsg)
		return
	}

	if we.CloseTime.IsZero() && we.Error == "" && !strings.HasPrefix(we.State, "provider.") {
		logMsg := fmt.Sprintf("Message ignored as service %v is still under deployment", we.Service.Name)
		log.Info(logMsg)
		return
	}

	we.setNextStep()
	we.next.execTransition(we, msg)
}

type providerDeleteState struct{}

func (s *providerDeleteState) execTransition(we *workflowEntry, msg messaging.Message) {
	if msg.Action != messaging.DeleteAction || !strings.HasPrefix(msg.Sender, "provider.") {
		logMsg := fmt.Sprintf("%v action not allowed in state %v %v", msg.Action, we.State, we.Service.Name)
		log.Warn(logMsg)
		return
	}

	// if WIP, we set entry to next step before roll backing
	// so that current step is also rolled back
	if we.WorkInProgress {
		we.setNextStep()
	}

	we.setTargetState(undeployedState)
	we.setNextStep()
	// update service with entry info as provider might not have all info in case service is un-deployed (ie host, dns alias)
	msg.Service = we.Service
	we.next.execTransition(we, msg)
}

type provisionerState struct{}

func (s *provisionerState) execTransition(we *workflowEntry, msg messaging.Message) {
	log.Debugf("ProvisionerState got msg %v", msg)

	// if wip, we expect an update from extension (meaning through message)
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

		if workflow.isLastStep(we.State, we.isReverse()) {
			we.next = &closeState{}
			we.next.execTransition(we, msg)
			return
		}
		log.Debugf("ProvisionerState sending %v state %v to next", msg.Service.Name, we.State)
		we.sendToExtension()
		return
	} else {
		// Transition triggered by previous state, send to extension
		log.Debugf("Provisioner %v for entry %v", we.State, we)
		we.sendToExtension()
	}

}

type closeState struct{}

func (s *closeState) execTransition(we *workflowEntry, msg messaging.Message) {
	log.Debugf("closeState for %v", msg.Service.Name)
	we.close(msg.Error)
}
