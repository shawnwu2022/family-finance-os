package auth

import (
	"strings"
	"testing"
)

func TestPasswordHashRoundTrip(t *testing.T) {
	password := "correct horse battery staple"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if hash == password || !strings.HasPrefix(hash, "$argon2id$v=19$m=65536,t=3,p=1$") {
		t.Fatalf("unexpected password hash format: %q", hash)
	}
	ok, err := VerifyPassword(hash, password)
	if err != nil || !ok {
		t.Fatalf("VerifyPassword valid: ok=%v err=%v", ok, err)
	}
	ok, err = VerifyPassword(hash, "correct horse battery staplf")
	if err != nil {
		t.Fatalf("VerifyPassword wrong password: %v", err)
	}
	if ok {
		t.Fatal("VerifyPassword accepted wrong password")
	}
}

func TestPasswordPolicy(t *testing.T) {
	if _, err := HashPassword("short-passwor"); err == nil {
		t.Fatal("HashPassword accepted fewer than 14 Unicode characters")
	}
	long := strings.Repeat("密", 43) // 129 UTF-8 bytes.
	if len([]byte(long)) != 129 {
		t.Fatalf("test setup long password bytes=%d", len([]byte(long)))
	}
	if _, err := HashPassword(long); err == nil {
		t.Fatal("HashPassword accepted more than 128 bytes")
	}
	validUnicode := strings.Repeat("密", 14)
	if _, err := HashPassword(validUnicode); err != nil {
		t.Fatalf("HashPassword rejected 14 Unicode characters: %v", err)
	}
}

func TestVerifyPasswordRejectsMalformedOrExpensiveHashes(t *testing.T) {
	cases := []string{
		"not-a-hash",
		"$argon2id$v=19$m=1048576,t=3,p=1$c2FsdA$a2V5",
		"$argon2id$v=19$m=65536,t=99,p=1$c2FsdA$a2V5",
		"$argon2id$v=19$m=65536,t=3,p=99$c2FsdA$a2V5",
	}
	for _, encoded := range cases {
		if _, err := VerifyPassword(encoded, "correct horse battery staple"); err == nil {
			t.Fatalf("VerifyPassword accepted malformed/unsafe hash %q", encoded)
		}
	}
}
