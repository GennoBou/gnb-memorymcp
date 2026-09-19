package main

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/gennobou/gnb-memorymcp/pkg/infra/sqlite"
	"github.com/gennobou/gnb-memorymcp/pkg/mcp"
)

func TestHandleRequest_AuthenticationAndRouting(t *testing.T) {
	// テスト用のグローバル変数設定
	apiKey = "test-secret-key"

	// メモリ内 SQLite を使用して mcpHandler を初期化
	store, err := sqlite.NewStore("file::memory:", "")
	if err != nil {
		t.Fatalf("failed to initialize store: %v", err)
	}
	defer store.Close()
	mcpHandler = mcp.NewHandler(store)

	// initializeOnce を済ませておく（環境変数チェックをスキップ）
	initOnce.Do(func() {})

	ctx := context.Background()

	// 1. Authorizationヘッダーなし
	req := events.APIGatewayV2HTTPRequest{
		Body: `{"jsonrpc":"2.0","method":"initialize","id":1}`,
	}
	resp, err := HandleRequest(ctx, req)
	if err != nil {
		t.Fatalf("HandleRequest failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	// 2. 無効なAPIキー (tools/list メソッドで検証)
	req.Body = `{"jsonrpc":"2.0","method":"tools/list","id":2}`
	req.Headers = map[string]string{
		"Authorization": "Bearer wrong-key",
	}
	resp, err = HandleRequest(ctx, req)
	if err != nil {
		t.Fatalf("HandleRequest failed: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, resp.StatusCode)
	}

	// 3. 有効なAPIキー、かつ正常なリクエスト
	req.Headers = map[string]string{
		"Authorization": "Bearer test-secret-key",
	}
	resp, err = HandleRequest(ctx, req)
	if err != nil {
		t.Fatalf("HandleRequest failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var mcpResp mcp.Response
	if err := json.Unmarshal([]byte(resp.Body), &mcpResp); err != nil {
		t.Fatalf("failed to unmarshal body: %v", err)
	}

	if mcpResp.Error != nil {
		t.Errorf("unexpected mcp error: %v", mcpResp.Error)
	}

	// 4. 有効なAPIキー、かつJSON-RPCパースエラー
	req.Headers = map[string]string{
		"Authorization": "Bearer test-secret-key",
	}
	req.Body = `{"invalid-json`
	resp, err = HandleRequest(ctx, req)
	if err != nil {
		t.Fatalf("HandleRequest failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d for parse error", http.StatusOK, resp.StatusCode)
	}

	var mcpErrResp mcp.Response
	if err := json.Unmarshal([]byte(resp.Body), &mcpErrResp); err != nil {
		t.Fatalf("failed to unmarshal parse error body: %v", err)
	}
	if mcpErrResp.Error == nil || mcpErrResp.Error.Code != mcp.CodeParseError {
		t.Errorf("expected parse error, got: %v", mcpErrResp.Error)
	}
}

func TestHandleRequest_CORS(t *testing.T) {
	ctx := context.Background()

	t.Run("OPTIONS request with Origin when ALLOWED_ORIGINS is unset", func(t *testing.T) {
		t.Setenv("ALLOWED_ORIGINS", "")
		req := events.APIGatewayV2HTTPRequest{
			RequestContext: events.APIGatewayV2HTTPRequestContext{
				HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{
					Method: "OPTIONS",
					Path:   "/",
				},
			},
			Headers: map[string]string{
				"origin": "https://untrusted-app.com",
			},
		}

		resp, err := HandleRequest(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if acao, ok := resp.Headers["Access-Control-Allow-Origin"]; ok && acao != "" {
			t.Errorf("expected no Access-Control-Allow-Origin when ALLOWED_ORIGINS is unset, got '%s'", acao)
		}
	})

	t.Run("OPTIONS request without Origin header when ALLOWED_ORIGINS is unset", func(t *testing.T) {
		t.Setenv("ALLOWED_ORIGINS", "")
		req := events.APIGatewayV2HTTPRequest{
			RequestContext: events.APIGatewayV2HTTPRequestContext{
				HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{
					Method: "OPTIONS",
					Path:   "/",
				},
			},
		}

		resp, err := HandleRequest(ctx, req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if acao, ok := resp.Headers["Access-Control-Allow-Origin"]; ok && acao != "" {
			t.Errorf("expected no Access-Control-Allow-Origin header without Origin, got '%s'", acao)
		}
	})

	t.Run("ALLOWED_ORIGINS whitelist matching", func(t *testing.T) {
		t.Setenv("ALLOWED_ORIGINS", "https://allowed.com, https://another.com")

		// Allowed origin
		reqAllowed := events.APIGatewayV2HTTPRequest{
			RequestContext: events.APIGatewayV2HTTPRequestContext{
				HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{
					Method: "OPTIONS",
					Path:   "/",
				},
			},
			Headers: map[string]string{
				"Origin": "https://allowed.com",
			},
		}
		respAllowed, err := HandleRequest(ctx, reqAllowed)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if respAllowed.Headers["Access-Control-Allow-Origin"] != "https://allowed.com" {
			t.Errorf("expected Access-Control-Allow-Origin 'https://allowed.com', got '%s'", respAllowed.Headers["Access-Control-Allow-Origin"])
		}

		// Disallowed origin
		reqDisallowed := events.APIGatewayV2HTTPRequest{
			RequestContext: events.APIGatewayV2HTTPRequestContext{
				HTTP: events.APIGatewayV2HTTPRequestContextHTTPDescription{
					Method: "OPTIONS",
					Path:   "/",
				},
			},
			Headers: map[string]string{
				"Origin": "https://malicious.com",
			},
		}
		respDisallowed, err := HandleRequest(ctx, reqDisallowed)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if acao, ok := respDisallowed.Headers["Access-Control-Allow-Origin"]; ok && acao != "" {
			t.Errorf("expected no Access-Control-Allow-Origin for disallowed origin, got '%s'", acao)
		}
	})
}

func TestGetHeaderValue(t *testing.T) {
	tests := []struct {
		name     string
		headers  map[string]string
		key      string
		expected string
	}{
		{
			name: "Exact match with lowercase key in map",
			headers: map[string]string{
				"accept": "application/json",
			},
			key:      "accept",
			expected: "application/json",
		},
		{
			name: "Case insensitive match with uppercase key in map and lowercase search key",
			headers: map[string]string{
				"Authorization": "Bearer test-token",
			},
			key:      "authorization",
			expected: "Bearer test-token",
		},
		{
			name: "Case insensitive match with lowercase key in map and uppercase search key",
			headers: map[string]string{
				"content-type": "application/json",
			},
			key:      "Content-Type",
			expected: "application/json",
		},
		{
			name: "Case insensitive match with mixed case key in map and mixed case search key",
			headers: map[string]string{
				"X-Custom-Header": "value123",
			},
			key:      "x-CUSTOM-header",
			expected: "value123",
		},
		{
			name: "Header key not present",
			headers: map[string]string{
				"host": "localhost",
			},
			key:      "authorization",
			expected: "",
		},
		{
			name: "Header key present but value is empty",
			headers: map[string]string{
				"x-empty-header": "",
			},
			key:      "x-empty-header",
			expected: "",
		},
		{
			name:     "Empty headers map",
			headers:  map[string]string{},
			key:      "host",
			expected: "",
		},
		{
			name:     "Nil headers map",
			headers:  nil,
			key:      "host",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getHeaderValue(tt.headers, tt.key)
			if got != tt.expected {
				t.Errorf("getHeaderValue() = %q, want %q", got, tt.expected)
			}
		})
	}
}
