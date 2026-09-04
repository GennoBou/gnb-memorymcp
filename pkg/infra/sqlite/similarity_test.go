package sqlite

import (
	"math"
	"reflect"
	"testing"
)

func TestNormalizeString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "lowercase conversion and space removal",
			input:    " Hello World ",
			expected: "helloworld",
		},
		{
			name:     "symbols and punctuation removal",
			input:    "Go!! 1.23, @#$%",
			expected: "go123",
		},
		{
			name:     "Japanese text and punctuation",
			input:    "こんにちは、世界！123",
			expected: "こんにちは世界123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeString(tt.input)
			if got != tt.expected {
				t.Errorf("normalizeString(%q) = %q; want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestCharBiGrams(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]bool
	}{
		{
			name:     "empty string",
			input:    "",
			expected: map[string]bool{},
		},
		{
			name:     "single character",
			input:    "a",
			expected: map[string]bool{"a": true},
		},
		{
			name:     "single character with punctuation",
			input:    "!a ",
			expected: map[string]bool{"a": true},
		},
		{
			name:  "multi character ascii",
			input: "hello",
			expected: map[string]bool{
				"he": true,
				"el": true,
				"ll": true,
				"lo": true,
			},
		},
		{
			name:  "japanese text",
			input: "あいうえお",
			expected: map[string]bool{
				"あい": true,
				"いう": true,
				"うえ": true,
				"えお": true,
			},
		},
		{
			name:  "duplicate bigrams",
			input: "banana",
			expected: map[string]bool{
				"ba": true,
				"an": true,
				"na": true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := charBiGrams(tt.input)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("charBiGrams(%q) = %v; want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestJaccardSimilarity(t *testing.T) {
	tests := []struct {
		name     string
		s1       string
		s2       string
		expected float64
	}{
		{
			name:     "both empty",
			s1:       "",
			s2:       "",
			expected: 0.0,
		},
		{
			name:     "one empty",
			s1:       "hello",
			s2:       "",
			expected: 0.0,
		},
		{
			name:     "only symbols",
			s1:       "!!!",
			s2:       "hello",
			expected: 0.0,
		},
		{
			name:     "identical strings",
			s1:       "hello world",
			s2:       "Hello, World!",
			expected: 1.0,
		},
		{
			name:     "completely disjoint",
			s1:       "abc",
			s2:       "xyz",
			expected: 0.0,
		},
		{
			name:     "partial match ascii",
			s1:       "night",
			s2:       "nacht",
			expected: 1.0 / 7.0,
		},
		{
			name:     "partial match japanese",
			s1:       "Pythonは機械学習でよく使われます。",
			s2:       "Pythonは機械学習でとてもよく使われています。",
			expected: 16.0 / 25.0,
		},
	}

	const eps = 1e-6

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := jaccardSimilarity(tt.s1, tt.s2)
			if math.Abs(got-tt.expected) > eps {
				t.Errorf("jaccardSimilarity(%q, %q) = %f; want %f", tt.s1, tt.s2, got, tt.expected)
			}
		})
	}
}
