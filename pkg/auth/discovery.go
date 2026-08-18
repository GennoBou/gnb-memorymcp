package auth

import (
	"encoding/json"
	"fmt"
	"strings"
)

// OAuthMetadata は RFC 8414 に準拠した OAuth 2.0 Authorization Server Metadata 構造体です
type OAuthMetadata struct {
	Issuer                                string   `json:"issuer"`
	AuthorizationEndpoint                string   `json:"authorization_endpoint"`
	TokenEndpoint                        string   `json:"token_endpoint"`
	UserinfoEndpoint                     string   `json:"userinfo_endpoint,omitempty"`
	JwksURI                              string   `json:"jwks_uri"`
	RegistrationEndpoint                  string   `json:"registration_endpoint,omitempty"`
	ResponseTypesSupported               []string `json:"response_types_supported"`
	GrantTypesSupported                  []string `json:"grant_types_supported"`
	TokenEndpointAuthMethodsSupported    []string `json:"token_endpoint_auth_methods_supported"`
	ScopesSupported                      []string `json:"scopes_supported"`
	CodeChallengeMethodsSupported        []string `json:"code_challenge_methods_supported"`
	TokenEndpointAuthSigningAlgSupported []string `json:"token_endpoint_auth_signing_alg_values_supported"`
}

// GetOAuthMetadata は Auth0 ドメインを基に OAuth ディスカバリー用メタデータを生成します
func GetOAuthMetadata(domain string) OAuthMetadata {
	domain = strings.TrimPrefix(domain, "https://")
	domain = strings.TrimPrefix(domain, "http://")
	domain = strings.TrimSuffix(domain, "/")

	baseURL := fmt.Sprintf("https://%s", domain)

	return OAuthMetadata{
		Issuer:                               fmt.Sprintf("%s/", baseURL),
		AuthorizationEndpoint:               fmt.Sprintf("%s/authorize", baseURL),
		TokenEndpoint:                       fmt.Sprintf("%s/oauth/token", baseURL),
		UserinfoEndpoint:                    fmt.Sprintf("%s/userinfo", baseURL),
		JwksURI:                             fmt.Sprintf("%s/.well-known/jwks.json", baseURL),
		ResponseTypesSupported:              []string{"code"},
		GrantTypesSupported:                 []string{"authorization_code", "refresh_token"},
		TokenEndpointAuthMethodsSupported:   []string{"client_secret_basic", "client_secret_post"},
		ScopesSupported:                     []string{"openid", "profile", "email", "offline_access"},
		CodeChallengeMethodsSupported:       []string{"S256"},
		TokenEndpointAuthSigningAlgSupported: []string{"RS256"},
	}
}

// GetOAuthMetadataJSON は Auth0 ドメインを基に JSON レスポンス用バイト列を返します
func GetOAuthMetadataJSON(domain string) ([]byte, error) {
	meta := GetOAuthMetadata(domain)
	return json.MarshalIndent(meta, "", "  ")
}
