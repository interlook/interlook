package f5ltm

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/interlook/interlook/comm"
	"github.com/interlook/interlook/log"
	"github.com/pkg/errors"
	"io/ioutil"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"sync"
)

const (
	virtualServerResource = "/mgmt/tm/ltm/virtual/"
	poolResource          = "/mgmt/tm/ltm/pool/"
	loginResource         = "/mgmt/shared/authn/login"
	selfUserResource      = "/mgmt/shared/authz/users/"
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
	shutdown          chan bool
	send              chan<- comm.Message
	wg                sync.WaitGroup
}

func (b *BigIP) initialize() {
	b.httpClient = newHttpClient()
	if b.HttpPort == 0 {
		b.HttpPort = 80
	}
	if b.HttpsPort == 0 {
		b.HttpsPort = 443
	}

	if b.LoadBalancingMode == "" {
		b.LoadBalancingMode = "least-connections-member"
	}

	if b.AuthProvider == "" && b.User != "" {
		b.AuthProvider = "tmos"
	}

	b.shutdown = make(chan bool)
}

//Start initialize extension and starts listening for messages from core
func (b *BigIP) Start(receive <-chan comm.Message, send chan<- comm.Message) error {
	b.initialize()
	b.send = send

	if _, err := b.testConnection(); err != nil {
		return err
	}

	for {
		select {
		case <-b.shutdown:
			// wait for messages to be processed
			b.wg.Wait()

			return nil

		case msg := <-receive:
			b.wg.Add(1)
			log.Debugf("BigIP f5ltm received a message %v", msg)

			switch msg.Action {
			case comm.AddAction, comm.UpdateAction:
				msg.Action = comm.UpdateAction
				// check if virtual server already vsExist
				vsExist := true
				currentVirtual, err := b.getVirtualServerByName(msg.Service.Name)
				if err != nil {
					vsExist = false
				}
				// check if pool attached to vs needs to be changed
				if vsExist {
					members, hostPort, err := b.getPoolMembers(msg.Service.Name)
					if err != nil {
						msg.Error = err.Error()
					}
					// check if current pool is as defined in msg
					if !reflect.DeepEqual(members, msg.Service.Hosts) || msg.Service.Port != hostPort {
						// hosts differ, update f5 pool
						log.Debugf("pool %v: host/hostPort differs", msg.Service.Name)
						if err := b.updatePoolMembers(msg); err != nil {
							msg.Error = err.Error()
							b.send <- msg
							b.wg.Done()
							continue
						}
					}
					// check if virtual's IP is the one we got in msg
					if !strings.Contains(currentVirtual.Destination, msg.Service.PublicIP+":"+strconv.Itoa(b.getLBPort(msg))) {
						log.Debugf("pool %v: exposed IP differs", msg.Service.Name)

						if err := b.updateVirtualServerIPDestination(currentVirtual, msg.Service.PublicIP, strconv.Itoa(b.getLBPort(msg))); err != nil {
							msg.Error = err.Error()
							b.send <- msg
							b.wg.Done()
							continue
						}
					}

					log.Debugf("f5ltm, nothing to do for service %v", msg.Service.Name)
					b.send <- msg
					b.wg.Done()
					continue
				}

				log.Debugf("%v not found, creating pool and virtual server", msg.Service.Name)

				if err := b.createPool(msg); err != nil {
					msg.Error = err.Error()
					b.send <- msg
					b.wg.Done()
					continue
				}

				if err := b.createVirtualServer(msg); err != nil {
					msg.Error = err.Error()
				}

				b.send <- msg
				b.wg.Done()

			case comm.DeleteAction:

				msg.Action = comm.UpdateAction
				vsExist, err := b.isResourceExists(msg.Service.Name, "virtual")
				if err != nil {
					msg.Error = err.Error()
					b.send <- msg
					b.wg.Done()
					continue
				}

				if vsExist {
					log.Debugf("Found virtual %v", msg.Service.Name)
					if err = b.deleteVirtualServer(msg); err != nil {
						msg.Error = err.Error()
						b.send <- msg
						b.wg.Done()
						continue
					}
				}

				poolExist, err := b.isResourceExists(msg.Service.Name, "pool")
				if err != nil {
					msg.Error = err.Error()
					b.send <- msg
					b.wg.Done()
					continue
				}

				if poolExist {
					log.Debugf("Found pool %v", msg.Service.Name)
					if err = b.deletePool(msg); err != nil {
						msg.Error = err.Error()
					}
				}

				b.send <- msg
				b.wg.Done()
			}
		}
	}
}

