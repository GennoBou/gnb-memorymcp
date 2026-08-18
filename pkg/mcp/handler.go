package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gennobou/gnb-memorymcp/pkg/domain"
	"github.com/oklog/ulid/v2"
)

type Handler struct {
	store domain.MemoryStore
}

func NewHandler(store domain.MemoryStore) *Handler {
	return &Handler{store: store}
}

type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema InputSchema `json:"inputSchema"`
}

type InputSchema struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties"`
	Required   []string            `json:"required,omitempty"`
}

type Property struct {
	Type        string              `json:"type"`
	Description string              `json:"description,omitempty"`
	Items       *Items              `json:"items,omitempty"`
	Properties  map[string]Property `json:"properties,omitempty"`
}

type Items struct {
	Type string `json:"type"`
}

type ToolsListResult struct {
	Tools []Tool `json:"tools"`
}

func (h *Handler) listTools() ToolsListResult {
	return ToolsListResult{
		Tools: []Tool{
			{
				Name:        "memory_create",
				Description: "新規記憶を保存します。ユーザーに関する事実や設定、プロジェクトの文脈、長期保存すべき重要な会話を記録するために使用します。",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"content":     {Type: "string", Description: "記憶したい事実、設定、または知識の本文。"},
						"source_tool": {Type: "string", Description: "呼び出し元のツール名（例: chatgpt, claude, gemini, antigravity など）。"},
						"tags":        {Type: "array", Items: &Items{Type: "string"}, Description: "記憶に関連するプロジェクトや技術のタグ（例: [\"go\", \"aws\"]）。"},
						"importance":  {Type: "integer", Description: "記憶の重要度（0〜10の範囲で、数値が大きいほど重要）。デフォルトは0。"},
						"metadata":    {Type: "object", Description: "URLや会話IDなどの追加のメタデータ。"},
					},
					Required: []string{"content", "source_tool"},
				},
			},
			{
				Name:        "memory_search",
				Description: "保存された記憶から、クエリにマッチするものを検索します。関連する文脈やユーザーの設定などを想起する際に使用します。",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"query": {Type: "string", Description: "検索キーワード。FTS5による高速全文検索が行われます。"},
						"top_k": {Type: "integer", Description: "取得する最大件数。デフォルトは5。"},
					},
					Required: []string{"query"},
				},
			},
			{
				Name:        "memory_list",
				Description: "保存されている記憶の一覧をフィルタリング・ソートして取得します。デバッグや履歴確認に使用します。",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"source_tool": {Type: "string", Description: "特定のツールで作成された記憶のみに絞り込みます。"},
						"tag":         {Type: "string", Description: "特定のタグを含む記憶のみに絞り込みます。"},
						"limit":       {Type: "integer", Description: "取得件数。デフォルトは20。"},
						"offset":      {Type: "integer", Description: "取得開始位置（ページネーション用、デフォルトは0）。"},
						"sort_by":     {Type: "string", Description: "ソートの基準（created_at, updated_at, importance）。デフォルトは created_at。"},
						"order":       {Type: "string", Description: "ソート順（asc: 昇順, desc: 降順）。デフォルトは desc。"},
					},
				},
			},
			{
				Name:        "memory_get",
				Description: "指定されたID（ULID）を持つ記憶を1件取得します。",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"id": {Type: "string", Description: "取得対象の記憶のID（ULID）。"},
					},
					Required: []string{"id"},
				},
			},
			{
				Name:        "tags_list",
				Description: "これまでに登録されたすべてのユニークなタグの一覧を取得します。表記ゆれの防止や、タグでの絞り込みの前に利用します。",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{},
				},
			},
			{
				Name:        "memory_update",
				Description: "既存の記憶を更新します。情報の修正や、関連情報の追加時に使用します。",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"id":          {Type: "string", Description: "更新対象の記憶のID（ULID）。"},
						"content":     {Type: "string", Description: "修正・更新後の本文。"},
						"source_tool": {Type: "string", Description: "更新元のツール名。"},
						"tags":        {Type: "array", Items: &Items{Type: "string"}, Description: "新しいタグの配列。"},
						"importance":  {Type: "integer", Description: "更新後の重要度（0〜10）。"},
						"metadata":    {Type: "object", Description: "更新後のメタデータ。"},
					},
					Required: []string{"id"},
				},
			},
			{
				Name:        "memory_delete",
				Description: "不要になった記憶を明示的に削除します。",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"id": {Type: "string", Description: "削除対象の記憶のID（ULID）。"},
					},
					Required: []string{"id"},
				},
			},
			{
				Name:        "memory_status",
				Description: "記憶データベースの全体ステータス（総件数、最終整理日時、整理が必要な候補数など）を取得します。",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{},
				},
			},
			{
				Name:        "memory_consolidate",
				Description: "重複または矛盾している可能性のある記憶のペア/グループの一覧を取得します。返された情報を基に、LLM自身が内容を統合（memory_update）し、不要な古いIDを削除（memory_delete）してください。",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"limit":  {Type: "integer", Description: "取得するグループの最大件数。デフォルトは3。"},
						"offset": {Type: "integer", Description: "取得開始位置（ページネーション用、デフォルトは0）。"},
					},
				},
			},
			{
				Name:        "memory_cleanup_complete",
				Description: "記憶の整理・統合（クレンジング）作業が完了したことをシステムに記録し、最終整理日時を更新します。",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{},
				},
			},
		},
	}
}

