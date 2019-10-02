package f5ltm

import (
	"fmt"
	"github.com/interlook/interlook/comm"
	"github.com/interlook/interlook/log"
	"github.com/pkg/errors"

	//"github.com/f5devcentral/go-bigip"
	"github.com/scottdware/go-bigip"
	"reflect"
	"strconv"
	"strings"
)

const (
	vsUpdateMode     = "vs"
	policyUpdateMode = "policy"
)

func (f5 *BigIP) updateService(msg comm.Message) comm.Message {
	switch f5.UpdateMode {
	case vsUpdateMode:
		m := f5.updateVS(msg)
		return m
	case policyUpdateMode:
		m := f5.updateGlobalPolicy(msg)
		return m
	default:
		msg.Error = fmt.Sprintf("unsupported updateMode %v", f5.UpdateMode)
		return msg
	}
}

func (f5 *BigIP) deleteService(msg comm.Message) comm.Message {
	if f5.UpdateMode == vsUpdateMode {
		m := f5.deleteVS(msg)
		return m
	}
	msg.Error = fmt.Sprintf("unsupported updateMode %v", f5.UpdateMode)
	return msg
}

// updateGlobalPolicy first check the pool. If it exist, update it as needed.
// If not, create it. Next, handle the policy
//
func (f5 *BigIP) updateGlobalPolicy(msg comm.Message) comm.Message {

	// handle Pool
	pool, err := f5.cli.GetPool(f5.addPartitionToName(msg.Service.Name))
	if err != nil {
		msg.Error = fmt.Sprintf("Could not get Pool %v %v", msg.Service.Name, err.Error())
		return msg
	}

	if pool == nil {
		pool, err = f5.createPool(msg)
		if err != nil {
			msg.Error = fmt.Sprintf("could not create pool %v %v", msg.Service.Name, err.Error())
			return msg
		}
	} else {
		if err := f5.updatePoolMembers(pool, msg); err != nil {
			msg.Error = err.Error()
			return msg
		}
	}

	// handle Policy
	var globalPolicy string
	policyRuleExist := false
	policyNeedsUpdate := false

	switch msg.Service.TLS {
	case true:
		globalPolicy = f5.GlobalSSLPolicy
	default:
		globalPolicy = f5.GlobalHTTPPolicy
	}

	//create a draft policy
	if err := f5.cli.CreateDraftFromPolicy(f5.addPartitionToPath(globalPolicy)); err != nil {
		msg.Error = fmt.Sprintf("error creating draft policy %v", err.Error())
		return msg
	}
	draft := f5.addPartitionToPath("Drafts/" + globalPolicy)

	defer func() {
		if err := f5.cli.DeletePolicy(f5.addPartitionToName("Drafts~" + globalPolicy)); err != nil {
			log.Warnf("Error closing filename %v", err)
		}
	}()

	policy, err := f5.cli.GetPolicy(f5.addPartitionToName("Drafts~" + globalPolicy))
	if err != nil {
		msg.Error = fmt.Sprintf("Could not get policy %v %v", f5.addPartitionToName("Drafts~"+globalPolicy), err.Error())
		return msg
	}

	// get the matching rule and check if they need update
	for _, r := range policy.Rules {
		if r.Name == msg.Service.Name {
			log.Debugf("found matching PolicyRule %v", r.Name)
			policyRuleExist = true
			for _, condition := range r.Conditions {
				if condition.HttpHost && !reflect.DeepEqual(condition.Values, msg.Service.Hosts) {
					log.Debugf("PolicyRule condition for %v differs", msg.Service.Name)
				}
				policyNeedsUpdate = true
			}
		}
		for _, action := range r.Actions {
			if action.Forward && action.Pool != f5.addPartitionToPath(msg.Service.Name) {
				log.Debugf("PolicyRule action for %v differs", msg.Service.Name)
				policyNeedsUpdate = true
			}
		}
	}

	if policyNeedsUpdate {

		log.Debugf("updating policy %v", policy.Name)

		if err := f5.cli.ModifyPolicyRule(draft, msg.Service.Name, f5.buildPolicyRuleFromMsg(msg)); err != nil {
			msg.Error = fmt.Sprintf("could not modify policy rule %v %v", msg.Service.Name, err.Error())
		}

		if err := f5.cli.PublishDraftPolicy(draft); err != nil {
			msg.Error = fmt.Sprintf("could not publish draft %v %v", policy.Name, err.Error())
			return msg
		}

		return msg
	}

	if !policyRuleExist {

		log.Debugf("updating policy %v with new rule for %v", policy.Name, msg.Service.Name)

		if err := f5.cli.AddRuleToPolicy(draft, f5.buildPolicyRuleFromMsg(msg)); err != nil {
			msg.Error = fmt.Sprintf("error adding rule %v to draft policy %v", msg.Service.Name, err.Error())
			return msg
		}

		if err := f5.cli.PublishDraftPolicy(draft); err != nil {
			msg.Error = fmt.Sprintf("could not publish draft %v %v", policy.Name, err.Error())
			return msg
		}

		return msg
	}

	log.Debugf("no policy update for %v", policy.Name)

	return msg
}

func (f5 *BigIP) buildPolicyRuleFromMsg(msg comm.Message) bigip.PolicyRule {

	prc := bigip.PolicyRuleCondition{
		Name:            "0",
		CaseInsensitive: true,
		Equals:          true,
		External:        true,
		Remote:          true,
		HttpHost:        true,
		Host:            true,
		Request:         true,
		Values:          msg.Service.DNSAliases,
	}

	pra := bigip.PolicyRuleAction{
		Name:    "0",
		Forward: true,
		Pool:    f5.addPartitionToPath(msg.Service.Name),
		Request: true,
		Select:  true,
	}

	pr := bigip.PolicyRule{
		Name:        msg.Service.Name,
		Description: fmt.Sprintf("route traffic for %v", msg.Service.Name),
		Conditions:  []bigip.PolicyRuleCondition{prc},
		Actions:     []bigip.PolicyRuleAction{pra},
	}
	return pr
}

func (f5 *BigIP) poolMembersNeedsUpdate(pool *bigip.Pool, msg comm.Message) (bool, error) {

	var (
		members []string
		port    int
	)

	pm, err := f5.cli.PoolMembers(pool.FullPath)
	if err != nil {
		return false, errors.New(fmt.Sprintf("Could not get members of pool %v %v", pool.FullPath, err.Error()))
	}

	for _, member := range pm.PoolMembers {
		i := strings.LastIndex(member.FullPath, ":")
		port, err = strconv.Atoi(member.FullPath[i+1:])
		if err != nil {
			return false, errors.New(fmt.Sprintf("Could not convert pool member port %v %v", member.FullPath, err.Error()))
		}
		members = append(members, member.Address)
	}
	// check if current pool is as defined in msg
	if !reflect.DeepEqual(members, msg.Service.Hosts) || msg.Service.Port != port {
		log.Debugf("pool %v: host/hostPort differs", msg.Service.Name)
		return true, nil
	}
	return false, nil
}

func (f5 *BigIP) updateVS(msg comm.Message) comm.Message {

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
		// if vs exist, we expect a pool, otherwise error is raised
		if err := f5.updatePoolMembers(pool, msg); err != nil {
			msg.Error = err.Error()
			return msg
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

	_, err = f5.createPool(msg)
	if err != nil {
		msg.Error = err.Error()
		return msg
	}

	if err := f5.createVirtualServer(msg); err != nil {
		msg.Error = err.Error()
		return msg
	}
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
