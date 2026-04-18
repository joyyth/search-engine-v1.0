package indexer

import (
	"testing"

	"github.com/joyyth/search-engine-v1.0/internal/model"
)

type mockStore struct {
	docs  []model.Document
	index map[string][]model.Posting
}

func (m *mockStore) SaveDocuments(docs []model.Document) error      { m.docs = docs; return nil }
func (m *mockStore) LoadDocuments() ([]model.Document, error)       { return m.docs, nil }
func (m *mockStore) SaveIndex(idx map[string][]model.Posting) error { m.index = idx; return nil }
func (m *mockStore) LoadIndex() (map[string][]model.Posting, error) { return m.index, nil }

func TestLookup(t *testing.T) {
	idx := NewIndexer(&mockStore{})
	idx.AddDocument(model.Document{URL: "https://example.com", Content: "This is an example document. Now is the time for all good men to come to the aid of their country."})
	idx.AddDocument(model.Document{URL: "https://example.org", Content: "This is another example document"})

	tests := []struct {
		name    string
		token   string
		wantIDs []int
		wantOk  bool
	}{{
		name:    "term in one document",
		token:   "good",
		wantIDs: []int{0},
		wantOk:  true,
	}, {
		name:    "term in multiple documents",
		token:   "document",
		wantIDs: []int{0, 1},
		wantOk:  true,
	}, {
		name:    "term not in any document",
		token:   "nonexistent",
		wantIDs: []int{},
		wantOk:  false,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ids, found := idx.Lookup(test.token)

			if found != test.wantOk {
				t.Errorf("Lookup(%q) found = %v, want %v", test.token, found, test.wantOk)
			}
			if len(ids) != len(test.wantIDs) {
				t.Fatalf("Lookup(%q) returned %v, want %v", test.token, ids, test.wantIDs)
			}
			for i, p := range ids {
				if p.DocID != test.wantIDs[i] {
					t.Errorf("Lookup(%q)[%d].DocID = %d, want %d", test.token, i, p.DocID, test.wantIDs[i])
				}
			}
		})
	}
}

func TestGetDocument(t *testing.T) {
	idx := NewIndexer(&mockStore{})
	idx.AddDocument(model.Document{ID: 0, URL: "https://example.com", Title: "Example", Content: "This is an example document"})
	tests := []struct {
		name string
		id   int
		want bool
	}{
		{"valid ID", 0, true},
		{"negative ID", -1, false},
		{"out of range ID", 99, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, ok := idx.GetDocument(test.id)
			if ok != test.want {
				t.Errorf("GetDocument(%d) = %v, want %v", test.id, ok, test.want)
			}
		})
	}
}
