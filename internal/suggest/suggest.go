// Package suggest gives prefix suggestions of real, unstemmed words ranked by
// document frequency.
package suggest

import (
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/joyyth/search-engine-v1.0/internal/indexer"
)

// MaxSuggestions is the most words Suggest returns.
const MaxSuggestions = 8

// minPrefixRunes is the shortest prefix, in runes, that Suggest answers.
const minPrefixRunes = 2

type entry struct {
	word string
	df   int
}

// Suggester holds every word of the corpus sorted by word. It is immutable
// after New, so concurrent Suggest calls need no locks.
type Suggester struct {
	entries []entry
}

// New builds a Suggester from document texts (title and content of each
// document). A word counts once per document.
func New(texts []string) *Suggester {
	df := make(map[string]int)
	for _, text := range texts {
		seen := make(map[string]struct{})
		for _, word := range indexer.Normalize(text) {
			if _, ok := seen[word]; ok {
				continue
			}
			seen[word] = struct{}{}
			df[word]++
		}
	}

	entries := make([]entry, 0, len(df))
	for word, n := range df {
		entries = append(entries, entry{word: word, df: n})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].word < entries[j].word })

	return &Suggester{entries: entries}
}

// Suggest returns at most MaxSuggestions words that start with prefix, by
// document frequency descending, then alphabetically. The prefix is trimmed
// and lowercased; one shorter than two runes gives an empty slice. The result
// is never nil.
func (s *Suggester) Suggest(prefix string) []string {
	prefix = strings.ToLower(strings.TrimSpace(prefix))
	if utf8.RuneCountInString(prefix) < minPrefixRunes {
		return []string{}
	}

	start := sort.Search(len(s.entries), func(i int) bool { return s.entries[i].word >= prefix })
	end := start
	for end < len(s.entries) && strings.HasPrefix(s.entries[end].word, prefix) {
		end++
	}

	matches := make([]entry, end-start)
	copy(matches, s.entries[start:end])
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].df != matches[j].df {
			return matches[i].df > matches[j].df
		}
		return matches[i].word < matches[j].word
	})

	if len(matches) > MaxSuggestions {
		matches = matches[:MaxSuggestions]
	}
	words := make([]string, len(matches))
	for i, m := range matches {
		words[i] = m.word
	}
	return words
}
