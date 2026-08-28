package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrLastOwner      = errors.New("household must retain an active owner")
	ErrUsernameExists = errors.New("username already exists")
)

type HouseholdMember struct {
	UserID       int64
	Username     string
	Role         Role
	Disabled     bool
	TOTPEnrolled bool
}

func (s *PostgresStore) GetUserRole(ctx context.Context, userID int64) (Role, error) {
	if s == nil || s.pool == nil {
		return "", errors.New("auth store database is required")
	}
	var raw string
	err := s.pool.QueryRow(ctx, `
		SELECT role
		FROM auth_users
		WHERE id = $1 AND disabled_at IS NULL
	`, userID).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("read auth user role: %w", err)
	}
	role, err := ParseRole(raw)
	if err != nil {
		return "", fmt.Errorf("read auth user role: %w", err)
	}
	return role, nil
}

func (s *PostgresStore) ListHouseholdMembers(ctx context.Context, householdID int64) ([]HouseholdMember, error) {
	if s == nil || s.pool == nil {
		return nil, errors.New("auth store database is required")
	}
	if householdID <= 0 {
		return nil, errors.New("household ID must be positive")
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, username, role, disabled_at IS NOT NULL, totp_enrolled_at IS NOT NULL
		FROM auth_users
		WHERE household_id = $1
		ORDER BY id
	`, householdID)
	if err != nil {
		return nil, fmt.Errorf("list household members: %w", err)
	}
	defer rows.Close()

	members := make([]HouseholdMember, 0)
	for rows.Next() {
		member, err := scanHouseholdMember(rows)
		if err != nil {
			return nil, err
		}
		members = append(members, member)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list household members: %w", err)
	}
	return members, nil
}

func (s *PostgresStore) CreateHouseholdMember(ctx context.Context, householdID int64, username, normalizedUsername, passwordHash string, role Role) (HouseholdMember, error) {
	if s == nil || s.pool == nil {
		return HouseholdMember{}, errors.New("auth store database is required")
	}
	if householdID <= 0 {
		return HouseholdMember{}, errors.New("household ID must be positive")
	}
	if _, err := ParseRole(string(role)); err != nil {
		return HouseholdMember{}, err
	}
	member, err := scanHouseholdMember(s.pool.QueryRow(ctx, `
		INSERT INTO auth_users (username, normalized_username, password_hash, household_id, role)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, username, role, disabled_at IS NOT NULL, totp_enrolled_at IS NOT NULL
	`, username, normalizedUsername, passwordHash, householdID, string(role)))
	if err == nil {
		return member, nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return HouseholdMember{}, ErrUsernameExists
	}
	return HouseholdMember{}, fmt.Errorf("create household member: %w", err)
}

func (s *PostgresStore) UpdateHouseholdMemberRole(ctx context.Context, householdID, userID int64, role Role, now time.Time) (HouseholdMember, error) {
	if s == nil || s.pool == nil {
		return HouseholdMember{}, errors.New("auth store database is required")
	}
	if householdID <= 0 || userID <= 0 {
		return HouseholdMember{}, errors.New("household and user IDs must be positive")
	}
	if _, err := ParseRole(string(role)); err != nil {
		return HouseholdMember{}, err
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return HouseholdMember{}, fmt.Errorf("begin household role update: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockHouseholdRBAC(ctx, tx, householdID); err != nil {
		return HouseholdMember{}, err
	}

	current, err := loadHouseholdMemberForUpdate(ctx, tx, householdID, userID)
	if err != nil {
		return HouseholdMember{}, err
	}
	if !current.Disabled && current.Role == RoleOwner && role != RoleOwner {
		if err := requireAnotherActiveOwner(ctx, tx, householdID); err != nil {
			return HouseholdMember{}, err
		}
	}

	updated, err := scanHouseholdMember(tx.QueryRow(ctx, `
		UPDATE auth_users
		SET role = $3, updated_at = $4
		WHERE household_id = $1 AND id = $2
		RETURNING id, username, role, disabled_at IS NOT NULL, totp_enrolled_at IS NOT NULL
	`, householdID, userID, string(role), now))
	if err != nil {
		return HouseholdMember{}, fmt.Errorf("update household member role: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return HouseholdMember{}, fmt.Errorf("commit household member role: %w", err)
	}
	return updated, nil
}

func (s *PostgresStore) DisableHouseholdMember(ctx context.Context, householdID, userID int64, now time.Time) (HouseholdMember, error) {
	if s == nil || s.pool == nil {
		return HouseholdMember{}, errors.New("auth store database is required")
	}
	if householdID <= 0 || userID <= 0 {
		return HouseholdMember{}, errors.New("household and user IDs must be positive")
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return HouseholdMember{}, fmt.Errorf("begin household member disable: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockHouseholdRBAC(ctx, tx, householdID); err != nil {
		return HouseholdMember{}, err
	}

	current, err := loadHouseholdMemberForUpdate(ctx, tx, householdID, userID)
	if err != nil {
		return HouseholdMember{}, err
	}
	if current.Disabled {
		if err := tx.Commit(ctx); err != nil {
			return HouseholdMember{}, fmt.Errorf("commit disabled household member read: %w", err)
		}
		return current, nil
	}
	if current.Role == RoleOwner {
		if err := requireAnotherActiveOwner(ctx, tx, householdID); err != nil {
			return HouseholdMember{}, err
		}
	}

	updated, err := scanHouseholdMember(tx.QueryRow(ctx, `
		UPDATE auth_users
		SET disabled_at = $3, updated_at = $3
		WHERE household_id = $1 AND id = $2 AND disabled_at IS NULL
		RETURNING id, username, role, disabled_at IS NOT NULL, totp_enrolled_at IS NOT NULL
	`, householdID, userID, now))
	if err != nil {
		return HouseholdMember{}, fmt.Errorf("disable household member: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE auth_sessions
		SET revoked_at = $2
		WHERE user_id = $1 AND revoked_at IS NULL
	`, userID, now); err != nil {
		return HouseholdMember{}, fmt.Errorf("revoke disabled member sessions: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return HouseholdMember{}, fmt.Errorf("commit household member disable: %w", err)
	}
	return updated, nil
}

func lockHouseholdRBAC(ctx context.Context, tx pgx.Tx, householdID int64) error {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, -householdID); err != nil {
		return fmt.Errorf("lock household RBAC: %w", err)
	}
	return nil
}

