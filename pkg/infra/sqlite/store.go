package sqlite

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gennobou/gnb-memorymcp/pkg/domain"
	_ "github.com/tursodatabase/libsql-client-go/libsql"
	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

type Store struct {
	db *sql.DB
}

// NewStore は新しい Store インスタンスを作成し、スキーマを適用（マイグレーション）します。
func NewStore(dbURL, token string) (*Store, error) {
	var connStr string
	if strings.HasPrefix(dbURL, "file:") || dbURL == ":memory:" {
		connStr = dbURL
	} else {
		// Turso リモート接続の場合
		if token != "" {
			connStr = fmt.Sprintf("%s?authToken=%s", dbURL, token)
		} else {
			connStr = dbURL
		}
	}

	db, err := sql.Open("libsql", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// pingの後にマイグレーションチェック
	var ftsSQL string
	err = db.QueryRow("SELECT sql FROM sqlite_master WHERE type='table' AND name='memories_fts'").Scan(&ftsSQL)
	if err == nil {
		// すでにテーブルが存在する場合、trigram が使われているかチェック
		if !strings.Contains(strings.ToLower(ftsSQL), "trigram") {
			log.Println("古い FTS5 インデックス (unicode61) を検出しました。trigram トークナイザにマイグレーションします...")
			// memories_fts をドロップする
			if _, err := db.Exec("DROP TABLE memories_fts;"); err != nil {
				db.Close()
				return nil, fmt.Errorf("failed to drop memories_fts: %w", err)
			}
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		db.Close()
		return nil, fmt.Errorf("failed to check memories_fts schema: %w", err)
	}

	// memoriesテーブルにversion列が存在するかチェックし、無ければ追加する
	var hasVersion bool
	rowsPragma, err := db.Query("PRAGMA table_info(memories)")
	if err == nil {
		for rowsPragma.Next() {
			var cid int
			var name, ctype string
			var notnull int
			var dfltVal interface{}
			var pk int
			if errScan := rowsPragma.Scan(&cid, &name, &ctype, &notnull, &dfltVal, &pk); errScan == nil {
				if name == "version" {
					hasVersion = true
					break
				}
			}
		}
		rowsPragma.Close()

		if !hasVersion {
			log.Println("データベースに version カラムを追加します...")
			// 既にテーブルが存在している場合のみALTER TABLE
			var memTableExists bool
			errCheck := db.QueryRow("SELECT 1 FROM sqlite_master WHERE type='table' AND name='memories'").Scan(&memTableExists)
			if errCheck == nil {
				if _, errAlter := db.Exec("ALTER TABLE memories ADD COLUMN version INTEGER DEFAULT 1"); errAlter != nil {
					db.Close()
					return nil, fmt.Errorf("failed to add version column: %w", errAlter)
				}
			} else if !errors.Is(errCheck, sql.ErrNoRows) {
				db.Close()
				return nil, fmt.Errorf("failed to check memories table existence: %w", errCheck)
			}
		}
	} else {
		db.Close()
		return nil, fmt.Errorf("failed to check memories table info: %w", err)
	}

	// テーブル初期化（簡易マイグレーション）
	var queries []string
	var currentQuery strings.Builder
	inTrigger := false

	lines := strings.Split(schemaSQL, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		if strings.HasPrefix(strings.ToUpper(trimmed), "CREATE TRIGGER") {
			inTrigger = true
		}

		currentQuery.WriteString(line + "\n")

		if inTrigger && strings.ToUpper(trimmed) == "END;" {
			queries = append(queries, currentQuery.String())
			currentQuery.Reset()
			inTrigger = false
		} else if !inTrigger && strings.HasSuffix(trimmed, ";") {
			queries = append(queries, currentQuery.String())
			currentQuery.Reset()
		}
	}

	for _, query := range queries {
		query = strings.TrimSpace(query)
		if query == "" {
			continue
		}
		if _, err := db.Exec(query); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to run schema query (%s): %w", query, err)
		}
	}

	// 既存データがある場合は再インデックス
	if _, err := db.Exec("INSERT INTO memories_fts(rowid, content) SELECT rowid, content FROM memories WHERE rowid NOT IN (SELECT rowid FROM memories_fts);"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to rebuild fts index: %w", err)
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) Create(ctx context.Context, m *domain.Memory) error {
	tagsJSON, err := json.Marshal(m.Tags)
	if err != nil {
		return fmt.Errorf("failed to marshal tags: %w", err)
	}
	metadataJSON, err := json.Marshal(m.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	nowStr := time.Now().UTC().Format(time.RFC3339)
	m.CreatedAt = time.Now().UTC()
	m.UpdatedAt = time.Now().UTC()
	m.Version = 1

	query := `
		INSERT INTO memories (
			id, content, source_tool, tags, metadata, importance,
			embedding, embedding_model, version, created_at, updated_at, last_accessed
		) VALUES (?, ?, ?, ?, ?, ?, NULL, ?, 1, ?, ?, NULL)
	`
	_, err = s.db.ExecContext(ctx, query,
		m.ID, m.Content, m.SourceTool, string(tagsJSON), string(metadataJSON), m.Importance,
		m.EmbeddingModel, nowStr, nowStr,
	)
	if err != nil {
		return fmt.Errorf("failed to insert memory: %w", err)
	}
	return nil
}

func (s *Store) Get(ctx context.Context, id string) (*domain.Memory, error) {
	query := `
		SELECT id, content, source_tool, tags, metadata, importance, embedding_model, version, created_at, updated_at, last_accessed
		FROM memories WHERE id = ?
	`
	row := s.db.QueryRowContext(ctx, query, id)

	var m domain.Memory
	var tagsStr, metadataStr sql.NullString
	var createdAtStr, updatedAtStr string
	var lastAccessedStr sql.NullString

	err := row.Scan(
		&m.ID, &m.Content, &m.SourceTool, &tagsStr, &metadataStr, &m.Importance,
		&m.EmbeddingModel, &m.Version, &createdAtStr, &updatedAtStr, &lastAccessedStr,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrMemoryNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to scan memory: %w", err)
	}

	m.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr)
	m.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAtStr)
	if lastAccessedStr.Valid {
		t, _ := time.Parse(time.RFC3339, lastAccessedStr.String)
		m.LastAccessed = &t
	}

	if tagsStr.Valid && tagsStr.String != "" {
		_ = json.Unmarshal([]byte(tagsStr.String), &m.Tags)
	}
	if metadataStr.Valid && metadataStr.String != "" {
		_ = json.Unmarshal([]byte(metadataStr.String), &m.Metadata)
	}

	return &m, nil
}

func (s *Store) Update(ctx context.Context, m *domain.Memory) error {
	tagsJSON, err := json.Marshal(m.Tags)
	if err != nil {
		return fmt.Errorf("failed to marshal tags: %w", err)
	}
	metadataJSON, err := json.Marshal(m.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	nowStr := time.Now().UTC().Format(time.RFC3339)
	m.UpdatedAt = time.Now().UTC()

	query := `
		UPDATE memories SET
			content = ?, source_tool = ?, tags = ?, metadata = ?, importance = ?,
			embedding_model = ?, version = version + 1, updated_at = ?
		WHERE id = ? AND version = ?
	`
	res, err := s.db.ExecContext(ctx, query,
		m.Content, m.SourceTool, string(tagsJSON), string(metadataJSON), m.Importance,
		m.EmbeddingModel, nowStr, m.ID, m.Version,
	)
	if err != nil {
		return fmt.Errorf("failed to update memory: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrConflict
	}

	m.Version++
	return nil
}

func (s *Store) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM memories WHERE id = ?`
	res, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete memory: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrMemoryNotFound
	}

	return nil
}

func (s *Store) Search(ctx context.Context, query string, topK int) ([]*domain.Memory, error) {
	var sqlQuery string
	var args []interface{}
	isFTS := false

	trimmed := strings.TrimSpace(query)
	// クエリの文字数チェック (trigram は3文字以上必要)
	if utf8.RuneCountInString(trimmed) < 3 {
		// 3文字未満の場合は LIKE 検索にフォールバック (部分一致)
		// LIKEの場合はrankが存在しないので疑似的に 0.0 をSELECT
		escapedQuery := escapeLikePattern(trimmed)
		sqlQuery = `
			SELECT id, content, source_tool, tags, metadata, importance, embedding_model, version, created_at, updated_at, last_accessed, 0.0 as rank
			FROM memories
			WHERE content LIKE ? ESCAPE '\'
			LIMIT ?
		`
		args = []interface{}{"%" + escapedQuery + "%", topK * 3} // 再ランキング用に多めに取得
	} else {
		isFTS = true
		// FTS5 全文検索 (trigram)
		// 特殊文字による構文エラーを防ぐためダブルクォートで囲む
		sanitized := strings.ReplaceAll(trimmed, `"`, `""`)
		ftsQuery := `"` + sanitized + `"`
		sqlQuery = `
			SELECT m.id, m.content, m.source_tool, m.tags, m.metadata, m.importance, m.embedding_model, m.version, m.created_at, m.updated_at, m.last_accessed, f.rank
			FROM memories m
			JOIN memories_fts f ON m.rowid = f.rowid
			WHERE memories_fts MATCH ?
			LIMIT ?
		`
		args = []interface{}{ftsQuery, topK * 3} // 再ランキング用に多めに取得
	}

	rows, err := s.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to search memories: %w", err)
	}
	defer rows.Close()

	var memories []*domain.Memory
	var ranks []float64

	for rows.Next() {
		var m domain.Memory
		var tagsStr, metadataStr sql.NullString
		var createdAtStr, updatedAtStr string
		var lastAccessedStr sql.NullString
		var rankVal float64

		err := rows.Scan(
			&m.ID, &m.Content, &m.SourceTool, &tagsStr, &metadataStr, &m.Importance,
			&m.EmbeddingModel, &m.Version, &createdAtStr, &updatedAtStr, &lastAccessedStr, &rankVal,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan search result: %w", err)
		}

		m.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr)
		m.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAtStr)
		if lastAccessedStr.Valid {
			t, _ := time.Parse(time.RFC3339, lastAccessedStr.String)
			m.LastAccessed = &t
		}

		if tagsStr.Valid && tagsStr.String != "" {
			_ = json.Unmarshal([]byte(tagsStr.String), &m.Tags)
		}
		if metadataStr.Valid && metadataStr.String != "" {
			_ = json.Unmarshal([]byte(metadataStr.String), &m.Metadata)
		}

		memories = append(memories, &m)
		ranks = append(ranks, rankVal)
	}

	// 再ランキング処理
	type rankedMemory struct {
		memory *domain.Memory
		score  float64
	}
	var ranked []*rankedMemory
	for i, m := range memories {
		var ftsScore float64
		if isFTS {
			// FTS5の rank は適合度が高いほど負数（値が小さい）。-rank にして適合度が高いほど大きな正数にする。
			ftsScore = -ranks[i]
		} else {
			ftsScore = 5.0 // LIKE検索時のデフォルト適合度
		}

		// 重要度スコア (0〜10) -> 最大 5.0
		importanceScore := float64(m.Importance) * 0.5
		// 鮮度スコア (経過日数による時間減衰) -> 最大 5.0
		days := time.Since(m.UpdatedAt).Hours() / 24.0
		recencyScore := 5.0 / (1.0 + (days / 14.0)) // 14日で半分に減衰

		totalScore := ftsScore + importanceScore + recencyScore
		ranked = append(ranked, &rankedMemory{memory: m, score: totalScore})
	}

	// スコア降順ソート
	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].score > ranked[j].score
	})

	// 指定された topK 件だけ抽出して返却
	var result []*domain.Memory
	for i := 0; i < len(ranked) && i < topK; i++ {
		result = append(result, ranked[i].memory)
	}

	return result, nil
}

