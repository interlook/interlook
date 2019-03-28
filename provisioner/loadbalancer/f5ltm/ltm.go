package f5ltm

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
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
	virtualServerResource = "virtual"
	poolResource          = "poolResource"
)

type Extension struct {
	Endpoint     string `yaml:"httpEndpoint"`
	User         string `yaml:"username"`
	Password     string `yaml:"password"`
	AuthProvider string `yaml:"authProvider"`
	AuthToken    string `yaml:"authToken"`
	HttpPort     int    `yaml:"httpPort"`
	HttpsPort    int    `yaml:"httpsPort"`
	MonitorName  string `yaml:"monitorName"`
	TCPProfile   string `yaml:"tcpProfile"`
	httpClient   http.Client
	shutdown     chan bool
}

func (e *Extension) initialize() {
	e.httpClient = makeHttpClient()
	if e.HttpPort == 0 {
		e.HttpPort = 80
	}
	if e.HttpsPort == 0 {
		e.HttpsPort = 443
	}

	e.shutdown = make(chan bool)
}

func (e *Extension) Start(receive <-chan service.Message, send chan<- service.Message) error {
	e.initialize()
	if _, err := e.testConnection(); err != nil {
		return err
	}
	for {
		select {
		case <-e.shutdown:
			log.Info("Extension f5ltm down")
			return nil

		case msg := <-receive:
			log.Debugf("Extension f5ltm received a message %v", msg)

			switch msg.Action {
			case service.AddAction, service.UpdateAction:
				msg.Action = service.UpdateAction
				// check if virtual server already vsExist
				vsExist := true
				currentVirtual, err := e.getVirtualServerByName(msg.Service.Name)
				if err != nil {
					vsExist = false
				}
				// check if pool attached to vs needs to be changed
				if vsExist {
					members, port, err := e.getPoolMembers(msg.Service.Name)
					if err != nil {
						msg.Error = err.Error()
					}
					// check if current pool is as defined in msg
					if !reflect.DeepEqual(members, msg.Service.Hosts) || msg.Service.Port != port {
						// hosts differ, patch f5 pool
						if err := e.patchPoolMembers(msg); err != nil {
							msg.Error = err.Error()
						}
						log.Debugf("pool %v: port differs", msg.Service.Name)
						send <- msg
						continue
					}
					// check if virtual's IP is the one we got in msg
					if !strings.Contains(currentVirtual.Destination, msg.Service.PublicIP+":"+strconv.Itoa(msg.Service.Port)) {
						if err := e.patchPoolDestination(currentVirtual, msg.Service.PublicIP, strconv.Itoa(e.getLBPort(msg))); err != nil {
							msg.Error = err.Error()
						}
					}
					send <- msg
					continue

				}
				log.Debugf("%v not found, creating pool and virtual", msg.Service.Name)
				//TODO: create vs and pool

				send <- msg
			case service.DeleteAction:

				send <- msg

			}
		}
	}
}

func (e *Extension) getLBPort(msg service.Message) int {
	if !msg.Service.TLS {
		return e.HttpPort
	}
	return e.HttpsPort
}

func (e *Extension) Stop() error {
	e.shutdown <- true

	return nil
}

func (e *Extension) patchPoolDestination(vs virtualServerResponse, ip, port string) error {

	req, err := e.newAuthRequest(http.MethodPatch, e.Endpoint+"/mgmt/tm/ltm/virtual/")
	if err != nil {
		return err
	}
	destinationCurrentHostPort := vs.Destination[strings.LastIndex(vs.Destination, "/")+1:]
	destination := strings.TrimRight(vs.Destination, destinationCurrentHostPort) + ip + ":" + port

	buf, err := json.Marshal(destination)
	if err != nil {
		return errors.WithStack(err)
	}

	req.Body = ioutil.NopCloser(bytes.NewReader(buf))

	return nil
}

func (e *Extension) patchPoolMembers(msg service.Message) error {

	var newPoolMembers poolMembers

	for k, host := range msg.Service.Hosts {
		newPoolMembers.Members[k].Address = host + strconv.Itoa(msg.Service.Port)
	}

	req, err := e.newAuthRequest(http.MethodPatch, e.Endpoint+"/mgmt/tm/ltm/pool/"+msg.Service.Name)
	if err != nil {
		return err
	}

	buf, err := json.Marshal(newPoolMembers)
	if err != nil {
		return errors.WithStack(err)
	}

	req.Body = ioutil.NopCloser(bytes.NewReader(buf))

	return nil
}

