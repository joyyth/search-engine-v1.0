package indexer

import (
	"fmt"
	"sort"

	"github.com/joyyth/search-engine-v1.0/internal/model"
	"github.com/joyyth/search-engine-v1.0/internal/store"
)

type Indexer struct {
	index     map[string][]model.Posting
	documents []model.Document
	store     store.Store
}
type Stats struct{
	TotalDocuments int `json:"total_documents"`
	TotalTerms int `json:"total_terms"`
	TopTerms []TermStat `json:"term_stats"`
}

type TermStat struct{
	Term string `json:"term"`
	DocCount int `json:"doc_count"`
}

func NewIndexer(s store.Store) *Indexer {
	return &Indexer{
		store:     s,
		index:     make(map[string][]model.Posting),
		documents: make([]model.Document, 0),
	}
}

// Lookup returns document IDs for a token and whether the token exists in the index.
func (i *Indexer) Lookup(token string) ([]model.Posting, bool) {
	ids, ok := i.index[token]
	return ids, ok
}

func (i *Indexer) AddDocument(doc model.Document) {
	doc.ID = len(i.documents)
	i.documents = append(i.documents, doc)
	tokens := append(Analyzer(doc.Content), Analyzer(doc.Title)...)

	// count the occurrences of each token in the document
	termCounts:= make(map[string]int)
	for _,t := range tokens{
		termCounts[t]++
	}
	totalTokens := len(tokens)
	for token, count := range termCounts{
		tf := float64(count) / float64(totalTokens)
		i.index[token] = append(i.index[token], model.Posting{DocID: doc.ID, TF: tf})
	}
}

func (i *Indexer) GetDocument(id int) (model.Document, bool) {
	if id < 0 || id >= len(i.documents) {
		return model.Document{}, false
	}
	return i.documents[id], true
}

// DocCount returns the number of documents indexed (for IDF and similar).
func (i *Indexer) DocCount() int {
	return len(i.documents)
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

func (idx *Indexer) Stats() Stats{

	type kv struct{
		term string 
		count int 
	}
    pairs := make([]kv,0, len(idx.index))
	for term, ids := range idx.index{
		 pairs = append(pairs, kv{term:term, count: len(ids)})
	}
	sort.Slice(pairs, func(i,j int) bool{
		return  pairs[i].count >pairs[j].count
	})

	top := min(10, len(pairs))
	topTerms := make([]TermStat,top)
	for idx, p := range pairs[:top] {
        topTerms[idx] = TermStat{Term: p.term, DocCount: p.count}
    }
	return Stats{
		TotalDocuments: len(idx.documents),
		TotalTerms: len(idx.index),
		TopTerms: topTerms,
	}

}