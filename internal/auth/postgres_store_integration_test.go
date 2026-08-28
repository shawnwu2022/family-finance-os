package auth

import (
	"context"
	"crypto/sha256"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/shawnwu2022/family-finance-os/internal/config"
	"github.com/shawnwu2022/family-finance-os/internal/store"
)

func TestPostgresAuthStoreChallengeSessionAndRecoveryLifecycleIntegration(t *testing.T) {
	cfg := authIntegrationDatabaseConfig(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
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
	`, "auth-integration-"+suffix).Scan(&householdID); err != nil {
		t.Fatalf("insert household: %v", err)
	}

	authStore := NewPostgresStore(pool)
	params := CreateAdminUserParams{
		Username:           "Owner-" + suffix,
		NormalizedUsername: "owner-" + suffix,
		PasswordHash:       "$argon2id$test",
		HouseholdID:        householdID,
	}
	user, created, err := authStore.CreateOrGetAdminUser(ctx, params)
	if err != nil {
		t.Fatalf("CreateOrGetAdminUser: %v", err)
	}
	if !created || user.HouseholdID != householdID || user.NormalizedUsername != params.NormalizedUsername {
		t.Fatalf("created user = %#v created=%v", user, created)
	}
	existing, created, err := authStore.CreateOrGetAdminUser(ctx, params)
	if err != nil {
		t.Fatalf("CreateOrGetAdminUser existing: %v", err)
	}
	if created || existing.ID != user.ID || existing.HouseholdID != householdID {
		t.Fatalf("existing user = %#v created=%v", existing, created)
	}

	now := time.Now().UTC().Truncate(time.Second)
	challengeHash := sha256.Sum256([]byte("challenge-token-" + suffix))
	if err := authStore.CreateChallenge(ctx, ChallengeRecord{
		TokenHash: challengeHash[:], UserID: user.ID, Kind: ChallengeLogin,
		CreatedAt: now, ExpiresAt: now.Add(5 * time.Minute),
	}); err != nil {
		t.Fatalf("CreateChallenge: %v", err)
	}
	if _, err := authStore.ConsumeChallenge(ctx, challengeHash[:], now.Add(time.Second)); err != nil {
		t.Fatalf("ConsumeChallenge first: %v", err)
	}
	if _, err := authStore.ConsumeChallenge(ctx, challengeHash[:], now.Add(2*time.Second)); err == nil {
		t.Fatal("ConsumeChallenge second succeeded; challenge must be single-use")
	}

	sessionHash := sha256.Sum256([]byte("session-token-" + suffix))
	csrfHash := sha256.Sum256([]byte("csrf-token-" + suffix))
	if err := authStore.CreateSession(ctx, SessionRecord{
		TokenHash: sessionHash[:], UserID: user.ID, CSRFTokenHash: csrfHash[:],
		CSRFTokenCiphertext: make([]byte, 28),
		CreatedAt:           now, LastSeenAt: now, ExpiresAt: now.Add(12 * time.Hour),
	}); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	session, err := authStore.GetSession(ctx, sessionHash[:], now.Add(time.Minute))
	if err != nil {
		t.Fatalf("GetSession active: %v", err)
	}
	if session.UserID != user.ID || session.HouseholdID != householdID {
		t.Fatalf("session = %#v", session)
	}
	if err := authStore.RevokeSession(ctx, sessionHash[:], now.Add(2*time.Minute)); err != nil {
		t.Fatalf("RevokeSession: %v", err)
	}
	if _, err := authStore.GetSession(ctx, sessionHash[:], now.Add(3*time.Minute)); err == nil {
		t.Fatal("GetSession returned revoked session")
	}

	recoveryHash := sha256.Sum256([]byte("recovery-code-" + suffix))
	if err := authStore.InsertRecoveryCodes(ctx, user.ID, [][]byte{recoveryHash[:]}); err != nil {
		t.Fatalf("InsertRecoveryCodes: %v", err)
	}
	used, err := authStore.ConsumeRecoveryCode(ctx, user.ID, recoveryHash[:], now.Add(4*time.Minute))
	if err != nil || !used {
		t.Fatalf("ConsumeRecoveryCode first: used=%v err=%v", used, err)
	}
	used, err = authStore.ConsumeRecoveryCode(ctx, user.ID, recoveryHash[:], now.Add(5*time.Minute))
	if err != nil {
		t.Fatalf("ConsumeRecoveryCode second: %v", err)
	}
	if used {
		t.Fatal("ConsumeRecoveryCode reused a one-time code")
	}
}

func authIntegrationDatabaseConfig(t *testing.T) config.DatabaseConfig {
	t.Helper()
	host := os.Getenv("TEST_POSTGRES_HOST")
	if host == "" {
		t.Skip("TEST_POSTGRES_HOST is not set")
	}
	port, err := strconv.ParseUint(os.Getenv("TEST_POSTGRES_PORT"), 10, 16)
	if err != nil {
		t.Fatalf("TEST_POSTGRES_PORT: %v", err)
	}
	return config.DatabaseConfig{
		Host:     host,
		Port:     uint16(port),
		Name:     os.Getenv("TEST_POSTGRES_DB"),
		User:     os.Getenv("TEST_POSTGRES_USER"),
		Password: os.Getenv("TEST_POSTGRES_PASSWORD"),
		SSLMode:  "disable",
	}
}
