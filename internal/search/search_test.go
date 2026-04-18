package search

import (
	"strings"
	"testing"

	"github.com/joyyth/search-engine-v1.0/internal/indexer"
	"github.com/joyyth/search-engine-v1.0/internal/model"
)

func TestPathURLSegments(t *testing.T) {
	t.Parallel()
	got := pathURLSegments("https://pkg.go.dev/flag@go1.26.2")
	want := []string{"flag"}
	if len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("got %v, want %v", got, want)
	}
	got2 := pathURLSegments("https://pkg.go.dev/compress/gzip")
	if len(got2) != 2 || got2[0] != "compress" || got2[1] != "gzip" {
		t.Fatalf("got %v", got2)
	}
}

func TestUrlPathMatchesQuery(t *testing.T) {
	t.Parallel()
	stemmed := []string{"flag"}
	raw := []string{"flag"}
	if !urlPathMatchesQuery("https://pkg.go.dev/flag", stemmed, raw) {
		t.Fatal("expected match")
	}
	if urlPathMatchesQuery("https://pkg.go.dev/compress/gzip", stemmed, raw) {
		t.Fatal("unexpected match")
	}
}

func TestRelevanceBoost_pkgPath(t *testing.T) {
	t.Parallel()
	doc := model.Document{
		URL:   "https://pkg.go.dev/flag@go1.26.2",
		Title: "flag package - flag - Go Packages",
	}
	b := relevanceBoost(doc, "flag", []string{"flag"})
	if b < urlSegmentBoost {
		t.Fatalf("expected URL boost, got %v", b)
	}
}

func TestRelevanceBoost_titleTokenRequiresWholeWord(t *testing.T) {
	t.Parallel()
	doc := model.Document{
		URL:   "https://pkg.go.dev/image",
		Title: "bitmap package",
	}
	b := relevanceBoost(doc, "map", indexer.Analyzer("map"))
	if b != 0 {
		t.Fatalf("got unexpected title boost %v", b)
	}
}

func TestRelevanceBoost_titlePhraseUsesNormalizedTerms(t *testing.T) {
	t.Parallel()
	doc := model.Document{
		URL:   "https://pkg.go.dev/flag",
		Title: "flag package - flag - Go Packages",
	}
	b := relevanceBoost(doc, "the flag package", indexer.Analyzer("the flag package"))
	if b < titleTokenBoost+titlePhraseBoost {
		t.Fatalf("expected normalized title phrase boost, got %v", b)
	}
}

func TestSearch_ranksPathMatchAboveIncidentalMention(t *testing.T) {
	idx := indexer.NewIndexer(nil)
	idx.AddDocument(model.Document{
		URL:     "https://pkg.go.dev/std",
		Title:   "Standard library",
		Content: "flag Package flag implements command-line flag parsing. " + strings.Repeat("other ", 200),
	})
	idx.AddDocument(model.Document{
		URL:     "https://pkg.go.dev/flag",
		Title:   "flag package - flag - Go Packages",
		Content: "Package flag implements command-line flag parsing.",
	})
	idx.AddDocument(model.Document{
		URL:     "https://pkg.go.dev/cmp",
		Title:   "cmp package",
		Content: "Package cmp is for comparisons.",
	})

	s := NewSearch(idx)
	docs, _ := s.Search("flag")
	if len(docs) < 2 {
		t.Fatalf("got %d docs", len(docs))
	}
	if docs[0].URL != "https://pkg.go.dev/flag" {
		t.Fatalf("first result = %s %q, want flag package URL on top", docs[0].URL, docs[0].Title)
	}
}

func TestSearch_ranksExactPackagePathAboveSubpackages(t *testing.T) {
	idx := indexer.NewIndexer(nil)
	idx.AddDocument(model.Document{
		URL:     "https://pkg.go.dev/hash",
		Title:   "hash package - hash - Go Packages",
		Content: "Package hash provides interfaces for hash functions.",
	})
	idx.AddDocument(model.Document{
		URL:     "https://pkg.go.dev/hash/fnv",
		Title:   "fnv package - hash/fnv - Go Packages",
		Content: "Package fnv implements FNV hash functions. The hash package interfaces are used here.",
	})
	idx.AddDocument(model.Document{
		URL:     "https://pkg.go.dev/hash/maphash",
		Title:   "maphash package - hash/maphash - Go Packages",
		Content: "Package maphash provides hash functions and uses hash package terminology throughout.",
	})

	s := NewSearch(idx)
	docs, _ := s.Search("hash")
	if len(docs) < 3 {
		t.Fatalf("got %d docs", len(docs))
	}
	if docs[0].URL != "https://pkg.go.dev/hash" {
		t.Fatalf("first result = %s %q, want exact hash package URL on top", docs[0].URL, docs[0].Title)
	}
}
