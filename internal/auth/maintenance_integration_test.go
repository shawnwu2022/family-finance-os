package auth

import (
	"context"
	"crypto/sha256"
	"strconv"
	"testing"
	"time"

	"github.com/shawnwu2022/family-finance-os/internal/store"
)

func TestAuthMaintenanceRevokesSessionsAndPreservesOrClearsSecondFactorIntegration(t *testing.T) {
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
	`, "auth-maintenance-"+suffix).Scan(&householdID); err != nil {
		t.Fatalf("insert household: %v", err)
	}

	oldPassword := "correct horse battery staple"
	oldHash, err := HashPassword(oldPassword)
	if err != nil {
		t.Fatalf("HashPassword old: %v", err)
	}
	authStore := NewPostgresStore(pool)
	username := "Owner-" + suffix
	user, created, err := authStore.CreateOrGetAdminUser(ctx, CreateAdminUserParams{
		Username:           username,
		NormalizedUsername: normalizeUsername(username),
		PasswordHash:       oldHash,
		HouseholdID:        householdID,
	})
	if err != nil || !created {
		t.Fatalf("CreateOrGetAdminUser: created=%v err=%v", created, err)
	}

	now := time.Now().UTC().Truncate(time.Second)
	totpCiphertext := []byte("encrypted-totp-state")
	if _, err := pool.Exec(ctx, `
		UPDATE auth_users
		SET totp_secret_ciphertext=$2, totp_last_counter=77, totp_enrolled_at=$3
		WHERE id=$1
	`, user.ID, totpCiphertext, now); err != nil {
		t.Fatalf("seed TOTP: %v", err)
	}
	recoveryHash := sha256.Sum256([]byte("recovery-" + suffix))
	if err := authStore.InsertRecoveryCodes(ctx, user.ID, [][]byte{recoveryHash[:]}); err != nil {
		t.Fatalf("seed recovery code: %v", err)
	}
	seedSession := func(label string, at time.Time) []byte {
		t.Helper()
		sessionHash := sha256.Sum256([]byte(label + "-session-" + suffix))
		csrfHash := sha256.Sum256([]byte(label + "-csrf-" + suffix))
		if err := authStore.CreateSession(ctx, SessionRecord{
			TokenHash: sessionHash[:], UserID: user.ID, CSRFTokenHash: csrfHash[:],
			CSRFTokenCiphertext: make([]byte, 28), CreatedAt: at, LastSeenAt: at, ExpiresAt: at.Add(12 * time.Hour),
		}); err != nil {
			t.Fatalf("seed %s session: %v", label, err)
		}
		return sessionHash[:]
	}
	passwordSessionHash := seedSession("password-reset", now)
	challengeHash := sha256.Sum256([]byte("challenge-" + suffix))
	if err := authStore.CreateChallenge(ctx, ChallengeRecord{
		TokenHash: challengeHash[:], UserID: user.ID, Kind: ChallengeLogin,
		CreatedAt: now, ExpiresAt: now.Add(5 * time.Minute),
	}); err != nil {
		t.Fatalf("seed challenge: %v", err)
	}

	newPassword := []byte("another correct secure password")
	if err := ResetPassword(ctx, cfg, username, newPassword, now.Add(time.Minute)); err != nil {
		t.Fatalf("ResetPassword: %v", err)
	}
	if _, err := authStore.GetSession(ctx, passwordSessionHash, now.Add(2*time.Minute)); err == nil {
		t.Fatal("ResetPassword left existing session active")
	}
	if _, err := authStore.GetChallenge(ctx, challengeHash[:], now.Add(2*time.Minute)); err == nil {
		t.Fatal("ResetPassword left existing challenge active")
	}
	updated, err := authStore.GetUserByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetUserByID after password reset: %v", err)
	}
	ok, err := VerifyPassword(updated.PasswordHash, string(newPassword))
	if err != nil || !ok {
		t.Fatalf("new password verification: ok=%v err=%v", ok, err)
	}
	if len(updated.TOTPSecretCiphertext) == 0 || updated.TOTPLastCounter == nil || updated.TOTPEnrolledAt == nil {
		t.Fatalf("ResetPassword cleared TOTP state: %#v", updated)
	}
	var recoveryCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM auth_recovery_codes WHERE user_id=$1 AND consumed_at IS NULL`, user.ID).Scan(&recoveryCount); err != nil {
		t.Fatalf("count recovery codes after password reset: %v", err)
	}
	if recoveryCount != 1 {
		t.Fatalf("recovery codes after password reset=%d want=1", recoveryCount)
	}

	totpResetAt := now.Add(3 * time.Minute)
	totpSessionHash := seedSession("totp-reset", totpResetAt)
	secondChallengeHash := sha256.Sum256([]byte("second-challenge-" + suffix))
	if err := authStore.CreateChallenge(ctx, ChallengeRecord{
		TokenHash: secondChallengeHash[:], UserID: user.ID, Kind: ChallengeLogin,
		CreatedAt: totpResetAt, ExpiresAt: totpResetAt.Add(5 * time.Minute),
	}); err != nil {
		t.Fatalf("seed second challenge: %v", err)
	}
	if err := ResetTOTP(ctx, cfg, username, totpResetAt.Add(time.Minute)); err != nil {
		t.Fatalf("ResetTOTP: %v", err)
	}
	if _, err := authStore.GetSession(ctx, totpSessionHash, totpResetAt.Add(2*time.Minute)); err == nil {
		t.Fatal("ResetTOTP left existing session active")
	}
	if _, err := authStore.GetChallenge(ctx, secondChallengeHash[:], totpResetAt.Add(2*time.Minute)); err == nil {
		t.Fatal("ResetTOTP left existing challenge active")
	}
	resetUser, err := authStore.GetUserByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetUserByID after TOTP reset: %v", err)
	}
	if len(resetUser.TOTPSecretCiphertext) != 0 || resetUser.TOTPLastCounter != nil || resetUser.TOTPEnrolledAt != nil {
		t.Fatalf("ResetTOTP did not clear TOTP state: %#v", resetUser)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM auth_recovery_codes WHERE user_id=$1`, user.ID).Scan(&recoveryCount); err != nil {
		t.Fatalf("count recovery codes after TOTP reset: %v", err)
	}
	if recoveryCount != 0 {
		t.Fatalf("recovery codes after TOTP reset=%d want=0", recoveryCount)
	}
}
