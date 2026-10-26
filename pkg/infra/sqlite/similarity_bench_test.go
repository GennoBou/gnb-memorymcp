package sqlite

import (
	"testing"
)

func BenchmarkCharBiGrams_ASCII(b *testing.B) {
	s := "This is a sample string for benchmarking charBiGrams optimization."
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = charBiGrams(s)
	}
}

func BenchmarkCharBiGrams_Japanese(b *testing.B) {
	s := "Pythonは機械学習でとてもよく使われているプログラミング言語です。"
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = charBiGrams(s)
	}
}

func BenchmarkJaccardSimilarity(b *testing.B) {
	s1 := "Pythonは機械学習でよく使われます。"
	s2 := "Pythonは機械学習でとてもよく使われています。"
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = jaccardSimilarity(s1, s2)
	}
}
