package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
)

var ErrInvalidSecretBoxKey = errors.New("auth secret encryption key must be exactly 32 bytes")

type SecretBox struct {
	aead cipher.AEAD
}

func NewSecretBox(key []byte) (*SecretBox, error) {
	if len(key) != 32 {
		return nil, ErrInvalidSecretBoxKey
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create auth secret cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create auth secret AEAD: %w", err)
	}
	return &SecretBox{aead: aead}, nil
}

func (b *SecretBox) Seal(plaintext []byte) ([]byte, error) {
	if b == nil || b.aead == nil {
		return nil, errors.New("auth secret box is not configured")
	}
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate auth secret nonce: %w", err)
	}
	out := make([]byte, 0, len(nonce)+len(plaintext)+b.aead.Overhead())
	out = append(out, nonce...)
	out = b.aead.Seal(out, nonce, plaintext, nil)
	return out, nil
}

func (b *SecretBox) Open(ciphertext []byte) ([]byte, error) {
	if b == nil || b.aead == nil {
		return nil, errors.New("auth secret box is not configured")
	}
	nonceSize := b.aead.NonceSize()
	if len(ciphertext) < nonceSize+b.aead.Overhead() {
		return nil, errors.New("invalid auth secret ciphertext")
	}
	plaintext, err := b.aead.Open(nil, ciphertext[:nonceSize], ciphertext[nonceSize:], nil)
	if err != nil {
		return nil, errors.New("invalid auth secret ciphertext")
	}
	return plaintext, nil
}
