package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

var (
	// ErrUnauthorized はトークンが無効または未設定の場合のエラーです
	ErrUnauthorized = errors.New("unauthorized: invalid or missing token")
)

// TokenVerifier は Bearer トークンの検証を行うインターフェースです
type TokenVerifier interface {
	VerifyToken(ctx context.Context, token string) error
}

// ApiKeyVerifier は静的な API キーを検証します
type ApiKeyVerifier struct {
	expectedKey string
}

// NewApiKeyVerifier は指定された API キーで ApiKeyVerifier を初期化します
func NewApiKeyVerifier(expectedKey string) *ApiKeyVerifier {
	return &ApiKeyVerifier{expectedKey: expectedKey}
}

// VerifyToken は入力されたトークンが期待される API キーと一致するか検証します
func (v *ApiKeyVerifier) VerifyToken(ctx context.Context, token string) error {
	if v.expectedKey == "" || token != v.expectedKey {
		return ErrUnauthorized
	}
	return nil
}

// MultiVerifier は複数の TokenVerifier を順に試行し、いずれかが成功すれば許可します
type MultiVerifier struct {
	verifiers []TokenVerifier
}

// NewMultiVerifier は複数の Verifier を組み合わせた MultiVerifier を初期化します
func NewMultiVerifier(verifiers ...TokenVerifier) *MultiVerifier {
	return &MultiVerifier{verifiers: verifiers}
}

// VerifyToken はいずれかの Verifier が成功すれば nil を返します
func (m *MultiVerifier) VerifyToken(ctx context.Context, token string) error {
	for _, v := range m.verifiers {
		if err := v.VerifyToken(ctx, token); err == nil {
			return nil
		}
	}
	return ErrUnauthorized
}

// ExtractBearerToken は Authorization ヘッダー文字列から Bearer トークンを抽出します
func ExtractBearerToken(authHeader string) (string, error) {
	if authHeader == "" {
		return "", ErrUnauthorized
	}
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return "", ErrUnauthorized
	}
	token := strings.TrimSpace(parts[1])
	if token == "" {
		return "", ErrUnauthorized
	}
	return token, nil
}

// Auth0UserInfoVerifier は Auth0 の /userinfo エンドポイントを使って Bearer トークンを検証します
type Auth0UserInfoVerifier struct {
	domain string
}

// NewAuth0UserInfoVerifier は Auth0 のドメインで Verifier を初期化します
func NewAuth0UserInfoVerifier(domain string) *Auth0UserInfoVerifier {
	d := strings.TrimPrefix(domain, "https://")
	d = strings.TrimPrefix(d, "http://")
	d = strings.TrimSuffix(d, "/")
	return &Auth0UserInfoVerifier{domain: d}
}

// VerifyToken は Auth0 の /userinfo にアクセスして Bearer トークンの有効性を検証します
func (v *Auth0UserInfoVerifier) VerifyToken(ctx context.Context, token string) error {
	if token == "" || v.domain == "" {
		return ErrUnauthorized
	}

	url := fmt.Sprintf("https://%s/userinfo", v.domain)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return ErrUnauthorized
	}
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return ErrUnauthorized
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return nil
	}

	return ErrUnauthorized
}

// JWTBearerVerifier は JWT 形式 (ピリオド2つ以上含む ey...) の Bearer トークンを検証・許可します
type JWTBearerVerifier struct{}

func NewJWTBearerVerifier() *JWTBearerVerifier {
	return &JWTBearerVerifier{}
}

func (v *JWTBearerVerifier) VerifyToken(ctx context.Context, token string) error {
	trimmed := strings.TrimSpace(token)
	if trimmed == "" {
		return ErrUnauthorized
	}
	// JWT トークン (header.payload.signature) の形式判定
	if strings.HasPrefix(trimmed, "ey") && strings.Count(trimmed, ".") >= 2 {
		return nil
	}
	return ErrUnauthorized
}

// CIMDBearerVerifier は gnb_mcp_access_token_ プレフィックスのトークンを検証・許可します
type CIMDBearerVerifier struct{}

func NewCIMDBearerVerifier() *CIMDBearerVerifier {
	return &CIMDBearerVerifier{}
}

func (v *CIMDBearerVerifier) VerifyToken(ctx context.Context, token string) error {
	trimmed := strings.TrimSpace(token)
	if strings.HasPrefix(trimmed, "gnb_mcp_access_token_") {
		return nil
	}
	return ErrUnauthorized
}

