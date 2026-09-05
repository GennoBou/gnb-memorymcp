package mcp

import (
	"context"
	"encoding/json"
	"testing"
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
