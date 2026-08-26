package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/shawnwu2022/family-finance-os/internal/config"
	"github.com/shawnwu2022/family-finance-os/internal/store"
)

func ResetPassword(ctx context.Context, cfg config.DatabaseConfig, username string, password []byte, now time.Time) error {
	normalized := normalizeUsername(username)
	if normalized == "" {
		return errors.New("username is required")
	}
	copyPassword := append([]byte(nil), password...)
	defer clear(copyPassword)
	passwordHash, err := HashPassword(string(copyPassword))
	if err != nil {
		return fmt.Errorf("validate new password: %w", err)
	}

	pool, err := store.OpenPostgres(ctx, cfg)
	if err != nil {
		return err
	}
	defer pool.Close()
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return fmt.Errorf("begin password reset transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var userID int64
	if err := tx.QueryRow(ctx, `
		UPDATE auth_users
		SET password_hash = $2, updated_at = $3
		WHERE normalized_username = $1 AND disabled_at IS NULL
		RETURNING id
	`, normalized, passwordHash, now).Scan(&userID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("reset password: %w", err)
	}
	if err := revokeAuthState(ctx, tx, userID, now, false); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit password reset: %w", err)
	}
	return nil
}

func ResetTOTP(ctx context.Context, cfg config.DatabaseConfig, username string, now time.Time) error {
	normalized := strings.ToLower(strings.TrimSpace(username))
	if normalized == "" {
		return errors.New("username is required")
	}
	pool, err := store.OpenPostgres(ctx, cfg)
	if err != nil {
		return err
	}
	defer pool.Close()
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return fmt.Errorf("begin TOTP reset transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var userID int64
	if err := tx.QueryRow(ctx, `
		UPDATE auth_users
		SET totp_secret_ciphertext = NULL,
		    totp_last_counter = NULL,
		    totp_enrolled_at = NULL,
		    updated_at = $2
		WHERE normalized_username = $1 AND disabled_at IS NULL
		RETURNING id
	`, normalized, now).Scan(&userID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("reset TOTP: %w", err)
	}
	if err := revokeAuthState(ctx, tx, userID, now, true); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit TOTP reset: %w", err)
	}
	return nil
}

func revokeAuthState(ctx context.Context, tx pgx.Tx, userID int64, now time.Time, clearRecoveryCodes bool) error {
	if _, err := tx.Exec(ctx, `
		UPDATE auth_sessions
		SET revoked_at = $2
		WHERE user_id = $1 AND revoked_at IS NULL
	`, userID, now); err != nil {
		return fmt.Errorf("revoke user sessions: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM auth_challenges WHERE user_id = $1`, userID); err != nil {
		return fmt.Errorf("clear user auth challenges: %w", err)
	}
	if clearRecoveryCodes {
		if _, err := tx.Exec(ctx, `DELETE FROM auth_recovery_codes WHERE user_id = $1`, userID); err != nil {
			return fmt.Errorf("clear recovery codes: %w", err)
		}
	}
	return nil
}
