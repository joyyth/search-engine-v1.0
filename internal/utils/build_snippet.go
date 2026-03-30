package utils

import "strings"

const snippetRadius = 150 // rune window on each side of the match

// BuildSnippet returns a short excerpt of content centered on the most relevant

func BuildSnippet(tokens []string, content string) string {
	clean := strings.Join(strings.Fields(content), " ")

	runes := []rune(clean)
	lowerRunes := []rune(strings.ToLower(clean))

	matchRune := -1
	bestLen := 0

	// Find best match (prefer longer tokens)
	for _, token := range tokens {
		tokenLower := []rune(strings.ToLower(token))
		tLen := len(tokenLower)

		if tLen == 0 || tLen > len(lowerRunes) {
			continue
		}

		for i := 0; i <= len(lowerRunes)-tLen; i++ {
			if runesEqual(lowerRunes[i:i+tLen], tokenLower) {
				if tLen > bestLen {
					matchRune = i
					bestLen = tLen
				}
				break // move to next token
			}
		}
	}

	// No match found → fallback to beginning snippet
	if matchRune < 0 {
		if len(runes) <= snippetRadius*2 {
			return string(runes)
		}
		return string(runes[:snippetRadius*2]) + "..."
	}

	// Compute window
	start := matchRune - snippetRadius
	start = max(0, start)

	end := matchRune + snippetRadius
	end = min(end, len(runes))

	// Snap start to word boundary
	for start > 0 && runes[start] != ' ' {
		start--
	}

	// Snap end to word boundary
	for end < len(runes) && runes[end] != ' ' {
		end++
	}

	prefix, suffix := "", ""
	if start > 0 {
		prefix = "..."
	}
	if end < len(runes) {
		suffix = "..."
	}

	return prefix + string(runes[start:end]) + suffix
}

func runesEqual(a, b []rune) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
