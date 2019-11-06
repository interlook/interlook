package f5ltm

import (
	"fmt"
	"github.com/interlook/interlook/comm"
	"github.com/scottdware/go-bigip"
	"net/http"
	"time"
)

func initTests() {
	go http.ListenAndServe(":9080", nil)
	time.Sleep(1 * time.Second)
	f5 = BigIP{
		Endpoint:                "http://localhost:9080",
		User:                    "dummy",
		Password:                "dummy",
		AuthProvider:            "tmos",
		HttpPort:                80,
		HttpsPort:               443,
		MonitorName:             "tcp",
		LoadBalancingMode:       "",
		Partition:               "interlook",
		UpdateMode:              "",
		GlobalHTTPPolicy:        "interlook_http_policy",
		GlobalSSLPolicy:         "interlook_https_policy",
		ObjectDescriptionSuffix: defaultDescriptionSuffix,
		//CliProxy:                "http://toto:titi@myproxy.com",
	}
	f5.cli, _ = bigip.NewTokenSession(f5.Endpoint, "dummy", "dummy", "tmos", f5.getCliConfigOptions())

	testPool = bigip.Pool{
		Name:              "test",
		Description:       "Pool for test " + defaultDescriptionSuffix,
		Partition:         "interlook",
		LoadBalancingMode: f5.LoadBalancingMode,
		Monitor:           f5.MonitorName,
	}

	msgOK = comm.Message{Service: comm.Service{
		Name:       "test",
		DNSAliases: []string{"test.caas.csnet.me"},
		Port:       30001,
		Hosts:      []string{"10.32.2.2", "10.32.2.3"},
		TLS:        false,
	}}

	msgUpdate = comm.Message{Service: comm.Service{
		Name:       "test",
		DNSAliases: []string{"testko.caas.csnet.me"},
		Port:       30001,
		Hosts:      []string{"10.32.2.2", "10.32.2.99"},
		TLS:        false,
	}}

	msgNew = comm.Message{Service: comm.Service{
		Name:       "test2",
		DNSAliases: []string{"testko.caas.csnet.me"},
		Port:       30001,
		Hosts:      []string{"10.32.2.2", "10.32.2.99"},
		TLS:        false,
	}}

	msgTLSOK = comm.Message{Service: comm.Service{
		Name:       "test",
		DNSAliases: []string{"test.caas.csnet.me"},
		Port:       30001,
		Hosts:      []string{"10.32.2.2", "10.32.2.3"},
		TLS:        true,
	}}

	msgTLSUpdate = comm.Message{Service: comm.Service{
		Name:       "test",
		DNSAliases: []string{"testko.caas.csnet.me"},
		Port:       30001,
		Hosts:      []string{"10.32.2.2", "10.32.2.99"},
		TLS:        true,
	}}

	msgExistingNode = comm.Message{Service: comm.Service{
		Name:       "test",
		DNSAliases: []string{"new.caas.csnet.me"},
		Port:       30001,
		Hosts:      []string{"10.32.2.40"},
		TLS:        true,
	}}

	msgNewNodes = comm.Message{Service: comm.Service{
		Name:       "test",
		DNSAliases: []string{"new.caas.csnet.me"},
		Port:       30001,
		Hosts:      []string{"10.32.2.50", "10.32.2.51"},
		TLS:        true,
	}}

	prCondition = bigip.PolicyRuleCondition{
		Name:            "0",
		CaseInsensitive: true,
		Host:            true,
		HttpHost:        true,
		Request:         true,
		Values:          []string{"test.caas.csnet.me"},
	}

	prAction = bigip.PolicyRuleAction{
		Name:    "0",
		Forward: true,
		Pool:    "/interlook/test",
		Request: true,
	}

	prSSLCondition = bigip.PolicyRuleCondition{
		Name:            "0",
		CaseInsensitive: true,
		Present:         true,
		ServerName:      true,
		SslClientHello:  true,
		SslExtension:    true,
		Values:          []string{"test.caas.csnet.me"},
	}

	prSSLAction = bigip.PolicyRuleAction{
		Name:           "0",
		Forward:        true,
		Pool:           "/interlook/test",
		SslClientHello: true,
	}

	pr = bigip.PolicyRule{
		Name:        "test",
		Description: "ingress rule for test (auto generated - do not edit)",
		Conditions:  []bigip.PolicyRuleCondition{prCondition},
		Actions:     []bigip.PolicyRuleAction{prAction},
	}

	prSSL = bigip.PolicyRule{
		Name:        "test",
		Description: "ingress rule for test (auto generated - do not edit)",
		Conditions:  []bigip.PolicyRuleCondition{prSSLCondition},
		Actions:     []bigip.PolicyRuleAction{prSSLAction},
	}

	pmNew1 = bigip.PoolMember{
		Name:        "10.32.2.50:30001",
		Address:     "10.32.2.50",
		Partition:   f5.Partition,
		Monitor:     f5.MonitorName,
		Description: fmt.Sprintf("Pool Member for test %v", f5.ObjectDescriptionSuffix),
	}

	pmNew2 = bigip.PoolMember{
		Name:        "10.32.2.51:30001",
		Address:     "10.32.2.51",
		Partition:   f5.Partition,
		Monitor:     f5.MonitorName,
		Description: fmt.Sprintf("Pool Member for test %v", f5.ObjectDescriptionSuffix),
	}

	pmExisting = bigip.PoolMember{
		Name:      "missing:30001",
		Address:   "10.32.2.40",
		Partition: "Common",
	}

	http.HandleFunc("/mgmt/tm/ltm/pool/~interlook~test", upsertPool)
	http.HandleFunc("/mgmt/tm/ltm/pool/~interlook~test2", poolNotFound)
	http.HandleFunc("/mgmt/tm/ltm/pool/~interlook~test/members", getPoolMembers)
	http.HandleFunc("/mgmt/tm/ltm/node", getNodes)
	http.HandleFunc("/mgmt/tm/ltm/policy/~interlook~interlook_http_policy", getPolicy)
	http.HandleFunc("/mgmt/tm/ltm/policy/~interlook~interlook_https_policy", getHTTPSPolicy)
	http.HandleFunc("/mgmt/tm/ltm/policy/~interlook~interlook_http_policy/rules", getHTTPPolicyRules)
	http.HandleFunc("/mgmt/tm/ltm/policy/~interlook~interlook_https_policy/rules", getHTTPSPolicyRules)
	http.HandleFunc("/mgmt/tm/ltm/policy/~interlook~interlook_http_policy/rules/test/conditions", getHTTPPolicyRuleConditions)
	http.HandleFunc("/mgmt/tm/ltm/policy/~interlook~interlook_http_policy/rules/test/actions", getHTTPPolicyRuleActions)
	http.HandleFunc("/mgmt/tm/ltm/policy/~interlook~interlook_https_policy/rules/test/conditions", getHTTPSPolicyRuleConditions)
	http.HandleFunc("/mgmt/tm/ltm/policy/~interlook~interlook_https_policy/rules/test/actions", getHTTPSPolicyRuleActions)
}