func escapeLikePattern(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}

func (s *Store) List(ctx context.Context, filter domain.MemoryFilter, limit int) ([]*domain.Memory, error) {
	var conditions []string
	var args []interface{}

	if filter.SourceTool != "" {
		conditions = append(conditions, "source_tool = ?")
		args = append(args, filter.SourceTool)
	}
	if filter.Tag != "" {
		conditions = append(conditions, "EXISTS (SELECT 1 FROM json_each(tags) WHERE value = ?)")
		args = append(args, filter.Tag)
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// ソートキーとソート順の決定 (SQLインジェクション防止のためのバリデーション)
	sortBy := "created_at"
	switch strings.ToLower(filter.SortBy) {
	case "created_at", "updated_at", "importance":
		sortBy = strings.ToLower(filter.SortBy)
	}

	order := "DESC"
	if strings.ToUpper(filter.Order) == "ASC" {
		order = "ASC"
	}

	sqlQuery := fmt.Sprintf(`
		SELECT id, content, source_tool, tags, metadata, importance, embedding_model, version, created_at, updated_at, last_accessed
		FROM memories
		%s
		ORDER BY %s %s
		LIMIT ? OFFSET ?
	`, whereClause, sortBy, order)
	args = append(args, limit, filter.Offset)

	rows, err := s.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list memories: %w", err)
	}
	defer rows.Close()

	var memories []*domain.Memory
	for rows.Next() {
		var m domain.Memory
		var tagsStr, metadataStr sql.NullString
		var createdAtStr, updatedAtStr string
		var lastAccessedStr sql.NullString

		err := rows.Scan(
			&m.ID, &m.Content, &m.SourceTool, &tagsStr, &metadataStr, &m.Importance,
			&m.EmbeddingModel, &m.Version, &createdAtStr, &updatedAtStr, &lastAccessedStr,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan list result: %w", err)
		}

		m.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr)
		m.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAtStr)
		if lastAccessedStr.Valid {
			t, _ := time.Parse(time.RFC3339, lastAccessedStr.String)
			m.LastAccessed = &t
		}

		if tagsStr.Valid && tagsStr.String != "" {
			_ = json.Unmarshal([]byte(tagsStr.String), &m.Tags)
		}
		if metadataStr.Valid && metadataStr.String != "" {
			_ = json.Unmarshal([]byte(metadataStr.String), &m.Metadata)
		}

		memories = append(memories, &m)
	}

	return memories, nil
}

