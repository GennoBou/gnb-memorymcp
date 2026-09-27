package domain

import (
	"context"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"
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

const (
	// MaxContentLength はコンテンツの最大文字数（ルーン数）です。
	MaxContentLength = 10000
	// MaxTagsCount は付与できるタグの最大数です。
	MaxTagsCount = 10
	// MinImportance は重要度の最小値です。
	MinImportance = 0
	// MaxImportance は重要度の最大値です。
	MaxImportance = 10
)

// Validate は Memory オブジェクトの整合性を検証します。
func (m *Memory) Validate() error {
	if m.Content == "" {
		return errors.New("content is required")
	}
	if err := ValidateContentLength(m.Content); err != nil {
		return err
	}
	if m.SourceTool == "" {
		return errors.New("source_tool is required")
	}
	if err := ValidateTags(m.Tags); err != nil {
		return err
	}
	if err := ValidateImportance(m.Importance); err != nil {
		return err
	}
	return nil
}

// ValidateContentLength はコンテンツの文字数が制限内かを検証します。
func ValidateContentLength(content string) error {
	if count := utf8.RuneCountInString(content); count > MaxContentLength {
		return fmt.Errorf("content exceeds maximum length of %d characters (got %d)", MaxContentLength, count)
	}
	return nil
}

// ValidateTags はタグの個数が制限内かを検証します。
func ValidateTags(tags []string) error {
	if len(tags) > MaxTagsCount {
		return fmt.Errorf("tags exceed maximum count of %d (got %d)", MaxTagsCount, len(tags))
	}
	return nil
}

// ValidateImportance は重要度が有効範囲内かを検証します。
func ValidateImportance(importance int) error {
	if importance < MinImportance || importance > MaxImportance {
		return fmt.Errorf("importance must be between %d and %d (got %d)", MinImportance, MaxImportance, importance)
	}
	return nil
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
	Count(ctx context.Context) (int, error)
	ListTags(ctx context.Context) ([]string, error)
	Update(ctx context.Context, m *Memory) error
	Delete(ctx context.Context, id string) error
	GetSystemSetting(ctx context.Context, key string) (string, error)
	SetSystemSetting(ctx context.Context, key, value string) error
	GetCleanupCandidates(ctx context.Context, limit, offset int) ([]*CleanupGroup, error)
}
