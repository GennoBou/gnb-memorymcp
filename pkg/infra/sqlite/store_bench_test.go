package sqlite

import (
	"context"
	"fmt"
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
