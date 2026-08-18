package domain

import "errors"

// ドメイン共通エラーの定義
var (
	ErrMemoryNotFound = errors.New("memory not found")
	ErrInvalidInput   = errors.New("invalid input data")
	ErrConflict       = errors.New("memory conflict: version mismatch")
)