func (h *Handler) Handle(ctx context.Context, req *Request) *Response {
	switch req.Method {
	case "notifications/initialized":
		return nil

	case "initialize":
		protoVer := ProtocolVersion
		if len(req.Params) > 0 {
			var p struct {
				ProtocolVersion string `json:"protocolVersion"`
			}
			if err := json.Unmarshal(req.Params, &p); err == nil && p.ProtocolVersion != "" {
				protoVer = p.ProtocolVersion
			}
		}

		return &Response{
			JSONRPC: "2.0",
			Result: map[string]interface{}{
				"protocolVersion": protoVer,
				"capabilities": map[string]interface{}{
					"tools": map[string]interface{}{},
				},
				"serverInfo": map[string]string{
					"name":    "GNB MemoryMCP",
					"version": "1.0.0",
				},
			},
			ID: req.ID,
		}

	case "tools/list":
		result := h.listTools()
		return &Response{
			JSONRPC: "2.0",
			Result:  result,
			ID:      req.ID,
		}

	case "tools/call":
		var params CallToolParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return NewErrorResponse(req.ID, CodeInvalidParams, "failed to unmarshal params")
		}

		result, err := h.callTool(ctx, params.Name, params.Arguments)
		if err != nil {
			return &Response{
				JSONRPC: "2.0",
				Result:  NewErrorResult(fmt.Sprintf("Error: %v", err)),
				ID:      req.ID,
			}
		}

		return &Response{
			JSONRPC: "2.0",
			Result:  result,
			ID:      req.ID,
		}

	default:
		return NewErrorResponse(req.ID, CodeMethodNotFound, fmt.Sprintf("method not found: %s", req.Method))
	}
}

