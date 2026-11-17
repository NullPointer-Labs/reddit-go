package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"reddit-lab/internal/comments/compare"
	"reddit-lab/internal/comments/store"
	"strconv"
	"sync/atomic"
	"time"
)

type Config struct {
	Prod    *store.Store
	Sister  *store.Store
	Legacy  string
	Primary bool
}

type Server struct {
	prod    *store.Store
	sister  *store.Store
	legacy  string
	client  *http.Client
	primary atomic.Bool
	stats   compare.Stats
}

func New(cfg Config) *Server {
	s := &Server{
		prod:   cfg.Prod,
		sister: cfg.Sister,
		legacy: cfg.Legacy,
		client: &http.Client{Timeout: 10 * time.Second},
	}
	s.primary.Store(cfg.Primary)
	return s
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /comments/{id}", s.getComment)
	mux.HandleFunc("GET /posts/{id}/comments", s.listComments)
	mux.HandleFunc("POST /comments", s.createComment)
	mux.HandleFunc("PATCH /comments/{id}", s.updateComment)
	mux.HandleFunc("POST /comments/{id}/vote", s.vote)
	mux.HandleFunc("GET /admin/stats", s.adminStats)
	mux.HandleFunc("POST /admin/mode", s.adminMode)
	mux.HandleFunc("/", s.passthrough)
	return mux
}

func (s *Server) passthrough(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	status, out, err := s.forward(r, body)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "legacy unavailable"})
		return
	}
	writeRaw(w, status, out)
}

func (s *Server) forward(r *http.Request, body []byte) (int, []byte, error) {
	req, err := http.NewRequestWithContext(r.Context(), r.Method, s.legacy+r.URL.RequestURI(), bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	out, err := io.ReadAll(resp.Body)
	return resp.StatusCode, out, err
}

func render(v any, err error) (int, []byte) {
	var se *store.Error
	if errors.As(err, &se) {
		b, _ := json.Marshal(map[string]string{"error": se.Msg})
		return se.Status, b
	}
	if err != nil {
		log.Printf("internal error: %v", err)
		b, _ := json.Marshal(map[string]string{"error": "internal error"})
		return http.StatusInternalServerError, b
	}
	b, _ := json.Marshal(v)
	return http.StatusOK, b
}

func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return 0, false
	}
	return id, true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	b, _ := json.Marshal(v)
	writeRaw(w, status, b)
}

func writeRaw(w http.ResponseWriter, status int, b []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(b)
}
