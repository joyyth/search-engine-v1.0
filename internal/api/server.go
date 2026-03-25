package api

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/joyyth/seacrh-engine-v1.0/internal/search"
)

type Server struct{
	search *search.Search
	mux *http.ServeMux
}

func NewServer(search *search.Search) *Server{
	 srv := &Server{
		search: search,
		mux: http.NewServeMux(),
	 }

	 srv.mux.HandleFunc("GET /search", srv.SearchHandler)
	 srv.mux.HandleFunc("GET /health", srv.HealthHandler)
	 return srv
}

func (s *Server) Start(addr string)error{
	log.Printf("Starting server on %s", addr)
	return  http.ListenAndServe(addr, s.mux)
}

func (s *Server) SearchHandler(w http.ResponseWriter, r *http.Request){
	query:= r.URL.Query().Get("q")
	if query == ""{
		http.Error(w, "Missing query parameter", http.StatusBadRequest)
		return
	}
	results := s.search.Search(query)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(results); err != nil {
		http.Error(w, "Failed to encode results", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}
func (s *Server) HealthHandler(w http.ResponseWriter, r *http.Request){
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}