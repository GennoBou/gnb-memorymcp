package sqlite

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/gennobou/gnb-memorymcp/pkg/domain"
)

func TestStore_All(t *testing.T) {
	ctx := context.Background()

	// インメモリデータベースでStoreを作成
	store, err := NewStore("file::memory:?cache=shared", "")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// 1. Create のテスト
	m1 := &domain.Memory{
		ID:             "mem_01",
		Content:        "Go言語でMemoryMCPを開発しています。",
		SourceTool:     "claude",
		Tags:           []string{"golang", "mcp"},
		Metadata:       map[string]interface{}{"project": "gnb"},
		Importance:     5,
		EmbeddingModel: "text-embedding-3-small",
	}

	err = store.Create(ctx, m1)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// 2. Get のテスト
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

	// 3. Update のテスト
	m1.Content = "Go言語でMemoryMCPを絶賛開発中です。"
	m1.Tags = []string{"golang", "mcp", "updated"}
	m1.Importance = 8

	err = store.Update(ctx, m1)
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

	// 3-2. 楽観的ロック（競合検知）のテスト
	staleMemory := &domain.Memory{
		ID:         m1.ID,
		Content:    "古いデータです",
		SourceTool: m1.SourceTool,
		Importance: 1,
		Version:    updated.Version - 1,
	}
	errConflict := store.Update(ctx, staleMemory)
	if !errors.Is(errConflict, domain.ErrConflict) {
		t.Errorf("Expected ErrConflict, got %v", errConflict)
	}

	// 4. Search (FTS5) のテスト
	// FTS5の同期トリガーが効いているか確認するため、別のデータを追加
	m2 := &domain.Memory{
		ID:         "mem_02",
		Content:    "Pythonは機械学習でよく使われます。",
		SourceTool: "gpt",
		Tags:       []string{"python", "ml"},
		Metadata:   map[string]interface{}{"author": "user"},
		Importance: 3,
	}
	err = store.Create(ctx, m2)
	if err != nil {
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

	// 追加テスト1: 日本語・英数字混在かつスペース無し ("SQLiteを利用した" を想定)
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

	// 追加テスト2: 2文字以下の日本語クエリ (LIKEフォールバック "開発" 2文字)
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

	// 追加テスト3: LIKEエスケープ文字を含むクエリ
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

	// 追加したテストデータを削除して後続のListテストへ影響を与えないようにする
	if err := store.Delete(ctx, mMix.ID); err != nil {
		t.Fatalf("Cleanup mMix failed: %v", err)
	}
	if err := store.Delete(ctx, mEsc.ID); err != nil {
		t.Fatalf("Cleanup mEsc failed: %v", err)
	}

	// 5. List のテスト
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

	// 5-2. Offset / ページネーションのテスト
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

	// 5-3. ListTags のテスト
	tagsGot, err := store.ListTags(ctx)
	if err != nil {
		t.Fatalf("ListTags failed: %v", err)
	}
	expectedTags := []string{"golang", "mcp", "ml", "python", "updated"}
	if !reflect.DeepEqual(tagsGot, expectedTags) {
		t.Errorf("ListTags mismatch: got %v, want %v", tagsGot, expectedTags)
	}

	// 6. Delete のテスト
	err = store.Delete(ctx, m1.ID)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = store.Get(ctx, m1.ID)
	if err != domain.ErrMemoryNotFound {
		t.Errorf("Get after delete expected ErrMemoryNotFound, got %v", err)
	}

	// 削除後のSearch結果に m1 が含まれないことの確認
	results, err = store.Search(ctx, "Go言語", 5)
	if err != nil {
		t.Fatalf("Search after delete failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Search after delete expected 0 results, got %d", len(results))
	}

	// 7. system_settings のテスト
	err = store.SetSystemSetting(ctx, "test_key", "test_val")
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

	// 8. 類似度クラスタリングのテスト
	m4 := &domain.Memory{
		ID:         "mem_04",
		Content:    "Pythonは機械学習でとてもよく使われています。",
		SourceTool: "gpt",
		Tags:       []string{"python", "ml"},
		Importance: 4,
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

