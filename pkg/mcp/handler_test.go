package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/gennobou/gnb-memorymcp/pkg/domain"
)

type dummyStore struct{}

func (d *dummyStore) Create(ctx context.Context, m interface{}) error { return nil }

func TestHandler_StatelessDirectCalls(t *testing.T) {
	// sqlite.Store 依存を直接呼ばない簡単なハンドラー動作テスト
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

type mockMemoryStore struct {
	countFunc         func(ctx context.Context) (int, error)
	getSettingFunc    func(ctx context.Context, key string) (string, error)
	getCandidatesFunc func(ctx context.Context, limit, offset int) ([]*domain.CleanupGroup, error)
}

func (m *mockMemoryStore) Create(ctx context.Context, mem *domain.Memory) error { return nil }
func (m *mockMemoryStore) Get(ctx context.Context, id string) (*domain.Memory, error)  { return nil, nil }
func (m *mockMemoryStore) Search(ctx context.Context, query string, topK int) ([]*domain.Memory, error) {
	return nil, nil
}
func (m *mockMemoryStore) List(ctx context.Context, filter domain.MemoryFilter, limit int) ([]*domain.Memory, error) {
	return nil, nil
}
func (m *mockMemoryStore) Count(ctx context.Context) (int, error) {
	if m.countFunc != nil {
		return m.countFunc(ctx)
	}
	return 0, nil
}
func (m *mockMemoryStore) ListTags(ctx context.Context) ([]string, error)             { return nil, nil }
func (m *mockMemoryStore) Update(ctx context.Context, mem *domain.Memory) error     { return nil }
func (m *mockMemoryStore) Delete(ctx context.Context, id string) error               { return nil }
func (m *mockMemoryStore) GetSystemSetting(ctx context.Context, key string) (string, error) {
	if m.getSettingFunc != nil {
		return m.getSettingFunc(ctx, key)
	}
	return "", nil
}
func (m *mockMemoryStore) SetSystemSetting(ctx context.Context, key, value string) error {
	return nil
}
func (m *mockMemoryStore) GetCleanupCandidates(ctx context.Context, limit, offset int) ([]*domain.CleanupGroup, error) {
	if m.getCandidatesFunc != nil {
		return m.getCandidatesFunc(ctx, limit, offset)
	}
	return nil, nil
}

func TestHandler_MemoryStatus(t *testing.T) {
	store := &mockMemoryStore{
		countFunc: func(ctx context.Context) (int, error) {
			return 42, nil
		},
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
