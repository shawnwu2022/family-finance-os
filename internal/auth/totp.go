package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	totpStepSeconds = int64(30)
	totpDigits      = 6
	totpSecretBytes = 20
)

var ErrInvalidTOTP = errors.New("invalid or replayed TOTP code")

func GenerateTOTPSecret() (string, error) {
	secret := make([]byte, totpSecretBytes)
	if _, err := rand.Read(secret); err != nil {
		return "", fmt.Errorf("generate TOTP secret: %w", err)
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(secret), nil
}

func TOTPCode(secret string, at time.Time) (string, int64, error) {
	if at.Unix() < 0 {
		return "", 0, errors.New("TOTP time must not be before Unix epoch")
	}
	decoded, err := decodeTOTPSecret(secret)
	if err != nil {
		return "", 0, err
	}
	counter := at.Unix() / totpStepSeconds
	return totpCodeForCounter(decoded, counter), counter, nil
}

func VerifyTOTP(secret, code string, at time.Time, lastCounter int64) (int64, error) {
	if at.Unix() < 0 || len(code) != totpDigits {
		return 0, ErrInvalidTOTP
	}
	for i := range code {
		if code[i] < '0' || code[i] > '9' {
			return 0, ErrInvalidTOTP
		}
	}
	decoded, err := decodeTOTPSecret(secret)
	if err != nil {
		return 0, ErrInvalidTOTP
	}
	current := at.Unix() / totpStepSeconds
	for _, offset := range []int64{0, -1, 1} {
		candidateCounter := current + offset
		if candidateCounter < 0 || candidateCounter <= lastCounter {
			continue
		}
		candidate := totpCodeForCounter(decoded, candidateCounter)
		if subtle.ConstantTimeCompare([]byte(candidate), []byte(code)) == 1 {
			return candidateCounter, nil
		}
	}
	return 0, ErrInvalidTOTP
}

func decodeTOTPSecret(secret string) ([]byte, error) {
	normalized := strings.ToUpper(strings.TrimSpace(secret))
	decoded, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(normalized)
	if err != nil || len(decoded) < 16 || len(decoded) > 64 {
		return nil, errors.New("invalid TOTP secret")
	}
	return decoded, nil
}

func totpCodeForCounter(secret []byte, counter int64) string {
	var message [8]byte
	binary.BigEndian.PutUint64(message[:], uint64(counter))
	mac := hmac.New(sha1.New, secret)
	_, _ = mac.Write(message[:])
	digest := mac.Sum(nil)
	offset := digest[len(digest)-1] & 0x0f
	binaryCode := (uint32(digest[offset])&0x7f)<<24 |
		uint32(digest[offset+1])<<16 |
		uint32(digest[offset+2])<<8 |
		uint32(digest[offset+3])
	return fmt.Sprintf("%06d", binaryCode%1_000_000)
}
