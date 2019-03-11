package core

import (
	"encoding/json"
	"net/http"

	"github.com/bhuisgen/interlook/log"
)

func (s *server) startHTTP() {
	//http.HandleFunc("/health", health)
	http.HandleFunc("/services", s.getServices)
	http.HandleFunc("/workflow", s.getWorkflow)
	http.HandleFunc("/extensions", s.getActiveExtensions)
	logger.DefaultLogger().Fatal(http.ListenAndServe(":8080", nil))
}

func (s *server) getServices(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.flowEntries.M)

}

func (s *server) getWorkflow(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.workflow)

}

func (s *server) getActiveExtensions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	json.NewEncoder(w).Encode(s.extensionChannels)
}
