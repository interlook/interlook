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

// getNodeByIP returns the node having IP
func (f5 *BigIP) getNodeByAddress(address string) (bigip.Node, bool) {
	list, err := f5.cli.Nodes()
	if err != nil {
		return bigip.Node{}, false
	}

	for _, node := range list.Nodes {
		if node.Address == address {
			return node, true
		}
	}

	return bigip.Node{}, false
}

// upsertPool update a pool. Create it if it doesn't exist
func (f5 *BigIP) upsertPool(msg comm.Message) error {

	pool, err := f5.cli.GetPool(f5.addPartitionToName(msg.Service.Name))
	if err != nil {
		return errors.New(fmt.Sprintf("Could not get Pool %v %v", msg.Service.Name, err.Error()))
	}

	if pool == nil {
		pool, err = f5.createPool(msg)
		if err != nil {
			return errors.New(fmt.Sprintf("could not create pool %v %v", msg.Service.Name, err.Error()))
		}
	} else {
		if err := f5.updatePoolMembers(pool, msg); err != nil {
			return err
		}
	}

	return nil
}

// buildPolicyRuleFromMsg returns a policy rule based on input message
func (f5 *BigIP) buildPolicyRuleFromMsg(msg comm.Message) bigip.PolicyRule {

	prc := bigip.PolicyRuleCondition{
		Name:            "0",
		CaseInsensitive: false,
		Equals:          false,
		External:        false,
		Remote:          false,
		Values:          msg.Service.DNSAliases,
	}

	pra := bigip.PolicyRuleAction{
		Name:    "0",
		Forward: true,
		Pool:    f5.addPartitionToPath(msg.Service.Name),
		Request: true,
	}

	switch msg.Service.TLS {
	case true:
		prc.Present = true
		prc.SslExtension = true
		prc.SslClientHello = true

		pra.SslClientHello = true

	case false:
		prc.HttpHost = true
		prc.Host = true
		prc.Request = true

		pra.Request = true
	}

	pr := bigip.PolicyRule{
		Name:        msg.Service.Name,
		Description: fmt.Sprintf("ingress rule for %v %v", msg.Service.Name, f5.ObjectDescriptionSuffix),
		Conditions:  []bigip.PolicyRuleCondition{prc},
		Actions:     []bigip.PolicyRuleAction{pra},
	}
	return pr
}

// policyNeedsUpdate checks if a given policy exist and if it needs to be update based on input message
func (f5 *BigIP) policyNeedsUpdate(name string, msg comm.Message) (updateNeeded, policyRuleExist bool, err error) {

	policy, err := f5.cli.GetPolicy(name)
	if err != nil {
		return false, false, errors.New(fmt.Sprintf("Could not get policy %v %v", name, err.Error()))
	}

	if policy == nil {
		log.Debugf("policy %v not found", name)
		return false, false, errors.New("policy not found")
	}

	// get the matching rule and check if they need update
	for _, r := range policy.Rules {
		if r.Name == msg.Service.Name {
			log.Debugf("found matching PolicyRule %v", r.Name)
			policyRuleExist = true
			for _, condition := range r.Conditions {
				if condition.HttpHost && !reflect.DeepEqual(condition.Values, msg.Service.Hosts) {
					log.Debugf("PolicyRule condition for %v differs", msg.Service.Name)
					return true, true, nil
				}
			}
			for _, action := range r.Actions {
				if action.Forward && action.Pool != f5.addPartitionToPath(msg.Service.Name) {
					log.Debugf("PolicyRule action for %v differs", msg.Service.Name)
					return true, policyRuleExist, nil
				}
			}
		}

	}
	return false, policyRuleExist, nil
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

func (f5 *BigIP) getLBPort(msg comm.Message) int {
	if !msg.Service.TLS {
		return f5.HttpPort
	}

	return f5.HttpsPort
}

// newPoolFromService returns a pool (name, hosts and port) from a Service
func (f5 *BigIP) newPoolFromService(msg comm.Message) *bigip.Pool {

	pool := &bigip.Pool{
		Name:              msg.Service.Name,
		Partition:         f5.Partition,
		Description:       fmt.Sprintf("Pool for %v %v", msg.Service.Name, f5.ObjectDescriptionSuffix),
		LoadBalancingMode: f5.LoadBalancingMode,
		Monitor:           f5.MonitorName,
	}

	return pool
}

func (f5 *BigIP) getGlobalPolicyInfo(tls bool) (name, fullName, path string) {
	globalPolicy := f5.GlobalHTTPPolicy
	if tls {
		globalPolicy = f5.GlobalSSLPolicy
	}

	return globalPolicy, f5.addPartitionToName("Drafts~" + globalPolicy), f5.addPartitionToPath("Drafts/" + globalPolicy)
}

// addPartitionToPath adds the name of the partition to the given name
// ie: myPool in partition myPartition -> /myPartition/myPool
func (f5 *BigIP) addPartitionToPath(name string) (fullName string) {
	if f5.Partition != "" {
		return "/" + f5.Partition + "/" + name
	}
	return name
}

// addPartitionToName adds the name of the partition to the given name
// ie: myPool in partition myPartition -> ~myPartition~myPool
func (f5 *BigIP) addPartitionToName(name string) (fullName string) {
	if f5.Partition != "" {
		return "~" + f5.Partition + "~" + name
	}
	return name
}
