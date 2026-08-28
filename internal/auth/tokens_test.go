package auth

import (
	"crypto/sha256"
	"encoding/base64"
	"testing"
)

func TestOpaqueTokenUses32RandomBytes(t *testing.T) {
	first, err := NewOpaqueToken()
	if err != nil {
		t.Fatalf("NewOpaqueToken first: %v", err)
	}
	second, err := NewOpaqueToken()
	if err != nil {
		t.Fatalf("NewOpaqueToken second: %v", err)
	}
	if first == second {
		t.Fatal("NewOpaqueToken returned duplicate values")
	}
	decoded, err := base64.RawURLEncoding.DecodeString(first)
	if err != nil {
		t.Fatalf("decode token: %v", err)
	}
	if len(decoded) != 32 {
		t.Fatalf("decoded token length=%d want=32", len(decoded))
	}

	got := HashOpaqueToken(first)
	want := sha256.Sum256([]byte(first))
	if got != want {
		t.Fatalf("HashOpaqueToken=%x want=%x", got, want)
	}
}