func (s *Store) ListTags(ctx context.Context) ([]string, error) {
	sqlQuery := `
		SELECT DISTINCT value
		FROM memories, json_each(tags)
		ORDER BY value
	`
	rows, err := s.db.QueryContext(ctx, sqlQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to list tags: %w", err)
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, fmt.Errorf("failed to scan tag: %w", err)
		}
		tags = append(tags, tag)
	}
	if tags == nil {
		tags = []string{}
	}
	return tags, nil
}

func (s *Store) GetSystemSetting(ctx context.Context, key string) (string, error) {
	query := `SELECT value FROM system_settings WHERE key = ?`
	var value string
	err := s.db.QueryRowContext(ctx, query, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to get system setting: %w", err)
	}
	return value, nil
}

func (s *Store) SetSystemSetting(ctx context.Context, key, value string) error {
	query := `
		INSERT INTO system_settings (key, value)
		VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`
	_, err := s.db.ExecContext(ctx, query, key, value)
	if err != nil {
		return fmt.Errorf("failed to set system setting: %w", err)
	}
	return nil
}

const (
	defaultSimilarityThreshold = 0.35
	// lengthRatioThreshold は Jaccard 類似度が similarityThreshold (0.35) 以上になり得ない長さの比（約2.85倍以上）を早期スキップするためのしきい値です。
	lengthRatioThreshold = 2.85
	defaultCleanupLimit  = 3
)

