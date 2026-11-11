package suggest

import (
	"reflect"
	"sync"
	"testing"
)

func TestSuggest(t *testing.T) {
	tests := []struct {
		name   string
		texts  []string
		prefix string
		want   []string
	}{
		{
			name:   "ranked by document frequency",
			texts:  []string{"golang gopher", "golang gopher", "golang goroutine"},
			prefix: "go",
			want:   []string{"golang", "gopher", "goroutine"},
		},
		{
			name:   "alphabetical tie-break with equal frequency",
			texts:  []string{"zone zest zebra zap"},
			prefix: "ze",
			want:   []string{"zebra", "zest"},
		},
		{
			name:   "limit of eight",
			texts:  []string{"aa1 aa2 aa3 aa4 aa5 aa6 aa7 aa8 aa9 aa10", "aa9 aa10"},
			prefix: "aa",
			want:   []string{"aa10", "aa9", "aa1", "aa2", "aa3", "aa4", "aa5", "aa6"},
		},
		{
			name:   "case-insensitive prefix with spaces",
			texts:  []string{"Search Engine searching"},
			prefix: "  SeA ",
			want:   []string{"search", "searching"},
		},
		{
			name:   "prefix shorter than two runes",
			texts:  []string{"apple"},
			prefix: "a",
			want:   []string{},
		},
		{
			name:   "empty prefix",
			texts:  []string{"apple"},
			prefix: "   ",
			want:   []string{},
		},
		{
			name:   "no match gives empty slice",
			texts:  []string{"apple banana"},
			prefix: "xy",
			want:   []string{},
		},
		{
			name:   "empty corpus",
			texts:  nil,
			prefix: "ap",
			want:   []string{},
		},
		{
			name:   "unstemmed words",
			texts:  []string{"Articles about exercise", "articles"},
			prefix: "ar",
			want:   []string{"articles"},
		},
		{
			name:   "unstemmed words second prefix",
			texts:  []string{"Articles about exercise"},
			prefix: "exer",
			want:   []string{"exercise"},
		},
		{
			name:   "stopwords absent",
			texts:  []string{"these thesis there theory"},
			prefix: "the",
			want:   []string{"theory", "thesis"},
		},
		{
			name:   "frequency counts once per document",
			texts:  []string{"beta beta beta beta", "bear", "bear"},
			prefix: "be",
			want:   []string{"bear", "beta"},
		},
		{
			name:   "unicode prefix",
			texts:  []string{"Über übung", "über"},
			prefix: "ÜB",
			want:   []string{"über", "übung"},
		},
		{
			name:   "two-rune unicode prefix of two bytes each",
			texts:  []string{"éé éèe"},
			prefix: "éé",
			want:   []string{"éé"},
		},
		{
			name:   "prefix equal to whole word",
			texts:  []string{"go gopher"},
			prefix: "gopher",
			want:   []string{"gopher"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := New(tt.texts).Suggest(tt.prefix)
			if got == nil {
				t.Fatal("Suggest returned nil")
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Suggest(%q) = %q, want %q", tt.prefix, got, tt.want)
			}
		})
	}
}

func TestSuggestConcurrent(t *testing.T) {
	s := New([]string{"concurrent concurrency", "concurrent"})
	want := []string{"concurrent", "concurrency"}
	var wg sync.WaitGroup
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if got := s.Suggest("con"); !reflect.DeepEqual(got, want) {
				t.Errorf("Suggest = %q, want %q", got, want)
			}
		}()
	}
	wg.Wait()
}
