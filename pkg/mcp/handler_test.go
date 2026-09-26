package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/gennobou/gnb-memorymcp/pkg/domain"
)

type mockMemoryStore struct {
	memories      map[string]*domain.Memory
	settings      map[string]string
	cleanupGroup  []*domain.CleanupGroup
	createErr     error
	getErr        error
	searchErr     error
	listErr       error
	countErr      error
	countFunc     func(ctx context.Context) (int, error)
	listTagsErr   error
	updateErr     error
	deleteErr     error
	getSettingErr error
	setSettingErr error
	getCleanupErr error
}

func newMockMemoryStore() *mockMemoryStore {
	return &mockMemoryStore{
		memories: make(map[string]*domain.Memory),
		settings: make(map[string]string),
	}
}

func (m *mockMemoryStore) Create(ctx context.Context, mem *domain.Memory) error {
	if m.createErr != nil {
		return m.createErr
	}
	mem.CreatedAt = time.Now().UTC()
	mem.UpdatedAt = time.Now().UTC()
	m.memories[mem.ID] = mem
	return nil
}

func (m *mockMemoryStore) Get(ctx context.Context, id string) (*domain.Memory, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	mem, ok := m.memories[id]
	if !ok {
		return nil, domain.ErrMemoryNotFound
	}
	return mem, nil
}

func (m *mockMemoryStore) Search(ctx context.Context, query string, topK int) ([]*domain.Memory, error) {
	if m.searchErr != nil {
		return nil, m.searchErr
	}
	var res []*domain.Memory
	for _, mem := range m.memories {
		if strings.Contains(mem.Content, query) {
			res = append(res, mem)
			if len(res) >= topK {
				break
			}
		}
	}
	return res, nil
}

func (m *mockMemoryStore) List(ctx context.Context, filter domain.MemoryFilter, limit int) ([]*domain.Memory, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	var res []*domain.Memory
	for _, mem := range m.memories {
		if filter.SourceTool != "" && mem.SourceTool != filter.SourceTool {
			continue
		}
		res = append(res, mem)
		if len(res) >= limit {
			break
		}
	}
	return res, nil
}

func (m *mockMemoryStore) Count(ctx context.Context) (int, error) {
	if m.countFunc != nil {
		return m.countFunc(ctx)
	}
	if m.countErr != nil {
		return 0, m.countErr
	}
	return len(m.memories), nil
}

func (m *mockMemoryStore) ListTags(ctx context.Context) ([]string, error) {
	if m.listTagsErr != nil {
		return nil, m.listTagsErr
	}
	tagSet := make(map[string]bool)
	for _, mem := range m.memories {
		for _, t := range mem.Tags {
			tagSet[t] = true
		}
	}
	var tags []string
	for t := range tagSet {
		tags = append(tags, t)
	}
	return tags, nil
}

func (m *mockMemoryStore) Update(ctx context.Context, mem *domain.Memory) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	if _, ok := m.memories[mem.ID]; !ok {
		return domain.ErrMemoryNotFound
	}
	mem.UpdatedAt = time.Now().UTC()
	m.memories[mem.ID] = mem
	return nil
}

func (m *mockMemoryStore) Delete(ctx context.Context, id string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	if _, ok := m.memories[id]; !ok {
		return domain.ErrMemoryNotFound
	}
	delete(m.memories, id)
	return nil
}

func (m *mockMemoryStore) GetSystemSetting(ctx context.Context, key string) (string, error) {
	if m.getSettingErr != nil {
		return "", m.getSettingErr
	}
	return m.settings[key], nil
}

func (m *mockMemoryStore) SetSystemSetting(ctx context.Context, key, value string) error {
	if m.setSettingErr != nil {
		return m.setSettingErr
	}
	m.settings[key] = value
	return nil
}

func (m *mockMemoryStore) GetCleanupCandidates(ctx context.Context, limit, offset int) ([]*domain.CleanupGroup, error) {
	if m.getCleanupErr != nil {
		return nil, m.getCleanupErr
	}
	if offset >= len(m.cleanupGroup) {
		return nil, nil
	}
	end := offset + limit
	if end > len(m.cleanupGroup) {
		end = len(m.cleanupGroup)
	}
	return m.cleanupGroup[offset:end], nil
}

