package f5ltm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/bhuisgen/interlook/log"
	"github.com/pkg/errors"
	"io/ioutil"
	"net/http"
)

type request struct {
	Req        *http.Request
	HTTPClient *http.Client
	Headers    http.Header
	Token      string
}

// Executes the raw request, does not parse Vault response
func (r *request) execute() (body []byte, httpCode int, err error) {

	res, err := r.HTTPClient.Do(r.Req)
	if err != nil {
		return nil, 0, err
	}
	defer func() {
		if err := res.Body.Close(); err != nil {
			log.Errorf("Error closing body %v", err)
		}
	}()

	if res.StatusCode != http.StatusOK {
		httpErr := fmt.Sprintf("Http call %v returned %v. Body: %v", r.Req.URL.String(), res.Status, string(body))
		return body, res.StatusCode, errors.New(httpErr)
	}

	body, readErr := ioutil.ReadAll(res.Body)
	if readErr != nil {
		return body, res.StatusCode, err
	}

	return body, res.StatusCode, nil
}

// Adds JSON formatted body to request
func (r *request) setJSONBody(val interface{}) error {

	buf, err := json.Marshal(val)
	if err != nil {
		return errors.WithStack(err)
	}
	log.Debugf("Payload %v", string(buf))
	r.Req.Body = ioutil.NopCloser(bytes.NewReader(buf))

	return nil
}
