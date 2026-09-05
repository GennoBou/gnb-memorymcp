package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseAllowedOrigins(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]bool
	}{
		{
			name:     "empty input",
			input:    "",
			expected: nil,
		},
		{
			name:  "single origin",
			input: "https://example.com",
			expected: map[string]bool{
				"https://example.com": true,
			},
		},
		{
			name:  "multiple origins with spaces",
			input: "https://example.com, http://localhost:3000, https://app.example.com ",
			expected: map[string]bool{
				"https://example.com":     true,
				"http://localhost:3000":   true,
				"https://app.example.com": true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseAllowedOrigins(tt.input)
			if tt.expected == nil {
				if got != nil {
					t.Errorf("expected nil, got %v", got)
				}
				return
			}
			if len(got) != len(tt.expected) {
				t.Errorf("expected length %d, got %d", len(tt.expected), len(got))
			}
			for k, v := range tt.expected {
				if got[k] != v {
					t.Errorf("expected got[%s] == %v, got %v", k, v, got[k])
				}
			}
		})
	}
}

func TestDiscoveryHandlerCORS(t *testing.T) {
	auth0Domain := "test.auth0.com"
	allowedOrigins := parseAllowedOrigins("https://trusted.com,https://app.trusted.com")
	handler := makeDiscoveryHandler(auth0Domain, allowedOrigins)

	tests := []struct {
		name           string
		origin         string
		expectedCORS   string
		expectedVary   string
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
			name:         "trusted origin 2",
			origin:       "https://app.trusted.com",
			expectedCORS: "https://app.trusted.com",
			expectedVary: "Origin",
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
