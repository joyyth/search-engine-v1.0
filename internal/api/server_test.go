package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/joyyth/search-engine-v1.0/internal/indexer"
	"github.com/joyyth/search-engine-v1.0/internal/model"
	"github.com/joyyth/search-engine-v1.0/internal/search"
)

type testStore struct{}

func (testStore) SaveDocuments([]model.Document) error {
	return nil
}

func (testStore) LoadDocuments() ([]model.Document, error) {
	return nil, nil
}

func (testStore) SaveIndex(map[string][]model.Posting) error {
	return nil
}

func (testStore) LoadIndex() (map[string][]model.Posting, error) {
	return nil, nil
}

func newTestHTTPServer(t *testing.T) *httptest.Server {
	t.Helper()

	idx := indexer.NewIndexer(testStore{})
	idx.AddDocument(model.Document{
		Title:   "Go concurrency patterns",
		URL:     "https://example.com/go",
		Content: "Go concurrency patterns use goroutines and channels for coordination.",
	})

	srv := NewServer(search.NewSearch(idx), nil)
	httpServer := srv.BuildHTTPServer(":0")
	return httptest.NewServer(httpServer.Handler)
}

func TestIndexHandlerServesSearchUI(t *testing.T) {
	server := newTestHTTPServer(t)
	defer server.Close()

	response, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatalf("get /: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	if got := response.Header.Get("Content-Type"); !strings.Contains(got, "text/html") {
		t.Fatalf("content-type = %q, want text/html", got)
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	page := string(body)
	if !strings.Contains(page, `placeholder="Search indexed pages"`) {
		t.Fatalf("page did not include search input placeholder")
	}
	if !strings.Contains(page, `fetch("/search?q=" + encodeURIComponent(trimmedQuery)`) {
		t.Fatalf("page did not include search API fetch")
	}
}

func TestSearchHandlerReturnsSearchResults(t *testing.T) {
	server := newTestHTTPServer(t)
	defer server.Close()

	response, err := http.Get(server.URL + "/search?q=goroutines")
	if err != nil {
		t.Fatalf("get /search: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	if got := response.Header.Get("Content-Type"); !strings.Contains(got, "application/json") {
		t.Fatalf("content-type = %q, want application/json", got)
	}

	var payload struct {
		Query   string `json:"query"`
		TookMs  int64  `json:"took_ms"`
		Total   int    `json:"total"`
		Results []struct {
			Document model.Document `json:"document"`
			Snippet  string         `json:"snippet"`
		} `json:"results"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if payload.Query != "goroutines" {
		t.Fatalf("query = %q, want %q", payload.Query, "goroutines")
	}
	if payload.TookMs < 0 {
		t.Fatalf("took_ms = %d, want non-negative", payload.TookMs)
	}
	if payload.Total != 1 {
		t.Fatalf("total = %d, want 1", payload.Total)
	}
	if len(payload.Results) != 1 {
		t.Fatalf("results = %d, want 1", len(payload.Results))
	}
	if payload.Results[0].Document.URL != "https://example.com/go" {
		t.Fatalf("url = %q, want %q", payload.Results[0].Document.URL, "https://example.com/go")
	}
	if !strings.Contains(strings.ToLower(payload.Results[0].Snippet), "goroutines") {
		t.Fatalf("snippet = %q, want it to mention goroutines", payload.Results[0].Snippet)
	}
}

func TestHealthAndStatsHandlersStayAvailable(t *testing.T) {
	server := newTestHTTPServer(t)
	defer server.Close()

	healthResponse, err := http.Get(server.URL + "/health")
	if err != nil {
		t.Fatalf("get /health: %v", err)
	}
	defer healthResponse.Body.Close()

	if healthResponse.StatusCode != http.StatusOK {
		t.Fatalf("health status = %d, want %d", healthResponse.StatusCode, http.StatusOK)
	}

	statsResponse, err := http.Get(server.URL + "/stats")
	if err != nil {
		t.Fatalf("get /stats: %v", err)
	}
	defer statsResponse.Body.Close()

	if statsResponse.StatusCode != http.StatusOK {
		t.Fatalf("stats status = %d, want %d", statsResponse.StatusCode, http.StatusOK)
	}

	var stats struct {
		TotalDocuments int `json:"total_documents"`
	}
	if err := json.NewDecoder(statsResponse.Body).Decode(&stats); err != nil {
		t.Fatalf("decode stats: %v", err)
	}

	if stats.TotalDocuments != 1 {
		t.Fatalf("total_documents = %d, want 1", stats.TotalDocuments)
	}
}

type fakeSuggester struct {
	words []string
	got   string
}

func (f *fakeSuggester) Suggest(prefix string) []string {
	f.got = prefix
	return f.words
}

func newSuggestTestServer(t *testing.T, suggester Suggester) *httptest.Server {
	t.Helper()

	idx := indexer.NewIndexer(testStore{})
	srv := NewServer(search.NewSearch(idx), suggester)
	return httptest.NewServer(srv.BuildHTTPServer(":0").Handler)
}

type suggestResponse struct {
	Query       string          `json:"query"`
	Suggestions json.RawMessage `json:"suggestions"`
}

func getSuggest(t *testing.T, server *httptest.Server, rawQuery string) (int, string, []byte) {
	t.Helper()

	response, err := http.Get(server.URL + "/suggest" + rawQuery)
	if err != nil {
		t.Fatalf("get /suggest: %v", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return response.StatusCode, response.Header.Get("Content-Type"), body
}

func TestSuggestHandlerRejectsMissingQuery(t *testing.T) {
	server := newSuggestTestServer(t, &fakeSuggester{words: []string{"array"}})
	defer server.Close()

	for _, rawQuery := range []string{"", "?q="} {
		status, _, _ := getSuggest(t, server, rawQuery)
		if status != http.StatusBadRequest {
			t.Fatalf("query %q: status = %d, want %d", rawQuery, status, http.StatusBadRequest)
		}
	}
}

func TestSuggestHandlerReturnsSuggestions(t *testing.T) {
	fake := &fakeSuggester{words: []string{"array", "archive"}}
	server := newSuggestTestServer(t, fake)
	defer server.Close()

	status, contentType, body := getSuggest(t, server, "?q=ar")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	if !strings.HasPrefix(contentType, "application/json") {
		t.Fatalf("content type = %q, want application/json", contentType)
	}
	if fake.got != "ar" {
		t.Fatalf("suggester got %q, want %q", fake.got, "ar")
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(fields) != 2 {
		t.Fatalf("fields = %v, want only query and suggestions", fields)
	}

	var payload struct {
		Query       string   `json:"query"`
		Suggestions []string `json:"suggestions"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload.Query != "ar" {
		t.Fatalf("query = %q, want %q", payload.Query, "ar")
	}
	if strings.Join(payload.Suggestions, ",") != "array,archive" {
		t.Fatalf("suggestions = %v, want [array archive]", payload.Suggestions)
	}
}

func TestSuggestHandlerReturnsEmptyArrayNotNull(t *testing.T) {
	cases := map[string]Suggester{
		"nil slice":     &fakeSuggester{words: nil},
		"empty slice":   &fakeSuggester{words: []string{}},
		"nil suggester": nil,
	}
	for name, suggester := range cases {
		t.Run(name, func(t *testing.T) {
			server := newSuggestTestServer(t, suggester)
			defer server.Close()

			status, _, body := getSuggest(t, server, "?q=a")
			if status != http.StatusOK {
				t.Fatalf("status = %d, want %d", status, http.StatusOK)
			}
			var payload suggestResponse
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if string(payload.Suggestions) != "[]" {
				t.Fatalf("suggestions = %s, want []", payload.Suggestions)
			}
			if payload.Query != "a" {
				t.Fatalf("query = %q, want %q", payload.Query, "a")
			}
		})
	}
}
