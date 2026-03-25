package api

import (
	"encoding/json"
	"net/http"
)

func SearchHandler(w http.ResponseWriter, r *http.Request){
   query := r.URL.Query().Get("q")
   results := 
   json.NewEncoder(w).Encode(results)
}