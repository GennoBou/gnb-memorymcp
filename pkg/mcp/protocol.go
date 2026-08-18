package mcp

import "encoding/json"

const (
	ProtocolVersion = "2026-07-28"
)

type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      interface{}     `json:"id,omitempty"`
	Meta    json.RawMessage `json:"_meta,omitempty"`
}

type Response struct {
	JSONRPC string      `json:"jsonrpc"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
	ID      interface{} `json:"id"`
}

type RPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

const (
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternalError  = -32603
)

func NewErrorResponse(id interface{}, code int, message string) *Response {
	return &Response{
		JSONRPC: "2.0",
		Error: &RPCError{
			Code:    code,
			Message: message,
		},
		ID: id,
	}
}

type CallToolParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type CallToolResult struct {
	Content []Content `json:"content"`
	IsError bool      `json:"isError,omitempty"`
}

type Content struct {
	Type string `json:"type"` // "text"
	Text string `json:"text"`
}

func NewTextResult(text string) *CallToolResult {
	return &CallToolResult{
		Content: []Content{
			{Type: "text", Text: text},
		},
	}
}

func NewErrorResult(errText string) *CallToolResult {
	return &CallToolResult{
		Content: []Content{
			{Type: "text", Text: errText},
		},
		IsError: true,
	}
}

type CreateArgs struct {
	Content    string                 `json:"content"`
	SourceTool string                 `json:"source_tool"`
	Tags       []string               `json:"tags,omitempty"`
	Importance int                    `json:"importance,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

type SearchArgs struct {
	Query string `json:"query"`
	TopK  int    `json:"top_k,omitempty"`
}

type ListArgs struct {
	SourceTool string `json:"source_tool,omitempty"`
	Tag        string `json:"tag,omitempty"`
	Limit      int    `json:"limit,omitempty"`
	Offset     int    `json:"offset,omitempty"`
	SortBy     string `json:"sort_by,omitempty"`
	Order      string `json:"order,omitempty"`
}

type UpdateArgs struct {
	ID         string                  `json:"id"`
	Content    *string                 `json:"content,omitempty"`
	SourceTool *string                 `json:"source_tool,omitempty"`
	Tags       *[]string               `json:"tags,omitempty"`
	Importance *int                    `json:"importance,omitempty"`
	Metadata   *map[string]interface{} `json:"metadata,omitempty"`
}

type DeleteArgs struct {
	ID string `json:"id"`
}

type GetArgs struct {
	ID string `json:"id"`
}

type ConsolidateArgs struct {
	Limit  int `json:"limit,omitempty"`
	Offset int `json:"offset,omitempty"`
}

type EmptyArgs struct{}
