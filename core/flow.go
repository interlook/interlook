package core

import (
	"errors"
	"sync"
	"time"

	"github.com/bhuisgen/interlook/log"
	"github.com/bhuisgen/interlook/service"
	"github.com/satori/uuid"
)

type workflow map[int]string

type flowEntry struct {
	CurrentStep  int             `json:"current_step,omitempty"`
	State        string          `json:"state,omitempty"`
	Info         string          `json:"info,omitempty"`
	TimeDetected time.Time       `json:"time_detected,omitempty"`
	LastUpdate   time.Time       `json:"last_update,omitempty"`
	Service      service.Service `json:"service,omitempty"`
}

type flowEntries struct {
	sync.Mutex
	M map[string]*flowEntry `json:"entries,omitempty"`
}

func newFlowEntries() *flowEntries {
	fe := new(flowEntries)
	fe.M = make(map[string]*flowEntry)
	return fe
}

func makeNewEntry() flowEntry {
	var ne flowEntry
	ne.TimeDetected = time.Now()
	return ne
}

func genUUID() string {
	id := uuid.NewV4()
	str := id.String()
	return str
}

func (f *flowEntries) insertToFlow(msg service.Message) error {
	logger.DefaultLogger().Debugf("InsertToFlow received %v\n", msg)
	serviceExist := true
	curSvc, err := f.getServiceByName(msg.Service.ServiceName)
	if err != nil {
		serviceExist = false
	}
	if serviceExist {
		logger.DefaultLogger().Debugf("InsertToFlow service %v exist\n", msg.Service)
	}
	switch msg.Action {
	case "add", "update":
		if serviceExist && msg.Service.IsSameThan(curSvc.Service) {
			logger.DefaultLogger().Debugf("Service %v already defined\n", msg.Service.ServiceName)
			return nil
		}
		logger.DefaultLogger().Debugf("InsertToFlow service looks different %v action\n", msg.Action)
		f.Lock()
		defer f.Unlock()
		msg.Service.ID = genUUID()
		ne := makeNewEntry()
		ne.Service = msg.Service
		// replace existing entry with new one, and let it follow the workflow
		f.M[msg.Service.ServiceName] = &ne

	case "delete":
		f.Lock()
		defer f.Unlock()
		delete(f.M, msg.Service.ServiceName)
	default:
		logger.DefaultLogger().Warnf("InsertToFlow could not handle %v action\n", msg.Action)
		return errors.New("Unhandled action")
	}

	return nil
}

func (f *flowEntries) getServiceByName(name string) (res *flowEntry, err error) {
	res = f.M[name]
	if res != nil {
		return res, nil
	}
	return nil, errors.New("No entry found")
}

// reconcile compares provided service with existing one
// triggers required action(s) to get existing service equal
// to received description
// func (f *flowEntries) reconcile(svc service.Service) error {
// 	curSvc, err := f.getServiceByName(svc.ServiceName)
// 	if err != nil {
// 		err = f.insertServiceToFlow(svc)
// 		if err != nil {
// 			return err
// 		}
// 		return nil
// 	}
// 	// make sure service come from same provider, otherwise raise conflict
// 	if curSvc.service.Provider != svc.Provider {
// 		if curSvc.state == "reconciled" {
// 			logger.DefaultLogger().Warnf("Service %v already created by %v\n", svc.ServiceName, curSvc.service.Provider)
// 		}
// 	}
// 	// compare service definition (DNS name, Hosts, Ports)
// 	if !curSvc.service.IsSameThan(svc) {

// 	}
// 	return nil
// }
