package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

const opaqueTokenBytes = 32

func NewOpaqueToken() (string, error) {
	raw := make([]byte, opaqueTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate opaque auth token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func HashOpaqueToken(token string) [sha256.Size]byte {
	return sha256.Sum256([]byte(token))
}
