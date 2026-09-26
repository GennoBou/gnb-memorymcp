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
			input:    "Hello World",
			expected: "helloworld",
		},
		{
			name:     "punctuation and special characters removal",
			input:    "Hello, World! 123 #$%^&*",
			expected: "helloworld123",
		},
		{
			name:     "Japanese hiragana, katakana, kanji and fullwidth digits",
			input:    "こんにちは、世界！ Golang 123",
			expected: "こんにちは世界golang123",
		},
		{
			name:     "only symbols and spaces",
			input:    " !!! ??? ---   ",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeString(tt.input)
			if got != tt.expected {
				t.Errorf("normalizeString(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestJaccardSimilarityFromBiGrams(t *testing.T) {
	tests := []struct {
		name     string
		g1       map[string]bool
		g2       map[string]bool
		expected float64
	}{
		{
			name:     "both maps nil",
			g1:       nil,
			g2:       nil,
			expected: 0.0,
		},
		{
			name:     "both maps empty",
			g1:       map[string]bool{},
			g2:       map[string]bool{},
			expected: 0.0,
		},
		{
			name:     "one map empty, one map non-empty",
			g1:       map[string]bool{"ab": true, "bc": true},
			g2:       map[string]bool{},
			expected: 0.0,
		},
		{
			name:     "one map nil, one map non-empty",
			g1:       nil,
			g2:       map[string]bool{"ab": true},
			expected: 0.0,
		},
		{
			name:     "completely disjoint bi-grams",
			g1:       map[string]bool{"ab": true, "bc": true},
			g2:       map[string]bool{"xy": true, "yz": true},
			expected: 0.0,
		},
		{
			name:     "identical bi-grams",
			g1:       map[string]bool{"he": true, "el": true, "ll": true, "lo": true},
			g2:       map[string]bool{"he": true, "el": true, "ll": true, "lo": true},
			expected: 1.0,
		},
		{
			name:     "partial match (intersection 1, union 3 -> 1/3)",
			g1:       map[string]bool{"ab": true, "bc": true},
			g2:       map[string]bool{"bc": true, "cd": true},
			expected: 1.0 / 3.0,
		},
		{
			name:     "partial match with subsets (intersection 2, union 4 -> 2/4 = 0.5)",
			g1:       map[string]bool{"ab": true, "bc": true, "cd": true},
			g2:       map[string]bool{"bc": true, "cd": true, "de": true},
			expected: 0.5,
		},
	}

	const epsilon = 1e-4

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := jaccardSimilarityFromBiGrams(tt.g1, tt.g2)
			if math.Abs(got-tt.expected) > epsilon {
				t.Errorf("jaccardSimilarityFromBiGrams(%v, %v) = %f, want %f", tt.g1, tt.g2, got, tt.expected)
			}

			// Verify symmetry: jaccardSimilarityFromBiGrams(g1, g2) == jaccardSimilarityFromBiGrams(g2, g1)
			gotSymmetric := jaccardSimilarityFromBiGrams(tt.g2, tt.g1)
			if math.Abs(gotSymmetric-tt.expected) > epsilon {
				t.Errorf("jaccardSimilarityFromBiGrams(%v, %v) [symmetric] = %f, want %f", tt.g2, tt.g1, gotSymmetric, tt.expected)
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
			name:     "only symbols",
			input:    "!!! ***",
			expected: map[string]bool{},
		},
		{
			name:  "single character",
			input: "a!",
			expected: map[string]bool{
				"a": true,
			},
		},
		{
			name:  "single Japanese character",
			input: "あ！",
			expected: map[string]bool{
				"あ": true,
			},
		},
		{
			name:  "multi-character ASCII string",
			input: "hello",
			expected: map[string]bool{
				"he": true,
				"el": true,
				"ll": true,
				"lo": true,
			},
		},
		{
			name:  "multi-character Japanese string",
			input: "あいうえお",
			expected: map[string]bool{
				"あい": true,
				"いう": true,
				"うえ": true,
				"えお": true,
			},
		},
		{
			name:  "repeated bi-grams deduplication",
			input: "aaaa",
			expected: map[string]bool{
				"aa": true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := charBiGrams(tt.input)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("charBiGrams(%q) = %v, want %v", tt.input, got, tt.expected)
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
			name:     "no valid alphanumeric chars in one string",
			s1:       "hello",
			s2:       "!!! ###",
			expected: 0.0,
		},
		{
			name:     "identical strings",
			s1:       "Pythonは機械学習でよく使われます。",
			s2:       "Pythonは機械学習でよく使われます。",
			expected: 1.0,
		},
		{
			name:     "identical normalized strings with different symbols and casing",
			s1:       "Hello, World!",
			s2:       "hello world",
			expected: 1.0,
		},
		{
			name:     "completely disjoint strings",
			s1:       "abcdef",
			s2:       "ghijkl",
			expected: 0.0,
		},
		{
			name: "similar Japanese sentences",
			s1:   "Pythonは機械学習でよく使われます。",
			s2:   "Pythonは機械学習でとてもよく使われています。",
			// s1 normalized: "pythonは機械学習でよく使われます"
			// s2 normalized: "pythonは機械学習でとてもよく使われています"
			// Intersection: 16, Union: 25 -> 16 / 25 = 0.64
			expected: 0.64,
		},
		{
			name:     "single character comparison (match)",
			s1:       "a",
			s2:       "a",
			expected: 1.0,
		},
		{
			name:     "single character comparison (mismatch)",
			s1:       "a",
			s2:       "b",
			expected: 0.0,
		},
	}

	const epsilon = 1e-4

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := jaccardSimilarity(tt.s1, tt.s2)
			if math.Abs(got-tt.expected) > epsilon {
				t.Errorf("jaccardSimilarity(%q, %q) = %f, want %f", tt.s1, tt.s2, got, tt.expected)
			}

			// Verify symmetry: jaccardSimilarity(s1, s2) == jaccardSimilarity(s2, s1)
			gotSymmetric := jaccardSimilarity(tt.s2, tt.s1)
			if math.Abs(gotSymmetric-tt.expected) > epsilon {
				t.Errorf("jaccardSimilarity(%q, %q) [symmetric] = %f, want %f", tt.s2, tt.s1, gotSymmetric, tt.expected)
			}
		})
	}
}
