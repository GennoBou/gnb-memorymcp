package sqlite

import (
	"strings"
	"unicode"
)

// normalizeString は類似度比較のために文字列を正規化します（小文字化、スペース・記号の除去）
func normalizeString(s string) string {
	var sb strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

// charBiGrams は文字列から文字2-gram（バイグラム）を抽出します。
func charBiGrams(s string) map[string]bool {
	grams := make(map[string]bool)
	normalized := normalizeString(s)
	runes := []rune(normalized)
	if len(runes) < 2 {
		if len(runes) == 1 {
			grams[string(runes[0])] = true
		}
		return grams
	}
	for i := 0; i < len(runes)-1; i++ {
		gram := string(runes[i : i+2])
		grams[gram] = true
	}
	return grams
}

// jaccardSimilarityFromBiGrams は事前抽出された2つの文字バイグラムマップから Jaccard 係数類似度を計算します。
func jaccardSimilarityFromBiGrams(g1, g2 map[string]bool) float64 {
	if len(g1) == 0 || len(g2) == 0 {
		return 0.0
	}

	intersection := 0
	for gram := range g1 {
		if g2[gram] {
			intersection++
		}
	}

	union := len(g1) + len(g2) - intersection
	return float64(intersection) / float64(union)
}

// jaccardSimilarity は2つの文字列の文字バイグラムによる Jaccard 係数類似度を計算します。
func jaccardSimilarity(s1, s2 string) float64 {
	g1 := charBiGrams(s1)
	g2 := charBiGrams(s2)
	return jaccardSimilarityFromBiGrams(g1, g2)
}