func loadHouseholdMemberForUpdate(ctx context.Context, tx pgx.Tx, householdID, userID int64) (HouseholdMember, error) {
	member, err := scanHouseholdMember(tx.QueryRow(ctx, `
		SELECT id, username, role, disabled_at IS NOT NULL, totp_enrolled_at IS NOT NULL
		FROM auth_users
		WHERE household_id = $1 AND id = $2
		FOR UPDATE
	`, householdID, userID))
	if errors.Is(err, ErrNotFound) {
		return HouseholdMember{}, ErrNotFound
	}
	if err != nil {
		return HouseholdMember{}, fmt.Errorf("load household member: %w", err)
	}
	return member, nil
}

func requireAnotherActiveOwner(ctx context.Context, tx pgx.Tx, householdID int64) error {
	var count int
	if err := tx.QueryRow(ctx, `
		SELECT count(*)
		FROM auth_users
		WHERE household_id = $1 AND role = 'owner' AND disabled_at IS NULL
	`, householdID).Scan(&count); err != nil {
		return fmt.Errorf("count active household owners: %w", err)
	}
	if count <= 1 {
		return ErrLastOwner
	}
	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanHouseholdMember(row rowScanner) (HouseholdMember, error) {
	var member HouseholdMember
	var rawRole string
	if err := row.Scan(&member.UserID, &member.Username, &rawRole, &member.Disabled, &member.TOTPEnrolled); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return HouseholdMember{}, ErrNotFound
		}
		return HouseholdMember{}, err
	}
	role, err := ParseRole(rawRole)
	if err != nil {
		return HouseholdMember{}, err
	}
	member.Role = role
	return member, nil
}
