package core

import (
	"os"
	"reflect"
	"testing"
)

func TestMain(m *testing.M) {
	initConfFile()
	rc := m.Run()
	cleanup()
	os.Exit(rc)
}

func initConfFile() {

}

func cleanup() {

}

func Test_initServer(t *testing.T) {
	tests := []struct {
		name    string
		want    server
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := initServer()
			if (err != nil) != tt.wantErr {
				t.Errorf("initServer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("initServer() got = %v, want %v", got, tt.want)
			}
		})
	}
}
