package appapi

import (
	"context"
	"fmt"

	"github.com/cockroachdb/apd/v3"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shawnwu2022/family-finance-os/internal/budget"
	"github.com/shawnwu2022/family-finance-os/internal/goals"
	"github.com/shawnwu2022/family-finance-os/internal/household"
	storesqlc "github.com/shawnwu2022/family-finance-os/internal/store/sqlc"
	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

type PostgresPlanner struct {
	households *household.Service
	budgets    *budget.Service
	goals      *goals.Service
	queries    *storesqlc.Queries
}

func NewPostgresPlanner(pool *pgxpool.Pool) *PostgresPlanner {
	return &PostgresPlanner{
		households: household.NewService(pool),
		budgets:    budget.NewService(pool),
		goals:      goals.NewService(pool),
		queries:    storesqlc.New(pool),
	}
}

func (p *PostgresPlanner) Profile(ctx context.Context, householdID int64) (household.Profile, error) {
	profile, err := p.households.GetProfile(ctx, householdID)
	if err != nil {
		return household.Profile{}, fmt.Errorf("get household profile: %w", err)
	}
	return profile, nil
}

func (p *PostgresPlanner) BudgetPlan(ctx context.Context, householdID int64, period string) (budget.BudgetPlan, error) {
	plan, err := p.budgets.GetPlan(ctx, householdID, period)
	if err != nil {
		return budget.BudgetPlan{}, fmt.Errorf("get budget plan: %w", err)
	}
	return plan, nil
}

func (p *PostgresPlanner) Goals(ctx context.Context, householdID int64) ([]goals.FinancialGoal, error) {
	rows, err := p.goals.ListGoals(ctx, householdID)
	if err != nil {
		return nil, fmt.Errorf("list financial goals: %w", err)
	}
	active := make([]goals.FinancialGoal, 0, len(rows))
	for _, goal := range rows {
		if goal.Active {
			active = append(active, goal)
		}
	}
	return active, nil
}

func (p *PostgresPlanner) Debts(ctx context.Context, householdID int64) ([]DebtSnapshot, error) {
	rows, err := p.queries.ListDebtsByHousehold(ctx, householdID)
	if err != nil {
		return nil, fmt.Errorf("list household debts: %w", err)
	}
	debts := make([]DebtSnapshot, 0, len(rows))
	for _, row := range rows {
		if !row.Active {
			continue
		}
		apr, err := numericString(row.Apr)
		if err != nil {
			return nil, fmt.Errorf("decode debt %d APR: %w", row.ID, err)
		}
		debts = append(debts, DebtSnapshot{
			ID:                  row.ID,
			Name:                row.Name,
			Type:                row.DebtType,
			Balance:             money.Money{Minor: row.BalanceMinor, Currency: row.Currency},
			APR:                 apr,
			RepaymentType:       row.RepaymentType,
			MinimumPayment:      money.Money{Minor: row.MinimumPaymentMinor, Currency: row.Currency},
			ScheduledPayment:    money.Money{Minor: row.ScheduledPaymentMinor, Currency: row.Currency},
			TermRemainingMonths: row.TermRemainingMonths,
			DueDay:              row.DueDay,
			SourceAccountRef:    row.SourceAccountRef.String,
			Active:              true,
		})
	}
	return debts, nil
}

func numericString(value pgtype.Numeric) (string, error) {
	if !value.Valid {
		return "", nil
	}
	driverValue, err := value.Value()
	if err != nil {
		return "", err
	}
	if driverValue == nil {
		return "", nil
	}
	decimal, _, err := apd.NewFromString(fmt.Sprint(driverValue))
	if err != nil {
		return "", err
	}
	return decimal.String(), nil
}
