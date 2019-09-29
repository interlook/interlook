package f5ltm

import (
	"fmt"
	"github.com/interlook/interlook/comm"
	"github.com/interlook/interlook/log"
	//"github.com/f5devcentral/go-bigip"
	"github.com/scottdware/go-bigip"
	"reflect"
	"strconv"
	"strings"
)

const (
	vsUpdateMode = "vs"
	//policyUpdateMode = "policy"
)

func (f5 *BigIP) updateService(msg comm.Message) comm.Message {
	if f5.UpdateMode == vsUpdateMode {
		m := f5.updateVS(msg)
		return m
	}
	msg.Error = fmt.Sprintf("unsupported updateMode %v", f5.UpdateMode)
	return msg
}

func (f5 *BigIP) deleteService(msg comm.Message) comm.Message {
	if f5.UpdateMode == vsUpdateMode {
		m := f5.deleteVS(msg)
		return m
	}
	msg.Error = fmt.Sprintf("unsupported updateMode %v", f5.UpdateMode)
	return msg
}

func (f5 *BigIP) deleteVS(msg comm.Message) comm.Message {

	msg.Action = comm.UpdateAction

	if err := f5.cli.DeleteVirtualServer(f5.addPartitionToName(msg.Service.Name)); err != nil {
		msg.Error = err.Error()
	}

	if err := f5.cli.DeletePool(f5.addPartitionToName(msg.Service.Name)); err != nil {
		msg.Error = err.Error()
	}

	return msg
}

func (f5 *BigIP) updateVS(msg comm.Message) comm.Message {
	var members []string
	var port int
	//    vsExist := true
	vs, err := f5.cli.GetVirtualServer(f5.addPartitionToName(msg.Service.Name))
	if err != nil {
		msg.Error = fmt.Sprintf("Could not get VS %v %v", msg.Service.Name, err.Error())
		return msg
	}

	// check if pool attached to vs needs to be changed
	if vs != nil {
		pool, err := f5.cli.GetPool(vs.Pool)
		if err != nil {
			msg.Error = fmt.Sprintf("Could not get Pool %v %v", vs.Pool, err.Error())
			return msg
		}

		pm, err := f5.cli.PoolMembers(pool.FullPath)
		if err != nil {
			msg.Error = fmt.Sprintf("Could not get members of pool %v %v", pool.FullPath, err.Error())
			return msg
		}

		for _, member := range pm.PoolMembers {
			i := strings.LastIndex(member.FullPath, ":")
			port, err = strconv.Atoi(member.FullPath[i+1:])
			if err != nil {
				msg.Error = fmt.Sprintf("Could not convert pool member port %v %v", member.FullPath, err.Error())
				return msg
			}
			members = append(members, member.Address)
		}
		// check if current pool is as defined in msg
		if !reflect.DeepEqual(members, msg.Service.Hosts) || msg.Service.Port != port {
			// hosts differ, update f5 pool
			log.Debugf("pool %v: host/hostPort differs", msg.Service.Name)

			if err := f5.updatePoolMembers(pool, msg); err != nil {
				msg.Error = err.Error()
				return msg
			}
		}
		// check if virtual's IP is the one we got in msg
		if !strings.Contains(vs.Destination, msg.Service.PublicIP+":"+strconv.Itoa(f5.getLBPort(msg))) {
			log.Debugf("pool %v: exposed IP differs", msg.Service.Name)
			if err := f5.cli.ModifyVirtualServer(vs.Name, &bigip.VirtualServer{Destination: msg.Service.PublicIP + ":" + strconv.Itoa(f5.getLBPort(msg))}); err != nil {
				msg.Error = err.Error()
				return msg
			}
		}

		log.Debugf("f5ltm, nothing to do for service %v", msg.Service.Name)
		return msg
	}

	log.Debugf("%v not found, creating pool and virtual server", msg.Service.Name)

	if err := f5.createPool(msg); err != nil {
		msg.Error = err.Error()
		return msg
	}

	if err := f5.createVirtualServer(msg); err != nil {
		msg.Error = err.Error()
		return msg
	}
	return msg
}
