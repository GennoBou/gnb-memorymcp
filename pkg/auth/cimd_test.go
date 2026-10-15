package auth

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestCIMDOAuthMetadata(t *testing.T) {
	baseURL := "https://oajuohz4tofchqb4hy62yzq65m0paryw.lambda-url.ap-northeast-3.on.aws"
	meta := GetCIMDOAuthMetadata(baseURL)

	if meta.AuthorizationEndpoint != "https://oajuohz4tofchqb4hy62yzq65m0paryw.lambda-url.ap-northeast-3.on.aws/authorize" {
		t.Errorf("unexpected authorization endpoint: %s", meta.AuthorizationEndpoint)
	}

	if meta.TokenEndpoint != "https://oajuohz4tofchqb4hy62yzq65m0paryw.lambda-url.ap-northeast-3.on.aws/token" {
		t.Errorf("unexpected token endpoint: %s", meta.TokenEndpoint)
	}

	redirectURL, err := BuildAuthorizeRedirectURL("https://google.com/callback", "state123")
	if err != nil {
		t.Fatalf("failed to build authorize redirect URL: %v", err)
	}
	if !strings.Contains(redirectURL, "code=gnb_code_") || !strings.Contains(redirectURL, "state=state123") {
		t.Errorf("unexpected redirect URL format: %s", redirectURL)
	}

	tokenBytes, err := IssueCIMDToken()
	if err != nil {
		t.Fatalf("failed to issue CIMD token: %v", err)
	}

	if !strings.Contains(string(tokenBytes), "gnb_mcp_access_token_") {
		t.Errorf("token response missing expected access_token prefix: %s", string(tokenBytes))
	}

	var resp TokenResponse
	if err := json.Unmarshal(tokenBytes, &resp); err != nil {
		t.Fatalf("failed to unmarshal token response: %v", err)
	}

	if !ValidateCIMDToken(resp.AccessToken) {
		t.Errorf("issued token %s was not validated by ValidateCIMDToken", resp.AccessToken)
	}
}

func TestGenerateRandomHex(t *testing.T) {
	hex1, err := generateRandomHex(16)
	if err != nil {
		t.Fatalf("generateRandomHex(16) returned error: %v", err)
	}
	if len(hex1) != 32 {
		t.Errorf("expected hex string length 32 for 16 bytes, got %d", len(hex1))
	}

	hex2, err := generateRandomHex(16)
	if err != nil {
		t.Fatalf("generateRandomHex(16) returned error: %v", err)
	}

	if hex1 == hex2 {
		t.Errorf("generateRandomHex generated identical consecutive strings: %s", hex1)
	}
}

func TestIssueDCRRegistrationResponse(t *testing.T) {
	reqBody := []byte(`{"client_name":"test-client","redirect_uris":["https://example.com/callback"]}`)
	respBytes, err := IssueDCRRegistrationResponse(reqBody)
	if err != nil {
		t.Fatalf("IssueDCRRegistrationResponse failed: %v", err)
	}
	respStr := string(respBytes)
	if !strings.Contains(respStr, "gnb_dcr_client_") || !strings.Contains(respStr, "gnb_dcr_secret_") {
		t.Errorf("unexpected DCR response: %s", respStr)
	}
}

func TestCIMDStatelessTokenValidation(t *testing.T) {
	tokenBytes, err := IssueCIMDToken()
	if err != nil {
		t.Fatalf("failed to issue CIMD token: %v", err)
	}

	var resp TokenResponse
	if err := json.Unmarshal(tokenBytes, &resp); err != nil {
		t.Fatalf("failed to unmarshal token: %v", err)
	}

	// 1. 通常の検証 (インメモリ / ステートレス署名)
	if !ValidateCIMDToken(resp.AccessToken) {
		t.Errorf("token validation failed initially")
	}

	// 2. インメモリマップを意図的にクリア（Lambda 再起動・別インスタンスを模倣）
	globalTokenStore.mu.Lock()
	globalTokenStore.tokens = make(map[string]time.Time)
	globalTokenStore.mu.Unlock()

	// インメモリが空でも、HMAC 署名によってステートレスに検証成功すること
	if !ValidateCIMDToken(resp.AccessToken) {
		t.Errorf("stateless token validation failed after memory store cleared")
	}

	// 3. 改ざんされたトークンは失敗すること
	tampered := resp.AccessToken + "x"
	if ValidateCIMDToken(tampered) {
		t.Errorf("tampered token should not validate")
	}

	// 4. 不正なプレフィックス
	if ValidateCIMDToken("invalid_token_format") {
		t.Errorf("invalid token should not validate")
	}
}

func TestCIMDStatelessCodeValidation(t *testing.T) {
	secret := getTokenSigningSecret()
	code, err := GenerateStatelessCode(secret, 10*time.Minute)
	if err != nil {
		t.Fatalf("failed to generate code: %v", err)
	}

	// 1. 通常検証
	if !ValidateCIMDCode(code) {
		t.Errorf("code validation failed initially")
	}

	// 2. インメモリストアをクリア
	globalCodeStore.mu.Lock()
	globalCodeStore.codes = make(map[string]time.Time)
	globalCodeStore.mu.Unlock()

	// インメモリが空でも検証成功すること
	if !ValidateCIMDCode(code) {
		t.Errorf("stateless code validation failed after memory store cleared")
	}

	// 3. 改ざんされたコード
	if ValidateCIMDCode(code + "bad") {
		t.Errorf("tampered code should not validate")
	}
}
