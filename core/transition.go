package core

import (
	"fmt"
	"github.com/interlook/interlook/comm"
	"github.com/interlook/interlook/log"
	"strings"
)

type transition interface {
	execute(*workflowEntry, comm.Message)
}

type providerState struct{}

func (s *providerState) execute(we *workflowEntry, msg comm.Message) {

	switch msg.Action {
	case comm.AddAction:
		we.setLastUpdate()
		we.setTargetState(deployedState)
		we.Lock()
		we.transition = &providerAddState{}
		we.Unlock()
		we.setState(msg, false)
	case comm.DeleteAction:
		we.transition = &providerDeleteState{}
	default:
		logMsg := fmt.Sprintf("Unhandled action %v", msg.Action)
		we.setError(logMsg)
		log.Error(logMsg)
		return
	}
	log.Debugf("ProviderState got msg %v", msg)
	we.transition.execute(we, msg)
}

type providerAddState struct{}

func (s *providerAddState) execute(we *workflowEntry, msg comm.Message) {
	log.Debugf("ProviderAddState got msg %v", msg)
	if msg.Action != comm.AddAction || !strings.HasPrefix(msg.Sender, "provider.") {
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
	we.transition.execute(we, msg)
}

type providerDeleteState struct{}

func (s *providerDeleteState) execute(we *workflowEntry, msg comm.Message) {
	if msg.Action != comm.DeleteAction || !strings.HasPrefix(msg.Sender, "provider.") {
		logMsg := fmt.Sprintf("%v action not allowed in state %v %v", msg.Action, we.State, we.Service.Name)
		log.Warn(logMsg)
		return
	}

	// if WIP, we set entry to transition step before roll backing
	// so that current step is also rolled back
	if we.WorkInProgress {
		we.setNextStep()
	}

	we.setTargetState(undeployedState)
	we.setNextStep()
	// update service with entry info as provider might not have all info in case service is un-deployed (ie host, dns alias)
	msg.Service = we.Service
	we.transition.execute(we, msg)
}

type provisionerState struct{}

func (s *provisionerState) execute(we *workflowEntry, msg comm.Message) {
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
			log.Errorf("%v entry in error %v", msg.Service.Name, msg.Error)
			return
		}

		we.updateService(msg)
		we.setNextStep()

		if workflow.isLastStep(we.State, we.isReverse()) {
			we.transition = &closeState{}
			we.transition.execute(we, msg)
			return
		}
		log.Debugf("ProvisionerState sending %v state %v to transition", msg.Service.Name, we.State)
		we.sendToExtension()
		return
	} else {
		// Transition triggered by previous state, send to extension
		log.Debugf("Provisioner %v for entry %v", we.State, we)
		we.sendToExtension()
	}

}

type closeState struct{}

func (s *closeState) execute(we *workflowEntry, msg comm.Message) {
	log.Debugf("closeState for %v", msg.Service.Name)
	we.close(msg.Error)
}