func poolNotFound(w http.ResponseWriter, r *http.Request) {
	rsp := `{
    "code": 404,
    "message": "01020036:3: The requested Pool (/interlook/test2) was not found.",
    "errorStack": [],
    "apiError": 3
}`
	w.Write([]byte(rsp))
}
func upsertPool(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}

func getNodes(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var rsp = `{
    "kind": "tm:ltm:node:nodecollectionstate",
    "selfLink": "https://localhost/mgmt/tm/ltm/node?ver=14.1.2",
    "items": [
        {
            "kind": "tm:ltm:node:nodestate",
            "name": "missing",
            "partition": "Common",
            "fullPath": "/Common/missing",
            "generation": 839,
            "selfLink": "https://localhost/mgmt/tm/ltm/node/~Common~missing?ver=14.1.2",
            "address": "10.32.2.40",
            "connectionLimit": 0,
            "dynamicRatio": 1,
            "ephemeral": "false",
            "fqdn": {
                "addressFamily": "ipv4",
                "autopopulate": "disabled",
                "downInterval": 5,
                "interval": "3600"
            },
            "logging": "disabled",
            "monitor": "default",
            "rateLimit": "disabled",
            "ratio": 1,
            "session": "user-enabled",
            "state": "unchecked"
        },
        {
            "kind": "tm:ltm:node:nodestate",
            "name": "10.32.2.2",
            "partition": "interlook",
            "fullPath": "/interlook/10.32.2.2",
            "generation": 569,
            "selfLink": "https://localhost/mgmt/tm/ltm/node/~interlook~10.32.2.41?ver=14.1.2",
            "address": "10.32.2.2",
            "connectionLimit": 0,
            "dynamicRatio": 1,
            "ephemeral": "false",
            "fqdn": {
                "addressFamily": "ipv4",
                "autopopulate": "disabled",
                "downInterval": 5,
                "interval": "3600"
            },
            "logging": "disabled",
            "monitor": "default",
            "rateLimit": "disabled",
            "ratio": 1,
            "session": "user-enabled",
            "state": "unchecked"
        },
        {
            "kind": "tm:ltm:node:nodestate",
            "name": "10.32.2.3",
            "partition": "interlook",
            "fullPath": "/interlook/10.32.2.3",
            "generation": 828,
            "selfLink": "https://localhost/mgmt/tm/ltm/node/~interlook~10.32.2.42?ver=14.1.2",
            "address": "10.32.2.3",
            "connectionLimit": 0,
            "dynamicRatio": 1,
            "ephemeral": "false",
            "fqdn": {
                "addressFamily": "ipv4",
                "autopopulate": "disabled",
                "downInterval": 5,
                "interval": "3600"
            },
            "logging": "disabled",
            "monitor": "default",
            "rateLimit": "disabled",
            "ratio": 1,
            "session": "user-enabled",
            "state": "unchecked"
        }
    ]
}`
	w.Write([]byte(rsp))
}

