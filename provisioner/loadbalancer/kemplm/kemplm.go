package kemplm

import (
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/bhuisgen/interlook/log"
	"github.com/bhuisgen/interlook/messaging"
	"io/ioutil"
	"net/http"
	"strconv"
)

type KempLM struct {
	Endpoint   string `yaml:"endpoint"`
	User       string `yaml:"username"`
	Password   string `yaml:"password"`
	HttpPort   int    `yaml:"httpPort"`
	HttpsPort  int    `yaml:"httpsPort"`
	shutdown   chan bool
	httpClient http.Client
}

func (k *KempLM) initialize() {
	k.httpClient = makeHttpClient()
	if k.HttpPort == 0 {
		k.HttpPort = 80
	}
	if k.HttpsPort == 0 {
		k.HttpsPort = 443
	}

	k.shutdown = make(chan bool)

}

func (k *KempLM) Start(receive <-chan messaging.Message, send chan<- messaging.Message) error {
	k.initialize()

	if err := k.testConnection(); err != nil {
		return err
	}

	for {
		select {
		case <-k.shutdown:
			log.Info("Extension lb.kemplm down")
			return nil

		case msg := <-receive:
			log.Debugf("Extension kemplm received a message")
			switch msg.Action {
			case messaging.AddAction:
				msg.Action = messaging.UpdateAction

				if err := k.addVS(msg); err != nil {
					log.Debugf("error %v in addVS", err.Error())
					msg.Error = err.Error()
					send <- msg
					continue
				}

				if err := k.addRS(msg); err != nil {
					log.Debugf("error %v in addRS", err.Error())
					msg.Error = err.Error()
					send <- msg
					continue

				}
				send <- msg

			case messaging.UpdateAction:
				// check if rs and or vs needs to be updated
				// delete vs
				// create vs and rs
				send <- msg

			case messaging.DeleteAction:
				msg.Action = messaging.UpdateAction

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

func (k *KempLM) Stop() error {
	k.shutdown <- true

	return nil
}

func (k *KempLM) testConnection() error {
	req, err := k.newAuthRequest(http.MethodGet, k.Endpoint+"/access/listvs")
	if err != nil {
		return err
	}

	_, httpCode, err := k.executeRequest(req)
	if err != nil {
		return err
	}

	if httpCode != 200 {
		return errors.New("could not establish connection")
	}

	return nil
}

func (k *KempLM) deleteVS(msg messaging.Message) error {
	req, err := k.newVSRequest("/access/delvs", msg)
	if err != nil {
		return err
	}

	_, httpCode, err := k.executeRequest(req)
	if err != nil {
		return err
	}

	if httpCode != 200 {
		return errors.New("could not delete VS")
	}

	return nil
}

func (k *KempLM) isRSDefined(msg messaging.Message, host string) (bool, error) {
	req, err := k.newVSRequest("/access/showrs", msg)
	if err != nil {
		return false, err
	}

	q := req.URL.Query()
	q.Add("rsport", strconv.Itoa(msg.Service.Port))
	q.Add("rs", host)

	req.URL.RawQuery = q.Encode()

	_, httpCode, err := k.executeRequest(req)
	if err != nil {
		return false, err
	}
	if httpCode == 200 {
		return true, nil
	}
	return false, nil
}

func (k *KempLM) isVSDefined(msg messaging.Message) (bool, error) {
	req, err := k.newVSRequest("/access/showvs", msg)
	if err != nil {
		return false, err
	}

	_, httpCode, err := k.executeRequest(req)
	if err != nil {
		return false, err
	}

	switch httpCode {
	case 200:
		return true, nil

	case 422:
		return false, nil

	default:
		return false, errors.New(fmt.Sprintf("Unexpected http exit code %v", httpCode))
	}
}

func (k *KempLM) addVS(msg messaging.Message) error {

	exist, err := k.isVSDefined(msg)
	if err != nil {
		return err
	}

	if !exist {
		req, err := k.newVSRequest("/access/addvs", msg)
		if err != nil {
			return err
		}

		q := req.URL.Query()
		q.Add("nickname", msg.Service.Name)
		q.Add("vstype", "gen")
		q.Add("checktype", "icmp")

		req.URL.RawQuery = q.Encode()

		_, _, err = k.executeRequest(req)
		if err != nil {
			return err
		}
	}

	return nil
}

func (k *KempLM) addRS(msg messaging.Message) error {
	for _, host := range msg.Service.Hosts {
		rsExists, _ := k.isRSDefined(msg, host)

		if !rsExists {
			req, err := k.newVSRequest("/access/addrs", msg)
			if err != nil {
				return err
			}
			log.Debugf("rs does not exists for host %v", host)
			q := req.URL.Query()
			q.Add("rs", host)
			q.Add("rsport", strconv.Itoa(msg.Service.Port))
			q.Add("non_local", "1")

			req.URL.RawQuery = q.Encode()
			log.Debugf("url: %v", req.URL.String())
			req, err = k.newAuthRequest(http.MethodGet, req.URL.String())
			if err != nil {
				return err
			}
			body, httpCode, err := k.executeRequest(req)
			if err != nil {
				return err
			}
			if httpCode != 200 {
				log.Debugf("non 200 return code (%v). Body: %v", httpCode, string(body))
			}
			log.Debugf("http response code %v", httpCode)
			// }
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

func (k *KempLM) newVSRequest(path string, msg messaging.Message) (*http.Request, error) {
	req, err := k.newAuthRequest(http.MethodGet, k.Endpoint+path)
	if err != nil {
		return req, err
	}

	port := strconv.Itoa(k.HttpsPort)
	if !msg.Service.TLS {
		port = strconv.Itoa(k.HttpPort)
	}

	q := req.URL.Query()
	q.Add("vs", msg.Service.PublicIP)
	q.Add("port", port)
	q.Add("prot", "tcp")

	req.URL.RawQuery = q.Encode()

	return req, nil
}

// Executes the raw request, returns raw response body
func (k *KempLM) executeRequest(r *http.Request) (body []byte, statusCode int, err error) {
	//var err error
	//log.Debugf("exec url: %v", r.URL.String())

	res, err := k.httpClient.Do(r)
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
