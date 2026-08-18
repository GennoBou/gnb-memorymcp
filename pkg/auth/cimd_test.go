package auth

import (
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

	redirectURL := BuildAuthorizeRedirectURL("https://google.com/callback", "state123")
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
}
