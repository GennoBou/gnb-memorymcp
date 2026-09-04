package auth

import (
	"context"
	"testing"
	"time"
)

func TestExtractBearerToken(t *testing.T) {
	tests := []struct {
		name       string
		authHeader string
		wantToken  string
		wantErr    bool
	}{
		{"Valid Bearer Header", "Bearer my-secret-token", "my-secret-token", false},
		{"Case insensitive prefix", "bearer my-secret-token", "my-secret-token", false},
		{"Missing header", "", "", true},
		{"Invalid format", "Basic username:password", "", true},
		{"Empty token", "Bearer ", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := ExtractBearerToken(tt.authHeader)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ExtractBearerToken() error = %v, wantErr %v", err, tt.wantErr)
			}
			if token != tt.wantToken {
				t.Errorf("ExtractBearerToken() got = %v, want %v", token, tt.wantToken)
			}
		})
	}
}

func TestApiKeyVerifier(t *testing.T) {
	verifier := NewApiKeyVerifier("secret-123")
	ctx := context.Background()

	if err := verifier.VerifyToken(ctx, "secret-123"); err != nil {
		t.Errorf("expected success, got error: %v", err)
	}

	if err := verifier.VerifyToken(ctx, "wrong-key"); err == nil {
		t.Errorf("expected error for wrong key, got nil")
	}
}

func TestMultiVerifier(t *testing.T) {
	v1 := NewApiKeyVerifier("key1")
	v2 := NewApiKeyVerifier("key2")
	multi := NewMultiVerifier(v1, v2)
	ctx := context.Background()

	if err := multi.VerifyToken(ctx, "key1"); err != nil {
		t.Errorf("expected key1 to pass")
	}

	if err := multi.VerifyToken(ctx, "key2"); err != nil {
		t.Errorf("expected key2 to pass")
	}

	if err := multi.VerifyToken(ctx, "invalid"); err == nil {
		t.Errorf("expected invalid token to fail")
	}
}

func TestCIMDBearerVerifier(t *testing.T) {
	ctx := context.Background()
	verifier := NewCIMDBearerVerifier()

	// 1. Valid token issued via IssueCIMDToken
	validToken := "gnb_mcp_access_token_valid123456789"
	StoreCIMDToken(validToken, 10*time.Minute)

	if err := verifier.VerifyToken(ctx, validToken); err != nil {
		t.Errorf("expected valid issued token to pass verification, got: %v", err)
	}

	// 2. Fake token with matching prefix but not issued
	fakeToken := "gnb_mcp_access_token_fake_unauthorized"
	if err := verifier.VerifyToken(ctx, fakeToken); err == nil {
		t.Errorf("expected fake token with matching prefix to fail verification, got nil")
	}

	// 3. Expired token
	expiredToken := "gnb_mcp_access_token_expired123"
	StoreCIMDToken(expiredToken, -1*time.Minute)
	if err := verifier.VerifyToken(ctx, expiredToken); err == nil {
		t.Errorf("expected expired token to fail verification, got nil")
	}
}
