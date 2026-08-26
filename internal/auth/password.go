package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

const (
	passwordMinRunes = 14
	passwordMaxBytes = 128
	argonMemoryKiB   = 64 * 1024
	argonIterations  = 3
	argonParallelism = 1
	argonSaltBytes   = 16
	argonKeyBytes    = 32
)

var (
	ErrPasswordTooShort    = errors.New("password must contain at least 14 Unicode characters")
	ErrPasswordTooLong     = errors.New("password must not exceed 128 bytes")
	ErrInvalidPasswordHash = errors.New("invalid password hash")
)

func HashPassword(password string) (string, error) {
	if err := validateNewPassword(password); err != nil {
		return "", err
	}
	salt := make([]byte, argonSaltBytes)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, argonIterations, argonMemoryKiB, argonParallelism, argonKeyBytes)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		argonMemoryKiB,
		argonIterations,
		argonParallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

func VerifyPassword(encodedHash, password string) (bool, error) {
	if len([]byte(password)) > passwordMaxBytes {
		return false, ErrPasswordTooLong
	}
	params, salt, expected, err := parsePasswordHash(encodedHash)
	if err != nil {
		return false, err
	}
	actual := argon2.IDKey([]byte(password), salt, params.iterations, params.memoryKiB, params.parallelism, uint32(len(expected)))
	return subtle.ConstantTimeCompare(actual, expected) == 1, nil
}

func validateNewPassword(password string) error {
	if len([]byte(password)) > passwordMaxBytes {
		return ErrPasswordTooLong
	}
	if utf8.RuneCountInString(password) < passwordMinRunes {
		return ErrPasswordTooShort
	}
	return nil
}

type argonParameters struct {
	memoryKiB   uint32
	iterations  uint32
	parallelism uint8
}

func parsePasswordHash(encoded string) (argonParameters, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return argonParameters{}, nil, nil, ErrInvalidPasswordHash
	}
	version, err := parsePrefixedUint(parts[2], "v=", 32)
	if err != nil || version != argon2.Version {
		return argonParameters{}, nil, nil, ErrInvalidPasswordHash
	}
	paramParts := strings.Split(parts[3], ",")
	if len(paramParts) != 3 {
		return argonParameters{}, nil, nil, ErrInvalidPasswordHash
	}
	memory, err := parsePrefixedUint(paramParts[0], "m=", 32)
	if err != nil || memory != argonMemoryKiB {
		return argonParameters{}, nil, nil, ErrInvalidPasswordHash
	}
	iterations, err := parsePrefixedUint(paramParts[1], "t=", 32)
	if err != nil || iterations != argonIterations {
		return argonParameters{}, nil, nil, ErrInvalidPasswordHash
	}
	parallelism, err := parsePrefixedUint(paramParts[2], "p=", 8)
	if err != nil || parallelism != argonParallelism {
		return argonParameters{}, nil, nil, ErrInvalidPasswordHash
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) != argonSaltBytes {
		return argonParameters{}, nil, nil, ErrInvalidPasswordHash
	}
	key, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(key) != argonKeyBytes {
		return argonParameters{}, nil, nil, ErrInvalidPasswordHash
	}
	return argonParameters{
		memoryKiB:   uint32(memory),
		iterations:  uint32(iterations),
		parallelism: uint8(parallelism),
	}, salt, key, nil
}

func parsePrefixedUint(value, prefix string, bits int) (uint64, error) {
	if !strings.HasPrefix(value, prefix) || len(value) == len(prefix) {
		return 0, ErrInvalidPasswordHash
	}
	parsed, err := strconv.ParseUint(value[len(prefix):], 10, bits)
	if err != nil {
		return 0, ErrInvalidPasswordHash
	}
	return parsed, nil
}
