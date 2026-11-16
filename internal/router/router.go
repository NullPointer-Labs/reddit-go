package router

import (
	"log"
	"math/rand/v2"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"sync/atomic"
)

type Router struct {
	legacy  *httputil.ReverseProxy
	goSvc   *httputil.ReverseProxy
	percent atomic.Int64
}

func New(legacyURL, goURL string, percent int) (*Router, error) {
	legacy, err := proxy(legacyURL)
	if err != nil {
		return nil, err
	}
	goSvc, err := proxy(goURL)
	if err != nil {
		return nil, err
	}
	rt := &Router{legacy: legacy, goSvc: goSvc}
	rt.percent.Store(int64(percent))
	return rt, nil
}

func (rt *Router) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/admin/percent", rt.adminPercent)
	mux.HandleFunc("/", rt.route)
	return mux
}

func (rt *Router) route(w http.ResponseWriter, r *http.Request) {
	if rand.Int64N(100) < rt.percent.Load() {
		w.Header().Set("X-Routed-To", "go")
		rt.goSvc.ServeHTTP(w, r)
		return
	}
	w.Header().Set("X-Routed-To", "legacy")
	rt.legacy.ServeHTTP(w, r)
}

func (rt *Router) adminPercent(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		p, err := strconv.Atoi(r.URL.Query().Get("v"))
		if err != nil || p < 0 || p > 100 {
			http.Error(w, `{"error":"v must be 0..100"}`, http.StatusBadRequest)
			return
		}
		rt.percent.Store(int64(p))
		log.Printf("go traffic set to %d%%", p)
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"go_percent":` + strconv.FormatInt(rt.percent.Load(), 10) + `}`))
}

func proxy(raw string) (*httputil.ReverseProxy, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	p := httputil.NewSingleHostReverseProxy(u)
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.MaxIdleConnsPerHost = 512
	p.Transport = t
	return p, nil
}
