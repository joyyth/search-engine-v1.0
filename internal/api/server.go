package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/joyyth/search-engine-v1.0/internal/search"
)

type Server struct {
	search *search.Search
	mux    *http.ServeMux
}

func NewServer(search *search.Search) *Server {
	srv := &Server{
		search: search,
		mux:    http.NewServeMux(),
	}

	srv.mux.HandleFunc("GET /search", srv.SearchHandler)
	srv.mux.HandleFunc("GET /health", srv.HealthHandler)
	return srv
}

func (s *Server) NewServer(addr string) *http.Server {
	return &http.Server{
		Addr:         addr,
		Handler:      s.mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}

func (s *Server) SearchHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		http.Error(w, "Missing query parameter", http.StatusBadRequest)
		return
	}
	results := s.search.Search(query)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(results); err != nil {
		http.Error(w, "Failed to encode results", http.StatusInternalServerError)
		return
	}
}
func (s *Server) HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}
