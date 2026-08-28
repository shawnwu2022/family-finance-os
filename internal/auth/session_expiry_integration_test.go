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

func TestServiceSessionIdleAndAbsoluteExpiryIntegration(t *testing.T) {
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
	`, "auth-expiry-"+suffix).Scan(&householdID); err != nil {
		t.Fatalf("insert household: %v", err)
	}

	password := "expiry correct horse battery staple"
	passwordHash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	authStore := NewPostgresStore(pool)
	username := "Expiry-" + suffix
	if _, created, err := authStore.CreateOrGetAdminUser(ctx, CreateAdminUserParams{
		Username:           username,
		NormalizedUsername: normalizeUsername(username),
		PasswordHash:       passwordHash,
		HouseholdID:        householdID,
	}); err != nil || !created {
		t.Fatalf("CreateOrGetAdminUser: created=%v err=%v", created, err)
	}

	secretBox, err := NewSecretBox(bytes.Repeat([]byte{0x5c}, 32))
	if err != nil {
		t.Fatalf("NewSecretBox: %v", err)
	}
	service, err := NewService(authStore, secretBox)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	now := time.Now().UTC().Truncate(time.Second)

	enrollment, err := service.BeginLogin(ctx, username, password, now)
	if err != nil {
		t.Fatalf("BeginLogin enrollment: %v", err)
	}
	code, counter, err := TOTPCode(enrollment.TOTPSecret, now)
	if err != nil {
		t.Fatalf("TOTPCode enrollment: %v", err)
	}
	idleIssue, err := service.ConfirmEnrollment(ctx, enrollment.ChallengeToken, code, now)
	if err != nil {
		t.Fatalf("ConfirmEnrollment: %v", err)
	}

	idleAt := now.Add(sessionIdleTimeout).Add(time.Second)
	if _, err := service.AuthenticateSession(ctx, idleIssue.SessionToken, idleAt); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("idle-expired session error=%v want ErrUnauthenticated", err)
	}

	secondAt := time.Unix((counter+1)*30, 0).UTC()
	second, err := service.BeginLogin(ctx, username, password, secondAt)
	if err != nil {
		t.Fatalf("BeginLogin second session: %v", err)
	}
	secondCode, _, err := TOTPCode(enrollment.TOTPSecret, secondAt)
	if err != nil {
		t.Fatalf("TOTPCode second session: %v", err)
	}
	absoluteIssue, err := service.VerifySecondFactor(ctx, second.ChallengeToken, secondCode, false, secondAt)
	if err != nil {
		t.Fatalf("VerifySecondFactor second session: %v", err)
	}

	absoluteHash := HashOpaqueToken(absoluteIssue.SessionToken)
	touchAt := secondAt.Add(sessionAbsoluteTTL - 5*time.Minute)
	if err := authStore.TouchSession(ctx, absoluteHash[:], touchAt); err != nil {
		t.Fatalf("TouchSession before absolute expiry: %v", err)
	}
	absoluteAt := secondAt.Add(sessionAbsoluteTTL).Add(time.Second)
	if _, err := service.AuthenticateSession(ctx, absoluteIssue.SessionToken, absoluteAt); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("absolute-expired session error=%v want ErrUnauthenticated", err)
	}
}