func (e *Extension) getVirtualServerByName(poolName string) (vs virtualServerResponse, err error) {

	req, err := e.newAuthRequest(http.MethodGet, e.Endpoint+"/mgmt/tm/ltm/pool/"+poolName)
	if err != nil {
		return vs, err
	}

	response, httpCode, err := e.executeRequest(req)
	if err != nil {
		return vs, err
	}
	if httpCode != 200 {
		return vs, err
	}

	err = json.Unmarshal(response, &vs)
	if err != nil {
		return vs, err
	}

	return vs, nil
}

func (e *Extension) getPoolMembers(poolName string) (members []string, port int, err error) {

	var membersResponse poolMembersResponse
	req, err := e.newAuthRequest(http.MethodGet, e.Endpoint+"/mgmt/tm/ltm/pool/"+poolName)
	if err != nil {
		return members, port, err
	}

	response, httpCode, err := e.executeRequest(req)
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

func (e *Extension) isResourceExists(resourceName, resourceType string) (bool, error) {

	req, err := e.newAuthRequest(http.MethodGet, e.Endpoint+"/mgmt/tm/ltm/"+resourceType+"/"+resourceName)
	if err != nil {
		return false, err
	}

	_, httpCode, err := e.executeRequest(req)
	if err != nil {
		return false, err
	}
	if httpCode != 200 {
		return false, err
	}

	return true, nil
}

func (e *Extension) isPoolMembersSame(vs virtualServer) (bool, error) {
	return false, nil
}

func (e *Extension) addVirtualServer(vs virtualServer) error {
	return nil
}

func (e *Extension) deleteVirtualServer(vs virtualServer) error {
	return nil
}

func (e *Extension) addPool(pool pool) error {
	return nil
}

func (e *Extension) deletePool(pool pool) error {
	return nil
}

func (e *Extension) testConnection() (httpRspCode int, err error) {
	req, err := e.newAuthRequest(http.MethodGet, e.Endpoint+"/mgmt/shared/authz/tokens")
	if err != nil {
		return 0, err
	}

	_, httpCode, err := e.executeRequest(req)
	if err != nil {
		return httpCode, err
	}

	if httpCode != 200 {
		return httpCode, errors.New("could not establish connection")
	}

	return httpCode, nil
}

func (e *Extension) getAuthToken() error {
	authPayload := &getTokenPayload{Username: e.User,
		Password:          e.Password,
		LoginProviderName: e.AuthProvider}

	authReq, err := http.NewRequest(http.MethodPost, e.Endpoint+"/mgmt/shared/authn/login", nil)
	if err != nil {
		return errors.New("Could not authenticate " + err.Error())
	}

	buf, err := json.Marshal(authPayload)
	if err != nil {
		return errors.New("Could not authenticate" + err.Error())
	}
	authReq.Body = ioutil.NopCloser(bytes.NewReader(buf))

	rsp, httpCode, err := e.executeRequest(authReq)
	if err != nil {
		return err
	}

	if httpCode != 200 {
		return err
	}

	log.Info(rsp)
	var tokenRsp tokenResponse
	err = json.Unmarshal(rsp, &tokenRsp)
	if err != nil {
		return err
	}

	e.AuthToken = tokenRsp.Token.Token

	return nil
}

// Returns a ready to execute request
func (e *Extension) newAuthRequest(method, url string) (*http.Request, error) {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return req, err
	}

	req.Header.Set("Content-Type", "application/json")

	if e.AuthToken != "" {
		rspCode, err := e.testConnection()
		if err != nil {
			// TODO: check if this is ok or if we should manage token ttl in sep object
			if rspCode == 401 {
				if err = e.getAuthToken(); err != nil {
					return req, err
				}
				if err := e.getAuthToken(); err != nil {
					return req, err
				}
			}
		}
		req.Header.Set("X-F5-Auth-Token", e.AuthToken)
		return req, err
	}
	// basic auth
	if e.AuthProvider == "" {
		req.SetBasicAuth(e.User, e.Password)
		return req, nil
	}
	// get new auth token
	if err := e.getAuthToken(); err != nil {
		return req, err
	}
	req.Header.Set("X-F5-Auth-Token", e.AuthToken)

	return req, nil
}

// Executes the raw request, does not parse response
func (e *Extension) executeRequest(r *http.Request) (responseBody []byte, httpCode int, err error) {

	res, err := e.httpClient.Do(r)
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
		log.Debugf(err.Error())
		return body, res.StatusCode, err
	}

	return body, res.StatusCode, nil
}

type request http.Request

// Adds JSON formatted body to request
func (r *request) setJSONBody(val interface{}) error {
	buf, err := json.Marshal(val)
	if err != nil {
		return errors.WithStack(err)
	}

	r.Body = ioutil.NopCloser(bytes.NewReader(buf))

	return nil
}

func makeHttpClient() http.Client {
	httpClient := http.Client{}

	httpClient.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: true,
		},
	}

	return httpClient
}
