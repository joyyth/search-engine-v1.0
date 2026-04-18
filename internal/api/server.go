package api

import (
	"embed"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/joyyth/search-engine-v1.0/internal/search"
)

//go:embed web/index.html
var webFiles embed.FS

type Server struct {
	search *search.Search
	mux    *http.ServeMux
}

func NewServer(search *search.Search) *Server {
	srv := &Server{
		search: search,
		mux:    http.NewServeMux(),
	}

	srv.mux.HandleFunc("GET /{$}", srv.IndexHandler)
	srv.mux.HandleFunc("GET /search", srv.SearchHandler)
	srv.mux.HandleFunc("GET /health", srv.HealthHandler)
	srv.mux.HandleFunc("GET /stats", srv.StatsHandler)
	return srv
}

func (s *Server) BuildHTTPServer(addr string) *http.Server {
	return &http.Server{
		Addr:         addr,
		Handler:      s.withLogging(s.withCORS(s.mux)),
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
	start := time.Now()
	results, total := s.search.SearchWithSnippets(query)
	w.Header().Set("Content-Type", "application/json")
	payload := struct {
		Query   string          `json:"query"`
		TookMs  int64           `json:"took_ms"`
		Total   int             `json:"total"`
		Results []search.Result `json:"results"`
	}{
		Query:   query,
		TookMs:  time.Since(start).Milliseconds(),
		Total:   total,
		Results: results,
	}
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		http.Error(w, "Failed to encode results", http.StatusInternalServerError)
		return
	}
}

func (s *Server) IndexHandler(w http.ResponseWriter, r *http.Request) {
	content, err := webFiles.ReadFile("web/index.html")
	if err != nil {
		http.Error(w, "Failed to load UI", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write(content)
}

func (s *Server) HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

func (s *Server) StatsHandler(w http.ResponseWriter, r *http.Request) {
	stats := s.search.Stats()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(stats); err != nil {
		http.Error(w, "Failed to encode stats", http.StatusInternalServerError)
		return
	}
}

func (s *Server) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		duration := time.Since(start)
		slog.Info("request", "method", r.Method, "url", r.URL.Path, "remote_addr", r.RemoteAddr, "duration", duration)

	})
}