func getPoolMembers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	rsp := `{
    "kind": "tm:ltm:testPool:members:memberscollectionstate",
    "selfLink": "https://localhost/mgmt/tm/ltm/testPool/~interlook~test/members?ver=14.1.2",
    "items": [
        {
            "kind": "tm:ltm:testPool:members:membersstate",
            "name": "10.32.2.61:30001",
            "partition": "interlook",
            "fullPath": "/interlook/10.32.2.2:30001",
            "generation": 919,
            "selfLink": "https://localhost/mgmt/tm/ltm/testPool/~interlook~test/members/~interlook~10.32.2.2:30000?ver=14.1.2",
            "address": "10.32.2.2",
            "connectionLimit": 0,
            "dynamicRatio": 1,
            "ephemeral": "false",
            "fqdn": {
                "autopopulate": "disabled"
            },
            "inheritProfile": "enabled",
            "logging": "disabled",
            "monitor": "default",
            "priorityGroup": 0,
            "rateLimit": "disabled",
            "ratio": 1,
            "session": "monitor-enabled",
            "state": "up"
        },
        {
            "kind": "tm:ltm:testPool:members:membersstate",
            "name": "10.32.2.62:30001",
            "partition": "interlook",
            "fullPath": "/interlook/10.32.2.3:30001",
            "generation": 919,
            "selfLink": "https://localhost/mgmt/tm/ltm/testPool/~interlook~test/members/~interlook~10.32.2.3:30000?ver=14.1.2",
            "address": "10.32.2.3",
            "connectionLimit": 0,
            "dynamicRatio": 1,
            "ephemeral": "false",
            "fqdn": {
                "autopopulate": "disabled"
            },
            "inheritProfile": "enabled",
            "logging": "disabled",
            "monitor": "default",
            "priorityGroup": 0,
            "rateLimit": "disabled",
            "ratio": 1,
            "session": "monitor-enabled",
            "state": "up"
        }
    ]
}`
	w.Write([]byte(rsp))
}

func getPolicy(w http.ResponseWriter, r *http.Request) {
	// /policy/~interlook~interlook_http_policy/
	rsp := `{
    "kind": "tm:ltm:policy:policystate",
    "name": "interlook_http_policy",
    "partition": "interlook",
    "fullPath": "/interlook/interlook_http_policy",
    "generation": 930,
    "selfLink": "https://localhost/mgmt/tm/ltm/policy/~interlook~interlook_http_policy?ver=14.1.2",
    "controls": [
        "forwarding"
    ],
    "description": "Global policy for routing CaaS HTTP traffic",
    "lastModified": "2019-10-13T20:11:11Z",
    "requires": [
        "http"
    ],
    "status": "published",
    "strategy": "/Common/first-match",
    "strategyReference": {
        "link": "https://localhost/mgmt/tm/ltm/policy-strategy/~Common~first-match?ver=14.1.2"
    },
    "references": {},
    "rulesReference": {
        "link": "https://localhost/mgmt/tm/ltm/policy/~interlook~interlook_http_policy/rules?ver=14.1.2",
        "isSubcollection": true
    }
}`
	w.Write([]byte(rsp))

}

