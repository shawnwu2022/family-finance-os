package appapi

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shawnwu2022/family-finance-os/internal/budget"
	"github.com/shawnwu2022/family-finance-os/internal/goals"
	"github.com/shawnwu2022/family-finance-os/internal/household"
	"github.com/shawnwu2022/family-finance-os/internal/ledger"
	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

func TestSimulateGoalUsesExistingProjectionWithoutMutatingStoredGoal(t *testing.T) {
	now := time.Date(2026, 8, 17, 3, 30, 0, 0, time.UTC)
	original := goals.FinancialGoal{
		ID:                  7,
		HouseholdID:         42,
		Name:                "教育金",
		Target:              money.Money{Minor: 120_000, Currency: "CNY"},
		Funded:              money.Money{Minor: 20_000, Currency: "CNY"},
		TargetDate:          time.Date(2027, 6, 1, 0, 0, 0, 0, time.UTC),
		Priority:            1,
		Flexibility:         goals.GoalFlexibilityHard,
		MonthlyContribution: money.Money{Minor: 5_000, Currency: "CNY"},
		Active:              true,
	}
	planner := fakePlanner{
		profile: household.Profile{
			Household: household.Household{ID: 42, Name: "测试家庭", BaseCurrency: "CNY", Timezone: "Asia/Shanghai"},
			Policy: household.HouseholdPolicy{
				HouseholdID:    42,
				LiquidityFloor: money.Money{Minor: 20_000, Currency: "CNY"},
			},
		},
		plan: budget.BudgetPlan{ID: 1, HouseholdID: 42, Period: "2026-08", Currency: "CNY"},
		goals: []goals.FinancialGoal{original},
	}
	book := fakeLedger{
		accounts: []ledger.Account{
			{ID: "checking", Category: ledger.AccountCategoryChecking, Structure: ledger.AccountStructureSingle, Balance: money.Money{Minor: 100_000, Currency: "CNY"}, IsAsset: true},
		},
		transactions: []ledger.Transaction{
			{ID: "income", Type: ledger.TransactionTypeIncome, OccurredAt: time.Date(2026, 8, 5, 1, 0, 0, 0, time.UTC), SourceAccountID: "checking", SourceAmount: money.Money{Minor: 100_000, Currency: "CNY"}},
		},
	}
	api, err := New(Dependencies{Ledger: book, Planner: planner, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result, err := api.SimulateGoal(context.Background(), 42, 7, 20_000)
	if err != nil {
		t.Fatalf("SimulateGoal: %v", err)
	}
	if !result.DataAsOf.Equal(now) || result.Quality != "good" || result.GoalID != 7 {
		t.Fatalf("metadata=%#v", result)
	}
	if result.MonthlyContribution.Minor != 20_000 || result.MonthlyContribution.Currency != "CNY" {
		t.Fatalf("contribution=%#v", result.MonthlyContribution)
	}
	if result.MonthsRemaining != 10 || result.RequiredMonthly.Minor != 10_000 {
		t.Fatalf("months/required=%d/%#v", result.MonthsRemaining, result.RequiredMonthly)
	}
	if result.ProjectedFunded.Minor != 220_000 || result.GapAtTarget.Minor != 0 || result.CapacityShortfall.Minor != 0 {
		t.Fatalf("projection=%#v", result)
	}
	if result.Status != goals.GoalStatusOnTrack {
		t.Fatalf("status=%q want %q", result.Status, goals.GoalStatusOnTrack)
	}
	if planner.goals[0].MonthlyContribution.Minor != original.MonthlyContribution.Minor {
		t.Fatalf("stored goal mutated: %#v", planner.goals[0])
	}
}

func TestSimulateGoalRejectsInvalidInputsAndMissingGoal(t *testing.T) {
	now := time.Date(2026, 8, 17, 3, 30, 0, 0, time.UTC)
	planner := fakePlanner{
		profile: household.Profile{
			Household: household.Household{ID: 42, Name: "测试家庭", BaseCurrency: "CNY", Timezone: "Asia/Shanghai"},
			Policy:    household.HouseholdPolicy{HouseholdID: 42, LiquidityFloor: money.Money{Currency: "CNY"}},
		},
		plan: budget.BudgetPlan{ID: 1, HouseholdID: 42, Period: "2026-08", Currency: "CNY"},
		goals: []goals.FinancialGoal{
			{ID: 7, HouseholdID: 42, Name: "教育金", Target: money.Money{Minor: 120_000, Currency: "CNY"}, Funded: money.Money{Minor: 20_000, Currency: "CNY"}, TargetDate: time.Date(2027, 6, 1, 0, 0, 0, 0, time.UTC), Priority: 1, Flexibility: goals.GoalFlexibilityHard, MonthlyContribution: money.Money{Minor: 5_000, Currency: "CNY"}, Active: true},
		},
	}
	book := fakeLedger{accounts: []ledger.Account{{ID: "checking", Category: ledger.AccountCategoryChecking, Structure: ledger.AccountStructureSingle, Balance: money.Money{Minor: 100_000, Currency: "CNY"}, IsAsset: true}}}
	api, err := New(Dependencies{Ledger: book, Planner: planner, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if _, err := api.SimulateGoal(context.Background(), 42, 0, 1_000); err == nil {
		t.Fatal("zero goal id accepted")
	}
	if _, err := api.SimulateGoal(context.Background(), 42, 7, -1); err == nil {
		t.Fatal("negative contribution accepted")
	}
	if _, err := api.SimulateGoal(context.Background(), 42, 999, 1_000); !errors.Is(err, ErrGoalNotFound) {
		t.Fatalf("missing goal error=%v, want ErrGoalNotFound", err)
	}
}
