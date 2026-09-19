package sqlite

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/gennobou/gnb-memorymcp/pkg/domain"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	// テストごとの独立性を高めるため、テスト名を付与したインメモリDBを使用
	dbURL := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	store, err := NewStore(dbURL, "")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	t.Cleanup(func() {
		store.Close()
	})
	return store
}

func TestStore_Delete(t *testing.T) {
	store, err := NewStore("file::memory:?cache=shared", "")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	m1 := &domain.Memory{
		ID:         "del_mem_01",
		Content:    "削除テスト用メモリ1",
		SourceTool: "test",
	}
	if err := store.Create(ctx, m1); err != nil {
		t.Fatalf("failed to create test memory: %v", err)
	}

	canceledCtx, cancel := context.WithCancel(ctx)
	cancel()

	tests := []struct {
		name    string
		ctx     context.Context
		id      string
		wantErr error
	}{
		{
			name:    "正常系: 存在するIDの削除",
			ctx:     ctx,
			id:      m1.ID,
			wantErr: nil,
		},
		{
			name:    "異常系: 存在しないIDの削除で ErrMemoryNotFound",
			ctx:     ctx,
			id:      "non_existent_id",
			wantErr: domain.ErrMemoryNotFound,
		},
		{
			name:    "異常系: キャンセル済み Context によるエラー",
			ctx:     canceledCtx,
			id:      "canceled_mem_01",
			wantErr: context.Canceled,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := store.Delete(tt.ctx, tt.id)
			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error matching %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("expected error %v, got %v", tt.wantErr, err)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				// 削除されたことを Get で確認
				_, getErr := store.Get(ctx, tt.id)
				if !errors.Is(getErr, domain.ErrMemoryNotFound) {
					t.Errorf("expected ErrMemoryNotFound on Get after Delete, got %v", getErr)
				}
			}
		})
	}
}

