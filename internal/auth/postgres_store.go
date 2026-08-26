package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("auth record not found")

type ChallengeKind string

const (
	ChallengeLogin          ChallengeKind = "login"
	ChallengeTOTPEnrollment ChallengeKind = "totp_enrollment"
)

type UserRecord struct {
	ID                 int64
	Username           string
	NormalizedUsername string
	PasswordHash       string
	HouseholdID        int64
}

type CreateAdminUserParams struct {
	Username           string
	NormalizedUsername string
	PasswordHash       string
	HouseholdID        int64
}

type ChallengeRecord struct {
	TokenHash                   []byte
	UserID                      int64
	Kind                        ChallengeKind
	PendingTOTPSecretCiphertext []byte
	CreatedAt                   time.Time
	ExpiresAt                   time.Time
	ConsumedAt                  *time.Time
}

type SessionRecord struct {
	TokenHash     []byte
	UserID        int64
	CSRFTokenHash []byte
	CreatedAt     time.Time
	LastSeenAt    time.Time
	ExpiresAt     time.Time
}

type SessionView struct {
	SessionRecord
	Username    string
	HouseholdID int64
}

type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

func (s *PostgresStore) CreateOrGetAdminUser(ctx context.Context, params CreateAdminUserParams) (UserRecord, bool, error) {
	if s == nil || s.pool == nil {
		return UserRecord{}, false, errors.New("auth store database is required")
	}
	var user UserRecord
	err := s.pool.QueryRow(ctx, `
		INSERT INTO auth_users (username, normalized_username, password_hash, household_id)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (normalized_username) DO NOTHING
		RETURNING id, username, normalized_username, password_hash, household_id
	`, params.Username, params.NormalizedUsername, params.PasswordHash, params.HouseholdID).Scan(
		&user.ID,
		&user.Username,
		&user.NormalizedUsername,
		&user.PasswordHash,
		&user.HouseholdID,
	)
	if err == nil {
		return user, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return UserRecord{}, false, fmt.Errorf("create auth user: %w", err)
	}
	if err := s.pool.QueryRow(ctx, `
		SELECT id, username, normalized_username, password_hash, household_id
		FROM auth_users
		WHERE normalized_username = $1
	`, params.NormalizedUsername).Scan(
		&user.ID,
		&user.Username,
		&user.NormalizedUsername,
		&user.PasswordHash,
		&user.HouseholdID,
	); err != nil {
		return UserRecord{}, false, fmt.Errorf("read existing auth user: %w", err)
	}
	return user, false, nil
}

func (s *PostgresStore) CreateChallenge(ctx context.Context, challenge ChallengeRecord) error {
	if s == nil || s.pool == nil {
		return errors.New("auth store database is required")
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO auth_challenges (
			token_hash, user_id, kind, pending_totp_secret_ciphertext, created_at, expires_at
		)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, challenge.TokenHash, challenge.UserID, string(challenge.Kind), nullableBytes(challenge.PendingTOTPSecretCiphertext), challenge.CreatedAt, challenge.ExpiresAt)
	if err != nil {
		return fmt.Errorf("create auth challenge: %w", err)
	}
	return nil
}

func (s *PostgresStore) ConsumeChallenge(ctx context.Context, tokenHash []byte, now time.Time) (ChallengeRecord, error) {
	if s == nil || s.pool == nil {
		return ChallengeRecord{}, errors.New("auth store database is required")
	}
	var challenge ChallengeRecord
	var kind string
	var consumedAt time.Time
	err := s.pool.QueryRow(ctx, `
		UPDATE auth_challenges
		SET consumed_at = $2
		WHERE token_hash = $1
		  AND consumed_at IS NULL
		  AND expires_at > $2
		RETURNING token_hash, user_id, kind, pending_totp_secret_ciphertext, created_at, expires_at, consumed_at
	`, tokenHash, now).Scan(
		&challenge.TokenHash,
		&challenge.UserID,
		&kind,
		&challenge.PendingTOTPSecretCiphertext,
		&challenge.CreatedAt,
		&challenge.ExpiresAt,
		&consumedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return ChallengeRecord{}, ErrNotFound
	}
	if err != nil {
		return ChallengeRecord{}, fmt.Errorf("consume auth challenge: %w", err)
	}
	challenge.Kind = ChallengeKind(kind)
	challenge.ConsumedAt = &consumedAt
	return challenge, nil
}

