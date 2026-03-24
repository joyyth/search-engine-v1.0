package search

import (
	"github.com/joyyth/seacrh-engine-v1.0/internal/indexer"
)

type Search struct {
	indexer *indexer.Indexer
}

func NewSearch (indexer *indexer.Indexer) *Search{
	return &Search{indexer: indexer}
}

func (s *Search) Search(query string) [][]int{
	var results [][]int
	tokens := indexer.Analyzer(query)
	for _, token := range tokens {
		if ids, ok := s.indexer.Lookup(token); ok {
			results = append(results, ids)
		}
	}
	return results
}
func (s *Search) Rank(results [][]int) [][]int{
	 ranked := make([][]int, len(results))

	 for i, ids := range results{
		for _, id := range ids{
			ranked[i]= append(ranked[i], id)
		}
	 }
	 return ranked
}