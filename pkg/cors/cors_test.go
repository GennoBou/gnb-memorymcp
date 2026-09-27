package cors

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseAllowedOrigins(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantNil    bool
		isAllowAll bool
		count      int
	}{
		{
			name:    "空文字",
			input:   "",
			wantNil: true,
		},
		{
			name:    "空白のみ",
			input:   "   ",
			wantNil: true,
		},
		{
			name:       "ワイルドカード単体",
			input:      "*",
			wantNil:    false,
			isAllowAll: true,
		},
		{
			name:       "単一の正規URL",
			input:      "https://example.com",
			wantNil:    false,
			isAllowAll: false,
			count:      1,
		},
		{
			name:       "複数オリジンと不正なURL混在",
			input:      "https://example.com, http://localhost:3000, not-a-url, ftp://bad.com",
			wantNil:    false,
			isAllowAll: false,
			count:      2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ao := ParseAllowedOrigins(tt.input)
			if tt.wantNil {
				if ao != nil {
					t.Errorf("ParseAllowedOrigins() = %v, want nil", ao)
				}
				return
			}
			if ao == nil {
				t.Fatalf("ParseAllowedOrigins() unexpectedly returned nil")
			}
			if ao.allowAll != tt.isAllowAll {
				t.Errorf("ao.allowAll = %v, want %v", ao.allowAll, tt.isAllowAll)
			}
			if !tt.isAllowAll && len(ao.origins) != tt.count {
				t.Errorf("len(ao.origins) = %d, want %d", len(ao.origins), tt.count)
			}
		})
	}
}

func TestNormalizeOrigin(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		want   string
		wantOk bool
	}{
		{name: "ワイルドカード", input: "*", want: "*", wantOk: true},
		{name: "標準的なhttps", input: "https://example.com", want: "https://example.com", wantOk: true},
		{name: "大文字ホスト", input: "HTTPS://EXAMPLE.COM", want: "https://example.com", wantOk: true},
		{name: "ポート付き", input: "http://localhost:8080", want: "http://localhost:8080", wantOk: true},
		{name: "末尾スラッシュ付き", input: "https://example.com/", want: "https://example.com", wantOk: true},
		{name: "パス付きは拒否", input: "https://example.com/foo", want: "", wantOk: false},
		{name: "クエリ付きは拒否", input: "https://example.com?query=1", want: "", wantOk: false},
		{name: "フラグメント付きは拒否", input: "https://example.com#section", want: "", wantOk: false},
		{name: "ユーザー情報付きは拒否", input: "https://user:pass@example.com", want: "", wantOk: false},
		{name: "未対応スキーム", input: "ftp://example.com", want: "", wantOk: false},
		{name: "空文字", input: "", want: "", wantOk: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := NormalizeOrigin(tt.input)
			if ok != tt.wantOk || got != tt.want {
				t.Errorf("NormalizeOrigin(%q) = (%q, %v), want (%q, %v)", tt.input, got, ok, tt.want, tt.wantOk)
			}
		})
	}
}

func TestCheckOrigin(t *testing.T) {
	ao := ParseAllowedOrigins("https://app.example.com, http://localhost:3000")

	tests := []struct {
		name        string
		origin      string
		wantAllowed bool
		wantVal     string
	}{
		{
			name:        "許可されたオリジン (app.example.com)",
			origin:      "https://app.example.com",
			wantAllowed: true,
			wantVal:     "https://app.example.com",
		},
		{
			name:        "大文字混じりのリクエストオリジン",
			origin:      "HTTP://LOCALHOST:3000",
			wantAllowed: true,
			wantVal:     "HTTP://LOCALHOST:3000",
		},
		{
			name:        "未許可のオリジン",
			origin:      "https://evil.com",
			wantAllowed: false,
			wantVal:     "",
		},
		{
			name:        "リクエストオリジンにワイルドカード偽装",
			origin:      "*",
			wantAllowed: false,
			wantVal:     "",
		},
		{
			name:        "空文字オリジン",
			origin:      "",
			wantAllowed: false,
			wantVal:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, val := ao.CheckOrigin(tt.origin)
			if allowed != tt.wantAllowed || val != tt.wantVal {
				t.Errorf("CheckOrigin(%q) = (%v, %q), want (%v, %q)", tt.origin, allowed, val, tt.wantAllowed, tt.wantVal)
			}
		})
	}
}

func TestApplyHeaders(t *testing.T) {
	ao := ParseAllowedOrigins("https://example.com")

	// 許可されたオリジンの場合
	req1 := httptest.NewRequest(http.MethodGet, "/", nil)
	req1.Header.Set("Origin", "https://example.com")
	w1 := httptest.NewRecorder()
	ao.ApplyHeaders(w1, req1)
	if got := w1.Header().Get("Access-Control-Allow-Origin"); got != "https://example.com" {
		t.Errorf("ApplyHeaders() Allow-Origin = %q, want 'https://example.com'", got)
	}
	if got := w1.Header().Get("Vary"); got != "Origin" {
		t.Errorf("ApplyHeaders() Vary = %q, want 'Origin'", got)
	}

	// 拒否されたオリジンの場合
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.Header.Set("Origin", "https://attacker.com")
	w2 := httptest.NewRecorder()
	ao.ApplyHeaders(w2, req2)
	if got := w2.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("ApplyHeaders() unexpectedly set Allow-Origin = %q", got)
	}

	// ao が nil の場合
	var nilAo *AllowedOrigins
	w3 := httptest.NewRecorder()
	nilAo.ApplyHeaders(w3, req1)
	if got := w3.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("nil AllowedOrigins unexpectedly set Allow-Origin = %q", got)
	}
}

func TestGetCORSHeaders(t *testing.T) {
	ao := ParseAllowedOrigins("https://example.com")
	headers := ao.GetCORSHeaders("https://example.com")
	if headers["Access-Control-Allow-Origin"] != "https://example.com" {
		t.Errorf("GetCORSHeaders() = %v, want Access-Control-Allow-Origin: https://example.com", headers)
	}
	if headers["Vary"] != "Origin" {
		t.Errorf("GetCORSHeaders() = %v, want Vary: Origin", headers)
	}

	// 不許可オリジン
	unauthorized := ao.GetCORSHeaders("https://attacker.com")
	if len(unauthorized) != 0 {
		t.Errorf("GetCORSHeaders() for unauthorized origin returned %v, want empty map", unauthorized)
	}

	// ワイルドカードの場合
	aoWildcard := ParseAllowedOrigins("*")
	wildcardHeaders := aoWildcard.GetCORSHeaders("https://anywhere.com")
	if wildcardHeaders["Access-Control-Allow-Origin"] != "*" {
		t.Errorf("GetCORSHeaders() for wildcard = %v, want '*'", wildcardHeaders)
	}
	if _, ok := wildcardHeaders["Vary"]; ok {
		t.Errorf("GetCORSHeaders() for wildcard should not have Vary header")
	}
}
