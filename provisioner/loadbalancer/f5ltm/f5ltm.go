package f5ltm

import (
	"fmt"
	"github.com/interlook/interlook/comm"
	"github.com/interlook/interlook/log"
	"strings"
	"time"

	"github.com/scottdware/go-bigip"
	"strconv"
	"sync"
)

const (
	defaultDescriptionSuffix = "(auto generated - do not edit)"
	tmosAuthProvider         = "tmos"
	leastConnectionLBMode    = "least-connections-member"
	httpPort                 = 80
	httpsPort                = 443
	vsUpdateMode             = "vs"
	policyUpdateMode         = "policy"
)

type BigIP struct {
	Endpoint                string `yaml:"httpEndpoint"`
	User                    string `yaml:"username"`
	Password                string `yaml:"password"`
	AuthProvider            string `yaml:"authProvider"`
	HttpPort                int    `yaml:"httpPort"`
	HttpsPort               int    `yaml:"httpsPort"`
	MonitorName             string `yaml:"monitorName"`
	LoadBalancingMode       string `yaml:"loadBalancingMode"`
	Partition               string `yaml:"partition"`
	UpdateMode              string `yaml:"updateMode"`
	GlobalHTTPPolicy        string `yaml:"globalHTTPPolicy"`
	GlobalSSLPolicy         string `yaml:"globalSSLPolicy"`
	ObjectDescriptionSuffix string `yaml:"objectDescriptionSuffix"`
	//CliProxy                string `yaml:"proxy"`
	cli      *bigip.BigIP
	shutdown chan bool
	send     chan<- comm.Message
	wg       sync.WaitGroup
}

/*func (f5 *BigIP) getCliConfigOptions() *bigip.ConfigOptions {
	if f5.CliProxy != "" {
		return &bigip.ConfigOptions{Proxy: f5.CliProxy}
	}
	return nil
}*/

func (f5 *BigIP) initialize() error {

	if f5.HttpPort == 0 {
		f5.HttpPort = httpPort
	}
	if f5.HttpsPort == 0 {
		f5.HttpsPort = httpsPort
	}

	if f5.LoadBalancingMode == "" {
		f5.LoadBalancingMode = leastConnectionLBMode
	}

	if f5.AuthProvider == "" && f5.User != "" {
		f5.AuthProvider = tmosAuthProvider
	}

	if f5.ObjectDescriptionSuffix == "" {
		f5.ObjectDescriptionSuffix = defaultDescriptionSuffix
	}

	var err error
	f5.cli, err = bigip.NewTokenSession(f5.Endpoint, f5.User, f5.Password, f5.AuthProvider, nil)
	if err != nil {
		log.Errorf("Could not establish connection to f5 %v", err.Error())
		return err
	}
	log.Info("initial f5 connection established")

	f5.shutdown = make(chan bool)
	return nil
}

//Start initialize extension and starts listening for messages from core
func (f5 *BigIP) Start(receive <-chan comm.Message, send chan<- comm.Message) error {
	if err := f5.initialize(); err != nil {
		return err
	}

	f5.send = send

	for {
		select {
		case <-f5.shutdown:
			// wait for messages to be processed
			f5.wg.Wait()

			return nil

		case msg := <-receive:
			log.Debugf("BigIP f5ltm received message %v", msg)

			f5.wg.Add(1)

			// "renew" connection
			//f5.cli, _ = bigip.NewTokenSession(f5.Endpoint, f5.User, f5.Password, f5.AuthProvider, nil)
			_ = f5.cli.RefreshTokenSession(10 * time.Minute)
			switch msg.Action {
			case comm.AddAction, comm.UpdateAction:
				updatedMsg := f5.handleUpdate(msg)
				f5.send <- updatedMsg
				f5.wg.Done()

			case comm.DeleteAction:
				updatedMsg := f5.handleDelete(msg)
				f5.send <- updatedMsg
				f5.wg.Done()
			}
		}
	}
}

// Stop stops the extension
func (f5 *BigIP) Stop() error {
	f5.shutdown <- true

	return nil
}

