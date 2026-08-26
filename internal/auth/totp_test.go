package auth

import (
	"testing"
	"time"
)

func TestTOTPWindowAndReplayCounter(t *testing.T) {
	const secret = "JBSWY3DPEHPK3PXP"
	at := time.Unix(1_700_000_000, 0).UTC()
	code, counter, err := TOTPCode(secret, at)
	if err != nil {
		t.Fatalf("TOTPCode: %v", err)
	}
	accepted, err := VerifyTOTP(secret, code, at, -1)
	if err != nil {
		t.Fatalf("VerifyTOTP current: %v", err)
	}
	if accepted != counter {
		t.Fatalf("accepted counter=%d want=%d", accepted, counter)
	}
	if _, err := VerifyTOTP(secret, code, at, accepted); err == nil {
		t.Fatal("VerifyTOTP accepted replayed counter")
	}

	previousCode, previousCounter, err := TOTPCode(secret, at.Add(-30*time.Second))
	if err != nil {
		t.Fatalf("TOTPCode previous: %v", err)
	}
	accepted, err = VerifyTOTP(secret, previousCode, at, -1)
	if err != nil || accepted != previousCounter {
		t.Fatalf("VerifyTOTP previous window: accepted=%d err=%v", accepted, err)
	}

	nextCode, nextCounter, err := TOTPCode(secret, at.Add(30*time.Second))
	if err != nil {
		t.Fatalf("TOTPCode next: %v", err)
	}
	accepted, err = VerifyTOTP(secret, nextCode, at, -1)
	if err != nil || accepted != nextCounter {
		t.Fatalf("VerifyTOTP next window: accepted=%d err=%v", accepted, err)
	}

	farCode, _, err := TOTPCode(secret, at.Add(60*time.Second))
	if err != nil {
		t.Fatalf("TOTPCode far: %v", err)
	}
	if _, err := VerifyTOTP(secret, farCode, at, -1); err == nil {
		t.Fatal("VerifyTOTP accepted code outside ±1 window")
	}
}

func TestGenerateTOTPSecretIsValidAndRandom(t *testing.T) {
	first, err := GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("GenerateTOTPSecret first: %v", err)
	}
	second, err := GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("GenerateTOTPSecret second: %v", err)
	}
	if first == second {
		t.Fatal("GenerateTOTPSecret returned duplicate secrets")
	}
	if _, _, err := TOTPCode(first, time.Unix(1_700_000_000, 0)); err != nil {
		t.Fatalf("generated secret unusable: %v", err)
	}
}