func (h *Handler) callTool(ctx context.Context, name string, argsJSON json.RawMessage) (*CallToolResult, error) {
	switch name {
	case "memory_create":
		var args CreateArgs
		if err := json.Unmarshal(argsJSON, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments for memory_create: %w", err)
		}
		if args.Content == "" {
			return nil, errors.New("content is required")
		}
		if utf8.RuneCountInString(args.Content) > 10000 {
			return nil, fmt.Errorf("content exceeds maximum length of 10000 characters (got %d)", utf8.RuneCountInString(args.Content))
		}
		if args.SourceTool == "" {
			return nil, errors.New("source_tool is required")
		}
		if len(args.Tags) > 10 {
			return nil, fmt.Errorf("tags exceed maximum count of 10 (got %d)", len(args.Tags))
		}
		if args.Importance < 0 || args.Importance > 10 {
			return nil, fmt.Errorf("importance must be between 0 and 10 (got %d)", args.Importance)
		}

		id := ulid.Make().String()

		m := &domain.Memory{
			ID:         id,
			Content:    args.Content,
			SourceTool: args.SourceTool,
			Tags:       args.Tags,
			Metadata:   args.Metadata,
			Importance: args.Importance,
		}

		if err := h.store.Create(ctx, m); err != nil {
			return nil, err
		}

		return NewTextResult(fmt.Sprintf("Memory created successfully with ID: %s", id)), nil

	case "memory_search":
		var args SearchArgs
		if err := json.Unmarshal(argsJSON, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments for memory_search: %w", err)
		}
		if args.Query == "" {
			return nil, errors.New("query is required")
		}
		topK := args.TopK
		if topK <= 0 {
			topK = 5
		} else if topK > 50 {
			topK = 50
		}

		memories, err := h.store.Search(ctx, args.Query, topK)
		if err != nil {
			return nil, err
		}

		if len(memories) == 0 {
			return NewTextResult("No memories found matching the query."), nil
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Found %d memories:\n\n", len(memories)))
		for _, m := range memories {
			tagsStr := strings.Join(m.Tags, ", ")
			sb.WriteString(fmt.Sprintf("--- Memory ID: %s ---\n", m.ID))
			sb.WriteString(fmt.Sprintf("Source: %s | Importance: %d | Tags: [%s]\n", m.SourceTool, m.Importance, tagsStr))
			sb.WriteString(fmt.Sprintf("Content: %s\n\n", m.Content))
		}

		return NewTextResult(sb.String()), nil

	case "memory_list":
		var args ListArgs
		if err := json.Unmarshal(argsJSON, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments for memory_list: %w", err)
		}
		limit := args.Limit
		if limit <= 0 {
			limit = 20
		} else if limit > 100 {
			limit = 100
		}

		filter := domain.MemoryFilter{
			SourceTool: args.SourceTool,
			Tag:        args.Tag,
			Offset:     args.Offset,
			SortBy:     args.SortBy,
			Order:      args.Order,
		}

		memories, err := h.store.List(ctx, filter, limit)
		if err != nil {
			return nil, err
		}

		if len(memories) == 0 {
			return &CallToolResult{
				Content: []Content{
					{Type: "text", Text: "No memories found."},
				},
			}, nil
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Listing %d memories:\n\n", len(memories)))
		for _, m := range memories {
			tagsStr := strings.Join(m.Tags, ", ")
			sb.WriteString(fmt.Sprintf("- ID: %s | Source: %s | Importance: %d | Tags: [%s] | Created: %s\n  Content: %s\n",
				m.ID, m.SourceTool, m.Importance, tagsStr, m.CreatedAt.Format(time.RFC3339), m.Content))
		}

		return &CallToolResult{
			Content: []Content{
				{Type: "text", Text: sb.String()},
			},
		}, nil

	case "memory_get":
		var args GetArgs
		if err := json.Unmarshal(argsJSON, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments for memory_get: %w", err)
		}
		if args.ID == "" {
			return nil, errors.New("id is required")
		}

		m, err := h.store.Get(ctx, args.ID)
		if err != nil {
			if errors.Is(err, domain.ErrMemoryNotFound) {
				return &CallToolResult{
					Content: []Content{
						{Type: "text", Text: fmt.Sprintf("Memory with ID %s not found.", args.ID)},
					},
					IsError: true,
				}, nil
			}
			return nil, err
		}

		tagsStr := strings.Join(m.Tags, ", ")
		metadataJSON, _ := json.MarshalIndent(m.Metadata, "", "  ")

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Memory ID: %s\n", m.ID))
		sb.WriteString(fmt.Sprintf("Source: %s\n", m.SourceTool))
		sb.WriteString(fmt.Sprintf("Importance: %d\n", m.Importance))
		sb.WriteString(fmt.Sprintf("Tags: [%s]\n", tagsStr))
		sb.WriteString(fmt.Sprintf("Created: %s\n", m.CreatedAt.Format(time.RFC3339)))
		sb.WriteString(fmt.Sprintf("Updated: %s\n", m.UpdatedAt.Format(time.RFC3339)))
		sb.WriteString(fmt.Sprintf("Metadata:\n%s\n\n", string(metadataJSON)))
		sb.WriteString(fmt.Sprintf("Content: %s\n", m.Content))

		return &CallToolResult{
			Content: []Content{
				{Type: "text", Text: sb.String()},
			},
		}, nil

	case "tags_list":
		tags, err := h.store.ListTags(ctx)
		if err != nil {
			return nil, err
		}

		if len(tags) == 0 {
			return &CallToolResult{
				Content: []Content{
					{Type: "text", Text: "No tags found."},
				},
			}, nil
		}

		var sb strings.Builder
		sb.WriteString("Available tags:\n")
		for _, tag := range tags {
			sb.WriteString(fmt.Sprintf("- %s\n", tag))
		}

		return &CallToolResult{
			Content: []Content{
				{Type: "text", Text: sb.String()},
			},
		}, nil

	case "memory_update":
		var args UpdateArgs
		if err := json.Unmarshal(argsJSON, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments for memory_update: %w", err)
		}
		if args.ID == "" {
			return nil, errors.New("id is required")
		}

		m, err := h.store.Get(ctx, args.ID)
		if err != nil {
			return nil, err
		}

		if args.Content != nil {
			if utf8.RuneCountInString(*args.Content) > 10000 {
				return nil, fmt.Errorf("content exceeds maximum length of 10000 characters (got %d)", utf8.RuneCountInString(*args.Content))
			}
			m.Content = *args.Content
		}
		if args.SourceTool != nil {
			m.SourceTool = *args.SourceTool
		}
		if args.Tags != nil {
			if len(*args.Tags) > 10 {
				return nil, fmt.Errorf("tags exceed maximum count of 10 (got %d)", len(*args.Tags))
			}
			m.Tags = *args.Tags
		}
		if args.Importance != nil {
			if *args.Importance < 0 || *args.Importance > 10 {
				return nil, fmt.Errorf("importance must be between 0 and 10 (got %d)", *args.Importance)
			}
			m.Importance = *args.Importance
		}
		if args.Metadata != nil {
			m.Metadata = *args.Metadata
		}

		if err := h.store.Update(ctx, m); err != nil {
			if errors.Is(err, domain.ErrConflict) {
				return nil, fmt.Errorf("concurrent update conflict: this memory has been modified by another process. Please retrieve the latest memory and try again: %w", err)
			}
			return nil, err
		}

		return &CallToolResult{
			Content: []Content{
				{Type: "text", Text: fmt.Sprintf("Memory ID: %s updated successfully.", m.ID)},
			},
		}, nil

	case "memory_delete":
		var args DeleteArgs
		if err := json.Unmarshal(argsJSON, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments for memory_delete: %w", err)
		}
		if args.ID == "" {
			return nil, errors.New("id is required")
		}

		if err := h.store.Delete(ctx, args.ID); err != nil {
			return nil, err
		}

		return &CallToolResult{
			Content: []Content{
				{Type: "text", Text: fmt.Sprintf("Memory ID: %s deleted successfully.", args.ID)},
			},
		}, nil

	case "memory_status":
		memories, err := h.store.List(ctx, domain.MemoryFilter{}, 1000)
		if err != nil {
			return nil, err
		}
		total := len(memories)

		lastCleanup, err := h.store.GetSystemSetting(ctx, "last_cleanup_at")
		if err != nil {
			return nil, err
		}

		daysSinceLastCleanup := -1
		if lastCleanup != "" {
			parsedTime, err := time.Parse(time.RFC3339, lastCleanup)
			if err == nil {
				daysSinceLastCleanup = int(time.Since(parsedTime).Hours() / 24)
			}
		}

		candidates, err := h.store.GetCleanupCandidates(ctx, 100, 0)
		if err != nil {
			return nil, err
		}
		candidatesCount := len(candidates)

		requiresCleanup := false
		if candidatesCount > 0 || daysSinceLastCleanup >= 7 || lastCleanup == "" {
			requiresCleanup = true
		}

		var sb strings.Builder
		sb.WriteString("Database Status:\n")
		sb.WriteString(fmt.Sprintf("- Total memories: %d\n", total))
		if lastCleanup != "" {
			sb.WriteString(fmt.Sprintf("- Last cleanup: %s (%d days ago)\n", lastCleanup, daysSinceLastCleanup))
		} else {
			sb.WriteString("- Last cleanup: never\n")
		}
		sb.WriteString(fmt.Sprintf("- Cleanup candidate groups: %d\n", candidatesCount))
		sb.WriteString(fmt.Sprintf("- Requires cleanup: %v\n", requiresCleanup))

		return &CallToolResult{
			Content: []Content{
				{Type: "text", Text: sb.String()},
			},
		}, nil

	case "memory_consolidate":
		var args ConsolidateArgs
		if err := json.Unmarshal(argsJSON, &args); err != nil {
			return nil, fmt.Errorf("invalid arguments for memory_consolidate: %w", err)
		}
		limit := args.Limit
		if limit <= 0 {
			limit = 3
		}

		candidates, err := h.store.GetCleanupCandidates(ctx, limit, args.Offset)
		if err != nil {
			return nil, err
		}

		if len(candidates) == 0 {
			return &CallToolResult{
				Content: []Content{
					{Type: "text", Text: "No memories require consolidation. The database is clean."},
				},
			}, nil
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Found %d groups of memories that may be duplicates or contradict each other.\n", len(candidates)))
		sb.WriteString("Please review them and merge them using `memory_update` and `memory_delete`.\n\n")

		for idx, group := range candidates {
			sb.WriteString(fmt.Sprintf("=== Group %d (%s) ===\n", idx+1, group.GroupID))
			for _, m := range group.Memories {
				tagsStr := strings.Join(m.Tags, ", ")
				sb.WriteString(fmt.Sprintf("- ID: %s | Tags: [%s] | Created: %s\n  Content: %s\n",
					m.ID, tagsStr, m.CreatedAt.Format(time.RFC3339), m.Content))
			}
			sb.WriteString("\n")
		}

		sb.WriteString("Instructions:\n")
		sb.WriteString("1. For each group, determine the most accurate and up-to-date information.\n")
		sb.WriteString("2. Use `memory_update` on one memory ID to hold the consolidated final information.\n")
		sb.WriteString("3. Use `memory_delete` on the other memory IDs in the group to remove redundant records.\n")
		sb.WriteString("4. Once you have finished cleaning all groups, call `memory_cleanup_complete` to update the cleanup status.\n")

		return &CallToolResult{
			Content: []Content{
				{Type: "text", Text: sb.String()},
			},
		}, nil

	case "memory_cleanup_complete":
		nowStr := time.Now().UTC().Format(time.RFC3339)
		err := h.store.SetSystemSetting(ctx, "last_cleanup_at", nowStr)
		if err != nil {
			return nil, err
		}

		return &CallToolResult{
			Content: []Content{
				{Type: "text", Text: fmt.Sprintf("Cleanup completed successfully. Timestamp updated to %s", nowStr)},
			},
		}, nil

	default:
		return nil, fmt.Errorf("unsupported tool: %s", name)
	}
}