// calls update func based on updateMode config
func (f5 *BigIP) handleUpdate(msg comm.Message) comm.Message {
	switch f5.UpdateMode {
	case vsUpdateMode:
		m := f5.HandleVSUpdate(msg)
		return m
	case policyUpdateMode:
		m := f5.handleGlobalPolicyUpdate(msg)
		return m
	default:
		msg.Error = fmt.Sprintf("unsupported updateMode %v", f5.UpdateMode)
		return msg
	}
}

// calls delete func based on updateMode config
func (f5 *BigIP) handleDelete(msg comm.Message) comm.Message {
	switch f5.UpdateMode {
	case vsUpdateMode:
		m := f5.handleVSDelete(msg)
		return m
	case policyUpdateMode:
		m := f5.handleGlobalPolicyDelete(msg)
		return m
	default:
		msg.Error = fmt.Sprintf("unsupported updateMode %v", f5.UpdateMode)
		return msg
	}
}

// manages the update event for virtual server mode
func (f5 *BigIP) HandleVSUpdate(msg comm.Message) comm.Message {

	vs, err := f5.cli.GetVirtualServer(f5.addPartitionToName(msg.Service.Name))
	if err != nil {
		msg.Error = fmt.Sprintf("Could not get VS %v %v", msg.Service.Name, err.Error())
		return msg
	}

	// check if pool attached to vs needs to be changed
	if vs != nil {
		if err := f5.upsertPool(msg); err != nil {
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

// manages the delete event for virtual server mode
func (f5 *BigIP) handleVSDelete(msg comm.Message) comm.Message {

	msg.Action = comm.UpdateAction

	if err := f5.cli.DeleteVirtualServer(f5.addPartitionToName(msg.Service.Name)); err != nil {
		msg.Error = err.Error()
	}

	if err := f5.cli.DeletePool(f5.addPartitionToName(msg.Service.Name)); err != nil {
		msg.Error = err.Error()
	}

	return msg
}

// handleGlobalPolicyUpdate first check the pool. If it exist, update it as needed.
// If not, create it. Next, handle the policy
//
func (f5 *BigIP) handleGlobalPolicyUpdate(msg comm.Message) comm.Message {

	if err := f5.upsertPool(msg); err != nil {
		msg.Error = err.Error()
		return msg
	}

	//create a draftPath policy
	globalPolicy, draftName, draftPath := f5.getGlobalPolicyInfo(msg.Service.TLS)

	policyNeedsUpdate, policyRuleExist, err := f5.policyNeedsUpdate(f5.addPartitionToName(globalPolicy), msg)
	if err != nil {
		msg.Error = err.Error()
		return msg
	}

	if policyRuleExist && !policyNeedsUpdate {
		log.Debugf("no policy update for %v", msg.Service.Name)
		return msg
	}

	if err := f5.cli.CreateDraftFromPolicy(f5.addPartitionToName(globalPolicy)); err != nil {
		msg.Error = fmt.Sprintf("error creating %v policy %v", draftPath, err.Error())
		return msg
	}
	defer func() {
		if err := f5.cli.DeletePolicy(draftName); err != nil {
			log.Warnf("Error deleting draftPath policy %v %v", globalPolicy, err)
		}
	}()

	if policyNeedsUpdate {

		log.Debugf("updating policy %v", globalPolicy)

		if err := f5.cli.ModifyPolicyRule(draftName, msg.Service.Name, f5.buildPolicyRuleFromMsg(msg)); err != nil {
			msg.Error = fmt.Sprintf("could not modify policy rule %v %v", msg.Service.Name, err.Error())
			return msg
		}

		if err := f5.cli.PublishDraftPolicy(draftPath); err != nil {
			msg.Error = fmt.Sprintf("could not publish draft %v %v", globalPolicy, err.Error())
			return msg
		}

		return msg
	}

	if !policyRuleExist {

		log.Debugf("updating policy %v with new rule for %v", globalPolicy, msg.Service.Name)

		if err := f5.cli.AddRuleToPolicy(draftName, f5.buildPolicyRuleFromMsg(msg)); err != nil {
			msg.Error = fmt.Sprintf("error adding rule %v to draftPath policy %v", msg.Service.Name, err.Error())
			return msg
		}

		if err := f5.cli.PublishDraftPolicy(draftPath); err != nil {
			msg.Error = fmt.Sprintf("could not publish draft %v %v", draftPath, err.Error())
			return msg
		}

		return msg
	}

	return msg
}

func (f5 *BigIP) handleGlobalPolicyDelete(msg comm.Message) comm.Message {
	// create draft
	globalPolicy, draftName, draftPath := f5.getGlobalPolicyInfo(msg.Service.TLS)
	if err := f5.cli.CreateDraftFromPolicy(f5.addPartitionToName(globalPolicy)); err != nil {
		msg.Error = fmt.Sprintf("error creating %v policy %v", draftPath, err.Error())
		return msg
	}
	defer func() {
		if err := f5.cli.DeletePolicy(draftName); err != nil {
			log.Warnf("Error deleting draftPath policy %v %v", globalPolicy, err)
		}
	}()

	// remove policy rule
	if err := f5.cli.RemoveRuleFromPolicy(msg.Service.Name, draftName); err != nil {
		msg.Error = fmt.Sprintf("error remove rule %v from policy %v", msg.Service.Name, err.Error())
	}

	// publish draft
	if err := f5.cli.PublishDraftPolicy(draftPath); err != nil {
		msg.Error = fmt.Sprintf("could not publish draft %v %v", draftPath, err.Error())
		return msg
	}

	// delete pool
	if err := f5.cli.DeletePool(f5.addPartitionToName(msg.Service.Name)); err != nil {
		msg.Error = err.Error()
	}
	return msg
}

// createPool creates the pool with information from the message
func (f5 *BigIP) createPool(msg comm.Message) (pool *bigip.Pool, err error) {

	pool = f5.newPoolFromService(msg)
	if err := f5.cli.AddPool(pool); err != nil {
		return pool, err
	}

	members := f5.buildPoolMembersFromMessage(msg)

	if err := f5.cli.UpdatePoolMembers(f5.addPartitionToName(pool.Name), &members.PoolMembers); err != nil {
		return pool, err
	}
	if err := f5.cli.ModifyPool(f5.addPartitionToName(pool.Name), pool); err != nil {
		return pool, err
	}

	return pool, nil
}

// updatePoolMembers replace the members of the pool with the ones from the message
func (f5 *BigIP) updatePoolMembers(pool *bigip.Pool, msg comm.Message) error {

	un, err := f5.poolMembersNeedsUpdate(pool, msg)
	if err != nil {
		return err
	}

	if !un {
		return nil
	}

	members := f5.buildPoolMembersFromMessage(msg) //make([]bigip.PoolMember, 0)

	if err := f5.cli.UpdatePoolMembers(f5.addPartitionToName(pool.Name), &members.PoolMembers); err != nil {
		return err
	}

	// update pool as pool definition get overwritten by bigip.UpdatePoolMembers
	if err := f5.cli.ModifyPool(f5.addPartitionToName(pool.Name), pool); err != nil {
		return err
	}

	return nil
}

// createVirtualServer created a virtual server from a service
func (f5 *BigIP) createVirtualServer(msg comm.Message) error {

	vs := bigip.VirtualServer{
		Name:        msg.Service.Name,
		Destination: msg.Service.PublicIP + ":" + strconv.Itoa(f5.getLBPort(msg)),
		IPProtocol:  "tcp",
		Pool:        msg.Service.Name,
		Partition:   f5.Partition,
		Description: fmt.Sprintf("Virtual Server for %v %v", msg.Service.Name, f5.ObjectDescriptionSuffix),
	}

	vs.SourceAddressTranslation.Type = "automap"

	if err := f5.cli.AddVirtualServer(&vs); err != nil {
		return err
	}

	return nil
}
