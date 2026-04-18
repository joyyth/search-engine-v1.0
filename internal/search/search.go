package search

import (
	"math"
	"sort"
	"strings"

	"github.com/joyyth/search-engine-v1.0/internal/indexer"
	"github.com/joyyth/search-engine-v1.0/internal/model"
	"github.com/joyyth/search-engine-v1.0/internal/utils"
)

const maxResults = 20

type Result struct {
	Document model.Document `json:"document"`
	Snippet  string         `json:"snippet"`
}

type Search struct {
	indexer *indexer.Indexer
}

func NewSearch(indexer *indexer.Indexer) *Search {
	return &Search{indexer: indexer}
}
func (s *Search) SearchWithSnippets(query string) ([]Result, int) {
	docs, total := s.Search(query)
	rawTokens := strings.Fields(strings.ToLower(query))
	results := make([]Result, len(docs))
	for i, doc := range docs {
		results[i] = Result{
			Document: doc,
			Snippet:  utils.BuildSnippet(rawTokens, snippetSource(doc.Title, doc.Content)),
		}
	}
	return results, total
}

func (s *Search) Search(query string) ([]model.Document, int) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, 0
	}

	tokens := indexer.Analyzer(query)
	rawTerms := indexer.Normalize(query)

	scores := make(map[int]float64)
	n := float64(s.indexer.DocCount())
	if n == 0 {
		return nil, 0
	}
	for _, token := range tokens {
		postings, ok := s.indexer.Lookup(token)
		if !ok {
			continue
		}
		idf := math.Log(1 + n/float64(len(postings)))
		for _, p := range postings {
			scores[p.DocID] += p.TF * idf
		}
	}

	for id := 0; id < s.indexer.DocCount(); id++ {
		if _, has := scores[id]; has {
			continue
		}
		doc, ok := s.indexer.GetDocument(id)
		if !ok {
			continue
		}
		if urlPathMatchesQuery(doc.URL, tokens, rawTerms) {
			scores[id] = urlSegmentBoost * 0.3
		}
	}

	if len(scores) == 0 {
		return nil, 0
	}

	for id := range scores {
		doc, ok := s.indexer.GetDocument(id)
		if !ok {
			continue
		}
		scores[id] += relevanceBoost(doc, query, tokens)
	}

	type scoredDocument struct {
		id    int
		score float64
	}
	ranked := make([]scoredDocument, 0, len(scores))
	for id, score := range scores {
		ranked = append(ranked, scoredDocument{id: id, score: score})
	}
	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].score > ranked[j].score
	})

	total := len(ranked)
	results := make([]model.Document, 0, len(ranked))
	for _, r := range ranked {
		if doc, ok := s.indexer.GetDocument(r.id); ok {
			results = append(results, doc)
			if len(results) >= maxResults {
				break
			}
		}
	}

	return results, total
}

func (s *Search) Stats() indexer.Stats {
	return s.indexer.Stats()
}
