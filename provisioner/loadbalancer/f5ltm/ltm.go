package f5ltm

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/bhuisgen/interlook/log"
	"github.com/bhuisgen/interlook/service"
	"github.com/pkg/errors"
	"io/ioutil"
	"net/http"
	"reflect"
	"strconv"
	"strings"
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
}

func (b *BigIP) initialize() {
	b.httpClient = newHttpClient()
	if b.HttpPort == 0 {
		b.HttpPort = 80
	}
	if b.HttpsPort == 0 {
		b.HttpsPort = 443
	}

	b.shutdown = make(chan bool)
}

func (b *BigIP) Start(receive <-chan service.Message, send chan<- service.Message) error {
	b.initialize()
	if _, err := b.testConnection(); err != nil {
		return err
	}
	for {
		select {
		case <-b.shutdown:
			log.Info("BigIP f5ltm down")
			return nil

		case msg := <-receive:
			log.Debugf("BigIP f5ltm received a message %v", msg)

			switch msg.Action {
			case service.AddAction, service.UpdateAction:
				msg.Action = service.UpdateAction
				// check if virtual server already vsExist
				vsExist := true
				currentVirtual, err := b.getVirtualServerByName(msg.Service.Name)
				if err != nil {
					vsExist = false
				}
				// check if pool attached to vs needs to be changed
				if vsExist {
					members, port, err := b.getPoolMembers(msg.Service.Name)
					if err != nil {
						msg.Error = err.Error()
					}
					// check if current pool is as defined in msg
					if !reflect.DeepEqual(members, msg.Service.Hosts) || msg.Service.Port != port {
						// hosts differ, update f5 pool
						log.Debugf("pool %v: host/port differs", msg.Service.Name)
						if err := b.updatePoolMembers(msg); err != nil {
							msg.Error = err.Error()
						}
						send <- msg
						continue
					}
					// check if virtual's IP is the one we got in msg
					log.Debugf("pool %v: exposed IP differs", msg.Service.Name)
					if !strings.Contains(currentVirtual.Destination, msg.Service.PublicIP+":"+strconv.Itoa(msg.Service.Port)) {
						if err := b.updateVirtualServerIPDestination(currentVirtual, msg.Service.PublicIP, strconv.Itoa(b.getLBPort(msg))); err != nil {
							msg.Error = err.Error()
						}
					}
					send <- msg
					continue
				}

				log.Debugf("%v not found, creating pool and virtual server", msg.Service.Name)

				if err := b.createPool(msg); err != nil {
					msg.Error = err.Error()
				}

				if err := b.createVirtualServer(msg); err != nil {
					msg.Error = err.Error()
				}

				send <- msg
			case service.DeleteAction:
				msg.Action = service.UpdateAction
				vsExist, err := b.isResourceExists(msg.Service.Name, "virtual")
				if err != nil {
					msg.Error = err.Error()
				}

				if vsExist {
					log.Debugf("Found virtual %v", msg.Service.Name)
					if err = b.deleteVirtualServer(msg); err != nil {
						msg.Error = err.Error()
					}
				}

				poolExist, err := b.isResourceExists(msg.Service.Name, "pool")
				if err != nil {
					msg.Error = err.Error()
				}

				if poolExist {
					log.Debugf("Found pool %v", msg.Service.Name)
					if err = b.deletePool(msg); err != nil {
						msg.Error = err.Error()
					}
				}

				send <- msg
			}
		}
	}
}

func (b *BigIP) Stop() error {
	b.shutdown <- true

	return nil
}

func (b *BigIP) getVirtualServerByName(poolName string) (vs virtualServerResponse, err error) {

	r, err := b.newAuthRequest(http.MethodGet, b.Endpoint+poolResource+b.getNameWithPartition(poolName))
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

func (b *BigIP) createVirtualServer(msg service.Message) error {

	vs := virtualServer{
		Name:        msg.Service.Name,
		Destination: msg.Service.PublicIP + ":" + strconv.Itoa(b.getLBPort(msg)),
		IPProtocol:  "tcp",
		Pool:        b.getNameWithPartition(msg.Service.Name),
		Description: msg.Service.Name + " (by Interlook)",
	}
	vs.SourceAddressTranslation.Type = "automap"

	r, err := b.newAuthRequest(http.MethodPost, b.Endpoint+virtualServerResource)
	if err != nil {
		return err
	}

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

func (b *BigIP) updateVirtualServerIPDestination(vs virtualServerResponse, ip, port string) error {

	r, err := b.newAuthRequest(http.MethodPatch, b.Endpoint+virtualServerResource+b.getNameWithPartition(vs.Name))
	if err != nil {
		return err
	}

	destinationCurrentHostPort := vs.Destination[strings.LastIndex(vs.Destination, "/")+1:]
	destination := strings.TrimRight(vs.Destination, destinationCurrentHostPort) + ip + ":" + port

	if err := r.setJSONBody(destination); err != nil {
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

func (b *BigIP) deleteVirtualServer(msg service.Message) error {

	r, err := b.newAuthRequest(http.MethodDelete, b.Endpoint+virtualServerResource+b.getNameWithPartition(msg.Service.Name))
	if err != nil {
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

func (b *BigIP) getPoolMembers(poolName string) (members []string, port int, err error) {

	var membersResponse poolMembersResponse
	r, err := b.newAuthRequest(http.MethodGet, b.Endpoint+poolResource+b.getNameWithPartition(poolName))
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

func (b *BigIP) createPool(msg service.Message) error {

	pool := b.getPoolFromMsg(msg)

	r, err := b.newAuthRequest(http.MethodPost, b.Endpoint+poolResource)
	if err != nil {
		return err
	}

	if err := r.setJSONBody(pool); err != nil {
		return err
	}

	_, httpCode, err := b.executeRequest(r.Req)
	if err != nil {
		return err
	}

	if httpCode != http.StatusOK {
		return errors.New("non 200 return code")
	}

	return nil
}

func (b *BigIP) updatePoolMembers(msg service.Message) error {

	newPoolMembers := poolMembers{}
	members := make([]member, 0)

	for _, host := range msg.Service.Hosts {
		members = append(members, member{
			Name:    host + ":" + strconv.Itoa(msg.Service.Port),
			Address: host})
	}
	newPoolMembers.Members = members

	r, err := b.newAuthRequest(http.MethodPatch, b.Endpoint+poolResource+b.getNameWithPartition(msg.Service.Name))
	if err != nil {
		return err
	}
	if err := r.setJSONBody(newPoolMembers); err != nil {
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

func (b *BigIP) deletePool(msg service.Message) error {

	r, err := b.newAuthRequest(http.MethodDelete, b.Endpoint+poolResource+b.getNameWithPartition(msg.Service.Name))
	if err != nil {
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

func (b *BigIP) isResourceExists(resourceName, resourceType string) (bool, error) {

	r, err := b.newAuthRequest(http.MethodGet, b.Endpoint+"/mgmt/tm/ltm/"+resourceType+"/"+b.getNameWithPartition(resourceName))
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
		return body, res.StatusCode, err
	}

	return body, res.StatusCode, nil
}

func (b *BigIP) getLBPort(msg service.Message) int {
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

func (b *BigIP) getNameWithPartition(name string) (fullName string) {
	if b.Partition != "" {
		return "~" + b.Partition + "~" + name
	}
	return name
}

func (b *BigIP) getPoolFromMsg(msg service.Message) pool {

	var hosts []string

	port := strconv.Itoa(msg.Service.Port)

	for _, host := range msg.Service.Hosts {
		hosts = append(hosts, host+":"+port)
	}

	pool := pool{
		Name:        msg.Service.Name,
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
