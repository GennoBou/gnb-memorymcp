package auth

import (
	"strings"
	"testing"
)

func TestGetOAuthMetadata(t *testing.T) {
	domain := "gennobou.jp.auth0.com"
	meta := GetOAuthMetadata(domain)

	if meta.AuthorizationEndpoint != "https://gennobou.jp.auth0.com/authorize" {
		t.Errorf("unexpected authorization endpoint: %s", meta.AuthorizationEndpoint)
	}

	if meta.TokenEndpoint != "https://gennobou.jp.auth0.com/oauth/token" {
		t.Errorf("unexpected token endpoint: %s", meta.TokenEndpoint)
	}

	jsonBytes, err := GetOAuthMetadataJSON(domain)
	if err != nil {
		t.Fatalf("failed to marshal metadata: %v", err)
	}

	if !strings.Contains(string(jsonBytes), "https://gennobou.jp.auth0.com/authorize") {
		t.Errorf("JSON output missing authorization endpoint")
	}
}
