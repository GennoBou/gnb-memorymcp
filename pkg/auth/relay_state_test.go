package auth

import (
	"encoding/base64"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestRelayStateEncodeDecode(t *testing.T) {
	redirectURI := "https://antigravity.google/oauth-callback"
	state := "random-state-12345"

	encoded, err := EncodeRelayState(redirectURI, state)
	if err != nil {
		t.Fatalf("failed to encode relay state: %v", err)
	}

	gotURI, gotState, err := DecodeRelayState(encoded)
	if err != nil {
		t.Fatalf("failed to decode relay state: %v", err)
	}

	if gotURI != redirectURI {
		t.Errorf("expected redirectURI %q, got %q", redirectURI, gotURI)
	}
	if gotState != state {
		t.Errorf("expected state %q, got %q", state, gotState)
	}

	// 改ざん検証
	tampered := encoded + "x"
	if _, _, err := DecodeRelayState(tampered); err == nil {
		t.Errorf("tampered relay state should fail verification")
	}

	// 不正フォーマット
	if _, _, err := DecodeRelayState("invalid"); err == nil {
		t.Errorf("invalid format should fail verification")
	}
}

func TestDecodeRelayState_TableDriven(t *testing.T) {
	secret := getTokenSigningSecret()

	// 正常ケースの有効な JSON ペイロード
	validJSON := `{"redirect_uri":"https://example.com/cb","state":"abc","ts":` + fmt.Sprintf("%d", time.Now().Unix()) + `}`
	validB64 := base64.RawURLEncoding.EncodeToString([]byte(validJSON))
	validSig := computeHMAC(secret, validB64)

	// 期限切れの JSON ペイロード (20分前)
	expiredJSON := `{"redirect_uri":"https://example.com/cb","state":"abc","ts":` + fmt.Sprintf("%d", time.Now().Unix()-20*60) + `}`
	expiredB64 := base64.RawURLEncoding.EncodeToString([]byte(expiredJSON))
	expiredSig := computeHMAC(secret, expiredB64)

	// JSON フォーマット不正なペイロード (デコードは成功するが Unmarshal に失敗)
	invalidJSON := `not-a-json`
	invalidJSONB64 := base64.RawURLEncoding.EncodeToString([]byte(invalidJSON))
	invalidJSONSig := computeHMAC(secret, invalidJSONB64)

	// 不正な Base64 ペイロード（base64.RawURLEncoding.DecodeString でエラーを発生させる）
	invalidB64Payload := "!!!invalid_base64_chars!!!"
	invalidB64Sig := computeHMAC(secret, invalidB64Payload)

	tests := []struct {
		name        string
		encoded     string
		wantURI     string
		wantState   string
		wantErr     error
	}{
		{
			name:      "Valid relay state",
			encoded:   fmt.Sprintf("%s.%s", validB64, validSig),
			wantURI:   "https://example.com/cb",
			wantState: "abc",
			wantErr:   nil,
		},
		{
			name:    "Invalid format (no dot)",
			encoded: "invalidformat",
			wantErr: ErrInvalidRelayState,
		},
		{
			name:    "Invalid format (too many dots)",
			encoded: "part1.part2.part3",
			wantErr: ErrInvalidRelayState,
		},
		{
			name:    "Tampered signature",
			encoded: fmt.Sprintf("%s.%s", validB64, "invalid_signature_hex"),
			wantErr: ErrInvalidRelayState,
		},
		{
			name:    "Invalid base64 decode error",
			encoded: fmt.Sprintf("%s.%s", invalidB64Payload, invalidB64Sig),
			wantErr: ErrInvalidRelayState,
		},
		{
			name:    "Invalid JSON unmarshal error",
			encoded: fmt.Sprintf("%s.%s", invalidJSONB64, invalidJSONSig),
			wantErr: ErrInvalidRelayState,
		},
		{
			name:    "Expired relay state",
			encoded: fmt.Sprintf("%s.%s", expiredB64, expiredSig),
			wantErr: ErrExpiredRelayState,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotURI, gotState, err := DecodeRelayState(tt.encoded)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("DecodeRelayState() error = %v, wantErr %v", err, tt.wantErr)
				}
			} else {
				if err != nil {
					t.Errorf("DecodeRelayState() unexpected error = %v", err)
				}
				if gotURI != tt.wantURI {
					t.Errorf("DecodeRelayState() gotURI = %v, want %v", gotURI, tt.wantURI)
				}
				if gotState != tt.wantState {
					t.Errorf("DecodeRelayState() gotState = %v, want %v", gotState, tt.wantState)
				}
			}
		})
	}
}
