package api

import (
	"log"
	"net/http"
)

func (s *Server) adminStats(w http.ResponseWriter, r *http.Request) {
	out := map[string]any{"primary": s.primary.Load()}
	for k, v := range s.stats.Snapshot() {
		out[k] = v
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) adminMode(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Query().Get("v") {
	case "primary":
		s.primary.Store(true)
	case "shadow":
		s.primary.Store(false)
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "v must be primary or shadow"})
		return
	}
	log.Printf("mode changed: primary=%v", s.primary.Load())
	s.adminStats(w, r)
}
