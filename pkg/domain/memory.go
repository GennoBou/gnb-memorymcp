package domain

import (
	"context"
	"time"
)

// Memory は記憶データのドメインモデルです。
type Memory struct {
	ID             string                 `json:"id"`
	Content        string                 `json:"content"`
	SourceTool     string                 `json:"source_tool"`
	Tags           []string               `json:"tags"`
	Metadata       map[string]interface{} `json:"metadata"`
	Importance     int                    `json:"importance"`
	Embedding      []float32              `json:"embedding,omitempty"`
	EmbeddingModel string                 `json:"embedding_model,omitempty"`
	Version        int                    `json:"version"`
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
	LastAccessed   *time.Time             `json:"last_accessed,omitempty"`
}

// MemoryFilter は一覧取得時の絞り込み条件です。
type MemoryFilter struct {
	SourceTool string
	Tag        string
	Offset     int
	SortBy     string
	Order      string
}

// CleanupGroup は整理（クレンジング）対象となる、重複または類似した記憶のグループです。
type CleanupGroup struct {
	GroupID  string    `json:"group_id"`
	Memories []*Memory `json:"memories"`
}

// MemoryStore は記憶データを永続化するためのインターフェース（ポート）です。
type MemoryStore interface {
	Create(ctx context.Context, m *Memory) error
	Get(ctx context.Context, id string) (*Memory, error)
	Search(ctx context.Context, query string, topK int) ([]*Memory, error)
	List(ctx context.Context, filter MemoryFilter, limit int) ([]*Memory, error)
	ListTags(ctx context.Context) ([]string, error)
	Update(ctx context.Context, m *Memory) error
	Delete(ctx context.Context, id string) error
	GetSystemSetting(ctx context.Context, key string) (string, error)
	SetSystemSetting(ctx context.Context, key, value string) error
	GetCleanupCandidates(ctx context.Context, limit, offset int) ([]*CleanupGroup, error)
}
