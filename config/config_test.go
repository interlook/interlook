package config

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	initTests()
	rc := m.Run()
	cleanUP()
	os.Exit(rc)
}

func initTests() {
	validYAML := `---
core:
    logLevel: DEBUG
    listenPort: 8080
    logFile : stdout
    workflowSteps: provider.swarm,lb.f5ltm

provider:
    swarm:
        endpoint: tcp://mySwarm:2376
        tlsCa: ca.pem
        tlsCert: cert.pem
        tlsKey: key.pem
        pollInterval: 5s

ipam:
    ipalloc:
        network_cidr: 10.32.30.0/24
        db_file: ./share/ipalloc.db

dns:
    consul:
        url: http://127.0.0.1:8500

lb:
    kemplm:
        endpoint: https://192.168.99.2
        username: api

    f5ltm:
        httpEndpoint: https://10.32.20.100
        username: api
        authProvider: tmos
`
	invalidYAML := `---
{-core:
    -logLevel: DEBUG}
`
	_ = ioutil.WriteFile("./configOK.yml", []byte(validYAML), 0644)
	_ = ioutil.WriteFile("./configKO.yml", []byte(invalidYAML), 0644)

}

func cleanUP() {
	_ = os.Remove("./configOK.yml")
	_ = os.Remove("./configKO.yml")
}
func TestReadConfig(t *testing.T) {

	type args struct {
		filename string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{"ok", args{filename: "./configOK.yml"}, false},
		{"ko", args{filename: "./configKO.yml"}, true},
		{"missing", args{filename: "./noVonfig.yml"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ReadConfig(tt.args.filename)
			if (err != nil) != tt.wantErr {
				t.Errorf("ReadConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if (got.Core.LogLevel != "DEBUG" || got.LB.F5LTM.AuthProvider != "tmos") && !tt.wantErr {
				t.Errorf("ReadConfig() got = %v, want DEBUG", got.Core.LogLevel)
			}
		})
	}
}
