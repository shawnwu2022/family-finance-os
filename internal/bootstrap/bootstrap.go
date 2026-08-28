package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/shawnwu2022/family-finance-os/internal/auth"
	"github.com/shawnwu2022/family-finance-os/internal/config"
	"github.com/shawnwu2022/family-finance-os/internal/store"
)

const advisoryLockID int64 = 7_442_881_993_210_047

var (
	ErrInvalidInput       = errors.New("invalid bootstrap input")
	ErrDuplicateHousehold = errors.New("multiple households match bootstrap identity")
	currencyPattern       = regexp.MustCompile(`^[A-Z]{3}$`)
)

type Input struct {
	Name                string
	Currency            string
	Timezone            string
	Period              string
	LiquidityFloorMinor int64
}

type AdminInput struct {
	Username string
	Password []byte
}

type Result struct {
	HouseholdID  int64
	BudgetPlanID int64
	AdminUserID  int64
}

type preparedAdmin struct {
	username           string
	normalizedUsername string
	passwordHash       string
}

func Run(ctx context.Context, cfg config.DatabaseConfig, input Input) (Result, error) {
	return run(ctx, cfg, input, nil)
}

func RunWithAdmin(ctx context.Context, cfg config.DatabaseConfig, input Input, admin AdminInput) (Result, error) {
	prepared, err := prepareAdmin(admin)
	if err != nil {
		return Result{}, err
	}
	return run(ctx, cfg, input, &prepared)
}

