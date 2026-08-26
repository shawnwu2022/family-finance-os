package auth

import (
	"bytes"
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/shawnwu2022/family-finance-os/internal/store"
)

func TestServiceEnrollmentTOTPRecoveryAndSessionLifecycleIntegration(t *testing.T) {
	cfg := authIntegrationDatabaseConfig(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := store.OpenPostgres(ctx, cfg)
	if err != nil {
		t.Fatalf("OpenPostgres: %v", err)
	}
	defer pool.Close()

	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	var householdID int64
	if err := pool.QueryRow(ctx, `
		INSERT INTO households (name, base_currency, timezone)
		VALUES ($1, 'CNY', 'Asia/Shanghai')
		RETURNING id
	`, "auth-service-"+suffix).Scan(&householdID); err != nil {
		t.Fatalf("insert household: %v", err)
	}

	password := "correct horse battery staple"
	passwordHash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	authStore := NewPostgresStore(pool)
	username := "Owner-" + suffix
	user, created, err := authStore.CreateOrGetAdminUser(ctx, CreateAdminUserParams{
		Username:           username,
		NormalizedUsername: normalizeUsername(username),
		PasswordHash:       passwordHash,
		HouseholdID:        householdID,
	})
	if err != nil || !created {
		t.Fatalf("CreateOrGetAdminUser: created=%v err=%v", created, err)
	}

	secretBox, err := NewSecretBox(bytes.Repeat([]byte{0x7a}, 32))
	if err != nil {
		t.Fatalf("NewSecretBox: %v", err)
	}
	service, err := NewService(authStore, secretBox)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	now := time.Now().UTC().Truncate(time.Second)

	first, err := service.BeginLogin(ctx, username, password, now)
	if err != nil {
		t.Fatalf("BeginLogin enrollment: %v", err)
	}
	if first.Step != LoginStepEnrollTOTP || first.ChallengeToken == "" || first.TOTPSecret == "" || first.OTPAuthURI == "" {
		t.Fatalf("enrollment result = %#v", first)
	}
	firstCode, firstCounter, err := TOTPCode(first.TOTPSecret, now)
	if err != nil {
		t.Fatalf("TOTPCode enrollment: %v", err)
	}
	firstIssue, err := service.ConfirmEnrollment(ctx, first.ChallengeToken, firstCode, now)
	if err != nil {
		t.Fatalf("ConfirmEnrollment: %v", err)
	}
	if firstIssue.SessionToken == "" || firstIssue.CSRFToken == "" || len(firstIssue.RecoveryCodes) != 10 {
		t.Fatalf("enrollment session issue = %#v", firstIssue)
	}

	identity, err := service.AuthenticateSession(ctx, firstIssue.SessionToken, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("AuthenticateSession: %v", err)
	}
	if identity.UserID != user.ID || identity.Username != username || identity.HouseholdID != householdID || identity.CSRFToken != firstIssue.CSRFToken {
		t.Fatalf("session identity = %#v", identity)
	}

	secondAt := time.Unix((firstCounter+1)*30, 0).UTC()
	second, err := service.BeginLogin(ctx, username, password, secondAt)
	if err != nil {
		t.Fatalf("BeginLogin TOTP: %v", err)
	}
	if second.Step != LoginStepVerifyTOTP || second.ChallengeToken == "" || second.TOTPSecret != "" {
		t.Fatalf("second login = %#v", second)
	}
	secondCode, _, err := TOTPCode(first.TOTPSecret, secondAt)
	if err != nil {
		t.Fatalf("TOTPCode second: %v", err)
	}
	secondIssue, err := service.VerifySecondFactor(ctx, second.ChallengeToken, secondCode, false, secondAt)
	if err != nil {
		t.Fatalf("VerifySecondFactor TOTP: %v", err)
	}

	replay, err := service.BeginLogin(ctx, username, password, secondAt)
	if err != nil {
		t.Fatalf("BeginLogin replay challenge: %v", err)
	}
	if _, err := service.VerifySecondFactor(ctx, replay.ChallengeToken, secondCode, false, secondAt); !errors.Is(err, ErrInvalidSecondFactor) {
		t.Fatalf("replayed TOTP error = %v, want ErrInvalidSecondFactor", err)
	}

	recoveryAt := secondAt.Add(30 * time.Second)
	recovery, err := service.BeginLogin(ctx, username, password, recoveryAt)
	if err != nil {
		t.Fatalf("BeginLogin recovery: %v", err)
	}
	recoveryIssue, err := service.VerifySecondFactor(ctx, recovery.ChallengeToken, firstIssue.RecoveryCodes[0], true, recoveryAt)
	if err != nil || recoveryIssue.SessionToken == "" {
		t.Fatalf("VerifySecondFactor recovery: issue=%#v err=%v", recoveryIssue, err)
	}

	reuseAt := recoveryAt.Add(30 * time.Second)
	reuse, err := service.BeginLogin(ctx, username, password, reuseAt)
	if err != nil {
		t.Fatalf("BeginLogin recovery reuse: %v", err)
	}
	if _, err := service.VerifySecondFactor(ctx, reuse.ChallengeToken, firstIssue.RecoveryCodes[0], true, reuseAt); !errors.Is(err, ErrInvalidSecondFactor) {
		t.Fatalf("reused recovery code error = %v, want ErrInvalidSecondFactor", err)
	}

	if err := service.Logout(ctx, secondIssue.SessionToken, reuseAt.Add(time.Second)); err != nil {
		t.Fatalf("Logout: %v", err)
	}
	if _, err := service.AuthenticateSession(ctx, secondIssue.SessionToken, reuseAt.Add(2*time.Second)); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("revoked session error = %v, want ErrUnauthenticated", err)
	}

	if _, err := service.BeginLogin(ctx, "missing-"+suffix, password, reuseAt); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("unknown username error = %v, want ErrInvalidCredentials", err)
	}
}
