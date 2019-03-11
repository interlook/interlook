package core

import (
	"errors"
	"github.com/bhuisgen/interlook/log"
	"github.com/bhuisgen/interlook/service"
	"time"
)

// mergeMessage manages messages received from extensions
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

	if serviceUnchanged && msg.Action == service.MsgAddAction && serviceStateOK {
		logger.DefaultLogger().Debugf("Service %v already in desired state\n", msg.Service.Name)
		return nil
	}

	switch msg.Action {
	case service.MsgAddAction:
		f.Lock()
		defer f.Unlock()
		ne := makeNewFlowEntry()
		ne.Service = msg.Service
		f.M[msg.Service.Name] = &ne
		f.M[msg.Service.Name].State = msg.Sender
		f.M[msg.Service.Name].ExpectedState = flowDeployedState

		if serviceExist {
			f.M[msg.Service.Name].LastUpdate = time.Now()
		}

		f.M[msg.Service.Name].Service = service.Service{
			Name:       msg.Service.Name,
			Hosts:      msg.Service.Hosts,
			DNSAliases: msg.Service.DNSAliases,
			Port:       msg.Service.Port,
			TLS:        msg.Service.TLS,
		}

	case service.MsgUpdateFromExtension:
		logger.DefaultLogger().Debugf("Got msg from extension")
		f.Lock()
		defer f.Unlock()
		f.M[msg.Service.Name].WorkInProgress = false
		f.M[msg.Service.Name].Service = msg.Service
		if msg.Error != "" {
			f.M[msg.Service.Name].Error = msg.Error
			return nil
		}

	case service.MsgDeleteAction:
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
