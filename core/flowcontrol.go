package core

import (
	"errors"
	"github.com/bhuisgen/interlook/log"
	"github.com/bhuisgen/interlook/service"
	"time"
)

// flowControl compares provided service with existing one
// triggers required action(s) to bring service state
// to desired state (deployed or undeployed)
func (s *server) flowControl() {
	for {
		// TODO: remove sleep in favour of ticker
		// TODO: rewrite/refactor
		time.Sleep(s.config.Core.CheckFlowInterval)
		logger.DefaultLogger().Debug("Running flowControl")
		for k, v := range s.flowEntries.M {
			if v.State != v.ExpectedState && !v.WorkInProgress {
				logger.DefaultLogger().Debugf("flowControl: Service %v current state differs from target state", k)
				reverse := false
				if v.ExpectedState == flowUndeployedState {
					reverse = true
				}
				nextStep, err := s.workflow.getNextStep(v.CurrentState, reverse)
				if err != nil {
					logger.DefaultLogger().Errorf("Could not find next step for %v %v\n", k, err)
					continue
				}
				if s.workflow.isLastStep(nextStep, reverse) {
					logger.DefaultLogger().Debugf("Closing flow entry %v (state: %v)\n", k, nextStep)
					s.flowEntries.Lock()
					s.flowEntries.M[k].WorkInProgress = false
					s.flowEntries.M[k].CurrentState = ""
					s.flowEntries.M[k].State = flowDeployedState
					s.flowEntries.Unlock()
					continue
				}

				s.flowEntries.Lock()
				s.flowEntries.M[k].WorkInProgress = true
				s.flowEntries.M[k].CurrentState = nextStep
				s.flowEntries.Unlock()
				var msg service.Message
				msg.Action = "add" // add, remove, update, check
				msg.Service = v.Service
				// get the extension channel to write message to
				ext, ok := s.extensionChannels[nextStep]
				if !ok {
					logger.DefaultLogger().Errorf("flowControl could not find channel for ext %v\n", nextStep)
					continue
				}
				ext.receive <- msg
			}
		}
	}
}

// mergeMessage manages messages received from extensions
// TODO: code review
func (f *flowEntries) mergeMessage(msg service.Message) error {
	logger.DefaultLogger().Debugf("mergeMessage received %v\n", msg)
	var serviceExist, serviceUnchanged, serviceStateOK bool
	serviceExist = true

	curSvc, err := f.getServiceByName(msg.Service.Name)
	if err != nil {
		logger.DefaultLogger().Debugf("Service %v: %v", msg.Service.Name, err)
		serviceExist = false
	}

	if serviceExist {
		logger.DefaultLogger().Debugf("mergeMessage service %v exist\n", msg.Service.Name)
		serviceUnchanged, _ = curSvc.Service.IsSameThan(msg.Service)
		serviceStateOK = curSvc.ExpectedState == curSvc.State
	}

	if serviceUnchanged && msg.Action == msgAddAction && serviceStateOK {
		logger.DefaultLogger().Debugf("Service %v already defined\n", msg.Service.Name)
		return nil
	}

	switch msg.Action {
	case msgAddAction:
		if !serviceExist {
			ne := makeNewEntry()
			ne.Service = msg.Service
			ne.ExpectedState = flowDeployedState
			ne.State = flowDeployState
			ne.CurrentState = msg.Sender
			f.Lock()
			f.M[msg.Service.Name] = &ne
			defer f.Unlock()
			logger.DefaultLogger().Debugf("mergeToFlow added new service entry %v", f.M[msg.Service.Name])
			return nil
		}
		f.Lock()
		defer f.Unlock()
		f.M[msg.Service.Name].Service.Hosts = msg.Service.Hosts
		f.M[msg.Service.Name].Service.Port = msg.Service.Port
		f.M[msg.Service.Name].Service.TLS = msg.Service.TLS
		f.M[msg.Service.Name].ExpectedState = flowDeployedState
		f.M[msg.Service.Name].LastUpdate = time.Now()

	case msgUpdateFromExtension:
		logger.DefaultLogger().Debugf("Got msg from extension")
		f.Lock()
		defer f.Unlock()
		f.M[msg.Service.Name].WorkInProgress = false
		f.M[msg.Service.Name].Service = msg.Service
		if msg.Error != "" {
			f.M[msg.Service.Name].Error = msg.Error
			return nil
		}

	case msgDeleteAction:
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
