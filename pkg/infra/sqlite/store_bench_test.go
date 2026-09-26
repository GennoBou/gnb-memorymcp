package sqlite

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/gennobou/gnb-memorymcp/pkg/domain"
)

func BenchmarkGetCleanupCandidates(b *testing.B) {
	ctx := context.Background()

	store, err := NewStore("file::memory:?cache=shared", "")
	if err != nil {
		b.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// 100 種類のメモリを作成
	for i := 0; i < 100; i++ {
		m := &domain.Memory{
			ID:         fmt.Sprintf("mem_%d", i),
			Content:    fmt.Sprintf("This is sample memory content number %d for benchmarking jaccard similarity and clustering.", i%20),
			SourceTool: "benchmark",
			Importance: 3,
		}
		if err := store.Create(ctx, m); err != nil {
			b.Fatalf("failed to create memory: %v", err)
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := store.GetCleanupCandidates(ctx, 10, 0)
		if err != nil {
			b.Fatalf("GetCleanupCandidates failed: %v", err)
		}
	}
}

func BenchmarkListCount(b *testing.B) {
	ctx := context.Background()

	store, err := NewStore("file::memory:?cache=shared", "")
	if err != nil {
		b.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// 1000 種類のメモリを作成
	for i := 0; i < 1000; i++ {
		m := &domain.Memory{
			ID:         fmt.Sprintf("mem_%d", i),
			Content:    fmt.Sprintf("This is sample memory content number %d for benchmarking full table scan listing versus count query.", i),
			SourceTool: "benchmark",
			Importance: 3,
			Tags:       []string{"benchmark", "test"},
			Metadata:   map[string]interface{}{"index": i, "env": "bench"},
		}
		if err := store.Create(ctx, m); err != nil {
			b.Fatalf("failed to create memory: %v", err)
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		memories, err := store.List(ctx, domain.MemoryFilter{}, 1000)
		if err != nil {
			b.Fatalf("List failed: %v", err)
		}
		_ = len(memories)
	}
}

func BenchmarkCount(b *testing.B) {
	ctx := context.Background()

	store, err := NewStore("file::memory:?cache=shared", "")
	if err != nil {
		b.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// 1000 種類のメモリを作成
	for i := 0; i < 1000; i++ {
		m := &domain.Memory{
			ID:         fmt.Sprintf("mem_%d", i),
			Content:    fmt.Sprintf("This is sample memory content number %d for benchmarking full table scan listing versus count query.", i),
			SourceTool: "benchmark",
			Importance: 3,
			Tags:       []string{"benchmark", "test"},
			Metadata:   map[string]interface{}{"index": i, "env": "bench"},
		}
		if err := store.Create(ctx, m); err != nil {
			b.Fatalf("failed to create memory: %v", err)
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := store.Count(ctx)
		if err != nil {
			b.Fatalf("Count failed: %v", err)
		}
	}
}

// setupBenchmarkStore creates a store with numMemories records for benchmark testing.
func setupBenchmarkStore(b *testing.B, numMemories int) (*Store, func()) {
	b.Helper()
	dir := b.TempDir()
	dbPath := "file:" + filepath.Join(dir, "bench.db")

	store, err := NewStore(dbPath, "")
	if err != nil {
		b.Fatalf("failed to create store: %v", err)
	}

	ctx := context.Background()

	// データの作成（いくつかのカテゴリ・キーワードを散りばめる）
	keywords := []string{"Go", "Python", "Rust", "SQLite", "開発", "テスト", "設計", "実装", "パフォーマンス", "メモリ"}
	tools := []string{"claude", "gpt", "gemini"}

	for i := 0; i < numMemories; i++ {
		kw1 := keywords[i%len(keywords)]
		kw2 := keywords[(i+3)%len(keywords)]
		tool := tools[i%len(tools)]

		m := &domain.Memory{
			ID:         fmt.Sprintf("mem_%05d", i),
			Content:    fmt.Sprintf("This is memory item %d containing %s and %s for testing purposes.", i, kw1, kw2),
			SourceTool: tool,
			Tags:       []string{kw1, tool},
			Metadata:   map[string]interface{}{"index": i, "category": kw1},
			Importance: i % 10,
		}
		if err := store.Create(ctx, m); err != nil {
			store.Close()
			b.Fatalf("failed to create memory %d: %v", i, err)
		}
	}

	cleanup := func() {
		store.Close()
	}

	return store, cleanup
}

func BenchmarkSearch_ShortQuery_1Char(b *testing.B) {
	store, cleanup := setupBenchmarkStore(b, 2000)
	defer cleanup()

	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := store.Search(ctx, "開", 10)
		if err != nil {
			b.Fatalf("Search failed: %v", err)
		}
	}
}

func BenchmarkSearch_ShortQuery_2Char(b *testing.B) {
	store, cleanup := setupBenchmarkStore(b, 2000)
	defer cleanup()

	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := store.Search(ctx, "開発", 10)
		if err != nil {
			b.Fatalf("Search failed: %v", err)
		}
	}
}

func BenchmarkSearch_LongQuery_3Char(b *testing.B) {
	store, cleanup := setupBenchmarkStore(b, 2000)
	defer cleanup()

	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := store.Search(ctx, "SQLite", 10)
		if err != nil {
			b.Fatalf("Search failed: %v", err)
		}
	}
}