func run(ctx context.Context, cfg config.DatabaseConfig, input Input, admin *preparedAdmin) (Result, error) {
	input, err := Validate(input)
	if err != nil {
		return Result{}, err
	}
	pool, err := store.OpenPostgres(ctx, cfg)
	if err != nil {
		return Result{}, err
	}
	defer pool.Close()

	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return Result{}, fmt.Errorf("begin bootstrap transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, advisoryLockID); err != nil {
		return Result{}, fmt.Errorf("lock bootstrap transaction: %w", err)
	}

	householdID, err := ensureHousehold(ctx, tx, input)
	if err != nil {
		return Result{}, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO household_policies (household_id, liquidity_floor_minor, currency)
		VALUES ($1, $2, $3)
		ON CONFLICT (household_id) DO NOTHING
	`, householdID, input.LiquidityFloorMinor, input.Currency); err != nil {
		return Result{}, fmt.Errorf("ensure household policy: %w", err)
	}
	var policyCurrency string
	if err := tx.QueryRow(ctx, `
		SELECT currency
		FROM household_policies
		WHERE household_id = $1
	`, householdID).Scan(&policyCurrency); err != nil {
		return Result{}, fmt.Errorf("read household policy: %w", err)
	}
	if policyCurrency != input.Currency {
		return Result{}, fmt.Errorf("existing household policy currency %s differs from bootstrap currency %s", policyCurrency, input.Currency)
	}

	var budgetPlanID int64
	var budgetCurrency string
	if err := tx.QueryRow(ctx, `
		INSERT INTO budget_plans (household_id, period, currency)
		VALUES ($1, $2, $3)
		ON CONFLICT (household_id, period) DO UPDATE SET updated_at = budget_plans.updated_at
		RETURNING id, currency
	`, householdID, input.Period, input.Currency).Scan(&budgetPlanID, &budgetCurrency); err != nil {
		return Result{}, fmt.Errorf("ensure budget plan: %w", err)
	}
	if budgetCurrency != input.Currency {
		return Result{}, fmt.Errorf("existing budget plan currency %s differs from bootstrap currency %s", budgetCurrency, input.Currency)
	}

	var adminUserID int64
	if admin != nil {
		adminUserID, err = ensureAdmin(ctx, tx, householdID, *admin)
		if err != nil {
			return Result{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return Result{}, fmt.Errorf("commit bootstrap transaction: %w", err)
	}
	return Result{HouseholdID: householdID, BudgetPlanID: budgetPlanID, AdminUserID: adminUserID}, nil
}

func Validate(input Input) (Input, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Currency = strings.TrimSpace(input.Currency)
	input.Timezone = strings.TrimSpace(input.Timezone)
	input.Period = strings.TrimSpace(input.Period)
	if input.Name == "" {
		return Input{}, fmt.Errorf("%w: name is required", ErrInvalidInput)
	}
	if !currencyPattern.MatchString(input.Currency) {
		return Input{}, fmt.Errorf("%w: currency must be a three-letter uppercase code", ErrInvalidInput)
	}
	if _, err := time.LoadLocation(input.Timezone); err != nil {
		return Input{}, fmt.Errorf("%w: invalid timezone %q", ErrInvalidInput, input.Timezone)
	}
	period, err := time.Parse("2006-01", input.Period)
	if err != nil || period.Format("2006-01") != input.Period {
		return Input{}, fmt.Errorf("%w: invalid period %q", ErrInvalidInput, input.Period)
	}
	if input.LiquidityFloorMinor < 0 {
		return Input{}, fmt.Errorf("%w: liquidity floor cannot be negative", ErrInvalidInput)
	}
	return input, nil
}

func prepareAdmin(input AdminInput) (preparedAdmin, error) {
	username := strings.TrimSpace(input.Username)
	normalized := strings.ToLower(username)
	if normalized == "" || len([]byte(normalized)) > 128 {
		return preparedAdmin{}, fmt.Errorf("%w: administrator username must be 1-128 bytes", ErrInvalidInput)
	}
	password := append([]byte(nil), input.Password...)
	defer clear(password)
	passwordHash, err := auth.HashPassword(string(password))
	if err != nil {
		return preparedAdmin{}, fmt.Errorf("%w: invalid administrator password: %v", ErrInvalidInput, err)
	}
	return preparedAdmin{username: username, normalizedUsername: normalized, passwordHash: passwordHash}, nil
}

func ensureAdmin(ctx context.Context, tx pgx.Tx, householdID int64, admin preparedAdmin) (int64, error) {
	var userID int64
	err := tx.QueryRow(ctx, `
		INSERT INTO auth_users (username, normalized_username, password_hash, household_id)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (normalized_username) DO NOTHING
		RETURNING id
	`, admin.username, admin.normalizedUsername, admin.passwordHash, householdID).Scan(&userID)
	if err == nil {
		return userID, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return 0, fmt.Errorf("create finance administrator: %w", err)
	}

	var existingHouseholdID int64
	var disabled bool
	if err := tx.QueryRow(ctx, `
		SELECT id, household_id, disabled_at IS NOT NULL
		FROM auth_users
		WHERE normalized_username = $1
	`, admin.normalizedUsername).Scan(&userID, &existingHouseholdID, &disabled); err != nil {
		return 0, fmt.Errorf("read existing finance administrator: %w", err)
	}
	if existingHouseholdID != householdID {
		return 0, fmt.Errorf("existing finance administrator belongs to household %d, not bootstrap household %d", existingHouseholdID, householdID)
	}
	if disabled {
		return 0, errors.New("existing finance administrator is disabled")
	}
	return userID, nil
}

func ensureHousehold(ctx context.Context, tx pgx.Tx, input Input) (int64, error) {
	rows, err := tx.Query(ctx, `
		SELECT id
		FROM households
		WHERE name = $1 AND base_currency = $2 AND timezone = $3
		ORDER BY id
		LIMIT 2
	`, input.Name, input.Currency, input.Timezone)
	if err != nil {
		return 0, fmt.Errorf("find bootstrap household: %w", err)
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, fmt.Errorf("scan bootstrap household: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, fmt.Errorf("list bootstrap households: %w", err)
	}
	rows.Close()
	if len(ids) > 1 {
		return 0, ErrDuplicateHousehold
	}
	if len(ids) == 1 {
		return ids[0], nil
	}

	var householdID int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO households (name, base_currency, timezone)
		VALUES ($1, $2, $3)
		RETURNING id
	`, input.Name, input.Currency, input.Timezone).Scan(&householdID); err != nil {
		return 0, fmt.Errorf("create bootstrap household: %w", err)
	}
	return householdID, nil
}
