package budget

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cockroachdb/apd/v3"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	storesqlc "github.com/shawnwu2022/family-finance-os/internal/store/sqlc"
	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

var ErrNegativePlanned = errors.New("planned budget amount must be non-negative")

var utilizationContext = apd.BaseContext.WithPrecision(34)

type BudgetLineMetrics struct {
	Planned     money.Money
	Actual      money.Money
	Remaining   money.Money
	Utilization *apd.Decimal
}

type Service struct {
	pool    *pgxpool.Pool
	queries *storesqlc.Queries
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool, queries: storesqlc.New(pool)}
}

func CalculateBudgetLine(line BudgetLine, actual money.Money) (BudgetLineMetrics, error) {
	if line.Planned.Minor < 0 {
		return BudgetLineMetrics{}, ErrNegativePlanned
	}

	remaining, err := line.Planned.Sub(actual)
	if err != nil {
		return BudgetLineMetrics{}, fmt.Errorf("calculate budget remaining: %w", err)
	}

	result := BudgetLineMetrics{
		Planned:   line.Planned,
		Actual:    actual,
		Remaining: remaining,
	}
	if line.Planned.Minor == 0 {
		return result, nil
	}

	utilization := new(apd.Decimal)
	_, err = utilizationContext.Quo(
		utilization,
		apd.New(actual.Minor, 0),
		apd.New(line.Planned.Minor, 0),
	)
	if err != nil {
		return BudgetLineMetrics{}, fmt.Errorf("calculate budget utilization: %w", err)
	}
	result.Utilization = utilization
	return result, nil
}

func (s *Service) CreatePlan(ctx context.Context, input NewBudgetPlan) (BudgetPlan, error) {
	if input.HouseholdID <= 0 {
		return BudgetPlan{}, errors.New("household ID must be positive")
	}
	period, err := validatePeriod(input.Period)
	if err != nil {
		return BudgetPlan{}, err
	}
	home, err := s.queries.GetHousehold(ctx, input.HouseholdID)
	if err != nil {
		return BudgetPlan{}, fmt.Errorf("get household: %w", err)
	}
	currency := strings.ToUpper(strings.TrimSpace(input.Currency))
	if currency != home.BaseCurrency {
		return BudgetPlan{}, fmt.Errorf("budget currency: %w: %s != %s", money.ErrCurrencyMismatch, currency, home.BaseCurrency)
	}
	row, err := s.queries.CreateBudgetPlan(ctx, storesqlc.CreateBudgetPlanParams{
		HouseholdID: input.HouseholdID,
		Period:      period,
		Currency:    home.BaseCurrency,
	})
	if err != nil {
		return BudgetPlan{}, fmt.Errorf("create budget plan: %w", err)
	}
	return BudgetPlan{
		ID:          row.ID,
		HouseholdID: row.HouseholdID,
		Period:      row.Period,
		Currency:    row.Currency,
		Lines:       []BudgetLine{},
	}, nil
}

func (s *Service) AddLine(ctx context.Context, planID int64, input NewBudgetLine) (BudgetLine, error) {
	if planID <= 0 {
		return BudgetLine{}, errors.New("budget plan ID must be positive")
	}
	plan, err := s.queries.GetBudgetPlan(ctx, planID)
	if err != nil {
		return BudgetLine{}, fmt.Errorf("get budget plan: %w", err)
	}
	if input.Planned.Minor < 0 {
		return BudgetLine{}, ErrNegativePlanned
	}
	if input.Planned.Currency != plan.Currency {
		return BudgetLine{}, fmt.Errorf("planned amount: %w: %s != %s", money.ErrCurrencyMismatch, input.Planned.Currency, plan.Currency)
	}
	if !input.Kind.valid() {
		return BudgetLine{}, fmt.Errorf("invalid budget kind %q", input.Kind)
	}

	categoryRef := strings.TrimSpace(input.ExternalCategoryRef)
	semanticGroup := strings.TrimSpace(input.SemanticGroup)
	if (categoryRef == "") == (semanticGroup == "") {
		return BudgetLine{}, errors.New("budget line must bind exactly one category ref or semantic group")
	}

	categoryValue := pgtype.Text{}
	semanticValue := pgtype.Text{}
	if categoryRef != "" {
		categoryValue = pgtype.Text{String: categoryRef, Valid: true}
	} else {
		semanticValue = pgtype.Text{String: semanticGroup, Valid: true}
	}

	row, err := s.queries.CreateBudgetLine(ctx, storesqlc.CreateBudgetLineParams{
		BudgetPlanID:        planID,
		ExternalCategoryRef: categoryValue,
		SemanticGroup:       semanticValue,
		PlannedMinor:        input.Planned.Minor,
		Kind:                string(input.Kind),
	})
	if err != nil {
		return BudgetLine{}, fmt.Errorf("create budget line: %w", err)
	}
	return budgetLineFromValues(row.ID, row.BudgetPlanID, row.ExternalCategoryRef, row.SemanticGroup, row.PlannedMinor, row.Kind, plan.Currency), nil
}

func (s *Service) GetPlan(ctx context.Context, householdID int64, period string) (BudgetPlan, error) {
	if householdID <= 0 {
		return BudgetPlan{}, errors.New("household ID must be positive")
	}
	period, err := validatePeriod(period)
	if err != nil {
		return BudgetPlan{}, err
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return BudgetPlan{}, fmt.Errorf("begin budget plan transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := storesqlc.New(tx)

	row, err := queries.GetBudgetPlanByHouseholdPeriod(ctx, storesqlc.GetBudgetPlanByHouseholdPeriodParams{
		HouseholdID: householdID,
		Period:      period,
	})
	if err != nil {
		return BudgetPlan{}, fmt.Errorf("get budget plan: %w", err)
	}
	lineRows, err := queries.ListBudgetLines(ctx, row.ID)
	if err != nil {
		return BudgetPlan{}, fmt.Errorf("list budget lines: %w", err)
	}
	plan := BudgetPlan{
		ID:          row.ID,
		HouseholdID: row.HouseholdID,
		Period:      row.Period,
		Currency:    row.Currency,
		Lines:       make([]BudgetLine, 0, len(lineRows)),
	}
	for _, line := range lineRows {
		plan.Lines = append(plan.Lines, budgetLineFromValues(line.ID, line.BudgetPlanID, line.ExternalCategoryRef, line.SemanticGroup, line.PlannedMinor, line.Kind, row.Currency))
	}
	if err := tx.Commit(ctx); err != nil {
		return BudgetPlan{}, fmt.Errorf("commit budget plan transaction: %w", err)
	}
	return plan, nil
}

func validatePeriod(raw string) (string, error) {
	period := strings.TrimSpace(raw)
	parsed, err := time.Parse("2006-01", period)
	if err != nil || parsed.Format("2006-01") != period {
		return "", fmt.Errorf("invalid budget period %q; expected YYYY-MM", raw)
	}
	return period, nil
}

func budgetLineFromValues(id, planID int64, categoryRef, semanticGroup pgtype.Text, plannedMinor int64, kind, currency string) BudgetLine {
	line := BudgetLine{
		ID:           id,
		BudgetPlanID: planID,
		Planned:      money.Money{Minor: plannedMinor, Currency: currency},
		Kind:         BudgetKind(kind),
	}
	if categoryRef.Valid {
		line.ExternalCategoryRef = categoryRef.String
	}
	if semanticGroup.Valid {
		line.SemanticGroup = semanticGroup.String
	}
	return line
}
