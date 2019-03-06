package core

import (
	"errors"
	"github.com/bhuisgen/interlook/log"
	"github.com/bhuisgen/interlook/service"
	"time"
)

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
		// Check service spec has not changed
		serviceUnchanged, _ = curSvc.Service.IsSameThan(msg.Service)
		// Check current state is as requested by msg
		serviceStateOK = curSvc.isStateAsWanted(msg.Action)
	}

	if serviceUnchanged && msg.Action == msgAddAction && serviceStateOK {
		logger.DefaultLogger().Debugf("Service %v already defined\n", msg.Service.Name)
		return nil
	}

	switch msg.Action {
	case msgAddAction:
		if !serviceExist {
			ne := makeNewFlowEntry()
			ne.Service = msg.Service
			ne.ExpectedState = flowDeployedState
			ne.State = msg.Sender
			f.Lock()
			f.M[msg.Service.Name] = &ne
			f.Unlock()
			logger.DefaultLogger().Debugf("mergeToFlow added new service entry %v", f.M[msg.Service.Name])
			return nil
		}
		f.Lock()
		f.M[msg.Service.Name].Service.Hosts = msg.Service.Hosts
		f.M[msg.Service.Name].Service.DNSName = msg.Service.DNSName
		f.M[msg.Service.Name].Service.Port = msg.Service.Port
		f.M[msg.Service.Name].Service.TLS = msg.Service.TLS
		f.M[msg.Service.Name].State = msg.Sender
		f.M[msg.Service.Name].ExpectedState = flowDeployedState
		f.M[msg.Service.Name].LastUpdate = time.Now()
		f.Unlock()

	case MsgUpdateFromExtension:
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
