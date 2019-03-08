package kemplm

import (
	"crypto/tls"
	"errors"
	"github.com/bhuisgen/interlook/log"
	"github.com/bhuisgen/interlook/service"
	"io/ioutil"
	"net/http"
	"strconv"
)

// FIXME: Add LB port options in config + service overwrite for non http services?

type KempLM struct {
	Endpoint   string `yaml:"endpoint"`
	User       string `yaml:"username"`
	Password   string `yaml:"password"`
	shutdown   chan bool
	httpClient http.Client
}

func (k *KempLM) Start(receive <-chan service.Message, send chan<- service.Message) error {
	k.httpClient = makeHttpClient()
	// TODO: add connection test

	k.shutdown = make(chan bool)

	for {
		select {
		case <-k.shutdown:
			logger.DefaultLogger().Warn("Extension lb.kemplm down")
			return nil
		case msg := <-receive:
			logger.DefaultLogger().Debugf("Extension kemplm received a message")
			switch msg.Action {
			case service.MsgAddAction:
				msg.Action = service.MsgUpdateFromExtension

				if err := k.addVS(msg); err != nil {
					logger.DefaultLogger().Debugf("error %v in addVS", err.Error())
					msg.Error = err.Error()
				}

				if err := k.addRS(msg); err != nil {
					logger.DefaultLogger().Debugf("error %v in addRS", err.Error())
					msg.Error = err.Error()
				}
				send <- msg
			case service.MsgDeleteAction:
				msg.Action = service.MsgUpdateFromExtension

				exist, err := k.isVSDefined(msg)
				if err != nil {
					msg.Error = err.Error()
				}

				if exist {
					if err := k.deleteVS(msg); err != nil {
						msg.Error = err.Error()
					}
				}
				send <- msg
			}
		}
	}
}

func (k *KempLM) Stop() {
	k.shutdown <- true
}

func (k *KempLM) deleteVS(msg service.Message) error {
	req, err := k.newVSRequest("/access/delvs", msg)
	if err != nil {
		return err
	}

	_, err = k.executeRequest(req)
	if err != nil {
		return err
	}

	return nil
}

func (k *KempLM) isRSDefined(msg service.Message) (bool, error) {
	req, err := k.newVSRequest("/access/listrs", msg)
	if err != nil {
		return false, err
	}

	q := req.URL.Query()
	q.Add("rsport", strconv.Itoa(msg.Service.Port))
	q.Add("rs", msg.Service.Hosts[0])

	req.URL.RawQuery = q.Encode()

	_, err = k.executeRequest(req)
	if err != nil {
		return false, err
	}

	return true, nil
}

func (k *KempLM) isVSDefined(msg service.Message) (bool, error) {
	req, err := k.newVSRequest("/access/listvs", msg)
	if err != nil {
		return false, err
	}

	_, err = k.executeRequest(req)
	if err != nil {
		return false, err
	}

	return true, nil
}

func (k *KempLM) addVS(msg service.Message) error {
	req, err := k.newVSRequest("/access/addvs", msg)
	if err != nil {
		return err
	}

	q := req.URL.Query()
	q.Add("nickname", msg.Service.Name)
	q.Add("vstype", "gen")
	q.Add("checktype", "icmp")

	req.URL.RawQuery = q.Encode()

	_, err = k.executeRequest(req)
	if err != nil {
		return err
	}

	return nil
}

func (k *KempLM) addRS(msg service.Message) error {
	for _, host := range msg.Service.Hosts {
		req, err := k.newVSRequest("/access/addrs", msg)
		if err != nil {
			return err
		}

		q := req.URL.Query()
		q.Add("rs", host)
		q.Add("rsport", strconv.Itoa(msg.Service.Port))

		req.URL.RawQuery = q.Encode()

		req, err = k.newAuthRequest(http.MethodGet, req.URL.String())
		if err != nil {
			return err
		}

		_, err = k.executeRequest(req)
		if err != nil {
			return err
		}
	}
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

// Returns a ready to execute request
func (k *KempLM) newAuthRequest(method, url string) (*http.Request, error) {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return req, err
	}

	//TODO: add ssl auth support
	req.SetBasicAuth(k.User, k.Password)

	req.Header.Set("Content-Type", "application/xml")

	return req, nil
}

func (k *KempLM) newVSRequest(path string, msg service.Message) (*http.Request, error) {
	req, err := k.newAuthRequest(http.MethodGet, k.Endpoint+path)
	if err != nil {
		return req, err
	}
	port := "443"

	if !msg.Service.TLS {
		port = "80"
	}

	q := req.URL.Query()
	q.Add("vs", msg.Service.PublicIP)
	q.Add("port", port)
	q.Add("prot", "tcp")

	req.URL.RawQuery = q.Encode()

	return req, nil
}

// Executes the raw request, does not parse Vault response
func (k *KempLM) executeRequest(r *http.Request) ([]byte, error) {
	logger.DefaultLogger().Debugf("exec url: %v", r.URL.String())

	res, err := k.httpClient.Do(r)
	if err != nil {
		logger.DefaultLogger().Error(err.Error())
		return nil, err
	}
	defer res.Body.Close()

	body, readErr := ioutil.ReadAll(res.Body)
	if readErr != nil {
		logger.DefaultLogger().Debugf(err.Error())
		return body, err
	}

	if res.StatusCode != http.StatusOK {
		logger.DefaultLogger().Debugf(res.Status)
		return body, errors.New("non 200 return code ")
	}

	return body, nil
}