func TestStore_All(t *testing.T) {
	ctx := context.Background()

	t.Run("Create", func(t *testing.T) {
		store := newTestStore(t)

		m1 := &domain.Memory{
			ID:             "mem_01",
			Content:        "Go言語でMemoryMCPを開発しています。",
			SourceTool:     "claude",
			Tags:           []string{"golang", "mcp"},
			Metadata:       map[string]interface{}{"project": "gnb"},
			Importance:     5,
			EmbeddingModel: "text-embedding-3-small",
		}

		err := store.Create(ctx, m1)
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}

		got, err := store.Get(ctx, m1.ID)
		if err != nil {
			t.Fatalf("Get failed after Create: %v", err)
		}
		if got.ID != m1.ID {
			t.Errorf("expected ID %s, got %s", m1.ID, got.ID)
		}
	})

	t.Run("Get", func(t *testing.T) {
		store := newTestStore(t)

		m1 := &domain.Memory{
			ID:             "mem_01",
			Content:        "Go言語でMemoryMCPを開発しています。",
			SourceTool:     "claude",
			Tags:           []string{"golang", "mcp"},
			Metadata:       map[string]interface{}{"project": "gnb"},
			Importance:     5,
			EmbeddingModel: "text-embedding-3-small",
		}

		if err := store.Create(ctx, m1); err != nil {
			t.Fatalf("Create failed: %v", err)
		}

		got, err := store.Get(ctx, m1.ID)
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}

		if got.ID != m1.ID || got.Content != m1.Content || got.SourceTool != m1.SourceTool || got.Importance != m1.Importance || got.EmbeddingModel != m1.EmbeddingModel {
			t.Errorf("Get returned unexpected data: %+v", got)
		}

		if !reflect.DeepEqual(got.Tags, m1.Tags) {
			t.Errorf("Tags mismatch: got %v, want %v", got.Tags, m1.Tags)
		}

		if got.Metadata["project"] != m1.Metadata["project"] {
			t.Errorf("Metadata mismatch: got %v, want %v", got.Metadata, m1.Metadata)
		}

		// タイムスタンプの簡易チェック
		if got.CreatedAt.IsZero() || got.UpdatedAt.IsZero() {
			t.Error("timestamps should not be zero")
		}

		// 存在しないIDの検証
		_, err = store.Get(ctx, "non_existent_id")
		if !errors.Is(err, domain.ErrMemoryNotFound) {
			t.Errorf("expected ErrMemoryNotFound, got %v", err)
		}
	})

	t.Run("Update", func(t *testing.T) {
		store := newTestStore(t)

		m1 := &domain.Memory{
			ID:             "mem_01",
			Content:        "Go言語でMemoryMCPを開発しています。",
			SourceTool:     "claude",
			Tags:           []string{"golang", "mcp"},
			Metadata:       map[string]interface{}{"project": "gnb"},
			Importance:     5,
			EmbeddingModel: "text-embedding-3-small",
		}

		if err := store.Create(ctx, m1); err != nil {
			t.Fatalf("Create failed: %v", err)
		}

		m1.Content = "Go言語でMemoryMCPを絶賛開発中です。"
		m1.Tags = []string{"golang", "mcp", "updated"}
		m1.Importance = 8

		err := store.Update(ctx, m1)
		if err != nil {
			t.Fatalf("Update failed: %v", err)
		}

		updated, err := store.Get(ctx, m1.ID)
		if err != nil {
			t.Fatalf("Get after update failed: %v", err)
		}

		if updated.Content != m1.Content || updated.Importance != m1.Importance {
			t.Errorf("Update did not apply correctly: %+v", updated)
		}

		if !reflect.DeepEqual(updated.Tags, m1.Tags) {
			t.Errorf("Updated tags mismatch: got %v, want %v", updated.Tags, m1.Tags)
		}
	})

	t.Run("OptimisticLocking", func(t *testing.T) {
		store := newTestStore(t)

		m1 := &domain.Memory{
			ID:         "mem_01",
			Content:    "Go言語でMemoryMCPを開発しています。",
			SourceTool: "claude",
			Importance: 5,
		}

		if err := store.Create(ctx, m1); err != nil {
			t.Fatalf("Create failed: %v", err)
		}

		staleMemory := &domain.Memory{
			ID:         m1.ID,
			Content:    "古いデータです",
			SourceTool: m1.SourceTool,
			Importance: 1,
			Version:    m1.Version - 1,
		}
		errConflict := store.Update(ctx, staleMemory)
		if !errors.Is(errConflict, domain.ErrConflict) {
			t.Errorf("Expected ErrConflict, got %v", errConflict)
		}
	})

	t.Run("Search", func(t *testing.T) {
		store := newTestStore(t)

		m1 := &domain.Memory{
			ID:             "mem_01",
			Content:        "Go言語でMemoryMCPを開発しています。",
			SourceTool:     "claude",
			Tags:           []string{"golang", "mcp"},
			Importance:     5,
			EmbeddingModel: "text-embedding-3-small",
		}

		m2 := &domain.Memory{
			ID:         "mem_02",
			Content:    "Pythonは機械学習でよく使われます。",
			SourceTool: "gpt",
			Tags:       []string{"python", "ml"},
			Importance: 3,
		}

		if err := store.Create(ctx, m1); err != nil {
			t.Fatalf("Create m1 failed: %v", err)
		}
		if err := store.Create(ctx, m2); err != nil {
			t.Fatalf("Create m2 failed: %v", err)
		}

		// 「Go言語」で検索して m1 がヒットするか
		results, err := store.Search(ctx, "Go言語", 5)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		if len(results) != 1 {
			t.Errorf("Search query 'Go言語' expected 1 result, got %d", len(results))
		} else if results[0].ID != m1.ID {
			t.Errorf("Search query 'Go言語' expected result ID %s, got %s", m1.ID, results[0].ID)
		}

		// 「Python」で検索して m2 がヒットするか
		results, err = store.Search(ctx, "Python", 5)
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		if len(results) != 1 {
			t.Errorf("Search query 'Python' expected 1 result, got %d", len(results))
		} else if results[0].ID != m2.ID {
			t.Errorf("Search query 'Python' expected result ID %s, got %s", m2.ID, results[0].ID)
		}

		// 日本語・英数字混在かつスペース無し ("SQLiteを利用した" を想定)
		mMix := &domain.Memory{
			ID:         "mem_mix",
			Content:    "SQLiteを利用したメモリサーバー開発中",
			SourceTool: "gpt",
			Tags:       []string{"sqlite"},
			Importance: 4,
		}
		if err := store.Create(ctx, mMix); err != nil {
			t.Fatalf("Create mMix failed: %v", err)
		}

		// trigram によるFTS5検索 ("SQLite" 6文字)
		results, err = store.Search(ctx, "SQLite", 5)
		if err != nil {
			t.Fatalf("Search 'SQLite' failed: %v", err)
		}
		foundMix := false
		for _, r := range results {
			if r.ID == mMix.ID {
				foundMix = true
				break
			}
		}
		if !foundMix {
			t.Errorf("Search query 'SQLite' failed to find mMix (content: '%s')", mMix.Content)
		}

		// 2文字以下の日本語クエリ (LIKEフォールバック "開発" 2文字)
		results, err = store.Search(ctx, "開発", 5)
		if err != nil {
			t.Fatalf("Search '開発' failed: %v", err)
		}
		if len(results) < 2 {
			t.Errorf("Search query '開発' (LIKE fallback) expected at least 2 results, got %d", len(results))
		}
		foundM1 := false
		foundMix = false
		for _, r := range results {
			if r.ID == m1.ID {
				foundM1 = true
			}
			if r.ID == mMix.ID {
				foundMix = true
			}
		}
		if !foundM1 || !foundMix {
			t.Errorf("Search query '開発' failed to find expected memories: foundM1=%t, foundMix=%t", foundM1, foundMix)
		}

		// LIKEエスケープ文字を含むクエリ
		mEsc := &domain.Memory{
			ID:         "mem_esc",
			Content:    "進捗は 50% 完了しました。_tmpフォルダ参照",
			SourceTool: "gpt",
			Tags:       []string{"test"},
			Importance: 2,
		}
		if err := store.Create(ctx, mEsc); err != nil {
			t.Fatalf("Create mEsc failed: %v", err)
		}

		results, err = store.Search(ctx, "50%", 5)
		if err != nil {
			t.Fatalf("Search '50%%' failed: %v", err)
		}
		if len(results) != 1 || results[0].ID != mEsc.ID {
			t.Errorf("Search '50%%' expected mem_esc, got %d results", len(results))
		}
	})

	t.Run("List", func(t *testing.T) {
		store := newTestStore(t)

		m1 := &domain.Memory{
			ID:         "mem_01",
			Content:    "Go言語でMemoryMCPを開発しています。",
			SourceTool: "claude",
			Tags:       []string{"golang", "mcp"},
			Importance: 5,
		}
		m2 := &domain.Memory{
			ID:         "mem_02",
			Content:    "Pythonは機械学習でよく使われます。",
			SourceTool: "gpt",
			Tags:       []string{"python", "ml"},
			Importance: 3,
		}

		if err := store.Create(ctx, m1); err != nil {
			t.Fatalf("Create m1 failed: %v", err)
		}
		if err := store.Create(ctx, m2); err != nil {
			t.Fatalf("Create m2 failed: %v", err)
		}

		// source_tool での絞り込み
		listGot, err := store.List(ctx, domain.MemoryFilter{SourceTool: "claude"}, 10)
		if err != nil {
			t.Fatalf("List with SourceTool filter failed: %v", err)
		}
		if len(listGot) != 1 || listGot[0].ID != m1.ID {
			t.Errorf("List filtering by SourceTool failed: %+v", listGot)
		}

		// tag での絞り込み (json_eachによる配列内タグのEXISTS判定テスト)
		listGot, err = store.List(ctx, domain.MemoryFilter{Tag: "ml"}, 10)
		if err != nil {
			t.Fatalf("List with Tag filter failed: %v", err)
		}
		if len(listGot) != 1 || listGot[0].ID != m2.ID {
			t.Errorf("List filtering by Tag failed: %+v", listGot)
		}

		// Offset / ページネーションのテスト
		m3 := &domain.Memory{
			ID:         "mem_03",
			Content:    "GoとPythonの両方を使います。",
			SourceTool: "claude",
			Tags:       []string{"golang", "python"},
			Importance: 4,
		}
		if err := store.Create(ctx, m3); err != nil {
			t.Fatalf("Create m3 failed: %v", err)
		}

		// limit=1, offset=0
		listOffset0, err := store.List(ctx, domain.MemoryFilter{}, 1)
		if err != nil {
			t.Fatalf("List limit 1 failed: %v", err)
		}
		if len(listOffset0) != 1 || listOffset0[0].ID != m3.ID {
			t.Errorf("List page 1 expected m3, got: %+v", listOffset0)
		}

		// limit=1, offset=1
		listOffset1, err := store.List(ctx, domain.MemoryFilter{Offset: 1}, 1)
		if err != nil {
			t.Fatalf("List limit 1 offset 1 failed: %v", err)
		}
		if len(listOffset1) != 1 || listOffset1[0].ID != m2.ID {
			t.Errorf("List page 2 expected m2, got: %+v", listOffset1)
		}
	})

	t.Run("ListTags", func(t *testing.T) {
		store := newTestStore(t)

		m1 := &domain.Memory{
			ID:         "mem_01",
			Content:    "Go言語でMemoryMCPを開発しています。",
			SourceTool: "claude",
			Tags:       []string{"golang", "mcp", "updated"},
			Importance: 5,
		}
		m2 := &domain.Memory{
			ID:         "mem_02",
			Content:    "Pythonは機械学習でよく使われます。",
			SourceTool: "gpt",
			Tags:       []string{"python", "ml"},
			Importance: 3,
		}

		if err := store.Create(ctx, m1); err != nil {
			t.Fatalf("Create m1 failed: %v", err)
		}
		if err := store.Create(ctx, m2); err != nil {
			t.Fatalf("Create m2 failed: %v", err)
		}

		tagsGot, err := store.ListTags(ctx)
		if err != nil {
			t.Fatalf("ListTags failed: %v", err)
		}
		expectedTags := []string{"golang", "mcp", "ml", "python", "updated"}
		if !reflect.DeepEqual(tagsGot, expectedTags) {
			t.Errorf("ListTags mismatch: got %v, want %v", tagsGot, expectedTags)
		}
	})

	t.Run("Delete", func(t *testing.T) {
		store := newTestStore(t)

		m1 := &domain.Memory{
			ID:         "mem_01",
			Content:    "Go言語でMemoryMCPを開発しています。",
			SourceTool: "claude",
			Tags:       []string{"golang", "mcp"},
			Importance: 5,
		}

		if err := store.Create(ctx, m1); err != nil {
			t.Fatalf("Create m1 failed: %v", err)
		}

		err := store.Delete(ctx, m1.ID)
		if err != nil {
			t.Fatalf("Delete failed: %v", err)
		}

		_, err = store.Get(ctx, m1.ID)
		if err != domain.ErrMemoryNotFound {
			t.Errorf("Get after delete expected ErrMemoryNotFound, got %v", err)
		}

		// 削除後のSearch結果に m1 が含まれないことの確認
		results, err := store.Search(ctx, "Go言語", 5)
		if err != nil {
			t.Fatalf("Search after delete failed: %v", err)
		}
		if len(results) != 0 {
			t.Errorf("Search after delete expected 0 results, got %d", len(results))
		}
	})

	t.Run("SystemSettings", func(t *testing.T) {
		store := newTestStore(t)

		err := store.SetSystemSetting(ctx, "test_key", "test_val")
		if err != nil {
			t.Fatalf("SetSystemSetting failed: %v", err)
		}
		valGot, err := store.GetSystemSetting(ctx, "test_key")
		if err != nil {
			t.Fatalf("GetSystemSetting failed: %v", err)
		}
		if valGot != "test_val" {
			t.Errorf("GetSystemSetting expected 'test_val', got '%s'", valGot)
		}
	})

	t.Run("GetCleanupCandidates", func(t *testing.T) {
		store := newTestStore(t)

		m2 := &domain.Memory{
			ID:         "mem_02",
			Content:    "Pythonは機械学習でよく使われます。",
			SourceTool: "gpt",
			Tags:       []string{"python", "ml"},
			Importance: 3,
		}
		m4 := &domain.Memory{
			ID:         "mem_04",
			Content:    "Pythonは機械学習でとてもよく使われています。",
			SourceTool: "gpt",
			Tags:       []string{"python", "ml"},
			Importance: 4,
		}

		if err := store.Create(ctx, m2); err != nil {
			t.Fatalf("Create m2 failed: %v", err)
		}
		if err := store.Create(ctx, m4); err != nil {
			t.Fatalf("Create m4 failed: %v", err)
		}

		candidates, err := store.GetCleanupCandidates(ctx, 10, 0)
		if err != nil {
			t.Fatalf("GetCleanupCandidates failed: %v", err)
		}

		if len(candidates) != 1 {
			t.Errorf("GetCleanupCandidates expected 1 group, got %d", len(candidates))
		} else {
			group := candidates[0]
			if len(group.Memories) != 2 {
				t.Errorf("Cleanup group expected 2 memories, got %d", len(group.Memories))
			}
			id1 := group.Memories[0].ID
			id2 := group.Memories[1].ID
			if !((id1 == "mem_02" && id2 == "mem_04") || (id1 == "mem_04" && id2 == "mem_02")) {
				t.Errorf("Expected memories mem_02 and mem_04 in group, got %s and %s", id1, id2)
			}
		}
	})
}