func TestHandler_StatelessDirectCalls(t *testing.T) {
	h := NewHandler(nil)
	ctx := context.Background()

	// 1. initialize の検証
	initReq := &Request{
		JSONRPC: "2.0",
		Method:  "initialize",
		ID:      1,
	}
	initResp := h.Handle(ctx, initReq)
	if initResp == nil {
		t.Fatal("expected response for initialize, got nil")
	}
	resultMap, ok := initResp.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %T", initResp.Result)
	}
	if resultMap["protocolVersion"] != ProtocolVersion {
		t.Errorf("expected protocolVersion %s, got %v", ProtocolVersion, resultMap["protocolVersion"])
	}

	// 2. initialize を経由しない直接の tools/list 呼び出し (Stateless)
	listReq := &Request{
		JSONRPC: "2.0",
		Method:  "tools/list",
		ID:      2,
		Meta:    json.RawMessage(`{"client":"GeminiSpark"}`),
	}
	listResp := h.Handle(ctx, listReq)
	if listResp == nil {
		t.Fatal("expected response for tools/list without initialize, got nil")
	}
	toolsResult, ok := listResp.Result.(ToolsListResult)
	if !ok {
		t.Fatalf("expected ToolsListResult, got %T", listResp.Result)
	}
	if len(toolsResult.Tools) == 0 {
		t.Errorf("expected non-empty tools list")
	}
}

func TestHandler_MemoryStatus(t *testing.T) {
	store := newMockMemoryStore()
	store.countFunc = func(ctx context.Context) (int, error) {
		return 42, nil
	}
	h := NewHandler(store)
	ctx := context.Background()

	callReq := &Request{
		JSONRPC: "2.0",
		Method:  "tools/call",
		ID:      1,
		Params:  json.RawMessage(`{"name":"memory_status","arguments":{}}`),
	}
	resp := h.Handle(ctx, callReq)
	if resp == nil {
		t.Fatal("expected response for tools/call memory_status, got nil")
	}

	res, ok := resp.Result.(*CallToolResult)
	if !ok {
		t.Fatalf("expected *CallToolResult, got %T", resp.Result)
	}
	if len(res.Content) == 0 {
		t.Fatal("expected non-empty content")
	}
	if !strings.Contains(res.Content[0].Text, "Total memories: 42") {
		t.Errorf("expected text to contain 'Total memories: 42', got %q", res.Content[0].Text)
	}
}

