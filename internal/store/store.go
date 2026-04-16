package store

import "github.com/joyyth/search-engine-v1.0/internal/model"

type Store interface {
	SaveDocuments(docs []model.Document) error
	LoadDocuments() ([]model.Document, error)
	SaveIndex(index map[string][]model.Posting) error
	LoadIndex() (map[string][]model.Posting, error)
}
