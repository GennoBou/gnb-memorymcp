package domain

import (
	"strings"
	"testing"
)

func TestMemory_Validate(t *testing.T) {
	tests := []struct {
		name    string
		memory  Memory
		wantErr bool
		errMsg  string
	}{
		{
			name: "正常なMemory",
			memory: Memory{
				Content:    "テスト記憶データ",
				SourceTool: "antigravity",
				Tags:       []string{"test", "golang"},
				Importance: 5,
			},
			wantErr: false,
		},
		{
			name: "Contentが空",
			memory: Memory{
				Content:    "",
				SourceTool: "antigravity",
			},
			wantErr: true,
			errMsg:  "content is required",
		},
		{
			name: "Contentがちょうど10000文字（マルチバイト日本語含む）",
			memory: Memory{
				Content:    strings.Repeat("あ", 10000),
				SourceTool: "antigravity",
				Importance: 3,
			},
			wantErr: false,
		},
		{
			name: "Contentが10001文字で制限超過",
			memory: Memory{
				Content:    strings.Repeat("a", 10001),
				SourceTool: "antigravity",
			},
			wantErr: true,
			errMsg:  "content exceeds maximum length of 10000 characters",
		},
		{
			name: "SourceToolが空",
			memory: Memory{
				Content:    "有効なコンテンツ",
				SourceTool: "",
			},
			wantErr: true,
			errMsg:  "source_tool is required",
		},
		{
			name: "Tagsがちょうど10個",
			memory: Memory{
				Content:    "有効なコンテンツ",
				SourceTool: "antigravity",
				Tags:       []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10"},
			},
			wantErr: false,
		},
		{
			name: "Tagsが11個で制限超過",
			memory: Memory{
				Content:    "有効なコンテンツ",
				SourceTool: "antigravity",
				Tags:       []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11"},
			},
			wantErr: true,
			errMsg:  "tags exceed maximum count of 10",
		},
		{
			name: "Importanceが負の値 (-1)",
			memory: Memory{
				Content:    "有効なコンテンツ",
				SourceTool: "antigravity",
				Importance: -1,
			},
			wantErr: true,
			errMsg:  "importance must be between 0 and 10",
		},
		{
			name: "Importanceが上限超過 (11)",
			memory: Memory{
				Content:    "有効なコンテンツ",
				SourceTool: "antigravity",
				Importance: 11,
			},
			wantErr: true,
			errMsg:  "importance must be between 0 and 10",
		},
		{
			name: "Importanceが境界値 0 および 10",
			memory: Memory{
				Content:    "有効なコンテンツ",
				SourceTool: "antigravity",
				Importance: 10,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.memory.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Memory.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Memory.Validate() error message %q does not contain %q", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

func TestValidateContentLength(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "空文字列 (0文字)",
			content: "",
			wantErr: false,
		},
		{
			name:    "1文字の半角英字",
			content: "a",
			wantErr: false,
		},
		{
			name:    "1文字のマルチバイト文字 (日本語)",
			content: "あ",
			wantErr: false,
		},
		{
			name:    "1文字の絵文字",
			content: "😀",
			wantErr: false,
		},
		{
			name:    "境界値未満: 9999文字 (半角英字)",
			content: strings.Repeat("a", 9999),
			wantErr: false,
		},
		{
			name:    "境界値: 10000文字 (MaxContentLength - 半角英字)",
			content: strings.Repeat("a", 10000),
			wantErr: false,
		},
		{
			name:    "境界値超過: 10001文字 (半角英字)",
			content: strings.Repeat("a", 10001),
			wantErr: true,
			errMsg:  "content exceeds maximum length of 10000 characters (got 10001)",
		},
		{
			name:    "境界値: 10000ルーン (マルチバイト日本語)",
			content: strings.Repeat("あ", 10000),
			wantErr: false,
		},
		{
			name:    "境界値超過: 10001ルーン (マルチバイト日本語)",
			content: strings.Repeat("あ", 10001),
			wantErr: true,
			errMsg:  "content exceeds maximum length of 10000 characters (got 10001)",
		},
		{
			name:    "境界値: 10000ルーン (絵文字)",
			content: strings.Repeat("😀", 10000),
			wantErr: false,
		},
		{
			name:    "境界値超過: 10001ルーン (絵文字)",
			content: strings.Repeat("😀", 10001),
			wantErr: true,
			errMsg:  "content exceeds maximum length of 10000 characters (got 10001)",
		},
		{
			name:    "境界値: 10000ルーン (混在: 英数字、日本語、絵文字)",
			content: strings.Repeat("aあ😀", 3333) + "a",
			wantErr: false,
		},
		{
			name:    "境界値超過: 10001ルーン (混在: 英数字、日本語、絵文字)",
			content: strings.Repeat("aあ😀", 3333) + "aあ",
			wantErr: true,
			errMsg:  "content exceeds maximum length of 10000 characters (got 10001)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateContentLength(tt.content)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateContentLength() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errMsg != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateContentLength() error message %q does not contain %q", err, tt.errMsg)
				}
			}
		})
	}
}

func TestValidateIndividualHelpers(t *testing.T) {
	// ValidateContentLength は TestValidateContentLength で詳細に検証済み

	// ValidateTags
	if err := ValidateTags(make([]string, 10)); err != nil {
		t.Errorf("ValidateTags(10) returned unexpected error: %v", err)
	}
	if err := ValidateTags(make([]string, 11)); err == nil {
		t.Errorf("ValidateTags(11) expected error, got nil")
	}

	// ValidateImportance
	if err := ValidateImportance(0); err != nil {
		t.Errorf("ValidateImportance(0) returned unexpected error: %v", err)
	}
	if err := ValidateImportance(10); err != nil {
		t.Errorf("ValidateImportance(10) returned unexpected error: %v", err)
	}
	if err := ValidateImportance(-1); err == nil {
		t.Errorf("ValidateImportance(-1) expected error, got nil")
	}
	if err := ValidateImportance(11); err == nil {
		t.Errorf("ValidateImportance(11) expected error, got nil")
	}
}
