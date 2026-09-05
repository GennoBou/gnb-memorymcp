package mcp

import (
	"testing"
)

func TestNewErrorResponse(t *testing.T) {
	tests := []struct {
		name    string
		id      interface{}
		code    int
		message string
	}{
		{
			name:    "integer id with parse error",
			id:      1,
			code:    CodeParseError,
			message: "Parse error",
		},
		{
			name:    "string id with invalid request",
			id:      "req-123",
			code:    CodeInvalidRequest,
			message: "Invalid Request",
		},
		{
			name:    "nil id with internal error",
			id:      nil,
			code:    CodeInternalError,
			message: "Internal error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := NewErrorResponse(tt.id, tt.code, tt.message)

			if resp == nil {
				t.Fatal("expected non-nil response")
			}
			if resp.JSONRPC != "2.0" {
				t.Errorf("expected JSONRPC '2.0', got '%s'", resp.JSONRPC)
			}
			if resp.ID != tt.id {
				t.Errorf("expected ID %v, got %v", tt.id, resp.ID)
			}
			if resp.Result != nil {
				t.Errorf("expected nil Result, got %v", resp.Result)
			}
			if resp.Error == nil {
				t.Fatal("expected non-nil Error")
			}
			if resp.Error.Code != tt.code {
				t.Errorf("expected Code %d, got %d", tt.code, resp.Error.Code)
			}
			if resp.Error.Message != tt.message {
				t.Errorf("expected Message '%s', got '%s'", tt.message, resp.Error.Message)
			}
		})
	}
}

func TestNewTextResult(t *testing.T) {
	tests := []struct {
		name string
		text string
	}{
		{
			name: "plain text string",
			text: "hello world",
		},
		{
			name: "empty text string",
			text: "",
		},
		{
			name: "multiline text string",
			text: "line 1\nline 2\nline 3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewTextResult(tt.text)

			if result == nil {
				t.Fatal("expected non-nil CallToolResult")
			}
			if result.IsError != false {
				t.Errorf("expected IsError false, got %v", result.IsError)
			}
			if len(result.Content) != 1 {
				t.Fatalf("expected 1 content element, got %d", len(result.Content))
			}
			if result.Content[0].Type != "text" {
				t.Errorf("expected content type 'text', got '%s'", result.Content[0].Type)
			}
			if result.Content[0].Text != tt.text {
				t.Errorf("expected content text '%s', got '%s'", tt.text, result.Content[0].Text)
			}
		})
	}
}

func TestNewErrorResult(t *testing.T) {
	tests := []struct {
		name    string
		errText string
	}{
		{
			name:    "error message string",
			errText: "something went wrong",
		},
		{
			name:    "empty error message string",
			errText: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewErrorResult(tt.errText)

			if result == nil {
				t.Fatal("expected non-nil CallToolResult")
			}
			if result.IsError != true {
				t.Errorf("expected IsError true, got %v", result.IsError)
			}
			if len(result.Content) != 1 {
				t.Fatalf("expected 1 content element, got %d", len(result.Content))
			}
			if result.Content[0].Type != "text" {
				t.Errorf("expected content type 'text', got '%s'", result.Content[0].Type)
			}
			if result.Content[0].Text != tt.errText {
				t.Errorf("expected content text '%s', got '%s'", tt.errText, result.Content[0].Text)
			}
		})
	}
}
