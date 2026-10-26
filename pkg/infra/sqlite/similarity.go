package sqlite

import (
	"strings"
	"unicode"
)

// normalizeString は類似度比較のために文字列を正規化します（小文字化、スペース・記号の除去）
func normalizeString(s string) string {
	var sb strings.Builder
	sb.Grow(len(s))
	for _, r := range s {
		if ('a' <= r && r <= 'z') || ('0' <= r && r <= '9') {
			sb.WriteRune(r)
		} else if 'A' <= r && r <= 'Z' {
			sb.WriteRune(r + ('a' - 'A'))
		} else if r >= 0x80 {
			rLower := unicode.ToLower(r)
			if unicode.IsLetter(rLower) || unicode.IsDigit(rLower) {
				sb.WriteRune(rLower)
			}
		}
	}
	return sb.String()
}

// charBiGrams は文字列から文字2-gram（バイグラム）を抽出します。
func charBiGrams(s string) map[string]bool {
	normalized := normalizeString(s)
	if normalized == "" {
		return make(map[string]bool)
	}

	// ASCII 文字列か判定
	isASCII := true
	for i := 0; i < len(normalized); i++ {
		if normalized[i] >= 0x80 {
			isASCII = false
			break
		}
	}

	if isASCII {
		n := len(normalized)
		if n < 2 {
			grams := make(map[string]bool, 1)
			if n == 1 {
				grams[normalized] = true
			}
			return grams
		}
		grams := make(map[string]bool, n-1)
		for i := 0; i < n-1; i++ {
			grams[normalized[i:i+2]] = true
		}
		return grams
	}

	// Unicode (マルチバイト) 文字列の場合：各ルーンのバイト開始位置を収集
	var buf [64]int
	var offsets []int
	if len(normalized)+1 <= len(buf) {
		offsets = buf[:0]
	} else {
		offsets = make([]int, 0, len(normalized)+1)
	}

	for i := range normalized {
		offsets = append(offsets, i)
	}
	offsets = append(offsets, len(normalized))

	runeCount := len(offsets) - 1
	if runeCount < 2 {
		grams := make(map[string]bool, 1)
		if runeCount == 1 {
			grams[normalized] = true
		}
		return grams
	}

	grams := make(map[string]bool, runeCount-1)
	for i := 0; i < runeCount-1; i++ {
		grams[normalized[offsets[i]:offsets[i+2]]] = true
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
