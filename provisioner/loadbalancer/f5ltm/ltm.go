package f5ltm

import (
	"github.com/f5devcentral/go-bigip"
	"github.com/interlook/interlook/comm"
	"github.com/interlook/interlook/log"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"sync"
)

type BigIP struct {
	Endpoint          string `yaml:"httpEndpoint"`
	User              string `yaml:"username"`
	Password          string `yaml:"password"`
	AuthProvider      string `yaml:"authProvider"`
	AuthToken         string `yaml:"authToken"`
	HttpPort          int    `yaml:"httpPort"`
	HttpsPort         int    `yaml:"httpsPort"`
	MonitorName       string `yaml:"monitorName"`
	TCPProfile        string `yaml:"tcpProfile"`
	LoadBalancingMode string `yaml:"loadBalancingMode"`
	Partition         string `yaml:"partition"`
	httpClient        *http.Client
	cli               *bigip.BigIP
	shutdown          chan bool
	send              chan<- comm.Message
	wg                sync.WaitGroup
}

func (f5 *BigIP) initialize() error {

	if f5.HttpPort == 0 {
		f5.HttpPort = 80
	}
	if f5.HttpsPort == 0 {
		f5.HttpsPort = 443
	}

	if f5.LoadBalancingMode == "" {
		f5.LoadBalancingMode = "least-connections-member"
	}

	if f5.AuthProvider == "" && f5.User != "" {
		f5.AuthProvider = "tmos"
	}

	var err error
	f5.cli, err = bigip.NewTokenSession(f5.Endpoint, f5.User, f5.Password, f5.AuthProvider, nil)
	if err != nil {
		log.Errorf("Could not establish connection to f5 %v", err.Error())
	}

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
			f5.wg.Add(1)
			log.Debugf("BigIP f5ltm received a message %v", msg)

			switch msg.Action {
			case comm.AddAction, comm.UpdateAction:
				msg.Action = comm.UpdateAction
				// check if virtual server already vsExist
				var members []string
				var port int
				vsExist := true
				vs, err := f5.cli.GetVirtualServer(f5.addPartitionToName(msg.Service.Name))
				if err != nil || vs == nil {
					vsExist = false
				}

				// check if pool attached to vs needs to be changed
				if vsExist {
					pool, err := f5.cli.GetPool(vs.Pool)
					if err != nil {
						msg.Error = err.Error()
					}

					pm, err := f5.cli.PoolMembers(pool.FullPath)
					if err != nil {
						msg.Error = err.Error()
					}

					for _, member := range pm.PoolMembers {
						i := strings.LastIndex(member.FullPath, ":")
						port, err = strconv.Atoi(member.FullPath[i+1:])
						if err != nil {
							msg.Error = err.Error()
						}
						members = append(members, member.Address)
					}
					// check if current pool is as defined in msg
					if !reflect.DeepEqual(members, msg.Service.Hosts) || msg.Service.Port != port {
						// hosts differ, update f5 pool
						log.Debugf("pool %v: host/hostPort differs", msg.Service.Name)

						if err := f5.updatePoolMembers(vs.Pool, msg); err != nil {
							msg.Error = err.Error()
							f5.send <- msg
							f5.wg.Done()
							continue
						}
					}
					// check if virtual's IP is the one we got in msg
					if !strings.Contains(vs.Destination, msg.Service.PublicIP+":"+strconv.Itoa(f5.getLBPort(msg))) {
						log.Debugf("pool %v: exposed IP differs", msg.Service.Name)
						if err := f5.cli.ModifyVirtualServer(vs.Name, &bigip.VirtualServer{Destination: msg.Service.PublicIP + ":" + strconv.Itoa(f5.getLBPort(msg))}); err != nil {
							msg.Error = err.Error()
							f5.send <- msg
							f5.wg.Done()
							continue
						}
					}

					log.Debugf("f5ltm, nothing to do for service %v", msg.Service.Name)
					f5.send <- msg
					f5.wg.Done()
					continue
				}

				log.Debugf("%v not found, creating pool and virtual server", msg.Service.Name)

				if err := f5.createPool(msg); err != nil {
					msg.Error = err.Error()
					f5.send <- msg
					f5.wg.Done()
					continue
				}

				if err := f5.createVirtualServer(msg); err != nil {
					msg.Error = err.Error()
				}

				f5.send <- msg
				f5.wg.Done()

			case comm.DeleteAction:

				msg.Action = comm.UpdateAction

				if err := f5.cli.DeleteVirtualServer(msg.Service.Name); err != nil {
					msg.Error = err.Error()
				}

				if err := f5.cli.DeletePool(msg.Service.Name); err != nil {
					msg.Error = err.Error()
				}

				if err := f5.cli.DeletePool(msg.Service.Name); err != nil {
					msg.Error = err.Error()
				}

				f5.send <- msg
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

// createVirtualServer created a virtual server from a service
func (f5 *BigIP) createVirtualServer(msg comm.Message) error {

	vs := bigip.VirtualServer{
		Name:        msg.Service.Name,
		Destination: msg.Service.PublicIP + ":" + strconv.Itoa(f5.getLBPort(msg)),
		IPProtocol:  "tcp",
		Pool:        msg.Service.Name,
		Partition:   f5.Partition,
		Description: msg.Service.Name + " (by Interlook)",
	}

	vs.SourceAddressTranslation.Type = "automap"

	if err := f5.cli.AddVirtualServer(&vs); err != nil {
		return err
	}

	return nil
}

func (f5 *BigIP) buildPoolMembersFromMessage(msg comm.Message) bigip.PoolMembers {
	members := make([]bigip.PoolMember, 0)
	for _, host := range msg.Service.Hosts {
		members = append(members, bigip.PoolMember{
			Name:      host,
			Address:   host + ":" + strconv.Itoa(msg.Service.Port),
			Partition: f5.Partition,
		})
	}
	return bigip.PoolMembers{PoolMembers: members}
}

// createPool creates the pool with information from the message
func (f5 *BigIP) createPool(msg comm.Message) error {

	pool := f5.newPoolFromService(msg)
	if err := f5.cli.AddPool(&pool); err != nil {
		return err
	}
	members := f5.buildPoolMembersFromMessage(msg)
	if err := f5.cli.UpdatePoolMembers(pool.Name, &members.PoolMembers); err != nil {
		return err
	}

	return nil
}

// updatePoolMembers replace the members of the pool with the ones from the message
func (f5 *BigIP) updatePoolMembers(pool string, msg comm.Message) error {

	members := make([]bigip.PoolMember, 0)

	for _, host := range msg.Service.Hosts {
		members = append(members, bigip.PoolMember{
			Name:      host + ":" + strconv.Itoa(msg.Service.Port),
			Address:   host,
			Partition: f5.Partition,
		})
	}

	if err := f5.cli.UpdatePoolMembers(pool, &members); err != nil {
		return err
	}

	return nil
}

func (f5 *BigIP) getLBPort(msg comm.Message) int {
	if !msg.Service.TLS {
		return f5.HttpPort
	}
	return f5.HttpsPort
}

// newPoolFromService returns a pool (name, hosts and port) from a Service
func (f5 *BigIP) newPoolFromService(msg comm.Message) bigip.Pool {

	pool := bigip.Pool{
		Name:              msg.Service.Name,
		Partition:         f5.Partition,
		Description:       msg.Service.Name + " pool (by Interlook)",
		LoadBalancingMode: f5.LoadBalancingMode,
		Monitor:           f5.MonitorName,
	}

	return pool
}

// addPartitionToPath adds the name of the partition to the given name
// ie: myPool in partition myPartition -> /myPartition/myPool
/*func (f5 *BigIP) addPartitionToPath(name string) (fullName string) {
    if f5.Partition != "" {
        return "/" + f5.Partition + "/" + name
    }
    return name
}*/

// addPartitionToName adds the name of the partition to the given name
// ie: myPool in partition myPartition -> ~myPartition~myPool
func (f5 *BigIP) addPartitionToName(name string) (fullName string) {
	if f5.Partition != "" {
		return "~" + f5.Partition + "~" + name
	}
	return name
}