func getHTTPSPolicy(w http.ResponseWriter, r *http.Request) {
	// /policy/~interlook~interlook_https_policy/
	rsp := `{
    "kind": "tm:ltm:policy:policystate",
    "name": "interlook_https_policy",
    "partition": "interlook",
    "fullPath": "/interlook/interlook_https_policy",
    "generation": 697,
    "selfLink": "https://localhost/mgmt/tm/ltm/policy/~interlook~interlook_https_policy?ver=14.1.2",
    "controls": [
        "forwarding"
    ],
    "description": "Global policy for routing CaaS HTTPS traffic",
    "lastModified": "2019-10-04T20:06:40Z",
    "requires": [
        "client-ssl"
    ],
    "status": "published",
    "strategy": "/Common/first-match",
    "strategyReference": {
        "link": "https://localhost/mgmt/tm/ltm/policy-strategy/~Common~first-match?ver=14.1.2"
    },
    "references": {},
    "rulesReference": {
        "link": "https://localhost/mgmt/tm/ltm/policy/~interlook~interlook_https_policy/rules?ver=14.1.2",
        "isSubcollection": true
    }
}`
	w.Write([]byte(rsp))

}

func getHTTPPolicyRules(w http.ResponseWriter, r *http.Request) {
	// /policy/~interlook~interlook_http_policy/rules
	rsp := `{
    "kind": "tm:ltm:policy:rules:rulescollectionstate",
    "selfLink": "https://localhost/mgmt/tm/ltm/policy/~interlook~interlook_http_policy/rules?ver=14.1.2",
    "items": [
        {
            "kind": "tm:ltm:policy:rules:rulesstate",
            "name": "test",
            "fullPath": "test",
            "generation": 929,
            "selfLink": "https://localhost/mgmt/tm/ltm/policy/~interlook~interlook_http_policy/rules/test?ver=14.1.2",
            "description": "ingress rule for swarmnginx_nginx (auto generated - do not edit)",
            "ordinal": 0,
            "actionsReference": {
                "link": "https://localhost/mgmt/tm/ltm/policy/~interlook~interlook_http_policy/rules/test/actions?ver=14.1.2",
                "isSubcollection": true
            },
            "conditionsReference": {
                "link": "https://localhost/mgmt/tm/ltm/policy/~interlook~interlook_http_policy/rules/test/conditions?ver=14.1.2",
                "isSubcollection": true
            }
        }
    ]
}`

	w.Write([]byte(rsp))

}

func getHTTPSPolicyRules(w http.ResponseWriter, r *http.Request) {
	// /policy/~interlook~interlook_https_policy/rules
	rsp := `{
    "kind": "tm:ltm:policy:rules:rulescollectionstate",
    "selfLink": "https://localhost/mgmt/tm/ltm/policy/~interlook~interlook_https_policy/rules?ver=14.1.2",
    "items": [
        {
            "kind": "tm:ltm:policy:rules:rulesstate",
            "name": "test",
            "fullPath": "test",
            "generation": 696,
            "selfLink": "https://localhost/mgmt/tm/ltm/policy/~interlook~interlook_https_policy/rules/test?ver=14.1.2",
            "ordinal": 0,
            "actionsReference": {
                "link": "https://localhost/mgmt/tm/ltm/policy/~interlook~interlook_https_policy/rules/test/actions?ver=14.1.2",
                "isSubcollection": true
            },
            "conditionsReference": {
                "link": "https://localhost/mgmt/tm/ltm/policy/~interlook~interlook_https_policy/rules/test/conditions?ver=14.1.2",
                "isSubcollection": true
            }
        }
    ]
}`
	w.Write([]byte(rsp))
}

