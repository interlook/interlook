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
	logger.DefaultLogger().Debug("Running flowControl")
	for k, v := range s.flowEntries.M {
		if v.State != v.ExpectedState && !v.WorkInProgress {
			if v.Error != "" {
				logger.DefaultLogger().Warnf("Service %v is in error %v", v.Service.Name, v.Error)
				continue
			}
			logger.DefaultLogger().Debugf("flowControl: Service %v current state differs from target state", k)
			reverse := false
			if v.ExpectedState == flowUndeployedState {
				reverse = true
			}
			nextStep, err := s.workflow.getNextStep(v.State, reverse)
			if err != nil {
				logger.DefaultLogger().Errorf("Could not find next step for %v %v\n", k, err)
				continue
			}
			// add test if service existed and provider wants to recreate
			// provider extensions are not receiving "delete" messages
			//if strings.Contains(nextStep, "provider") {
			//    if reverse {
			//        lastStep, err := s.workflow.getNextStep(nextStep, reverse)
			//        if err != nil {
			//            logger.DefaultLogger().Errorf("Could not find next step for %v %v\n", k, err)
			//            continue
			//        }
			//        nextStep = lastStep
			//    }
			logger.DefaultLogger().Debugf("next step: %v", nextStep)
			if s.workflow.isLastStep(nextStep, reverse) {
				logger.DefaultLogger().Debugf("Closing flow entry %v (state: %v)\n", k, nextStep)
				s.flowEntries.Lock()
				s.flowEntries.M[k].WorkInProgress = false
				if reverse {
					s.flowEntries.M[k].State = flowUndeployedState
				} else {
					s.flowEntries.M[k].State = flowDeployedState
				}

				s.flowEntries.Unlock()
				continue
			}

			s.flowEntries.Lock()
			s.flowEntries.M[k].WorkInProgress = true
			s.flowEntries.M[k].State = nextStep
			s.flowEntries.Unlock()
			var msg service.Message
			if reverse {
				msg.Action = msgDeleteAction
			} else {
				msg.Action = msgAddAction
			}
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
		if curSvc.ExpectedState == curSvc.State &&
			((curSvc.State == flowDeployedState && msg.Action == msgAddAction) ||
				(curSvc.State == flowUndeployedState && msg.Action == msgDeleteAction)) {
			serviceStateOK = true
			logger.DefaultLogger().Debug("service state is OK")
		}
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
			//ne.State = flowDeployState
			ne.State = msg.Sender
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
		f.M[msg.Service.Name].State = msg.Sender
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
