package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
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

func TestAuth0UserInfoVerifier(t *testing.T) {
	t.Run("Domain normalization", func(t *testing.T) {
		tests := []struct {
			input    string
			expected string
		}{
			{"example.auth0.com", "example.auth0.com"},
			{"https://example.auth0.com/", "example.auth0.com"},
			{"http://example.auth0.com/", "example.auth0.com"},
		}

		for _, tt := range tests {
			v := NewAuth0UserInfoVerifier(tt.input)
			if v.domain != tt.expected {
				t.Errorf("NewAuth0UserInfoVerifier(%q) domain = %q, want %q", tt.input, v.domain, tt.expected)
			}
		}
	})

	t.Run("Empty token or domain returns ErrUnauthorized", func(t *testing.T) {
		ctx := context.Background()

		vEmptyDomain := NewAuth0UserInfoVerifier("")
		if err := vEmptyDomain.VerifyToken(ctx, "valid-token"); err != ErrUnauthorized {
			t.Errorf("expected ErrUnauthorized for empty domain, got %v", err)
		}

		vValid := NewAuth0UserInfoVerifier("dev.auth0.com")
		if err := vValid.VerifyToken(ctx, ""); err != ErrUnauthorized {
			t.Errorf("expected ErrUnauthorized for empty token, got %v", err)
		}
	})

	t.Run("VerifyToken with HTTP test server", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/userinfo" {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			authHeader := r.Header.Get("Authorization")
			if authHeader == "Bearer valid-token" {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"sub": "auth0|12345"}`))
			} else {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"error": "Unauthorized"}`))
			}
		}))
		defer ts.Close()

		verifier := NewAuth0UserInfoVerifier("unused-domain.com",
			WithAuth0BaseURL(ts.URL),
			WithAuth0HTTPClient(ts.Client()),
		)

		ctx := context.Background()

		// Valid token
		if err := verifier.VerifyToken(ctx, "valid-token"); err != nil {
			t.Errorf("expected nil error for valid token, got %v", err)
		}

		// Invalid token
		if err := verifier.VerifyToken(ctx, "invalid-token"); err != ErrUnauthorized {
			t.Errorf("expected ErrUnauthorized for invalid token, got %v", err)
		}
	})

	t.Run("VerifyToken handles server errors and network failure", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))

		verifier := NewAuth0UserInfoVerifier("unused-domain.com",
			WithAuth0BaseURL(ts.URL),
			WithAuth0HTTPClient(ts.Client()),
		)
		ts.Close() // Close immediately so request fails with network error

		ctx := context.Background()

		if err := verifier.VerifyToken(ctx, "valid-token"); err != ErrUnauthorized {
			t.Errorf("expected ErrUnauthorized on network failure, got %v", err)
		}
	})
}
