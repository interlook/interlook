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
)

type Provisioner struct {
	Endpoint     string
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

type tokenResponse struct {
	Username       string `json:"username"`
	LoginReference struct {
		Link string `json:"link"`
	} `json:"loginReference"`
	LoginProviderName string `json:"loginProviderName"`
	Token             struct {
		Token            string `json:"token"`
		Name             string `json:"name"`
		UserName         string `json:"userName"`
		AuthProviderName string `json:"authProviderName"`
		User             struct {
			Link string `json:"link"`
		} `json:"user"`
		Timeout          int    `json:"timeout"`
		StartTime        string `json:"startTime"`
		Address          string `json:"address"`
		Partition        string `json:"partition"`
		Generation       int    `json:"generation"`
		LastUpdateMicros int64  `json:"lastUpdateMicros"`
		ExpirationMicros int64  `json:"expirationMicros"`
		Kind             string `json:"kind"`
		SelfLink         string `json:"selfLink"`
	} `json:"token"`
	Generation       int `json:"generation"`
	LastUpdateMicros int `json:"lastUpdateMicros"`
}

// https://f5IP/mgmt/shared/authz/users/{user}
type authSelfTestResponse struct {
	Name             string `json:"name"`
	DisplayName      string `json:"displayName"`
	Shell            string `json:"shell"`
	Generation       int    `json:"generation"`
	LastUpdateMicros int    `json:"lastUpdateMicros"`
	Kind             string `json:"kind"`
	SelfLink         string `json:"selfLink"`
}

type transaction struct {
	TransID          int64  `json:"transId"`
	State            string `json:"state"`
	TimeoutSeconds   int    `json:"timeoutSeconds"`
	AsyncExecution   bool   `json:"asyncExecution"`
	ValidateOnly     bool   `json:"validateOnly"`
	ExecutionTimeout int    `json:"executionTimeout"`
	ExecutionTime    int    `json:"executionTime"`
	FailureReason    string `json:"failureReason"`
	Kind             string `json:"kind"`
	SelfLink         string `json:"selfLink"`
}

type getTokenPayload struct {
	Username          string `json:"username"`
	Password          string `json:"password"`
	LoginProviderName string `json:"loginProviderName"`
}

type pool struct {
	Name    string   `json:"name"`
	Monitor string   `json:"monitor"`
	Members []string `json:"members"`
}

type virtualServer struct {
	Name                     string `json:"name"`
	Destination              string `json:"destination"`
	IPProtocol               string `json:"ipProtocol"`
	Pool                     string `json:"pool"`
	SourceAddressTranslation struct {
		Type string `json:"type"`
	} `json:"sourceAddressTranslation"`
	Profiles []interface{} `json:"profiles"`
}

func (p *Provisioner) initialize() {
	p.httpClient = makeHttpClient()
	if p.HttpPort == 0 {
		p.HttpPort = 80
	}
	if p.HttpsPort == 0 {
		p.HttpsPort = 443
	}

	p.shutdown = make(chan bool)
}

func (p *Provisioner) Start(receive <-chan service.Message, send chan<- service.Message) error {
	p.initialize()
	if _, err := p.testConnection(); err != nil {
		return err
	}
	for {
		select {
		case <-p.shutdown:
			log.Info("Extension f5ltm down")
			return nil
		case msg := <-receive:
			log.Debugf("Extension f5ltm received a message %v", msg)
		}
	}
}

func (p *Provisioner) Stop() error {
	p.shutdown <- true

	return nil
}

func (p *Provisioner) isVirtualServerExists(vs virtualServer) (bool, error) {
	return false, nil
}

func (p *Provisioner) isPoolExists(pool pool) (bool, error) {
	return false, nil
}

func (p *Provisioner) isPoolSame(pool pool) (bool, error) {
	return false, nil
}

func (p *Provisioner) isVirtualServerSame(vs virtualServer) (bool, error) {
	return false, nil
}

func (p *Provisioner) addVirtualServer(vs virtualServer) error {
	return nil
}

func (p *Provisioner) deleteVirtualServer(vs virtualServer) error {
	return nil
}

func (p *Provisioner) addPool(pool pool) error {
	return nil
}

func (p *Provisioner) deletePool(pool pool) error {
	return nil
}

func (p *Provisioner) testConnection() (httpRspCode int, err error) {
	req, err := p.newAuthRequest(http.MethodGet, p.Endpoint+"/mgmt/shared/authz/tokens")
	if err != nil {
		return 0, err
	}

	_, httpCode, err := p.executeRequest(req)
	if err != nil {
		return httpCode, err
	}

	if httpCode != 200 {
		return httpCode, errors.New("could not establish connection")
	}

	return httpCode, nil
}

func (p *Provisioner) getAuthToken() error {
	authPayload := &getTokenPayload{Username: p.User,
		Password:          p.Password,
		LoginProviderName: p.AuthProvider}

	authReq, err := http.NewRequest(http.MethodPost, p.Endpoint+"/mgmt/shared/authn/login", nil)
	if err != nil {
		return errors.New("Could not authenticate " + err.Error())
	}

	buf, err := json.Marshal(authPayload)
	if err != nil {
		return errors.New("Could not authenticate" + err.Error())
	}
	authReq.Body = ioutil.NopCloser(bytes.NewReader(buf))

	rsp, httpCode, err := p.executeRequest(authReq)
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

	p.AuthToken = tokenRsp.Token.Token

	return nil
}

// Returns a ready to execute request
func (p *Provisioner) newAuthRequest(method, url string) (*http.Request, error) {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return req, err
	}

	req.Header.Set("Content-Type", "application/json")

	if p.AuthToken != "" {
		rspCode, err := p.testConnection()
		if err != nil {
			// TODO: check if this is ok or if we should manage token ttl in sep object
			if rspCode == 401 {
				if err = p.getAuthToken(); err != nil {
					return req, err
				}
				if err := p.getAuthToken(); err != nil {
					return req, err
				}
			}
		}
		req.Header.Set("X-F5-Auth-Token", p.AuthToken)
		return req, err
	}
	// basic auth
	if p.AuthProvider == "" {
		req.SetBasicAuth(p.User, p.Password)
		return req, nil
	}
	// get new auth token
	if err := p.getAuthToken(); err != nil {
		return req, err
	}
	req.Header.Set("X-F5-Auth-Token", p.AuthToken)

	return req, nil
}

// Executes the raw request, does not parse response
func (p *Provisioner) executeRequest(r *http.Request) (body []byte, httpRspCode int, err error) {

	res, err := p.httpClient.Do(r)
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
