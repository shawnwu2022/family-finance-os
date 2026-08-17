package appapi

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/shawnwu2022/family-finance-os/internal/budget"
	"github.com/shawnwu2022/family-finance-os/internal/goals"
	"github.com/shawnwu2022/family-finance-os/internal/household"
	"github.com/shawnwu2022/family-finance-os/internal/ledger"
	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

func TestAPIComposesDeterministicHouseholdSnapshot(t *testing.T) {
	now := time.Date(2026, 8, 17, 3, 30, 0, 0, time.UTC)
	planner := fakePlanner{
		profile: household.Profile{
			Household: household.Household{ID: 42, Name: "测试家庭", BaseCurrency: "CNY", Timezone: "Asia/Shanghai"},
			Policy: household.HouseholdPolicy{HouseholdID: 42, LiquidityFloor: money.Money{Minor: 20_000, Currency: "CNY"}},
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
	ctx := context.Background()

	overview, err := api.Overview(ctx, 42)
	if err != nil {
		t.Fatalf("Overview: %v", err)
	}
	if overview.NetWorth.Minor != 80_000 || overview.NetCashflow.Minor != 80_000 {
		t.Fatalf("overview totals=%#v", overview)
	}
	if overview.SafeToSpend.Minor != 55_000 {
		t.Fatalf("safe-to-spend=%d want 55000", overview.SafeToSpend.Minor)
	}
	if overview.TotalDebt.Minor != 20_000 || overview.GoalCount != 1 {
		t.Fatalf("debt/goals=%#v", overview)
	}
	if overview.SavingsRate != "0.8" {
		t.Fatalf("savings rate=%q want 0.8", overview.SavingsRate)
	}
	if overview.Quality != "partial" || !containsWarning(overviewsWarnings(overview), "currency") {
		t.Fatalf("quality/warnings=%#v", overview)
	}

	cashflow, err := api.Cashflow(ctx, 42, "2026-08")
	if err != nil {
		t.Fatalf("Cashflow: %v", err)
	}
	if cashflow.Income.Minor != 100_000 || cashflow.Expense.Minor != 20_000 || cashflow.NetCashflow.Minor != 80_000 {
		t.Fatalf("cashflow=%#v", cashflow)
	}

	budgetStatus, err := api.Budget(ctx, 42, "2026-08")
	if err != nil {
		t.Fatalf("Budget: %v", err)
	}
	if len(budgetStatus.Lines) != 1 || budgetStatus.Lines[0].Actual.Minor != 20_000 || budgetStatus.Lines[0].Remaining.Minor != 10_000 {
		t.Fatalf("budget=%#v", budgetStatus)
	}

	goalStatus, err := api.Goals(ctx, 42)
	if err != nil {
		t.Fatalf("Goals: %v", err)
	}
	if len(goalStatus.Items) != 1 || goalStatus.Items[0].RequiredMonthly.Minor != 10_000 || goalStatus.Items[0].Status != "behind" {
		t.Fatalf("goals=%#v", goalStatus)
	}
}

func overviewsWarnings(value interface{ GetWarnings() []string }) []string {
	return value.GetWarnings()
}

func containsWarning(values []string, part string) bool {
	for _, value := range values {
		if strings.Contains(value, part) {
			return true
		}
	}
	return false
}

type fakeLedger struct {
	accounts     []ledger.Account
	transactions []ledger.Transaction
}

func (f fakeLedger) ListAccounts(context.Context) ([]ledger.Account, error) {
	return append([]ledger.Account(nil), f.accounts...), nil
}
func (f fakeLedger) ListCategories(context.Context) ([]ledger.Category, error) { return nil, nil }
func (f fakeLedger) ListTransactions(context.Context, ledger.TransactionQuery) ([]ledger.Transaction, error) {
	return append([]ledger.Transaction(nil), f.transactions...), nil
}

type fakePlanner struct {
	profile household.Profile
	plan    budget.BudgetPlan
	debts   []DebtSnapshot
	goals   []goals.FinancialGoal
}

func (f fakePlanner) Profile(context.Context, int64) (household.Profile, error) { return f.profile, nil }
func (f fakePlanner) BudgetPlan(context.Context, int64, string) (budget.BudgetPlan, error) { return f.plan, nil }
func (f fakePlanner) Debts(context.Context, int64) ([]DebtSnapshot, error) { return append([]DebtSnapshot(nil), f.debts...), nil }
func (f fakePlanner) Goals(context.Context, int64) ([]goals.FinancialGoal, error) { return append([]goals.FinancialGoal(nil), f.goals...), nil }