func (s *Store) GetCleanupCandidates(ctx context.Context, limit, offset int) ([]*domain.CleanupGroup, error) {
	memories, err := s.List(ctx, domain.MemoryFilter{}, 1000)
	if err != nil {
		return nil, err
	}

	if len(memories) < 2 {
		return nil, nil
	}

	type memoryMeta struct {
		memory  *domain.Memory
		runeLen int
		biGrams map[string]bool
	}

	metas := make([]memoryMeta, len(memories))
	for i, m := range memories {
		metas[i] = memoryMeta{
			memory:  m,
			runeLen: utf8.RuneCountInString(m.Content),
			biGrams: charBiGrams(m.Content),
		}
	}

	groupedIDs := make(map[string]bool)
	var allGroups []*domain.CleanupGroup

	for i := 0; i < len(metas); i++ {
		meta1 := metas[i]
		m1 := meta1.memory
		if groupedIDs[m1.ID] {
			continue
		}

		var currentGroup []*domain.Memory
		len1 := meta1.runeLen

		for j := i + 1; j < len(metas); j++ {
			meta2 := metas[j]
			m2 := meta2.memory
			if groupedIDs[m2.ID] {
				continue
			}

			len2 := meta2.runeLen
			// 数学的に Jaccard 類似度が similarityThreshold 以上になり得ない長さの比を早期スキップ
			if float64(len1) > float64(len2)*lengthRatioThreshold || float64(len2) > float64(len1)*lengthRatioThreshold {
				continue
			}

			sim := jaccardSimilarityFromBiGrams(meta1.biGrams, meta2.biGrams)
			if sim >= defaultSimilarityThreshold {
				if len(currentGroup) == 0 {
					currentGroup = append(currentGroup, m1)
					groupedIDs[m1.ID] = true
				}
				currentGroup = append(currentGroup, m2)
				groupedIDs[m2.ID] = true
			}
		}

		if len(currentGroup) > 0 {
			groupID := fmt.Sprintf("group_%s", m1.ID)
			allGroups = append(allGroups, &domain.CleanupGroup{
				GroupID:  groupID,
				Memories: currentGroup,
			})
		}
	}

	// ページネーションの適用
	if limit <= 0 {
		limit = defaultCleanupLimit
	}
	if offset < 0 {
		offset = 0
	}

	var pagedGroups []*domain.CleanupGroup
	for i := offset; i < len(allGroups) && len(pagedGroups) < limit; i++ {
		pagedGroups = append(pagedGroups, allGroups[i])
	}

	return pagedGroups, nil
}