func TestStore_Update(t *testing.T) {
	ctx := context.Background()

	store, err := NewStore("file::memory:?cache=shared", "")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// テスト用の初期記憶を作成
	initMem := &domain.Memory{
		ID:             "mem_update_test",
		Content:        "Initial Content",
		SourceTool:     "tool_a",
		Tags:           []string{"tag1"},
		Metadata:       map[string]interface{}{"key": "val"},
		Importance:     3,
		EmbeddingModel: "model_a",
	}
	if err := store.Create(ctx, initMem); err != nil {
		t.Fatalf("Create initMem failed: %v", err)
	}

	fetched, err := store.Get(ctx, initMem.ID)
	if err != nil {
		t.Fatalf("Get initMem failed: %v", err)
	}

	t.Run("Success Path", func(t *testing.T) {
		m := &domain.Memory{
			ID:             fetched.ID,
			Content:        "Updated Content",
			SourceTool:     "tool_b",
			Tags:           []string{"tag1", "tag2"},
			Metadata:       map[string]interface{}{"key": "val2"},
			Importance:     5,
			EmbeddingModel: "model_b",
			Version:        fetched.Version,
		}

		err := store.Update(ctx, m)
		if err != nil {
			t.Fatalf("expected update to succeed, got %v", err)
		}

		if m.Version != fetched.Version+1 {
			t.Errorf("expected version to be %d, got %d", fetched.Version+1, m.Version)
		}

		updated, err := store.Get(ctx, m.ID)
		if err != nil {
			t.Fatalf("Get updated memory failed: %v", err)
		}
		if updated.Content != "Updated Content" || updated.Version != m.Version {
			t.Errorf("Get returned unexpected content or version: %+v", updated)
		}
	})

	t.Run("Zero Rows Affected - Non-existent ID", func(t *testing.T) {
		m := &domain.Memory{
			ID:      "non_existent_id",
			Content: "Content",
			Version: 1,
		}
		err := store.Update(ctx, m)
		if !errors.Is(err, domain.ErrConflict) {
			t.Errorf("expected domain.ErrConflict, got %v", err)
		}
	})

	t.Run("Zero Rows Affected - Stale Version Conflict", func(t *testing.T) {
		latest, err := store.Get(ctx, initMem.ID)
		if err != nil {
			t.Fatalf("Get latest memory failed: %v", err)
		}

		staleMem := &domain.Memory{
			ID:      latest.ID,
			Content: "Stale Content",
			Version: latest.Version - 1, // 古いバージョン
		}
		err = store.Update(ctx, staleMem)
		if !errors.Is(err, domain.ErrConflict) {
			t.Errorf("expected domain.ErrConflict for stale version, got %v", err)
		}
	})

	t.Run("JSON Marshal Error - Invalid Tags", func(t *testing.T) {
		latest, err := store.Get(ctx, initMem.ID)
		if err != nil {
			t.Fatalf("Get latest memory failed: %v", err)
		}

		// json.Marshal は unsupported type のスライス（または interface{} 内の channel等）でエラーとなる
		// 期待される型が []string のため、リフレクションや Unsafe ではなく、タグ配列のマーシャル失敗をテストするために
		// Metadata にマーシャル不可能なチャネルを設定するケースと別にタグ単体のエラーも確認
		m := &domain.Memory{
			ID:      latest.ID,
			Content: "Content",
			Version: latest.Version,
			Tags:    []string{"valid"},
			Metadata: map[string]interface{}{
				"invalid": make(chan int),
			},
		}

		err = store.Update(ctx, m)
		if err == nil {
			t.Error("expected error due to unmarshalable metadata, got nil")
		}
	})

	t.Run("Context Canceled Error", func(t *testing.T) {
		latest, err := store.Get(ctx, initMem.ID)
		if err != nil {
			t.Fatalf("Get latest memory failed: %v", err)
		}

		cancelCtx, cancel := context.WithCancel(ctx)
		cancel() // 直ちにキャンセル

		m := &domain.Memory{
			ID:      latest.ID,
			Content: "Content",
			Version: latest.Version,
		}

		err = store.Update(cancelCtx, m)
		if err == nil {
			t.Error("expected error with canceled context, got nil")
		}
	})
}

