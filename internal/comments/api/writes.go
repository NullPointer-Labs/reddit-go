package api

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"reddit-lab/internal/comments/compare"
	"reddit-lab/internal/comments/store"
	"strconv"
	"time"
)

func (s *Server) createComment(w http.ResponseWriter, r *http.Request) {
	reqBody, _ := io.ReadAll(r.Body)
	var in store.CreateInput
	_ = json.Unmarshal(reqBody, &in)

	legStatus, legBody, err := s.forward(r, reqBody)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "legacy unavailable"})
		return
	}
	writeRaw(w, legStatus, legBody)

	var prodC store.Comment
	if legStatus == http.StatusCreated {
		_ = json.Unmarshal(legBody, &prodC)
	}
	go s.shadowWrite("POST /comments", legStatus, func(ctx context.Context) (store.Comment, error) {
		return s.sister.Create(ctx, in, prodC.ID)
	}, prodC.ID)
}

func (s *Server) updateComment(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	reqBody, _ := io.ReadAll(r.Body)
	var in struct {
		Body string `json:"body"`
	}
	_ = json.Unmarshal(reqBody, &in)
	s.mutate(w, r, id, reqBody, func(ctx context.Context, st *store.Store) (store.Comment, error) {
		return st.Update(ctx, id, in.Body)
	})
}

func (s *Server) vote(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	reqBody, _ := io.ReadAll(r.Body)
	var in struct {
		Delta int `json:"delta"`
	}
	_ = json.Unmarshal(reqBody, &in)
	s.mutate(w, r, id, reqBody, func(ctx context.Context, st *store.Store) (store.Comment, error) {
		return st.Vote(ctx, id, in.Delta)
	})
}

func (s *Server) mutate(w http.ResponseWriter, r *http.Request, id int64, reqBody []byte,
	apply func(context.Context, *store.Store) (store.Comment, error)) {

	before, beforeErr := s.prod.Get(r.Context(), id)

	legStatus, legBody, err := s.forward(r, reqBody)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "legacy unavailable"})
		return
	}
	writeRaw(w, legStatus, legBody)

	go s.shadowWrite(r.Method+" "+r.URL.Path, legStatus, func(ctx context.Context) (store.Comment, error) {
		if beforeErr == nil {
			if err := s.sister.Seed(ctx, before); err != nil {
				return store.Comment{}, err
			}
		}
		return apply(ctx, s.sister)
	}, id)
}

func (s *Server) shadowWrite(route string, legStatus int,
	apply func(context.Context) (store.Comment, error), id int64) {

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	mine, err := apply(ctx)
	status, _ := render(mine, err)
	s.stats.WritesCompared.Add(1)

	if legStatus >= 400 || status >= 400 {
		if legStatus != status {
			s.stats.WriteMismatches.Add(1)
			compare.LogMismatch("write", route, []string{"status: legacy=" + strconv.Itoa(legStatus) + " go=" + strconv.Itoa(status)})
		}
		return
	}

	prodNow, err := s.prod.Get(ctx, id)
	if err != nil {
		log.Printf("compare read failed %s: %v", route, err)
		return
	}

	prodJSON, _ := json.Marshal(prodNow)
	sisterJSON, _ := json.Marshal(mine)
	if diffs := compare.Diff(prodJSON, sisterJSON, nil); len(diffs) > 0 {
		s.stats.WriteMismatches.Add(1)
		compare.LogMismatch("write", route, diffs)
	}
}