func (s *PostgresStore) CreateSession(ctx context.Context, session SessionRecord) error {
	if s == nil || s.pool == nil {
		return errors.New("auth store database is required")
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO auth_sessions (
			token_hash, user_id, csrf_token_hash, created_at, last_seen_at, expires_at
		)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, session.TokenHash, session.UserID, session.CSRFTokenHash, session.CreatedAt, session.LastSeenAt, session.ExpiresAt)
	if err != nil {
		return fmt.Errorf("create auth session: %w", err)
	}
	return nil
}

func (s *PostgresStore) GetSession(ctx context.Context, tokenHash []byte, now time.Time) (SessionView, error) {
	if s == nil || s.pool == nil {
		return SessionView{}, errors.New("auth store database is required")
	}
	var session SessionView
	err := s.pool.QueryRow(ctx, `
		SELECT
			s.token_hash, s.user_id, s.csrf_token_hash, s.created_at, s.last_seen_at, s.expires_at,
			u.username, u.household_id
		FROM auth_sessions s
		JOIN auth_users u ON u.id = s.user_id
		WHERE s.token_hash = $1
		  AND s.revoked_at IS NULL
		  AND s.expires_at > $2
		  AND u.disabled_at IS NULL
	`, tokenHash, now).Scan(
		&session.TokenHash,
		&session.UserID,
		&session.CSRFTokenHash,
		&session.CreatedAt,
		&session.LastSeenAt,
		&session.ExpiresAt,
		&session.Username,
		&session.HouseholdID,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return SessionView{}, ErrNotFound
	}
	if err != nil {
		return SessionView{}, fmt.Errorf("read auth session: %w", err)
	}
	return session, nil
}

func (s *PostgresStore) RevokeSession(ctx context.Context, tokenHash []byte, now time.Time) error {
	if s == nil || s.pool == nil {
		return errors.New("auth store database is required")
	}
	command, err := s.pool.Exec(ctx, `
		UPDATE auth_sessions
		SET revoked_at = $2
		WHERE token_hash = $1 AND revoked_at IS NULL
	`, tokenHash, now)
	if err != nil {
		return fmt.Errorf("revoke auth session: %w", err)
	}
	if command.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresStore) InsertRecoveryCodes(ctx context.Context, userID int64, hashes [][]byte) error {
	if s == nil || s.pool == nil {
		return errors.New("auth store database is required")
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin recovery code insert: %w", err)
	}
	defer tx.Rollback(ctx)
	for _, hash := range hashes {
		if _, err := tx.Exec(ctx, `
			INSERT INTO auth_recovery_codes (user_id, code_hash)
			VALUES ($1, $2)
		`, userID, hash); err != nil {
			return fmt.Errorf("insert recovery code: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit recovery codes: %w", err)
	}
	return nil
}

func (s *PostgresStore) ConsumeRecoveryCode(ctx context.Context, userID int64, hash []byte, now time.Time) (bool, error) {
	if s == nil || s.pool == nil {
		return false, errors.New("auth store database is required")
	}
	var id int64
	err := s.pool.QueryRow(ctx, `
		UPDATE auth_recovery_codes
		SET consumed_at = $3
		WHERE user_id = $1 AND code_hash = $2 AND consumed_at IS NULL
		RETURNING id
	`, userID, hash, now).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("consume recovery code: %w", err)
	}
	return true, nil
}

func nullableBytes(value []byte) []byte {
	if len(value) == 0 {
		return nil
	}
	return value
}
