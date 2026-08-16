package goals

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	storesqlc "github.com/shawnwu2022/family-finance-os/internal/store/sqlc"
	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

type Service struct {
	queries *storesqlc.Queries
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{queries: storesqlc.New(pool)}
}

func (s *Service) CreateGoal(ctx context.Context, input NewFinancialGoal) (FinancialGoal, error) {
	if input.HouseholdID <= 0 {
		return FinancialGoal{}, errors.New("household ID must be positive")
	}

	candidate := FinancialGoal{
		HouseholdID:         input.HouseholdID,
		Name:                strings.TrimSpace(input.Name),
		Target:              input.Target,
		Funded:              input.Funded,
		TargetDate:          goalDate(input.TargetDate),
		Priority:            input.Priority,
		Flexibility:         input.Flexibility,
		MonthlyContribution: input.MonthlyContribution,
		Active:              input.Active,
	}
	if err := validateGoalFacts(candidate); err != nil {
		return FinancialGoal{}, err
	}

	home, err := s.queries.GetHousehold(ctx, input.HouseholdID)
	if err != nil {
		return FinancialGoal{}, fmt.Errorf("get household: %w", err)
	}
	if candidate.Target.Currency != home.BaseCurrency {
		return FinancialGoal{}, fmt.Errorf("goal currency: %w: %s != %s", money.ErrCurrencyMismatch, candidate.Target.Currency, home.BaseCurrency)
	}

	row, err := s.queries.CreateFinancialGoal(ctx, storesqlc.CreateFinancialGoalParams{
		HouseholdID:              candidate.HouseholdID,
		Name:                     candidate.Name,
		TargetMinor:              candidate.Target.Minor,
		FundedMinor:              candidate.Funded.Minor,
		TargetDate:               pgtype.Date{Time: candidate.TargetDate, Valid: true},
		Priority:                 candidate.Priority,
		Flexibility:              string(candidate.Flexibility),
		MonthlyContributionMinor: candidate.MonthlyContribution.Minor,
		Currency:                 home.BaseCurrency,
		Active:                   candidate.Active,
	})
	if err != nil {
		return FinancialGoal{}, fmt.Errorf("create financial goal: %w", err)
	}
	return goalFromValues(
		row.ID,
		row.HouseholdID,
		row.Name,
		row.TargetMinor,
		row.FundedMinor,
		row.TargetDate,
		row.Priority,
		row.Flexibility,
		row.MonthlyContributionMinor,
		row.Currency,
		row.Active,
	)
}

func (s *Service) GetGoal(ctx context.Context, goalID int64) (FinancialGoal, error) {
	if goalID <= 0 {
		return FinancialGoal{}, errors.New("goal ID must be positive")
	}
	row, err := s.queries.GetFinancialGoal(ctx, goalID)
	if err != nil {
		return FinancialGoal{}, fmt.Errorf("get financial goal: %w", err)
	}
	return goalFromValues(
		row.ID,
		row.HouseholdID,
		row.Name,
		row.TargetMinor,
		row.FundedMinor,
		row.TargetDate,
		row.Priority,
		row.Flexibility,
		row.MonthlyContributionMinor,
		row.Currency,
		row.Active,
	)
}

func (s *Service) ListGoals(ctx context.Context, householdID int64) ([]FinancialGoal, error) {
	if householdID <= 0 {
		return nil, errors.New("household ID must be positive")
	}
	rows, err := s.queries.ListFinancialGoalsByHousehold(ctx, householdID)
	if err != nil {
		return nil, fmt.Errorf("list financial goals: %w", err)
	}
	goals := make([]FinancialGoal, 0, len(rows))
	for _, row := range rows {
		goal, err := goalFromValues(
			row.ID,
			row.HouseholdID,
			row.Name,
			row.TargetMinor,
			row.FundedMinor,
			row.TargetDate,
			row.Priority,
			row.Flexibility,
			row.MonthlyContributionMinor,
			row.Currency,
			row.Active,
		)
		if err != nil {
			return nil, err
		}
		goals = append(goals, goal)
	}
	return goals, nil
}

func goalDate(value time.Time) time.Time {
	if value.IsZero() {
		return value
	}
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
}

func goalFromValues(
	id int64,
	householdID int64,
	name string,
	targetMinor int64,
	fundedMinor int64,
	targetDate pgtype.Date,
	priority int32,
	flexibility string,
	monthlyContributionMinor int64,
	currency string,
	active bool,
) (FinancialGoal, error) {
	if !targetDate.Valid {
		return FinancialGoal{}, errors.New("financial goal target date is invalid")
	}
	goal := FinancialGoal{
		ID:          id,
		HouseholdID: householdID,
		Name:        name,
		Target:      money.Money{Minor: targetMinor, Currency: currency},
		Funded:      money.Money{Minor: fundedMinor, Currency: currency},
		TargetDate:  goalDate(targetDate.Time),
		Priority:    priority,
		Flexibility: GoalFlexibility(flexibility),
		MonthlyContribution: money.Money{
			Minor: monthlyContributionMinor, Currency: currency,
		},
		Active: active,
	}
	if err := validateGoalFacts(goal); err != nil {
		return FinancialGoal{}, fmt.Errorf("decode financial goal: %w", err)
	}
	return goal, nil
}
