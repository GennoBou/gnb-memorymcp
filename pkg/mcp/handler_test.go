package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/gennobou/gnb-memorymcp/pkg/domain"
	"github.com/google/uuid"
)

type mockStore struct {
	createdMemory *domain.Memory
}

func (m *mockStore) Create(ctx context.Context, mem *domain.Memory) error {
	m.createdMemory = mem
	return nil
}
func (m *mockStore) Get(ctx context.Context, id string) (*domain.Memory, error) { return nil, nil }
func (m *mockStore) Search(ctx context.Context, query string, topK int) ([]*domain.Memory, error) {
	return nil, nil
}
func (m *mockStore) List(ctx context.Context, filter domain.MemoryFilter, limit int) ([]*domain.Memory, error) {
	return nil, nil
}
func (m *mockStore) ListTags(ctx context.Context) ([]string, error) { return nil, nil }
func (m *mockStore) Update(ctx context.Context, mem *domain.Memory) error { return nil }
func (m *mockStore) Delete(ctx context.Context, id string) error   { return nil }
func (m *mockStore) GetSystemSetting(ctx context.Context, key string) (string, error) {
	return "", nil
}
func (m *mockStore) SetSystemSetting(ctx context.Context, key, value string) error { return nil }
func (m *mockStore) GetCleanupCandidates(ctx context.Context, limit, offset int) ([]*domain.CleanupGroup, error) {
	return nil, nil
}

func TestHandler_MemoryCreate_UUID(t *testing.T) {
	store := &mockStore{}
	h := NewHandler(store)
	ctx := context.Background()

	createReq := &Request{
		JSONRPC: "2.0",
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"memory_create","arguments":{"content":"test memory","source_tool":"test"}}`),
		ID:      1,
	}

	resp := h.Handle(ctx, createReq)
	if resp == nil {
		t.Fatal("expected response for tools/call memory_create, got nil")
	}

	if store.createdMemory == nil {
		t.Fatal("expected memory to be created in store, got nil")
	}

	_, err := uuid.Parse(store.createdMemory.ID)
	if err != nil {
		t.Fatalf("expected created memory ID to be a valid UUID, got %s: %v", store.createdMemory.ID, err)
	}
}

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
