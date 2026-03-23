package store

import "github.com/joyyth/seacrh-engine-v1.0/internal/model"

type Store interface {
	SaveDocuments(docs []model.Document) error
	LoadDocuments() ([]model.Document, error)
	SaveIndex(index map[string][]int) error
	LoadIndex() (map[string][]int, error)
}