// Stop stops the extension
func (b *BigIP) Stop() error {
	b.shutdown <- true

	return nil
}

// getVirtualServerByName returns the virtual server from its name
func (b *BigIP) getVirtualServerByName(name string) (vs virtualServerResponse, err error) {

	r, err := b.newAuthRequest(http.MethodGet, b.Endpoint+virtualServerResource+b.addPartitionToName(name))
	if err != nil {
		return vs, err
	}

	res, httpCode, err := b.executeRequest(r.Req)
	if err != nil {
		return vs, err
	}

	err = json.Unmarshal(res, &vs)
	if err != nil {
		return vs, err
	}

	if httpCode != 200 {
		return vs, errors.New(fmt.Sprintf("HttpCode %v, message: %v", httpCode, string(res)))
	}

	return vs, nil
}

// createVirtualServer created a virtual server from a service
func (b *BigIP) createVirtualServer(msg comm.Message) error {

	vs := virtualServer{
		Name:        b.addPartitionToPath(msg.Service.Name),
		Destination: b.addPartitionToPath(msg.Service.PublicIP + ":" + strconv.Itoa(b.getLBPort(msg))),
		IPProtocol:  "tcp",
		Pool:        b.addPartitionToPath(msg.Service.Name),
		Description: msg.Service.Name + " (by Interlook)",
	}
	vs.SourceAddressTranslation.Type = "automap"

	r, err := b.newAuthRequest(http.MethodPost, b.Endpoint+virtualServerResource)
	if err != nil {
		return err
	}
	log.Debugf("url: %v", b.Endpoint+poolResource+b.addPartitionToName(msg.Service.Name))
	if err := r.setJSONBody(vs); err != nil {
		return err
	}

	res, httpCode, err := b.executeRequest(r.Req)
	if err != nil {
		return err
	}

	if httpCode != http.StatusOK {
		return errors.New(fmt.Sprintf("HttpCode %v, message: %v", httpCode, string(res)))
	}

	return nil
}

// updateVirtualServerIPDestination updates the public IP of the virtual server
func (b *BigIP) updateVirtualServerIPDestination(vs virtualServerResponse, ip, port string) error {

	var destPayload destinationPayload

	r, err := b.newAuthRequest(http.MethodPatch, b.Endpoint+virtualServerResource+b.addPartitionToName(vs.Name))
	if err != nil {
		return err
	}

	destinationCurrentHostPort := vs.Destination[strings.LastIndex(vs.Destination, "/")+1:]
	destPayload.Destination = strings.TrimRight(vs.Destination, destinationCurrentHostPort) + ip + ":" + port
	destPayload.Partition = b.Partition

	if err := r.setJSONBody(destPayload); err != nil {
		return err
	}

	res, httpCode, err := b.executeRequest(r.Req)
	if err != nil {
		return err
	}

	if httpCode != http.StatusOK {
		return errors.New(fmt.Sprintf("HttpCode %v, message: %v", httpCode, string(res)))
	}

	return nil
}

func (b *BigIP) deleteVirtualServer(msg comm.Message) error {

	var deletePayload deleteResourcePayload
	deletePayload.Partition = b.Partition
	deletePayload.FullPath = b.addPartitionToPath(msg.Service.Name)

	r, err := b.newAuthRequest(http.MethodDelete, b.Endpoint+virtualServerResource+b.addPartitionToName(msg.Service.Name))
	if err != nil {
		return err
	}

	if err := r.setJSONBody(deletePayload); err != nil {
		return err
	}

	res, httpCode, err := b.executeRequest(r.Req)
	if err != nil {
		return err
	}

	if httpCode != http.StatusOK {
		return errors.New(fmt.Sprintf("HttpCode %v, message: %v", httpCode, string(res)))
	}

	return nil
}

