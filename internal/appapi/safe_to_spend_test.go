package appapi

import (
	"context"
	"testing"
	"time"

	"github.com/shawnwu2022/family-finance-os/internal/budget"
	"github.com/shawnwu2022/family-finance-os/internal/goals"
	"github.com/shawnwu2022/family-finance-os/internal/household"
	"github.com/shawnwu2022/family-finance-os/internal/ledger"
	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

func TestSafeToSpendReturnsExistingDeterministicComponents(t *testing.T) {
	now := time.Date(2026, 8, 17, 3, 30, 0, 0, time.UTC)
	planner := fakePlanner{
		profile: household.Profile{
			Household: household.Household{ID: 42, Name: "测试家庭", BaseCurrency: "CNY", Timezone: "Asia/Shanghai"},
			Policy: household.HouseholdPolicy{
				HouseholdID:     42,
				LiquidityFloor: money.Money{Minor: 20_000, Currency: "CNY"},
			},
		},
		plan: budget.BudgetPlan{
			ID: 1, HouseholdID: 42, Period: "2026-08", Currency: "CNY",
			Lines: []budget.BudgetLine{
				{ID: 1, BudgetPlanID: 1, ExternalCategoryRef: "food", Planned: money.Money{Minor: 30_000, Currency: "CNY"}, Kind: budget.BudgetKindEssential},
			},
		},
		debts: []DebtSnapshot{
			{ID: 1, Name: "信用卡", Type: "credit_card", Balance: money.Money{Minor: 20_000, Currency: "CNY"}, APR: "0.18", RepaymentType: "revolving", MinimumPayment: money.Money{Minor: 5_000, Currency: "CNY"}, ScheduledPayment: money.Money{Minor: 10_000, Currency: "CNY"}, DueDay: 20},
		},
		goals: []goals.FinancialGoal{
			{ID: 1, HouseholdID: 42, Name: "教育金", Target: money.Money{Minor: 120_000, Currency: "CNY"}, Funded: money.Money{Minor: 20_000, Currency: "CNY"}, TargetDate: time.Date(2027, 6, 1, 0, 0, 0, 0, time.UTC), Priority: 1, Flexibility: goals.GoalFlexibilityHard, MonthlyContribution: money.Money{Minor: 5_000, Currency: "CNY"}, Active: true},
		},
	}
	book := fakeLedger{
		accounts: []ledger.Account{
			{ID: "checking", Category: ledger.AccountCategoryChecking, Structure: ledger.AccountStructureSingle, Balance: money.Money{Minor: 100_000, Currency: "CNY"}, IsAsset: true},
			{ID: "card", Category: ledger.AccountCategoryCreditCard, Structure: ledger.AccountStructureSingle, Balance: money.Money{Minor: 20_000, Currency: "CNY"}, IsLiability: true},
			{ID: "usd", Category: ledger.AccountCategoryChecking, Structure: ledger.AccountStructureSingle, Balance: money.Money{Minor: 10_000, Currency: "USD"}, IsAsset: true},
		},
		transactions: []ledger.Transaction{
			{ID: "income", Type: ledger.TransactionTypeIncome, OccurredAt: time.Date(2026, 8, 5, 1, 0, 0, 0, time.UTC), SourceAccountID: "checking", SourceAmount: money.Money{Minor: 100_000, Currency: "CNY"}},
			{ID: "food", Type: ledger.TransactionTypeExpense, CategoryID: "food", OccurredAt: time.Date(2026, 8, 8, 1, 0, 0, 0, time.UTC), SourceAccountID: "checking", SourceAmount: money.Money{Minor: 20_000, Currency: "CNY"}},
		},
	}
	api, err := New(Dependencies{Ledger: book, Planner: planner, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	safe, err := api.SafeToSpend(context.Background(), 42)
	if err != nil {
		t.Fatalf("SafeToSpend: %v", err)
	}
	if !safe.DataAsOf.Equal(now) || safe.Period != "2026-08" || safe.Quality != "partial" {
		t.Fatalf("metadata=%#v", safe)
	}
	if safe.Amount.Minor != 55_000 || safe.Amount.Currency != "CNY" || safe.IsDeficit {
		t.Fatalf("amount=%#v deficit=%v", safe.Amount, safe.IsDeficit)
	}
	c := safe.Components
	if c.LiquidDiscretionaryPool.Minor != 100_000 ||
		c.UpcomingMandatoryExpenses.Minor != 0 ||
		c.DebtCommitments.Minor != 10_000 ||
		c.EssentialReserveUntilPeriodEnd.Minor != 10_000 ||
		c.EmergencyFundGapReserved.Minor != 20_000 ||
		c.HardGoalContributions.Minor != 5_000 {
		t.Fatalf("components=%#v", c)
	}
	if !containsWarning(safe.Warnings, "currency") {
		t.Fatalf("warnings=%#v", safe.Warnings)
	}
}
