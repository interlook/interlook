package core

import (
	"encoding/json"
	"net/http"

	"github.com/bhuisgen/interlook/log"
)

func (s *server) startHTTP() {
	//http.HandleFunc("/health", health)
	http.HandleFunc("/services", s.services)
	logger.DefaultLogger().Fatal(http.ListenAndServe(":8080", nil))
}

func (s *server) services(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(s.flowEntries.M)

}
