package indexer

import (
	"strings"
	"unicode"

	snowball "github.com/kljensen/snowball/english"
)

var stopwords = map[string]struct{}{"i": {}, "me": {}, "my": {}, "myself": {}, "we": {}, "our": {}, "ours": {}, "ourselves": {}, "you": {}, "your": {}, "yours": {}, "yourself": {}, "yourselves": {}, "he": {}, "him": {}, "his": {}, "himself": {}, "she": {}, "her": {}, "hers": {}, "herself": {}, "it": {}, "its": {}, "itself": {}, "they": {}, "them": {}, "their": {}, "theirs": {}, "themselves": {}, "what": {}, "which": {}, "who": {}, "whom": {}, "this": {}, "that": {}, "these": {}, "those": {}, "am": {}, "is": {}, "are": {}, "was": {}, "were": {}, "be": {}, "been": {}, "being": {}, "have": {}, "has": {}, "had": {}, "having": {}, "do": {}, "does": {}, "did": {}, "doing": {}, "a": {}, "an": {}, "the": {}, "and": {}, "but": {}, "if": {}, "or": {}, "because": {}, "as": {}, "until": {}, "while": {}, "of": {}, "at": {}, "by": {}, "for": {}, "with": {}, "about": {}, "against": {}, "between": {}, "into": {}, "through": {}, "during": {}, "before": {}, "after": {}, "above": {}, "below": {}, "to": {}, "from": {}, "up": {}, "down": {}, "in": {}, "out": {}, "on": {}, "off": {}, "over": {}, "under": {}, "again": {}, "further": {}, "then": {}, "once": {}, "here": {}, "there": {}, "when": {}, "where": {}, "why": {}, "how": {}, "all": {}, "any": {}, "both": {}, "each": {}, "few": {}, "more": {}, "most": {}, "other": {}, "some": {}, "such": {}, "no": {}, "nor": {}, "not": {}, "only": {}, "own": {}, "same": {}, "so": {}, "than": {}, "too": {}, "very": {}, "s": {}, "t": {}, "can": {}, "will": {}, "just": {}, "don": {}, "should": {}, "now": {}}

func tokenize(text string) []string {
	lowercase := strings.ToLower(text)

	tokens := strings.FieldsFunc(lowercase, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})

	return tokens
}

func filterStopWords(tokens []string) []string {
	filtered := make([]string, 0, len(tokens))

	for _, token := range tokens {
		if len(token) < 2 {
			continue
		}
		if _, isStop := stopwords[token]; !isStop {
			filtered = append(filtered, token)
		}
	}
	return filtered
}
func stem(tokens []string) []string {
	stemmed := make([]string, len(tokens))
	for i, token := range tokens {
		stemmed[i] = snowball.Stem(token, false)
	}
	return stemmed
}

func Analyzer(text string) []string {
	tokenized := tokenize(text)
	filtered := filterStopWords(tokenized)
	stemmed := stem(filtered)
	return stemmed
}
