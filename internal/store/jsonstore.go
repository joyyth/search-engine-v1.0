package store

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/joyyth/seacrh-engine-v1.0/internal/model"
)

type JSONStore struct {
	docPath   string
	indexPath string
}

func NewJSONStore(docPath, indexPath string) *JSONStore {
	return &JSONStore{
		docPath:   docPath,
		indexPath: indexPath,
	}
}

func (s *JSONStore) SaveDocuments(docs []model.Document) error{
	return writeJSON(s.docPath,docs)
}
func (s *JSONStore) LoadDocuments() ([]model.Document, error){
	var docs []model.Document
	err := readJSON(s.docPath, &docs)
	return docs, err
}

func (s *JSONStore) SaveIndex(index map[string][]int) error{
      return  writeJSON(s.indexPath,index)
}

func (s *JSONStore) LoadIndex() (map[string][]int, error){
	index := make(map[string][]int)
	err := readJSON(s.indexPath, &index)
	return index, err
}

func writeJSON(path string, v any) error{
	f, err:=os.Create(path)
	if err!=nil{
		return fmt.Errorf("creating file: %w", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("encoding JSON: %w", err)
	}
	return nil
}
func readJSON(path string, v any) error{
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("opening file: %w", err)
	}
	defer f.Close()
	return json.NewDecoder(f).Decode(v)
}