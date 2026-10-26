package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	generatedSecret     string
	generatedSecretOnce sync.Once
)

func getGeneratedSecret() string {
	generatedSecretOnce.Do(func() {
		sec, err := generateRandomHex(32)
		if err != nil {
			panic("failed to generate random token signing secret: " + err.Error())
		}
		generatedSecret = sec
	})
	return generatedSecret
}

func getTokenSigningSecret() string {
	if sec := os.Getenv("TOKEN_SECRET"); sec != "" {
		return sec
	}
	if sec := os.Getenv("API_KEY"); sec != "" {
		return sec
	}
	return getGeneratedSecret()
}

func computeHMAC(secret, data string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}

// CodeStore は一時的な認可コードを保存するためのインメモリマップです
type CodeStore struct {
	mu    sync.Mutex
	codes map[string]time.Time
}

var globalCodeStore = &CodeStore{
	codes: make(map[string]time.Time),
}

// TokenStore は発行されたアクセストークンを保存するためのインメモリマップです
type TokenStore struct {
	mu     sync.Mutex
	tokens map[string]time.Time
}

var globalTokenStore = &TokenStore{
	tokens: make(map[string]time.Time),
}

// StoreCIMDToken はアクセストークンとその有効期限を保存します
func StoreCIMDToken(token string, expiry time.Duration) {
	globalTokenStore.mu.Lock()
	defer globalTokenStore.mu.Unlock()

	globalTokenStore.tokens[token] = time.Now().Add(expiry)
}

// GenerateStatelessToken は HMAC 署名付きのステートレスなアクセストークンを生成します
func GenerateStatelessToken(secret string, expiry time.Duration) (string, error) {
	randHex, err := generateRandomHex(16)
	if err != nil {
		return "", err
	}
	expUnix := time.Now().Add(expiry).Unix()
	payload := fmt.Sprintf("%s:%d", randHex, expUnix)
	sig := computeHMAC(secret, payload)
	return fmt.Sprintf("gnb_mcp_access_token_%s_%d_%s", randHex, expUnix, sig), nil
}

// GenerateStatelessCode は HMAC 署名付きのステートレスな認可コードを生成します
func GenerateStatelessCode(secret string, expiry time.Duration) (string, error) {
	randHex, err := generateRandomHex(16)
	if err != nil {
		return "", err
	}
	expUnix := time.Now().Add(expiry).Unix()
	payload := fmt.Sprintf("%s:%d", randHex, expUnix)
	sig := computeHMAC(secret, payload)
	return fmt.Sprintf("gnb_code_%s_%d_%s", randHex, expUnix, sig), nil
}

// ValidateCIMDToken は指定されたアクセストークンが有効（インメモリまたはHMAC署名有効かつ未期限切れ）か検証します
func ValidateCIMDToken(token string) bool {
	if !strings.HasPrefix(token, "gnb_mcp_access_token_") {
		return false
	}

	// 1. インメモリマップをチェック
	globalTokenStore.mu.Lock()
	exp, ok := globalTokenStore.tokens[token]
	if ok {
		if time.Now().Before(exp) {
			globalTokenStore.mu.Unlock()
			return true
		}
		delete(globalTokenStore.tokens, token)
	}
	globalTokenStore.mu.Unlock()

	// 2. ステートレス署名検証 (gnb_mcp_access_token_<randHex>_<expUnix>_<sig>)
	trimmed := strings.TrimPrefix(token, "gnb_mcp_access_token_")
	parts := strings.Split(trimmed, "_")
	if len(parts) == 3 {
		randHex := parts[0]
		expUnixStr := parts[1]
		sigHex := parts[2]

		expUnix, err := strconv.ParseInt(expUnixStr, 10, 64)
		if err != nil {
			return false
		}

		if time.Now().Unix() > expUnix {
			return false // 期限切れ
		}

		secret := getTokenSigningSecret()
		expectedSig := computeHMAC(secret, fmt.Sprintf("%s:%s", randHex, expUnixStr))
		if hmac.Equal([]byte(sigHex), []byte(expectedSig)) {
			return true
		}
	}

	return false
}

// ValidateCIMDCode は指定された認可コードが有効か検証します
func ValidateCIMDCode(code string) bool {
	if !strings.HasPrefix(code, "gnb_code_") {
		return false
	}

	globalCodeStore.mu.Lock()
	exp, ok := globalCodeStore.codes[code]
	if ok {
		if time.Now().Before(exp) {
			globalCodeStore.mu.Unlock()
			return true
		}
		delete(globalCodeStore.codes, code)
	}
	globalCodeStore.mu.Unlock()

	trimmed := strings.TrimPrefix(code, "gnb_code_")
	parts := strings.Split(trimmed, "_")
	if len(parts) == 3 {
		randHex := parts[0]
		expUnixStr := parts[1]
		sigHex := parts[2]

		expUnix, err := strconv.ParseInt(expUnixStr, 10, 64)
		if err != nil {
			return false
		}

		if time.Now().Unix() > expUnix {
			return false
		}

		secret := getTokenSigningSecret()
		expectedSig := computeHMAC(secret, fmt.Sprintf("%s:%s", randHex, expUnixStr))
		if hmac.Equal([]byte(sigHex), []byte(expectedSig)) {
			return true
		}
	}

	return false
}

func generateRandomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate secure random bytes: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// GetCIMDOAuthMetadata は MCP サーバー自身の URL を基に CIMD 対応の OAuth メタデータを生成します
func GetCIMDOAuthMetadata(mcpBaseURL string) OAuthMetadata {
	mcpBaseURL = strings.TrimSuffix(mcpBaseURL, "/")

	return OAuthMetadata{
		Issuer:                               mcpBaseURL,
		AuthorizationEndpoint:                fmt.Sprintf("%s/authorize", mcpBaseURL),
		TokenEndpoint:                        fmt.Sprintf("%s/token", mcpBaseURL),
		UserinfoEndpoint:                     fmt.Sprintf("%s/userinfo", mcpBaseURL),
		JwksURI:                              fmt.Sprintf("%s/.well-known/jwks.json", mcpBaseURL),
		RegistrationEndpoint:                 fmt.Sprintf("%s/register", mcpBaseURL),
		ResponseTypesSupported:               []string{"code"},
		GrantTypesSupported:                  []string{"authorization_code", "refresh_token"},
		TokenEndpointAuthMethodsSupported:    []string{"none", "client_secret_basic", "client_secret_post"},
		ScopesSupported:                      []string{"openid", "profile", "email", "offline_access", "mcp"},
		CodeChallengeMethodsSupported:        []string{"S256", "plain"},
		TokenEndpointAuthSigningAlgSupported: []string{"RS256", "HS256"},
	}
}

type DCRRequest struct {
	ClientName              string   `json:"client_name"`
	RedirectURIs            []string `json:"redirect_uris"`
	GrantTypes              []string `json:"grant_types"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
}

type DCRResponse struct {
	ClientID                string   `json:"client_id"`
	ClientSecret            string   `json:"client_secret,omitempty"`
	ClientName              string   `json:"client_name,omitempty"`
	RedirectURIs            []string `json:"redirect_uris,omitempty"`
	GrantTypes              []string `json:"grant_types,omitempty"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method,omitempty"`
}

// IssueDCRRegistrationResponse は Dynamic Client Registration (RFC 7591) 用の完全なレスポンスを返します
func IssueDCRRegistrationResponse(reqBytes []byte) ([]byte, error) {
	var dcrReq DCRRequest
	_ = json.Unmarshal(reqBytes, &dcrReq)

	redirectURIs := dcrReq.RedirectURIs
	if len(redirectURIs) == 0 {
		redirectURIs = []string{"https://oauth-redirect.googleusercontent.com/r/mcp"}
	}

	grantTypes := dcrReq.GrantTypes
	if len(grantTypes) == 0 {
		grantTypes = []string{"authorization_code", "refresh_token"}
	}

	authMethod := dcrReq.TokenEndpointAuthMethod
	if authMethod == "" {
		authMethod = "client_secret_post"
	}

	clientName := dcrReq.ClientName
	if clientName == "" {
		clientName = "Google"
	}

	clientIDHex, err := generateRandomHex(16)
	if err != nil {
		return nil, fmt.Errorf("failed to generate client id: %w", err)
	}
	clientSecretHex, err := generateRandomHex(24)
	if err != nil {
		return nil, fmt.Errorf("failed to generate client secret: %w", err)
	}

	resp := DCRResponse{
		ClientID:                "gnb_dcr_client_" + clientIDHex,
		ClientSecret:            "gnb_dcr_secret_" + clientSecretHex,
		ClientName:              clientName,
		RedirectURIs:            redirectURIs,
		GrantTypes:              grantTypes,
		TokenEndpointAuthMethod: authMethod,
	}
	return json.Marshal(resp)
}

// GetCIMDOAuthMetadataJSON は JSON バイト列を返します
func GetCIMDOAuthMetadataJSON(mcpBaseURL string) ([]byte, error) {
	meta := GetCIMDOAuthMetadata(mcpBaseURL)
	return json.MarshalIndent(meta, "", "  ")
}

// BuildAuthorizeRedirectURL は /authorize リクエストから認可コードを生成してリダイレクト先 URL を組み立てます
func BuildAuthorizeRedirectURL(redirectURI, state string) (string, error) {
	secret := getTokenSigningSecret()
	code, err := GenerateStatelessCode(secret, 10*time.Minute)
	if err != nil {
		return "", fmt.Errorf("failed to generate authorization code: %w", err)
	}
	globalCodeStore.mu.Lock()
	globalCodeStore.codes[code] = time.Now().Add(10 * time.Minute)
	globalCodeStore.mu.Unlock()

	sep := "?"
	if strings.Contains(redirectURI, "?") {
		sep = "&"
	}
	return fmt.Sprintf("%s%scode=%s&state=%s", redirectURI, sep, code, state), nil
}

// ValidateAndIssueToken は認可コードをチェックして Access Token を発行します
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

func IssueCIMDToken() ([]byte, error) {
	secret := getTokenSigningSecret()
	expiresInSec := 3600 * 24 * 365 // 1 年間有効
	expiry := time.Duration(expiresInSec) * time.Second

	accessToken, err := GenerateStatelessToken(secret, expiry)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	refreshTokenHex, err := generateRandomHex(24)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	StoreCIMDToken(accessToken, expiry)

	tokenResp := TokenResponse{
		AccessToken:  accessToken,
		TokenType:    "Bearer",
		ExpiresIn:    expiresInSec,
		RefreshToken: "gnb_mcp_refresh_token_" + refreshTokenHex,
	}
	return json.Marshal(tokenResp)
}
