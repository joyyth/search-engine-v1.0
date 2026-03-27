package search

import (
	"sort"

	"github.com/joyyth/search-engine-v1.0/internal/indexer"
	"github.com/joyyth/search-engine-v1.0/internal/model"
)

type Search struct {
	indexer *indexer.Indexer
}

func NewSearch (indexer *indexer.Indexer) *Search{
	return &Search{indexer: indexer}
}

func (s *Search) Search(query string) []model.Document{
	tokens := indexer.Analyzer(query)

	scores := make(map[int]int)
	for _, token := range tokens {
		if ids, ok := s.indexer.Lookup(token); ok {
			for _,id := range ids{
				scores[id]++
			}
		}
	}

	type scoredDocument struct {
		id int
		score int 
	}
	ranked := make([]scoredDocument,0,len(scores))
	for id, score := range scores{
		ranked= append(ranked, scoredDocument{id: id, score: score})
	}
	sort.Slice(ranked, func(i,j int)bool{
		return ranked[i].score > ranked[j].score
	})
	results := make([]model.Document,0,len(ranked))
	for _, ranked :=range ranked{
		if doc,ok := s.indexer.GetDocuments(ranked.id); ok{
			results = append(results, doc)
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