func TestToolHandlers(t *testing.T) {
	ctx := context.Background()

	t.Run("memory_create", func(t *testing.T) {
		store := newMockMemoryStore()
		h := NewHandler(store)

		// Invalid JSON
		_, err := h.callTool(ctx, "memory_create", json.RawMessage(`invalid json`))
		if err == nil {
			t.Error("expected error for invalid JSON")
		}

		// Missing content
		_, err = h.callTool(ctx, "memory_create", json.RawMessage(`{"source_tool":"chat"}`))
		if err == nil || err.Error() != "content is required" {
			t.Errorf("unexpected error: %v", err)
		}

		// Content too long
		longContent := strings.Repeat("a", 10001)
		_, err = h.callTool(ctx, "memory_create", json.RawMessage(`{"content":"`+longContent+`","source_tool":"chat"}`))
		if err == nil || !strings.Contains(err.Error(), "exceeds maximum length") {
			t.Errorf("unexpected error for long content: %v", err)
		}

		// Missing source_tool
		_, err = h.callTool(ctx, "memory_create", json.RawMessage(`{"content":"hello"}`))
		if err == nil || err.Error() != "source_tool is required" {
			t.Errorf("unexpected error: %v", err)
		}

		// Invalid tags count
		_, err = h.callTool(ctx, "memory_create", json.RawMessage(`{"content":"hello","source_tool":"chat","tags":["1","2","3","4","5","6","7","8","9","10","11"]}`))
		if err == nil || !strings.Contains(err.Error(), "tags exceed maximum count") {
			t.Errorf("unexpected error: %v", err)
		}

		// Invalid importance
		_, err = h.callTool(ctx, "memory_create", json.RawMessage(`{"content":"hello","source_tool":"chat","importance":11}`))
		if err == nil || !strings.Contains(err.Error(), "importance must be between 0 and 10") {
			t.Errorf("unexpected error: %v", err)
		}

		// Valid creation
		res, err := h.callTool(ctx, "memory_create", json.RawMessage(`{"content":"test memory","source_tool":"chat","tags":["go"],"importance":5}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(res.Content) == 0 || !strings.Contains(res.Content[0].Text, "Memory created successfully with ID:") {
			t.Errorf("unexpected result: %v", res)
		}
	})

	t.Run("memory_search", func(t *testing.T) {
		store := newMockMemoryStore()
		h := NewHandler(store)

		// Missing query
		_, err := h.callTool(ctx, "memory_search", json.RawMessage(`{}`))
		if err == nil || err.Error() != "query is required" {
			t.Errorf("unexpected error: %v", err)
		}

		// No matches
		res, err := h.callTool(ctx, "memory_search", json.RawMessage(`{"query":"none"}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Content[0].Text != "No memories found matching the query." {
			t.Errorf("unexpected result: %v", res.Content[0].Text)
		}

		// Match found
		_ = store.Create(ctx, &domain.Memory{ID: "m1", Content: "go programming", SourceTool: "chat", Tags: []string{"go"}})
		res, err = h.callTool(ctx, "memory_search", json.RawMessage(`{"query":"go"}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(res.Content[0].Text, "Found 1 memories:") {
			t.Errorf("unexpected result: %v", res.Content[0].Text)
		}
	})

	t.Run("memory_list", func(t *testing.T) {
		store := newMockMemoryStore()
		h := NewHandler(store)

		// Empty list
		res, err := h.callTool(ctx, "memory_list", json.RawMessage(`{}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Content[0].Text != "No memories found." {
			t.Errorf("unexpected result: %v", res.Content[0].Text)
		}

		// Non-empty list
		_ = store.Create(ctx, &domain.Memory{ID: "m1", Content: "memory 1", SourceTool: "chat"})
		res, err = h.callTool(ctx, "memory_list", json.RawMessage(`{"limit":10}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(res.Content[0].Text, "Listing 1 memories:") {
			t.Errorf("unexpected result: %v", res.Content[0].Text)
		}
	})

	t.Run("memory_get", func(t *testing.T) {
		store := newMockMemoryStore()
		h := NewHandler(store)

		// Missing ID
		_, err := h.callTool(ctx, "memory_get", json.RawMessage(`{}`))
		if err == nil || err.Error() != "id is required" {
			t.Errorf("unexpected error: %v", err)
		}

		// Not found
		res, err := h.callTool(ctx, "memory_get", json.RawMessage(`{"id":"non-existent"}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !res.IsError || !strings.Contains(res.Content[0].Text, "not found") {
			t.Errorf("unexpected result: %v", res)
		}

		// Found
		_ = store.Create(ctx, &domain.Memory{ID: "m1", Content: "memory 1", SourceTool: "chat"})
		res, err = h.callTool(ctx, "memory_get", json.RawMessage(`{"id":"m1"}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(res.Content[0].Text, "Memory ID: m1") {
			t.Errorf("unexpected result: %v", res.Content[0].Text)
		}
	})

	t.Run("tags_list", func(t *testing.T) {
		store := newMockMemoryStore()
		h := NewHandler(store)

		// Empty tags
		res, err := h.callTool(ctx, "tags_list", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Content[0].Text != "No tags found." {
			t.Errorf("unexpected result: %v", res.Content[0].Text)
		}

		// With tags
		_ = store.Create(ctx, &domain.Memory{ID: "m1", Content: "memory 1", SourceTool: "chat", Tags: []string{"tag1", "tag2"}})
		res, err = h.callTool(ctx, "tags_list", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(res.Content[0].Text, "Available tags:") {
			t.Errorf("unexpected result: %v", res.Content[0].Text)
		}
	})

	t.Run("memory_update", func(t *testing.T) {
		store := newMockMemoryStore()
		h := NewHandler(store)

		// Missing ID
		_, err := h.callTool(ctx, "memory_update", json.RawMessage(`{}`))
		if err == nil || err.Error() != "id is required" {
			t.Errorf("unexpected error: %v", err)
		}

		// Not found
		newContent := "updated content"
		_, err = h.callTool(ctx, "memory_update", json.RawMessage(`{"id":"non-existent","content":"`+newContent+`"}`))
		if err == nil {
			t.Error("expected error for non-existent memory update")
		}

		// Successful update
		_ = store.Create(ctx, &domain.Memory{ID: "m1", Content: "old content", SourceTool: "chat"})
		res, err := h.callTool(ctx, "memory_update", json.RawMessage(`{"id":"m1","content":"updated content"}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(res.Content[0].Text, "updated successfully") {
			t.Errorf("unexpected result: %v", res.Content[0].Text)
		}
		if store.memories["m1"].Content != "updated content" {
			t.Errorf("content not updated: %s", store.memories["m1"].Content)
		}

		// Update conflict
		store.updateErr = domain.ErrConflict
		_, err = h.callTool(ctx, "memory_update", json.RawMessage(`{"id":"m1","content":"conflict content"}`))
		if err == nil || !strings.Contains(err.Error(), "concurrent update conflict") {
			t.Errorf("unexpected error for conflict: %v", err)
		}
		store.updateErr = nil

		// Invalid content (length > 10000)
		tooLongContent := strings.Repeat("a", 10001)
		_, err = h.callTool(ctx, "memory_update", json.RawMessage(`{"id":"m1","content":"`+tooLongContent+`"}`))
		if err == nil || !strings.Contains(err.Error(), "content exceeds maximum length") {
			t.Errorf("expected error for content exceeding max length, got: %v", err)
		}

		// Invalid tags (count > 10)
		_, err = h.callTool(ctx, "memory_update", json.RawMessage(`{"id":"m1","tags":["1","2","3","4","5","6","7","8","9","10","11"]}`))
		if err == nil || !strings.Contains(err.Error(), "tags exceed maximum count") {
			t.Errorf("expected error for tags exceeding max count, got: %v", err)
		}

		// Invalid importance (< 0 or > 10)
		_, err = h.callTool(ctx, "memory_update", json.RawMessage(`{"id":"m1","importance":11}`))
		if err == nil || !strings.Contains(err.Error(), "importance must be between 0 and 10") {
			t.Errorf("expected error for invalid importance, got: %v", err)
		}

		// Successful update with source_tool, tags, importance, metadata
		newSource := "new_source"
		newImp := 8
		_, err = h.callTool(ctx, "memory_update", json.RawMessage(`{"id":"m1","source_tool":"new_source","tags":["t1","t2"],"importance":8,"metadata":{"key":"value"}}`))
		if err != nil {
			t.Fatalf("unexpected error for full update: %v", err)
		}
		m1 := store.memories["m1"]
		if m1.SourceTool != newSource || len(m1.Tags) != 2 || m1.Importance != newImp || m1.Metadata["key"] != "value" {
			t.Errorf("updated memory fields mismatched: %+v", m1)
		}
	})

	t.Run("memory_delete", func(t *testing.T) {
		store := newMockMemoryStore()
		h := NewHandler(store)

		// Missing ID
		_, err := h.callTool(ctx, "memory_delete", json.RawMessage(`{}`))
		if err == nil || err.Error() != "id is required" {
			t.Errorf("unexpected error: %v", err)
		}

		// Delete success
		_ = store.Create(ctx, &domain.Memory{ID: "m1", Content: "content", SourceTool: "chat"})
		res, err := h.callTool(ctx, "memory_delete", json.RawMessage(`{"id":"m1"}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(res.Content[0].Text, "deleted successfully") {
			t.Errorf("unexpected result: %v", res.Content[0].Text)
		}
	})

	t.Run("memory_status", func(t *testing.T) {
		store := newMockMemoryStore()
		h := NewHandler(store)

		res, err := h.callTool(ctx, "memory_status", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(res.Content[0].Text, "Database Status:") {
			t.Errorf("unexpected result: %v", res.Content[0].Text)
		}
	})

	t.Run("memory_consolidate", func(t *testing.T) {
		store := newMockMemoryStore()
		h := NewHandler(store)

		// No cleanup candidates
		res, err := h.callTool(ctx, "memory_consolidate", json.RawMessage(`{}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Content[0].Text != "No memories require consolidation. The database is clean." {
			t.Errorf("unexpected result: %v", res.Content[0].Text)
		}

		// With candidates
		store.cleanupGroup = []*domain.CleanupGroup{
			{
				GroupID: "g1",
				Memories: []*domain.Memory{
					{ID: "m1", Content: "c1", CreatedAt: time.Now()},
					{ID: "m2", Content: "c2", CreatedAt: time.Now()},
				},
			},
		}
		res, err = h.callTool(ctx, "memory_consolidate", json.RawMessage(`{"limit":3}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(res.Content[0].Text, "Found 1 groups of memories") {
			t.Errorf("unexpected result: %v", res.Content[0].Text)
		}
	})

	t.Run("memory_cleanup_complete", func(t *testing.T) {
		store := newMockMemoryStore()
		h := NewHandler(store)

		res, err := h.callTool(ctx, "memory_cleanup_complete", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(res.Content[0].Text, "Cleanup completed successfully") {
			t.Errorf("unexpected result: %v", res.Content[0].Text)
		}
		if store.settings["last_cleanup_at"] == "" {
			t.Error("last_cleanup_at setting was not set")
		}
	})

	t.Run("unsupported_tool", func(t *testing.T) {
		store := newMockMemoryStore()
		h := NewHandler(store)

		_, err := h.callTool(ctx, "unknown_tool", nil)
		if err == nil || !strings.Contains(err.Error(), "unsupported tool: unknown_tool") {
			t.Errorf("unexpected error for unsupported tool: %v", err)
		}
	})
}
