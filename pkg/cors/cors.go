package cors

import (
	"net/http"
	"net/url"
	"strings"
)

// AllowedOrigins は許可されたオリジンの一覧を管理します。
type AllowedOrigins struct {
	allowAll bool
	origins  map[string]bool
}

// ParseAllowedOrigins はカンマ区切りのオリジン文字列を解析して AllowedOrigins を返します。
// 空文字列または有効なオリジンが存在しない場合は nil を返します。
func ParseAllowedOrigins(originsStr string) *AllowedOrigins {
	if strings.TrimSpace(originsStr) == "" {
		return nil
	}
	ao := &AllowedOrigins{
		origins: make(map[string]bool),
	}
	for _, o := range strings.Split(originsStr, ",") {
		normalized, ok := NormalizeOrigin(o)
		if !ok {
			continue
		}
		if normalized == "*" {
			ao.allowAll = true
		} else {
			ao.origins[normalized] = true
		}
	}
	if !ao.allowAll && len(ao.origins) == 0 {
		return nil
	}
	return ao
}

// NormalizeOrigin はオリジン文字列をスキームおよびホスト（ポート含む）の正規形式に変換します。
// 不正な URL、サポート外のスキーム、パスやクエリ、認証情報が含まれる場合は false を返します。
func NormalizeOrigin(originStr string) (string, bool) {
	trimmed := strings.TrimSpace(originStr)
	if trimmed == "" {
		return "", false
	}
	if trimmed == "*" {
		return "*", true
	}
	u, err := url.Parse(trimmed)
	if err != nil {
		return "", false
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", false
	}
	if u.Host == "" {
		return "", false
	}
	if u.Path != "" && u.Path != "/" {
		return "", false
	}
	if u.RawQuery != "" || u.Fragment != "" || u.User != nil {
		return "", false
	}
	return scheme + "://" + strings.ToLower(u.Host), true
}

// CheckOrigin は指定されたオリジンが許可されているかを検証し、
// 許可されている場合は (true, 設定すべきAccess-Control-Allow-Origin値) を返します。
func (ao *AllowedOrigins) CheckOrigin(origin string) (bool, string) {
	if ao == nil {
		return false, ""
	}
	if ao.allowAll {
		return true, "*"
	}
	if origin == "" {
		return false, ""
	}
	normalized, ok := NormalizeOrigin(origin)
	if !ok || normalized == "*" {
		return false, ""
	}
	if ao.origins[normalized] {
		return true, origin
	}
	return false, ""
}

// ApplyHeaders は http.ResponseWriter に CORS ヘッダーを設定します。
func (ao *AllowedOrigins) ApplyHeaders(w http.ResponseWriter, r *http.Request) {
	if ao == nil {
		return
	}
	origin := r.Header.Get("Origin")
	allowed, headerVal := ao.CheckOrigin(origin)
	if !allowed {
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", headerVal)
	if headerVal != "*" {
		w.Header().Set("Vary", "Origin")
	}
}

// GetCORSHeaders は Lambda などのヘッダーマップ向けに CORS レスポンスヘッダーを返します。
func (ao *AllowedOrigins) GetCORSHeaders(origin string) map[string]string {
	res := make(map[string]string)
	if ao == nil {
		return res
	}
	allowed, headerVal := ao.CheckOrigin(origin)
	if !allowed {
		return res
	}
	res["Access-Control-Allow-Origin"] = headerVal
	if headerVal != "*" {
		res["Vary"] = "Origin"
	}
	return res
}
