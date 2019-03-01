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
		// TODO: add config for check interval
		// TODO: remove sleep in favour of ticker
		// TODO: rewrite/refactor
		time.Sleep(3 * time.Second)
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
				logger.DefaultLogger().Debugf("flowControl writing to %v %v channel\n", nextStep, ext.name)
				logger.DefaultLogger().Debugf("CheckServiceEntries receive: %v", ext.receive)
				ext.receive <- msg
			}
		}
	}
}

// mergeMessage manages messages received from extensions
// TODO: code review, remove current_step, rewrite
func (f *flowEntries) mergeMessage(msg service.Message) error {
	logger.DefaultLogger().Debugf("InsertToFlow received %v\n", msg)
	var serviceExist, serviceUnchanged, serviceStateOK bool
	serviceExist = true

	curSvc, err := f.getServiceByName(msg.Service.Name)
	if err != nil {
		logger.DefaultLogger().Debugf("Service %v: %v", msg.Service.Name, err)
		serviceExist = false
	}

	if serviceExist {
		logger.DefaultLogger().Debugf("InsertToFlow service %v exist\n", msg.Service.Name)
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
			f.Lock()
			defer f.Unlock()
			//msg.Service.ID = genUUID()
			ne := makeNewEntry()
			ne.Service = msg.Service
			ne.ExpectedState = flowDeployedState
			ne.State = flowDeployState
			ne.CurrentState = "provider." + msg.Service.Provider
			f.M[msg.Service.Name] = &ne
			logger.DefaultLogger().Debugf("InsertToFlow added new service entry %v", f.M[msg.Service.Name])
			return nil
		}
		f.Lock()
		defer f.Unlock()
		//msg.Service.ID = f.M[msg.Service.Name].Service.ID
		f.M[msg.Service.Name].Service = msg.Service
		f.M[msg.Service.Name].ExpectedState = flowDeployedState
		f.M[msg.Service.Name].LastUpdate = time.Now()

	case msgUpdateAction:
		// FIXME: only there if provider is able to send update msg
		f.Lock()
		defer f.Unlock()
		f.M[msg.Service.Name].WorkInProgress = false
		f.M[msg.Service.Name].Error = msg.Error

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
		logger.DefaultLogger().Warnf("InsertToFlow could not handle %v action\n", msg.Action)
		return errors.New("Unhandled action")
	}
	return nil
}
