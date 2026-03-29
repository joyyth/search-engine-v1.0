package indexer

import (
	"fmt"

	"github.com/joyyth/search-engine-v1.0/internal/model"
	"github.com/joyyth/search-engine-v1.0/internal/store"
)

// Indexer is NOT concurrency-safe.
// Callers must serialize access (single goroutine) until locking is added

type Indexer struct {
	index     map[string][]int
	documents []model.Document
	store     store.Store
}

func NewIndexer(s store.Store) *Indexer {
	return &Indexer{
		store:     s,
		index:     make(map[string][]int),
		documents: make([]model.Document, 0),
	}
}

// Lookup returns document IDs for a token and whether the token exists in the index.
func (i *Indexer) Lookup(token string) ([]int, bool) {
	ids, ok := i.index[token]
	return ids, ok
}

func (i *Indexer) AddDocument(doc model.Document) {
	doc.ID = len(i.documents)
	i.documents = append(i.documents, doc)
	tokens := append(Analyzer(doc.Content), Analyzer(doc.Title)...)
	for _, token := range tokens {
		ids := i.index[token]
		if len(ids) > 0 && ids[len(ids)-1] == doc.ID {
			continue
		}
		i.index[token] = append(ids, doc.ID)
	}
}

func (i *Indexer) GetDocument(id int) (model.Document, bool) {
	if id < 0 || id >= len(i.documents) {
		return model.Document{}, false
	}
	return i.documents[id], true
}

func (i *Indexer) Save() error {
	if err := i.store.SaveDocuments(i.documents); err != nil {
		return fmt.Errorf("saving documents: %w", err)
	}
	if err := i.store.SaveIndex(i.index); err != nil {
		return fmt.Errorf("saving index: %w", err)
	}
	return nil
}
func (i *Indexer) Load() error {
	documents, err := i.store.LoadDocuments()
	if err != nil {
		return fmt.Errorf("loading documents: %w", err)
	}
	index, err := i.store.LoadIndex()
	if err != nil {
		return fmt.Errorf("loading index: %w", err)
	}
	i.documents = documents
	i.index = index
	return nil
}
