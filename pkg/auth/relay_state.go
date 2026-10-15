package auth

import (
	"crypto/hmac"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrInvalidRelayState = errors.New("invalid or tampered relay state")
	ErrExpiredRelayState = errors.New("expired relay state")
)

type RelayState struct {
	RedirectURI string `json:"redirect_uri"`
	State       string `json:"state"`
	Timestamp   int64  `json:"ts"`
}

// EncodeRelayState は redirect_uri と state を HMAC 署名付きのステートレス文字列にエンコードします
func EncodeRelayState(redirectURI, state string) (string, error) {
	rs := RelayState{
		RedirectURI: redirectURI,
		State:       state,
		Timestamp:   time.Now().Unix(),
	}
	data, err := json.Marshal(rs)
	if err != nil {
		return "", fmt.Errorf("failed to marshal relay state: %w", err)
	}
	payloadB64 := base64.RawURLEncoding.EncodeToString(data)
	secret := getTokenSigningSecret()
	sig := computeHMAC(secret, payloadB64)
	return fmt.Sprintf("%s.%s", payloadB64, sig), nil
}

// DecodeRelayState は署名付きリレーステート文字列を検証・デコードして元の redirect_uri と state を復元します
func DecodeRelayState(encoded string) (string, string, error) {
	parts := strings.Split(encoded, ".")
	if len(parts) != 2 {
		return "", "", ErrInvalidRelayState
	}
	payloadB64 := parts[0]
	sigHex := parts[1]

	secret := getTokenSigningSecret()
	expectedSig := computeHMAC(secret, payloadB64)
	if !hmac.Equal([]byte(sigHex), []byte(expectedSig)) {
		return "", "", ErrInvalidRelayState
	}

	data, err := base64.RawURLEncoding.DecodeString(payloadB64)
	if err != nil {
		return "", "", ErrInvalidRelayState
	}

	var rs RelayState
	if err := json.Unmarshal(data, &rs); err != nil {
		return "", "", ErrInvalidRelayState
	}

	// 15 分以上の古いステートは期限切れ
	if time.Now().Unix()-rs.Timestamp > 15*60 {
		return "", "", ErrExpiredRelayState
	}

	return rs.RedirectURI, rs.State, nil
}
