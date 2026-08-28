package bootstrap

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/shawnwu2022/family-finance-os/internal/auth"
	"github.com/shawnwu2022/family-finance-os/internal/store"
)

func TestBootstrapCreatesAdminWithoutResettingExistingCredentialsIntegration(t *testing.T) {
	cfg := integrationDatabaseConfig(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	input := Input{
		Name: "bootstrap-auth-" + suffix, Currency: "CNY", Timezone: "Asia/Shanghai", Period: "2026-08", LiquidityFloorMinor: 500_000,
	}
	admin := AdminInput{Username: "Owner-" + suffix, Password: []byte("correct horse battery staple")}
	first, err := RunWithAdmin(ctx, cfg, input, admin)
	if err != nil {
		t.Fatalf("RunWithAdmin first: %v", err)
	}
	if first.HouseholdID <= 0 || first.AdminUserID <= 0 {
		t.Fatalf("first result = %#v", first)
	}

	pool, err := store.OpenPostgres(ctx, cfg)
	if err != nil {
		t.Fatalf("OpenPostgres: %v", err)
	}
	defer pool.Close()

	var passwordHash string
	if err := pool.QueryRow(ctx, `SELECT password_hash FROM auth_users WHERE id = $1`, first.AdminUserID).Scan(&passwordHash); err != nil {
		t.Fatalf("read initial password hash: %v", err)
	}
	ok, err := auth.VerifyPassword(passwordHash, string(admin.Password))
	if err != nil || !ok {
		t.Fatalf("initial password verification: ok=%v err=%v", ok, err)
	}

	seedTOTP := []byte("already-enrolled-totp-ciphertext")
	if _, err := pool.Exec(ctx, `
		UPDATE auth_users
		SET totp_secret_ciphertext = $2, totp_last_counter = 123, totp_enrolled_at = $3
		WHERE id = $1
	`, first.AdminUserID, seedTOTP, time.Now().UTC()); err != nil {
		t.Fatalf("seed TOTP state: %v", err)
	}

	secondAdmin := AdminInput{Username: admin.Username, Password: []byte("different secure password")}
	second, err := RunWithAdmin(ctx, cfg, input, secondAdmin)
	if err != nil {
		t.Fatalf("RunWithAdmin second: %v", err)
	}
	if second.AdminUserID != first.AdminUserID || second.HouseholdID != first.HouseholdID {
		t.Fatalf("second result = %#v first = %#v", second, first)
	}
	var secondHash string
	var secondTOTP []byte
	var counter int64
	if err := pool.QueryRow(ctx, `
		SELECT password_hash, totp_secret_ciphertext, totp_last_counter
		FROM auth_users WHERE id = $1
	`, first.AdminUserID).Scan(&secondHash, &secondTOTP, &counter); err != nil {
		t.Fatalf("read preserved auth state: %v", err)
	}
	if secondHash != passwordHash {
		t.Fatal("bootstrap silently reset existing admin password")
	}
	if string(secondTOTP) != string(seedTOTP) || counter != 123 {
		t.Fatalf("bootstrap changed existing TOTP state: ciphertext=%q counter=%d", secondTOTP, counter)
	}
}