func TestStore_ExplainQueryPlan(t *testing.T) {
	// インメモリデータベースでStoreを作成
	store, err := NewStore("file::memory:?cache=shared", "")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// 検証対象のクエリリスト
	queries := []struct {
		name  string
		query string
	}{
		{
			name: "Search Query (FTS5 Match)",
			query: `
				SELECT m.id, m.content, m.source_tool, m.tags, m.metadata, m.importance, m.embedding_model, m.created_at, m.updated_at, m.last_accessed
				FROM memories m
				JOIN memories_fts f ON m.rowid = f.rowid
				WHERE memories_fts MATCH 'test'
				ORDER BY rank
				LIMIT 5
			`,
		},
		{
			name: "List Query (SourceTool only)",
			query: `
				SELECT id, content, source_tool, tags, metadata, importance, embedding_model, created_at, updated_at, last_accessed
				FROM memories
				WHERE source_tool = 'claude'
				ORDER BY created_at DESC
				LIMIT 10
			`,
		},
		{
			name: "List Query (Tag filter using json_each)",
			query: `
				SELECT id, content, source_tool, tags, metadata, importance, embedding_model, created_at, updated_at, last_accessed
				FROM memories
				WHERE EXISTS (SELECT 1 FROM json_each(tags) WHERE value = 'golang')
				ORDER BY created_at DESC
				LIMIT 10
			`,
		},
		{
			name: "List Query (SourceTool + Tag filter)",
			query: `
				SELECT id, content, source_tool, tags, metadata, importance, embedding_model, created_at, updated_at, last_accessed
				FROM memories
				WHERE source_tool = 'claude' AND EXISTS (SELECT 1 FROM json_each(tags) WHERE value = 'golang')
				ORDER BY created_at DESC
				LIMIT 10
			`,
		},
	}

	t.Log("--- EXPLAIN QUERY PLAN Results ---")
	for _, tc := range queries {
		t.Run(tc.name, func(t *testing.T) {
			explainQuery := "EXPLAIN QUERY PLAN " + tc.query
			rows, err := store.db.Query(explainQuery)
			if err != nil {
				t.Fatalf("failed to explain query: %v", err)
			}
			defer rows.Close()

			t.Logf("Query: %s", tc.name)
			cols, _ := rows.Columns()

			// 汎用的にスキャンするためのスライス
			vals := make([]interface{}, len(cols))
			valPtrs := make([]interface{}, len(cols))
			for i := range vals {
				valPtrs[i] = &vals[i]
			}

			for rows.Next() {
				if err := rows.Scan(valPtrs...); err != nil {
					t.Fatalf("scan failed: %v", err)
				}

				// 各値を文字列化して表示
				rowStr := ""
				for i, val := range vals {
					valStr := ""
					switch v := val.(type) {
					case []byte:
						valStr = string(v)
					default:
						valStr = fmt.Sprintf("%v", v)
					}
					rowStr += fmt.Sprintf("%s: %s | ", cols[i], valStr)
				}
				t.Log(rowStr)
			}
		})
	}
}
