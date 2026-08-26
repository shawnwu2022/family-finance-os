package auth

import (
	"bytes"
	"testing"
)

func TestSecretBoxRoundTripAndTamper(t *testing.T) {
	key := bytes.Repeat([]byte{0x42}, 32)
	box, err := NewSecretBox(key)
	if err != nil {
		t.Fatalf("NewSecretBox: %v", err)
	}
	plaintext := []byte("JBSWY3DPEHPK3PXP")
	ciphertext, err := box.Seal(plaintext)
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	if bytes.Equal(ciphertext, plaintext) {
		t.Fatal("Seal returned plaintext")
	}
	opened, err := box.Open(ciphertext)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if !bytes.Equal(opened, plaintext) {
		t.Fatalf("Open = %q want %q", opened, plaintext)
	}

	tampered := append([]byte(nil), ciphertext...)
	tampered[len(tampered)-1] ^= 0x01
	if _, err := box.Open(tampered); err == nil {
		t.Fatal("Open accepted tampered ciphertext")
	}
}

func TestSecretBoxRejectsInvalidKeyLength(t *testing.T) {
	for _, n := range []int{0, 16, 31, 33} {
		if _, err := NewSecretBox(make([]byte, n)); err == nil {
			t.Fatalf("NewSecretBox accepted key length %d", n)
		}
	}
}
