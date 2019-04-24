package core

import (
	"context"
	"encoding/json"
	"github.com/bhuisgen/interlook/log"
	"net/http"
	"strconv"
)

func (s *server) startAPI() {
	mux := http.NewServeMux()
	s.apiServer = &http.Server{Handler: mux, Addr: ":" + strconv.Itoa(s.config.Core.ListenPort)}

	mux.HandleFunc("/health", s.health)
	mux.HandleFunc("/services", s.getServices)
	mux.HandleFunc("/workflow", s.getWorkflow)
	mux.HandleFunc("/extensions", s.getActiveExtensions)
	mux.HandleFunc("/version", s.getVersion)
	log.Infof("API server started on port %v", s.config.Core.ListenPort)
	log.Info(s.apiServer.ListenAndServe())
}

func (s *server) stopAPI() {
	defer s.coreWG.Done()

	if err := s.apiServer.Shutdown(context.Background()); err != nil {
		log.Errorf("Error shutting down api server: %v", err)
	}
}

func (s *server) health(w http.ResponseWriter, r *http.Request) {
	// TODO: improve: add check extensions up,...
	w.Header().Set("Content-Type", "application/json")

	w.WriteHeader(200)
}

func (s *server) getServices(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	err := json.NewEncoder(w).Encode(s.workflowEntries.Entries)
	if err != nil {
		log.Errorf("Error encoding JSON response %v", err)
	}
}

func (s *server) getWorkflow(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	// TODO: custom parser for better presentation
	err := json.NewEncoder(w).Encode(workflow)
	if err != nil {
		log.Errorf("Error encoding JSON response %v", err)
	}
}

func (s *server) getActiveExtensions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	err := json.NewEncoder(w).Encode(s.extensionChannels)
	if err != nil {
		log.Errorf("Error encoding JSON response %v", err)
	}
}

func (s *server) getVersion(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	err := json.NewEncoder(w).Encode(Version)
	if err != nil {
		log.Errorf("Error encoding JSON response %v", err)
	}
}
