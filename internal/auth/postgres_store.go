package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("auth record not found")

type ChallengeKind string

const (
	ChallengeLogin          ChallengeKind = "login"
	ChallengeTOTPEnrollment ChallengeKind = "totp_enrollment"
)

type UserRecord struct {
	ID                   int64
	Username             string
	NormalizedUsername   string
	PasswordHash         string
	HouseholdID          int64
	TOTPSecretCiphertext []byte
	TOTPLastCounter      *int64
	TOTPEnrolledAt       *time.Time
	DisabledAt           *time.Time
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

type CompleteEnrollmentParams struct {
	ChallengeTokenHash []byte
	Counter            int64
	Session            SessionRecord
	RecoveryCodeHashes [][]byte
	Now                time.Time
}

type CompleteTOTPLoginParams struct {
	ChallengeTokenHash []byte
	Counter            int64
	Session            SessionRecord
	Now                time.Time
}

type CompleteRecoveryLoginParams struct {
	ChallengeTokenHash []byte
	RecoveryCodeHash   []byte
	Session            SessionRecord
	Now                time.Time
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
	user, err := scanUser(s.pool.QueryRow(ctx, `
		INSERT INTO auth_users (username, normalized_username, password_hash, household_id)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (normalized_username) DO NOTHING
		RETURNING id, username, normalized_username, password_hash, household_id,
		          totp_secret_ciphertext, totp_last_counter, totp_enrolled_at, disabled_at
	`, params.Username, params.NormalizedUsername, params.PasswordHash, params.HouseholdID))
	if err == nil {
		return user, true, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return UserRecord{}, false, fmt.Errorf("create auth user: %w", err)
	}
	user, err = s.GetUserByNormalizedUsername(ctx, params.NormalizedUsername)
	if err != nil {
		return UserRecord{}, false, fmt.Errorf("read existing auth user: %w", err)
	}
	return user, false, nil
}

func (s *PostgresStore) GetUserByNormalizedUsername(ctx context.Context, normalizedUsername string) (UserRecord, error) {
	if s == nil || s.pool == nil {
		return UserRecord{}, errors.New("auth store database is required")
	}
	return scanUser(s.pool.QueryRow(ctx, `
		SELECT id, username, normalized_username, password_hash, household_id,
		       totp_secret_ciphertext, totp_last_counter, totp_enrolled_at, disabled_at
		FROM auth_users
		WHERE normalized_username = $1
	`, normalizedUsername))
}

func (s *PostgresStore) GetUserByID(ctx context.Context, userID int64) (UserRecord, error) {
	if s == nil || s.pool == nil {
		return UserRecord{}, errors.New("auth store database is required")
	}
	return scanUser(s.pool.QueryRow(ctx, `
		SELECT id, username, normalized_username, password_hash, household_id,
		       totp_secret_ciphertext, totp_last_counter, totp_enrolled_at, disabled_at
		FROM auth_users
		WHERE id = $1
	`, userID))
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

func (s *PostgresStore) GetChallenge(ctx context.Context, tokenHash []byte, now time.Time) (ChallengeRecord, error) {
	if s == nil || s.pool == nil {
		return ChallengeRecord{}, errors.New("auth store database is required")
	}
	return scanChallenge(s.pool.QueryRow(ctx, `
		SELECT token_hash, user_id, kind, pending_totp_secret_ciphertext, created_at, expires_at, consumed_at
		FROM auth_challenges
		WHERE token_hash = $1 AND consumed_at IS NULL AND expires_at > $2
	`, tokenHash, now))
}

func (s *PostgresStore) ConsumeChallenge(ctx context.Context, tokenHash []byte, now time.Time) (ChallengeRecord, error) {
	if s == nil || s.pool == nil {
		return ChallengeRecord{}, errors.New("auth store database is required")
	}
	return scanChallenge(s.pool.QueryRow(ctx, `
		UPDATE auth_challenges
		SET consumed_at = $2
		WHERE token_hash = $1
		  AND consumed_at IS NULL
		  AND expires_at > $2
		RETURNING token_hash, user_id, kind, pending_totp_secret_ciphertext, created_at, expires_at, consumed_at
	`, tokenHash, now))
}

func (s *PostgresStore) CreateSession(ctx context.Context, session SessionRecord) error {
	if s == nil || s.pool == nil {
		return errors.New("auth store database is required")
	}
	if err := insertSession(ctx, s.pool, session); err != nil {
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

func (s *PostgresStore) TouchSession(ctx context.Context, tokenHash []byte, now time.Time) error {
	if s == nil || s.pool == nil {
		return errors.New("auth store database is required")
	}
	command, err := s.pool.Exec(ctx, `
		UPDATE auth_sessions
		SET last_seen_at = $2
		WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > $2
	`, tokenHash, now)
	if err != nil {
		return fmt.Errorf("touch auth session: %w", err)
	}
	if command.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
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

func (s *PostgresStore) RevokeUserSessions(ctx context.Context, userID int64, now time.Time) error {
	if s == nil || s.pool == nil {
		return errors.New("auth store database is required")
	}
	if _, err := s.pool.Exec(ctx, `
		UPDATE auth_sessions
		SET revoked_at = $2
		WHERE user_id = $1 AND revoked_at IS NULL
	`, userID, now); err != nil {
		return fmt.Errorf("revoke user auth sessions: %w", err)
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
	defer func() { _ = tx.Rollback(ctx) }()
	if err := insertRecoveryCodes(ctx, tx, userID, hashes); err != nil {
		return err
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

func (s *PostgresStore) CompleteEnrollment(ctx context.Context, params CompleteEnrollmentParams) error {
	if s == nil || s.pool == nil {
		return errors.New("auth store database is required")
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin TOTP enrollment completion: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var userID int64
	var secretCiphertext []byte
	err = tx.QueryRow(ctx, `
		UPDATE auth_challenges
		SET consumed_at = $2
		WHERE token_hash = $1
		  AND kind = 'totp_enrollment'
		  AND consumed_at IS NULL
		  AND expires_at > $2
		RETURNING user_id, pending_totp_secret_ciphertext
	`, params.ChallengeTokenHash, params.Now).Scan(&userID, &secretCiphertext)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("consume TOTP enrollment challenge: %w", err)
	}
	if len(secretCiphertext) == 0 {
		return errors.New("TOTP enrollment challenge has no encrypted secret")
	}

	command, err := tx.Exec(ctx, `
		UPDATE auth_users
		SET totp_secret_ciphertext = $2,
		    totp_last_counter = $3,
		    totp_enrolled_at = $4,
		    updated_at = $4
		WHERE id = $1 AND disabled_at IS NULL AND totp_enrolled_at IS NULL
	`, userID, secretCiphertext, params.Counter, params.Now)
	if err != nil {
		return fmt.Errorf("store TOTP enrollment: %w", err)
	}
	if command.RowsAffected() != 1 {
		return ErrNotFound
	}
	if _, err := tx.Exec(ctx, `DELETE FROM auth_recovery_codes WHERE user_id = $1`, userID); err != nil {
		return fmt.Errorf("clear recovery codes before enrollment: %w", err)
	}
	if err := insertRecoveryCodes(ctx, tx, userID, params.RecoveryCodeHashes); err != nil {
		return err
	}
	params.Session.UserID = userID
	if err := insertSession(ctx, tx, params.Session); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit TOTP enrollment completion: %w", err)
	}
	return nil
}

func (s *PostgresStore) CompleteTOTPLogin(ctx context.Context, params CompleteTOTPLoginParams) error {
	if s == nil || s.pool == nil {
		return errors.New("auth store database is required")
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin TOTP login completion: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	userID, err := consumeLoginChallenge(ctx, tx, params.ChallengeTokenHash, params.Now)
	if err != nil {
		return err
	}
	command, err := tx.Exec(ctx, `
		UPDATE auth_users
		SET totp_last_counter = $2, updated_at = $3
		WHERE id = $1
		  AND disabled_at IS NULL
		  AND totp_enrolled_at IS NOT NULL
		  AND (totp_last_counter IS NULL OR totp_last_counter < $2)
	`, userID, params.Counter, params.Now)
	if err != nil {
		return fmt.Errorf("advance TOTP replay counter: %w", err)
	}
	if command.RowsAffected() != 1 {
		return ErrNotFound
	}
	params.Session.UserID = userID
	if err := insertSession(ctx, tx, params.Session); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit TOTP login completion: %w", err)
	}
	return nil
}

func (s *PostgresStore) CompleteRecoveryLogin(ctx context.Context, params CompleteRecoveryLoginParams) error {
	if s == nil || s.pool == nil {
		return errors.New("auth store database is required")
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin recovery login completion: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	userID, err := consumeLoginChallenge(ctx, tx, params.ChallengeTokenHash, params.Now)
	if err != nil {
		return err
	}
	var recoveryID int64
	err = tx.QueryRow(ctx, `
		UPDATE auth_recovery_codes
		SET consumed_at = $3
		WHERE user_id = $1 AND code_hash = $2 AND consumed_at IS NULL
		RETURNING id
	`, userID, params.RecoveryCodeHash, params.Now).Scan(&recoveryID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("consume recovery code in login: %w", err)
	}
	params.Session.UserID = userID
	if err := insertSession(ctx, tx, params.Session); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit recovery login completion: %w", err)
	}
	return nil
}

func scanUser(row pgx.Row) (UserRecord, error) {
	var user UserRecord
	var lastCounter pgtype.Int8
	var enrolledAt pgtype.Timestamptz
	var disabledAt pgtype.Timestamptz
	if err := row.Scan(
		&user.ID,
		&user.Username,
		&user.NormalizedUsername,
		&user.PasswordHash,
		&user.HouseholdID,
		&user.TOTPSecretCiphertext,
		&lastCounter,
		&enrolledAt,
		&disabledAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return UserRecord{}, ErrNotFound
		}
		return UserRecord{}, err
	}
	if lastCounter.Valid {
		value := lastCounter.Int64
		user.TOTPLastCounter = &value
	}
	if enrolledAt.Valid {
		value := enrolledAt.Time
		user.TOTPEnrolledAt = &value
	}
	if disabledAt.Valid {
		value := disabledAt.Time
		user.DisabledAt = &value
	}
	return user, nil
}

func scanChallenge(row pgx.Row) (ChallengeRecord, error) {
	var challenge ChallengeRecord
	var kind string
	var consumedAt pgtype.Timestamptz
	if err := row.Scan(
		&challenge.TokenHash,
		&challenge.UserID,
		&kind,
		&challenge.PendingTOTPSecretCiphertext,
		&challenge.CreatedAt,
		&challenge.ExpiresAt,
		&consumedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ChallengeRecord{}, ErrNotFound
		}
		return ChallengeRecord{}, err
	}
	challenge.Kind = ChallengeKind(kind)
	if consumedAt.Valid {
		value := consumedAt.Time
		challenge.ConsumedAt = &value
	}
	return challenge, nil
}

func insertRecoveryCodes(ctx context.Context, tx pgx.Tx, userID int64, hashes [][]byte) error {
	for _, hash := range hashes {
		if _, err := tx.Exec(ctx, `
			INSERT INTO auth_recovery_codes (user_id, code_hash)
			VALUES ($1, $2)
		`, userID, hash); err != nil {
			return fmt.Errorf("insert recovery code: %w", err)
		}
	}
	return nil
}

func consumeLoginChallenge(ctx context.Context, tx pgx.Tx, tokenHash []byte, now time.Time) (int64, error) {
	var userID int64
	err := tx.QueryRow(ctx, `
		UPDATE auth_challenges
		SET consumed_at = $2
		WHERE token_hash = $1
		  AND kind = 'login'
		  AND consumed_at IS NULL
		  AND expires_at > $2
		RETURNING user_id
	`, tokenHash, now).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("consume login challenge: %w", err)
	}
	return userID, nil
}

type sessionInserter interface {
	Exec(context.Context, string, ...any) (pgconnCommandTag, error)
}

type pgconnCommandTag interface {
	RowsAffected() int64
}

func insertSession(ctx context.Context, executor interface {
	Exec(context.Context, string, ...any) (pgconnCommandTag, error)
}, session SessionRecord) error {
	_, err := executor.Exec(ctx, `
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

func nullableBytes(value []byte) []byte {
	if len(value) == 0 {
		return nil
	}
	return value
}
