package search

import (
	"sort"
	"strings"

	"github.com/joyyth/search-engine-v1.0/internal/indexer"
	"github.com/joyyth/search-engine-v1.0/internal/model"
	"github.com/joyyth/search-engine-v1.0/internal/utils"
)

var maxResults = 20

type Search struct {
	indexer *indexer.Indexer
}

func NewSearch(indexer *indexer.Indexer) *Search {
	return &Search{indexer: indexer}
}
func (s *Search) SearchWithSnippets(query string) []Result {
	docs := s.Search(query)
	rawTokens := strings.Fields(strings.ToLower(query))
	results := make([]Result, len(docs))
	for i, doc := range docs {
		results[i] = Result{
			Document: doc,
			Snippet:  utils.BuildSnippet(rawTokens, doc.Content),
		}
	}
	return results
}

func (s *Search) Search(query string) []model.Document {
	tokens := indexer.Analyzer(query)

	scores := make(map[int]int)
	for _, token := range tokens {
		if ids, ok := s.indexer.Lookup(token); ok {
			for _, id := range ids {
				scores[id]++
			}
		}
	}
	if len(scores) == 0 {
		return []model.Document{}
	}

	type scoredDocument struct {
		id    int
		score int
	}
	ranked := make([]scoredDocument, 0, len(scores))
	for id, score := range scores {
		ranked = append(ranked, scoredDocument{id: id, score: score})
	}
	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].score > ranked[j].score
	})
	results := make([]model.Document, 0, maxResults)
	for _, r := range ranked {
		if doc, ok := s.indexer.GetDocument(r.id); ok {
			results = append(results, doc)
			if len(results) >= maxResults {
				break
			}
		}
	}

	return results
}
