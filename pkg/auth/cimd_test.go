package auth

import (
	"encoding/json"
	"strings"
	"testing"
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
