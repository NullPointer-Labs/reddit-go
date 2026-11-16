package api

import "net/http"

func (s *Server) adminStats(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.stats.Snapshot())
}
