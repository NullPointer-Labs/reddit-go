package api

import (
	"net/http"
	"reddit-lab/internal/comments/compare"
	"strconv"
)

func (s *Server) getComment(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	s.serveRead(w, r, func() (any, error) { return s.prod.Get(r.Context(), id) })
}

func (s *Server) listComments(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	s.serveRead(w, r, func() (any, error) { return s.prod.ListByPost(r.Context(), id) })
}

func (s *Server) serveRead(w http.ResponseWriter, r *http.Request, load func() (any, error)) {
	status, mine := render(load())
	if s.primary.Load() {
		writeRaw(w, status, mine)
		return
	}
	legStatus, legBody, ferr := s.forward(r, nil)
	if ferr != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "legacy unavailable"})
		return
	}
	s.stats.ReadsCompared.Add(1)
	diffs := readDiff(legStatus, legBody, status, mine)
	if len(diffs) > 0 {
		status, mine = render(load())
		if len(readDiff(legStatus, legBody, status, mine)) == 0 {
			s.stats.ReadRaces.Add(1)
			diffs = nil
		}
	}
	if len(diffs) > 0 {
		s.stats.ReadMismatches.Add(1)
		compare.LogMismatch("read", r.Method+" "+r.URL.Path, diffs)
	}
	writeRaw(w, legStatus, legBody)
}

func readDiff(legStatus int, legBody []byte, status int, mine []byte) []string {
	diffs := compare.Diff(legBody, mine, nil)
	if legStatus != status {
		diffs = append(diffs, "status: legacy="+strconv.Itoa(legStatus)+" go="+strconv.Itoa(status))
	}
	return diffs
}