func getHTTPPolicyRuleActions(w http.ResponseWriter, r *http.Request) {
	// /policy/~interlook~interlook_http_policy/test/actions
	rsp := `{
    "kind": "tm:ltm:policy:rules:actions:actionscollectionstate",
    "selfLink": "https://localhost/mgmt/tm/ltm/policy/~interlook~interlook_http_policy/rules/test/actions?ver=14.1.2",
    "items": [
        {
            "kind": "tm:ltm:policy:rules:actions:actionsstate",
            "name": "0",
            "fullPath": "0",
            "generation": 691,
            "selfLink": "https://localhost/mgmt/tm/ltm/policy/~interlook~interlook_http_policy/rules/test/actions/0?ver=14.1.2",
            "code": 0,
            "expirySecs": 0,
            "forward": true,
            "length": 0,
            "offset": 0,
            "pool": "/interlook/test",
            "poolReference": {
                "link": "https://localhost/mgmt/tm/ltm/pool/~interlook~test?ver=14.1.2"
            },
            "port": 0,
            "request": true,
            "select": true,
            "status": 0,
            "timeout": 0,
            "vlanId": 0
        }
    ]
}`
	w.Write([]byte(rsp))

}

func getHTTPPolicyRuleConditions(w http.ResponseWriter, r *http.Request) {
	rsp := `{
    "kind": "tm:ltm:policy:rules:conditions:conditionscollectionstate",
    "selfLink": "https://localhost/mgmt/tm/ltm/policy/~interlook~interlook_http_policy/rules/test/conditions?ver=14.1.2",
    "items": [
        {
            "kind": "tm:ltm:policy:rules:conditions:conditionsstate",
            "name": "0",
            "fullPath": "0",
            "generation": 691,
            "selfLink": "https://localhost/mgmt/tm/ltm/policy/~interlook~interlook_http_policy/rules/test/conditions/0?ver=14.1.2",
            "caseInsensitive": true,
            "equals": true,
            "external": true,
            "host": true,
            "httpHost": true,
            "index": 0,
            "present": true,
            "remote": true,
            "request": true,
            "values": [
                "test.caas.csnet.me"
            ]
        }
    ]
}`
	w.Write([]byte(rsp))

}

func getHTTPSPolicyRuleActions(w http.ResponseWriter, r *http.Request) {
	// /policy/~interlook~test/actions
	rsp := `{
    "kind": "tm:ltm:policy:rules:actions:actionscollectionstate",
    "selfLink": "https://localhost/mgmt/tm/ltm/policy/~interlook~interlook_https_policy/rules/test/actions?ver=14.1.2",
    "items": [
        {
            "kind": "tm:ltm:policy:rules:actions:actionsstate",
            "name": "0",
            "fullPath": "0",
            "generation": 695,
            "selfLink": "https://localhost/mgmt/tm/ltm/policy/~interlook~interlook_https_policy/rules/test/actions/0?ver=14.1.2",
            "code": 0,
            "expirySecs": 0,
            "forward": true,
            "length": 0,
            "offset": 0,
            "pool": "/interlook/test",
            "poolReference": {
                "link": "https://localhost/mgmt/tm/ltm/pool/~interlook~test?ver=14.1.2"
            },
            "port": 0,
            "select": true,
            "sslClientHello": true,
            "status": 0,
            "timeout": 0,
            "vlanId": 0
        }
    ]
}`
	w.Write([]byte(rsp))

}

func getHTTPSPolicyRuleConditions(w http.ResponseWriter, r *http.Request) {
	rsp := `{
    "kind": "tm:ltm:policy:rules:conditions:conditionscollectionstate",
    "selfLink": "https://localhost/mgmt/tm/ltm/policy/~interlook~interlook_https_policy/rules/test/conditions?ver=14.1.2",
    "items": [
        {
            "kind": "tm:ltm:policy:rules:conditions:conditionsstate",
            "name": "0",
            "fullPath": "0",
            "generation": 695,
            "selfLink": "https://localhost/mgmt/tm/ltm/policy/~interlook~interlook_https_policy/rules/test/conditions/0?ver=14.1.2",
            "caseInsensitive": true,
            "equals": true,
            "external": true,
            "index": 0,
            "present": true,
            "remote": true,
            "serverName": true,
            "sslClientHello": true,
            "sslExtension": true,
            "values": [
                "test.caas.csnet.me"
            ]
        }
    ]
}`
	w.Write([]byte(rsp))

}