// getPoolMembers returns the members of a pool
func (b *BigIP) getPoolMembers(poolName string) (members []string, port int, err error) {

	var membersResponse poolMembersResponse
	r, err := b.newAuthRequest(http.MethodGet, b.Endpoint+poolResource+b.addPartitionToName(poolName)+"/members")
	if err != nil {
		return members, port, err
	}

	response, httpCode, err := b.executeRequest(r.Req)
	if err != nil {
		return members, port, err
	}

	if httpCode != 200 {
		return members, port, err
	}

	err = json.Unmarshal(response, &membersResponse)
	if err != nil {
		return members, port, err
	}

	for _, member := range membersResponse.Items {
		i := strings.LastIndex(member.FullPath, ":")
		port, err = strconv.Atoi(member.FullPath[i+1:])
		if err != nil {
			return members, port, err
		}
		members = append(members, member.Address)
	}

	return members, port, nil
}

// createPool creates the pool with information from the message
func (b *BigIP) createPool(msg comm.Message) error {

	pool := b.getPoolFromService(msg)

	r, err := b.newAuthRequest(http.MethodPost, b.Endpoint+poolResource)
	if err != nil {
		return err
	}
	log.Debugf("url: %v", b.Endpoint+poolResource)
	if err := r.setJSONBody(pool); err != nil {
		return err
	}

	res, httpCode, err := b.executeRequest(r.Req)
	if err != nil {
		return err
	}

	if httpCode != http.StatusOK {
		return errors.New(fmt.Sprintf("HttpCode %v, message: %v", httpCode, string(res)))
	}

	return nil
}

// updatePoolMembers replace the members of the pool with the ones from the message
func (b *BigIP) updatePoolMembers(msg comm.Message) error {

	newPoolMembers := poolMembers{}
	members := make([]member, 0)

	for _, host := range msg.Service.Hosts {
		members = append(members, member{
			Name:    host + ":" + strconv.Itoa(msg.Service.Port),
			Address: host})
	}
	newPoolMembers.Members = members

	r, err := b.newAuthRequest(http.MethodPatch, b.Endpoint+poolResource+b.addPartitionToName(msg.Service.Name))
	if err != nil {
		return err
	}
	if err := r.setJSONBody(newPoolMembers); err != nil {
		return err
	}

	log.Debugf("url: %v", b.Endpoint+poolResource+b.addPartitionToName(msg.Service.Name))
	res, httpCode, err := b.executeRequest(r.Req)
	if err != nil {
		return err
	}

	if httpCode != http.StatusOK {
		return errors.New(fmt.Sprintf("HttpCode %v, message: %v", httpCode, string(res)))
	}

	return nil
}

// deletePool deletes the pool from f5
func (b *BigIP) deletePool(msg comm.Message) error {

	var deletePayload deleteResourcePayload
	deletePayload.Partition = b.Partition
	deletePayload.FullPath = b.addPartitionToPath(msg.Service.Name)

	r, err := b.newAuthRequest(http.MethodDelete, b.Endpoint+poolResource+b.addPartitionToName(msg.Service.Name))
	if err != nil {
		return err
	}

	if err := r.setJSONBody(deletePayload); err != nil {
		return err
	}

	res, httpCode, err := b.executeRequest(r.Req)
	if err != nil {
		return err
	}

	if httpCode != http.StatusOK {
		return errors.New(fmt.Sprintf("HttpCode %v, message: %v", httpCode, string(res)))
	}

	return nil
}

// isResourceExists checks if a given resource is already defined on f5
func (b *BigIP) isResourceExists(resourceName, resourceType string) (bool, error) {

	r, err := b.newAuthRequest(http.MethodGet, b.Endpoint+"/mgmt/tm/ltm/"+resourceType+"/"+b.addPartitionToName(resourceName))
	if err != nil {
		return false, err
	}

	_, httpCode, err := b.executeRequest(r.Req)
	if err != nil {
		return false, err
	}
	if httpCode != 200 {
		return false, err
	}

	return true, nil
}

// testConnection tests the authz f5 endpoint
func (b *BigIP) testConnection() (httpRspCode int, err error) {

	r, err := b.newAuthRequest(http.MethodGet, b.Endpoint+selfUserResource+b.User)
	//+"/mgmt/shared/authz/tokens")
	if err != nil {
		return 0, err
	}

	res, httpCode, err := b.executeRequest(r.Req)
	if err != nil {
		return httpCode, err
	}

	if httpCode != 200 {
		return httpCode, errors.New(fmt.Sprintf("Could not establish connection: HttpCode %v, message: %v", httpCode, string(res)))
	}

	return httpCode, nil
}

