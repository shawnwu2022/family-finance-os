package appapi

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/shawnwu2022/family-finance-os/internal/analytics"
	"github.com/shawnwu2022/family-finance-os/internal/budget"
	"github.com/shawnwu2022/family-finance-os/internal/goals"
	"github.com/shawnwu2022/family-finance-os/internal/household"
	"github.com/shawnwu2022/family-finance-os/internal/ledger"
	"github.com/shawnwu2022/family-finance-os/internal/server"
	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

func TestAPIComposesDeterministicHouseholdSnapshot(t *testing.T) {
	now := time.Date(2026, 8, 17, 3, 30, 0, 0, time.UTC)
	planner := fakePlanner{
		profile: household.Profile{
			Household: household.Household{ID: 42, Name: "测试家庭", BaseCurrency: "CNY", Timezone: "Asia/Shanghai"},
			Policy:    household.HouseholdPolicy{HouseholdID: 42, LiquidityFloor: money.Money{Minor: 20_000, Currency: "CNY"}},
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
	if overview.Quality != "partial" || !containsWarning(overview.Warnings, "currency") {
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

func TestScenarioHandlesZeroIncomeSavingsRate(t *testing.T) {
	now := time.Date(2026, 8, 17, 3, 30, 0, 0, time.UTC)
	planner := fakePlanner{
		profile: household.Profile{
			Household: household.Household{ID: 42, Name: "empty-income", BaseCurrency: "CNY", Timezone: "Asia/Shanghai"},
			Policy:    household.HouseholdPolicy{HouseholdID: 42, LiquidityFloor: money.Money{Currency: "CNY"}},
		},
		plan: budget.BudgetPlan{ID: 1, HouseholdID: 42, Period: "2026-08", Currency: "CNY"},
	}
	book := fakeLedger{accounts: []ledger.Account{
		{ID: "checking", Category: ledger.AccountCategoryChecking, Structure: ledger.AccountStructureSingle, Balance: money.Money{Minor: 10_000, Currency: "CNY"}, IsAsset: true},
	}}
	api, err := New(Dependencies{Ledger: book, Planner: planner, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result, err := api.Scenario(context.Background(), server.ScenarioRequest{
		HouseholdID: 42,
		Kind:        "purchase",
		Input:       json.RawMessage(`{"amount_minor":"100","currency":"CNY"}`),
	})
	if err != nil {
		t.Fatalf("Scenario: %v", err)
	}
	if !json.Valid(result.Result) {
		t.Fatalf("scenario result is not valid JSON: %q", result.Result)
	}
	if strings.Contains(string(result.Result), "savings_rate") {
		t.Fatalf("zero-income savings rate should be omitted: %s", result.Result)
	}
}

func TestBudgetHandlesZeroPlannedUtilization(t *testing.T) {
	now := time.Date(2026, 8, 17, 3, 30, 0, 0, time.UTC)
	planner := fakePlanner{
		profile: household.Profile{
			Household: household.Household{ID: 42, Name: "zero-budget", BaseCurrency: "CNY", Timezone: "Asia/Shanghai"},
			Policy:    household.HouseholdPolicy{HouseholdID: 42, LiquidityFloor: money.Money{Currency: "CNY"}},
		},
		plan: budget.BudgetPlan{
			ID: 1, HouseholdID: 42, Period: "2026-08", Currency: "CNY",
			Lines: []budget.BudgetLine{
				{ID: 1, BudgetPlanID: 1, ExternalCategoryRef: "food", Planned: money.Money{Currency: "CNY"}, Kind: budget.BudgetKindEssential},
			},
		},
	}
	book := fakeLedger{accounts: []ledger.Account{
		{ID: "checking", Category: ledger.AccountCategoryChecking, Structure: ledger.AccountStructureSingle, Balance: money.Money{Minor: 10_000, Currency: "CNY"}, IsAsset: true},
	}}
	api, err := New(Dependencies{Ledger: book, Planner: planner, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	status, err := api.Budget(context.Background(), 42, "2026-08")
	if err != nil {
		t.Fatalf("Budget: %v", err)
	}
	if len(status.Lines) != 1 {
		t.Fatalf("budget lines=%d want 1", len(status.Lines))
	}
	if status.Lines[0].Utilization != "" {
		t.Fatalf("zero-planned utilization=%q want empty", status.Lines[0].Utilization)
	}
}

func TestDashboardBuildsOneBoundedSnapshotForSelectedPeriod(t *testing.T) {
	now := time.Date(2026, 8, 17, 3, 30, 0, 0, time.UTC)
	queries := []ledger.TransactionQuery{}
	planner := fakePlanner{
		profile: household.Profile{
			Household: household.Household{ID: 42, Name: "测试家庭", BaseCurrency: "CNY", Timezone: "Asia/Shanghai"},
			Policy:    household.HouseholdPolicy{HouseholdID: 42, LiquidityFloor: money.Money{Currency: "CNY"}},
		},
		plan: budget.BudgetPlan{ID: 1, HouseholdID: 42, Period: "2026-08", Currency: "CNY"},
	}
	book := fakeLedger{
		accounts: []ledger.Account{{ID: "checking", Category: ledger.AccountCategoryChecking, Structure: ledger.AccountStructureSingle, Balance: money.Money{Minor: 100_000, Currency: "CNY"}, IsAsset: true}},
		queries:  &queries,
	}
	api, err := New(Dependencies{Ledger: book, Planner: planner, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	dashboard, err := api.Dashboard(context.Background(), 42, "2026-08")
	if err != nil {
		t.Fatalf("Dashboard: %v", err)
	}
	if dashboard.Cashflow.Period != "2026-08" || dashboard.Budget.Period != "2026-08" {
		t.Fatalf("dashboard period = %#v", dashboard)
	}
	if len(queries) != 1 {
		t.Fatalf("ledger transaction queries = %d, want 1", len(queries))
	}
	wantStart := time.Date(2026, 7, 31, 16, 0, 0, 0, time.UTC)
	wantEnd := time.Date(2026, 8, 31, 16, 0, 0, 0, time.UTC)
	if !queries[0].Start.Equal(wantStart) || !queries[0].End.Equal(wantEnd) {
		t.Fatalf("transaction query bounds = %v..%v, want %v..%v", queries[0].Start, queries[0].End, wantStart, wantEnd)
	}
}

func TestBudgetResponseEncodesEmptyLinesAsArray(t *testing.T) {
	response := budgetResponse(snapshot{
		profile: household.Profile{Household: household.Household{BaseCurrency: "CNY"}},
	}, "2026-08")
	if response.Lines == nil {
		t.Fatal("budget lines must be a non-nil empty slice")
	}

	encoded, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal budget response: %v", err)
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("decode budget response: %v", err)
	}
	if got := string(payload["lines"]); got != "[]" {
		t.Fatalf("budget lines JSON = %s, want []", got)
	}
}

func TestGoalProjectionAllocatesSharedCapacityByPriority(t *testing.T) {
	asOf := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	goalList := []goals.FinancialGoal{
		{ID: 2, Name: "次要目标", Target: money.Money{Minor: 80_000, Currency: "CNY"}, Funded: money.Money{Currency: "CNY"}, TargetDate: time.Date(2027, 6, 1, 0, 0, 0, 0, time.UTC), Priority: 2, Flexibility: goals.GoalFlexibilityHard, MonthlyContribution: money.Money{Minor: 8_000, Currency: "CNY"}, Active: true},
		{ID: 1, Name: "首要目标", Target: money.Money{Minor: 80_000, Currency: "CNY"}, Funded: money.Money{Currency: "CNY"}, TargetDate: time.Date(2027, 6, 1, 0, 0, 0, 0, time.UTC), Priority: 1, Flexibility: goals.GoalFlexibilityHard, MonthlyContribution: money.Money{Minor: 8_000, Currency: "CNY"}, Active: true},
	}
	items, err := projectGoalDTOs(goalList, analytics.CashflowResult{NetCashflow: money.Money{Minor: 10_000, Currency: "CNY"}}, asOf)
	if err != nil {
		t.Fatalf("projectGoalDTOs: %v", err)
	}
	if len(items) != 2 || items[0].ID != 1 || items[0].Status != string(goals.GoalStatusOnTrack) {
		t.Fatalf("priority allocation first goal = %#v", items)
	}
	if items[1].ID != 2 || items[1].Status != string(goals.GoalStatusConflicting) || items[1].CapacityShortfall.Minor != 6_000 {
		t.Fatalf("priority allocation second goal = %#v", items[1])
	}
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
	queries      *[]ledger.TransactionQuery
}

func (f fakeLedger) ListAccounts(context.Context) ([]ledger.Account, error) {
	return append([]ledger.Account(nil), f.accounts...), nil
}
func (f fakeLedger) ListCategories(context.Context) ([]ledger.Category, error) { return nil, nil }
func (f fakeLedger) ListTransactions(_ context.Context, query ledger.TransactionQuery) ([]ledger.Transaction, error) {
	if f.queries != nil {
		*f.queries = append(*f.queries, query)
	}
	return append([]ledger.Transaction(nil), f.transactions...), nil
}

type fakePlanner struct {
	profile household.Profile
	plan    budget.BudgetPlan
	debts   []DebtSnapshot
	goals   []goals.FinancialGoal
}

func (f fakePlanner) Profile(context.Context, int64) (household.Profile, error) {
	return f.profile, nil
}
func (f fakePlanner) BudgetPlan(context.Context, int64, string) (budget.BudgetPlan, error) {
	return f.plan, nil
}
func (f fakePlanner) Debts(context.Context, int64) ([]DebtSnapshot, error) {
	return append([]DebtSnapshot(nil), f.debts...), nil
}
func (f fakePlanner) Goals(context.Context, int64) ([]goals.FinancialGoal, error) {
	return append([]goals.FinancialGoal(nil), f.goals...), nil
}
