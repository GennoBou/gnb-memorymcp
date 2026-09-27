package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gennobou/gnb-memorymcp/pkg/cors"
)

func TestParseAllowedOrigins(t *testing.T) {
	ao := cors.ParseAllowedOrigins("https://example.com, http://localhost:3000")
	if ao == nil {
		t.Fatalf("expected non-nil AllowedOrigins")
	}
	if allowed, _ := ao.CheckOrigin("https://example.com"); !allowed {
		t.Errorf("expected https://example.com to be allowed")
	}
	if allowed, _ := ao.CheckOrigin("http://localhost:3000"); !allowed {
		t.Errorf("expected http://localhost:3000 to be allowed")
	}
	if allowed, _ := ao.CheckOrigin("https://untrusted.com"); allowed {
		t.Errorf("expected https://untrusted.com to not be allowed")
	}
}

func TestDiscoveryHandlerCORS(t *testing.T) {
	auth0Domain := "test.auth0.com"
	allowedOrigins := cors.ParseAllowedOrigins("https://trusted.com,https://app.trusted.com")
	handler := makeDiscoveryHandler(auth0Domain, allowedOrigins)

	tests := []struct {
		name         string
		origin       string
		expectedCORS string
		expectedVary string
	}{
		{
			name:         "no origin header",
			origin:       "",
			expectedCORS: "",
			expectedVary: "",
		},
		{
			name:         "untrusted origin",
			origin:       "https://evil.com",
			expectedCORS: "",
			expectedVary: "",
		},
		{
			name:         "trusted origin 1",
			origin:       "https://trusted.com",
			expectedCORS: "https://trusted.com",
			expectedVary: "Origin",
		},
		{
			name:         "trusted origin 2 with uppercase scheme/host and trailing slash in request",
			origin:       "HTTPS://APP.TRUSTED.COM/",
			expectedCORS: "HTTPS://APP.TRUSTED.COM/",
			expectedVary: "Origin",
		},
		{
			name:         "origin with path attempted attack",
			origin:       "https://trusted.com/evilpath",
			expectedCORS: "",
			expectedVary: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			resp := w.Result()
			if resp.StatusCode != http.StatusOK {
				t.Errorf("expected status OK, got %d", resp.StatusCode)
			}

			corsHeader := resp.Header.Get("Access-Control-Allow-Origin")
			if corsHeader != tt.expectedCORS {
				t.Errorf("expected Access-Control-Allow-Origin '%s', got '%s'", tt.expectedCORS, corsHeader)
			}

			varyHeader := resp.Header.Get("Vary")
			if varyHeader != tt.expectedVary {
				t.Errorf("expected Vary '%s', got '%s'", tt.expectedVary, varyHeader)
			}
		})
	}
}

func TestDiscoveryHandlerWildcardCORS(t *testing.T) {
	auth0Domain := "test.auth0.com"
	allowedOrigins := cors.ParseAllowedOrigins("*")
	handler := makeDiscoveryHandler(auth0Domain, allowedOrigins)

	req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", nil)
	req.Header.Set("Origin", "https://anyorigin.com")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()
	corsHeader := resp.Header.Get("Access-Control-Allow-Origin")
	if corsHeader != "*" {
		t.Errorf("expected Access-Control-Allow-Origin '*', got '%s'", corsHeader)
	}
	varyHeader := resp.Header.Get("Vary")
	if varyHeader != "" {
		t.Errorf("expected empty Vary header for wildcard, got '%s'", varyHeader)
	}
}

func TestDiscoveryHandlerNoAllowedOriginsConfigured(t *testing.T) {
	auth0Domain := "test.auth0.com"
	handler := makeDiscoveryHandler(auth0Domain, nil)

	req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()
	corsHeader := resp.Header.Get("Access-Control-Allow-Origin")
	if corsHeader != "" {
		t.Errorf("expected empty Access-Control-Allow-Origin when no allowed origins configured, got '%s'", corsHeader)
	}
}

func TestValidateAPIKey(t *testing.T) {
	tests := []struct {
		name    string
		apiKey  string
		host    string
		wantErr bool
	}{
		{
			name:    "empty api key on localhost",
			apiKey:  "",
			host:    "127.0.0.1",
			wantErr: true,
		},
		{
			name:    "empty api key on 0.0.0.0",
			apiKey:  "",
			host:    "0.0.0.0",
			wantErr: true,
		},
		{
			name:    "dev-key on 0.0.0.0",
			apiKey:  "dev-key",
			host:    "0.0.0.0",
			wantErr: true,
		},
		{
			name:    "dev-key on 127.0.0.1",
			apiKey:  "dev-key",
			host:    "127.0.0.1",
			wantErr: false,
		},
		{
			name:    "custom secure api key on 127.0.0.1",
			apiKey:  "my-secret-key",
			host:    "127.0.0.1",
			wantErr: false,
		},
		{
			name:    "custom secure api key on 0.0.0.0",
			apiKey:  "my-secret-key",
			host:    "0.0.0.0",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAPIKey(tt.apiKey, tt.host)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateAPIKey() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
