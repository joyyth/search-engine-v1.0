package search

import (
	"net/url"
	"strings"

	"github.com/joyyth/search-engine-v1.0/internal/indexer"
	"github.com/joyyth/search-engine-v1.0/internal/model"
)

const (
	exactURLPathBoost = 18.0
	urlSegmentBoost   = 14.0
	titleTokenBoost   = 4.0
	titlePhraseBoost  = 6.0
)

// pathURLSegments returns lowercased URL path segments, without version suffixes
// like "flag@go1.26.2" → "flag".
func pathURLSegments(rawURL string) []string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Path == "" {
		return nil
	}
	parts := strings.FieldsFunc(u.Path, func(r rune) bool { return r == '/' })
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p == "" {
			continue
		}
		if i := strings.Index(p, "@"); i >= 0 {
			p = p[:i]
		}
		p = strings.ToLower(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// relevanceBoost adds score for pkg.go.dev-style URLs and titles so a query like
// "flag" ranks /flag above pages that only mention the word in passing.
func relevanceBoost(doc model.Document, query string, stemmed []string) float64 {
	query = strings.TrimSpace(query)
	if query == "" {
		return 0
	}

	titleTerms := indexer.Normalize(doc.Title)
	queryTerms := indexer.Normalize(query)
	segs := pathURLSegments(doc.URL)

	seenSeg := make(map[string]struct{})
	var boost float64

	addSegMatches := func(term string) {
		if len(term) < 2 {
			return
		}
		for _, seg := range segs {
			if seg != term {
				continue
			}
			key := seg
			if _, ok := seenSeg[key]; ok {
				return
			}
			seenSeg[key] = struct{}{}
			boost += urlSegmentBoost
			return
		}
	}

	for _, t := range queryTerms {
		addSegMatches(t)
	}
	for _, t := range stemmed {
		addSegMatches(t)
	}
	if len(queryTerms) == 1 && strings.Join(segs, "/") == queryTerms[0] {
		boost += exactURLPathBoost
	}

	for _, t := range queryTerms {
		if containsToken(titleTerms, t) {
			boost += titleTokenBoost
			break
		}
	}
	if len(queryTerms) >= 2 && containsPhrase(titleTerms, queryTerms) {
		boost += titlePhraseBoost
	}

	return boost
}

// urlPathMatchesQuery reports whether any query term equals a URL path segment
// (e.g. query "flag" matches https://pkg.go.dev/flag).
func urlPathMatchesQuery(rawURL string, stemmed []string, rawTerms []string) bool {
	segs := pathURLSegments(rawURL)
	for _, term := range stemmed {
		if termMatchesPathSeg(term, segs) {
			return true
		}
	}
	for _, term := range rawTerms {
		if termMatchesPathSeg(term, segs) {
			return true
		}
	}
	return false
}

func termMatchesPathSeg(term string, segs []string) bool {
	if len(term) < 2 {
		return false
	}
	for _, seg := range segs {
		if seg == term {
			return true
		}
	}
	return false
}

func containsToken(tokens []string, want string) bool {
	for _, token := range tokens {
		if token == want {
			return true
		}
	}
	return false
}

func containsPhrase(tokens, phrase []string) bool {
	if len(phrase) == 0 || len(phrase) > len(tokens) {
		return false
	}
	for i := 0; i <= len(tokens)-len(phrase); i++ {
		matched := true
		for j := range phrase {
			if tokens[i+j] != phrase[j] {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}

func snippetSource(title, content string) string {
	title = strings.TrimSpace(title)
	content = strings.TrimSpace(content)
	switch {
	case title == "":
		return content
	case content == "":
		return title
	default:
		return title + " " + content
	}
}
