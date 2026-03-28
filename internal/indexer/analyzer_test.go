package indexer

import (
	"testing"
)

func TestAnalyzer(t *testing.T){
	tests := []struct{
		name string
		input string
		want []string
	}{
		{
			name: "check basic tokenization and stemming",
			input: "Running and walking are good forms of exercise",
			want :[]string{"run", "walk", "good", "form", "exercis"},
		},
		{
			name: "stop words filtering",
			input: "I am a good boy and I like to play with my friends",
			want :[]string{"good", "boy", "like", "play", "friend"},
		},
		{
			name : "punctuation handling",
			input : "Hello, world! This is a test.",
			want :[]string{"hello", "world", "test"},
		},
		{
			name : "empty input",
			input : "",
			want :[]string{},
		}, {
			name :"only stop words",
			input : "the and is at by for of to with",
			want :[]string{},
		},
		{
			name: "numbers preservation",
			input: "The price is 100 dollars and 50 cents",
			want :[]string{"price", "100", "dollar", "50", "cent"},
		},
		{
            name:  "duplicate tokens preserved",
            input: "go go go",
            want:  []string{"go", "go", "go"},
        },
        {
            name:  "mixed case normalized",
            input: "HTTP Server HANDLER",
            want:  []string{"http", "server", "handler"},
        },

	}
	for _, test := range tests{
		t.Run(test.name, func(t *testing.T){
			 got := Analyzer(test.input)

			 if len(got)==0 && len(test.want)==0{
				return
			 }
			 if len(got) != len(test.want) {
                t.Fatalf("Analyzer(%q) returned %d tokens, want %d\ngot:  %v\nwant: %v",
                    test.input, len(got), len(test.want), got, test.want)
            }
			for i := range got{
				if got[i] != test.want[i]{
					t.Errorf("Analyzer(%q)[%d] = %q, want %q",
                        test.input, i, got[i], test.want[i])
				}
			}
		})
	}
}