/*
 * Copyright (c) 2019 The Interlook authors
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in all
 * copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NON INFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
 * SOFTWARE.
 */

package messaging

import "reflect"

const (
	//define message actions
	AddAction     = "add"
	UpdateAction  = "update"
	DeleteAction  = "delete"
	RefreshAction = "refresh"
)

// Message holds config information with providers
type Message struct {
	// add update or remove
	Action string
	// will be filled by core's extensionListener
	Sender  string
	Error   string
	Service Service
}

// Service holds the containerized service
type Service struct {
	Provider   string   `json:"provider,omitempty"`
	Name       string   `json:"name,omitempty"`
	Hosts      []string `json:"hosts,omitempty"`
	Port       int      `json:"port,omitempty"`
	TLS        bool     `json:"tls,omitempty"`
	PublicIP   string   `json:"public_ip,omitempty"`
	DNSAliases []string `json:"dns_name,omitempty"`
	Info       string   `json:"info,omitempty"`
	Error      string   `json:"error,omitempty"`
}

// IsSameThan compares given service definition received from provider
// against current definition
// hosts list, port and tls
// returns a list of fields that differ
func (s *Service) IsSameThan(targetService Service) (bool, []string) {
	var diff []string
	if !reflect.DeepEqual(s.DNSAliases, targetService.DNSAliases) {
		diff = append(diff, "DNSNames")
	}
	if s.Port != targetService.Port {
		diff = append(diff, "Port")
	}
	if s.TLS != targetService.TLS {
		diff = append(diff, "TLS")
	}
	if !reflect.DeepEqual(s.Hosts, targetService.Hosts) {
		diff = append(diff, "Hosts")
	}
	if len(diff) > 0 {
		return false, diff
	}
	return true, nil
}