// Executes the raw request, does not parse response
func (b *BigIP) executeRequest(r *http.Request) (responseBody []byte, httpCode int, err error) {

	res, err := b.httpClient.Do(r)
	if err != nil {
		log.Error(err.Error())
		return nil, 0, err
	}
	defer func() {
		if err := res.Body.Close(); err != nil {
			log.Errorf("Error closing body", err)
		}
	}()

	body, readErr := ioutil.ReadAll(res.Body)
	if readErr != nil {
		log.Debugf(readErr.Error())
		return body, res.StatusCode, readErr
	}

	return body, res.StatusCode, nil
}

func (b *BigIP) getLBPort(msg comm.Message) int {
	if !msg.Service.TLS {
		return b.HttpPort
	}
	return b.HttpsPort
}

func (b *BigIP) getAuthToken() error {
	authPayload := &getTokenPayload{Username: b.User,
		Password:          b.Password,
		LoginProviderName: b.AuthProvider}

	authReq, err := http.NewRequest(http.MethodPost, b.Endpoint+loginResource, nil)
	if err != nil {
		return errors.New("Could not authenticate " + err.Error())
	}

	buf, err := json.Marshal(authPayload)
	if err != nil {
		return errors.New("Could not authenticate" + err.Error())
	}
	authReq.Body = ioutil.NopCloser(bytes.NewReader(buf))

	res, httpCode, err := b.executeRequest(authReq)
	if err != nil {
		return err
	}

	if httpCode != 200 {
		return errors.New(fmt.Sprintf("Could not authenticate: HttpCode %v, message: %v", httpCode, string(res)))
	}

	var tokenRsp tokenResponse
	err = json.Unmarshal(res, &tokenRsp)
	if err != nil {
		return err
	}

	b.AuthToken = tokenRsp.Token.Token

	return nil
}

// Returns a ready to execute request
func (b *BigIP) newAuthRequest(method, url string) (*request, error) {

	var err error
	r := new(request)

	r.Req, err = http.NewRequest(method, url, nil)
	if err != nil {
		return r, err
	}

	r.HTTPClient = b.httpClient

	r.Req.Header.Set("Content-Type", "application/json")

	if b.AuthToken != "" {
		rspCode, err := b.testConnection()
		if err != nil {
			// TODO: check if this is ok or if we should manage token ttl in sep object
			if rspCode == 401 {
				if err = b.getAuthToken(); err != nil {
					return r, err
				}
				if err := b.getAuthToken(); err != nil {
					return r, err
				}
			}
		}
		r.Req.Header.Set("X-F5-Auth-Token", b.AuthToken)
		return r, err
	}
	// basic auth
	if b.AuthProvider == "tmos" {
		r.Req.SetBasicAuth(b.User, b.Password)
		return r, nil
	}
	// get new auth token
	if err := b.getAuthToken(); err != nil {
		return r, err
	}
	r.Req.Header.Set("X-F5-Auth-Token", b.AuthToken)

	return r, nil
}

// addPartitionToPath adds the name of the partition to the given name
// ie: myPool in partition myPartition -> /myPartition/myPool
func (b *BigIP) addPartitionToPath(name string) (fullName string) {
	if b.Partition != "" {
		return "/" + b.Partition + "/" + name
	}
	return name
}

// addPartitionToName adds the name of the partition to the given name
// ie: myPool in partition myPartition -> ~myPartition~myPool
func (b *BigIP) addPartitionToName(name string) (fullName string) {
	if b.Partition != "" {
		return "~" + b.Partition + "~" + name
	}
	return name
}

// getPoolFromService returns a pool (name, hosts and port) from a Service
func (b *BigIP) getPoolFromService(msg comm.Message) pool {

	var hosts []string

	port := strconv.Itoa(msg.Service.Port)

	for _, host := range msg.Service.Hosts {
		hosts = append(hosts, b.addPartitionToPath("")+host+":"+port)
	}

	pool := pool{
		Name:        b.addPartitionToPath(msg.Service.Name),
		Members:     hosts,
		Description: msg.Service.Name + " (by Interlook)",
	}

	if b.LoadBalancingMode != "" {
		pool.LoadBalancingMode = b.LoadBalancingMode
	}

	if b.MonitorName != "" {
		pool.Monitor = b.MonitorName
	}

	return pool
}

func newHttpClient() *http.Client {
	httpClient := http.Client{}

	httpClient.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: true,
		},
	}

	return &httpClient
}
