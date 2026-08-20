package appapi

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/shawnwu2022/family-finance-os/internal/agentadapter"
	"github.com/shawnwu2022/family-finance-os/internal/budget"
	"github.com/shawnwu2022/family-finance-os/internal/goals"
	"github.com/shawnwu2022/family-finance-os/internal/household"
	"github.com/shawnwu2022/family-finance-os/internal/ledger"
	"github.com/shawnwu2022/family-finance-os/internal/server"
	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

func TestAgentAdapterDeterministicParity(t *testing.T) {
	now := time.Date(2026, 8, 17, 3, 30, 0, 0, time.UTC)
	api, err := New(Dependencies{
		Ledger: fakeLedger{
			accounts: []ledger.Account{
				{ID: "checking", Category: ledger.AccountCategoryChecking, Structure: ledger.AccountStructureSingle, Balance: money.Money{Minor: 100_000, Currency: "CNY"}, IsAsset: true},
				{ID: "card", Category: ledger.AccountCategoryCreditCard, Structure: ledger.AccountStructureSingle, Balance: money.Money{Minor: 20_000, Currency: "CNY"}, IsLiability: true},
				{ID: "usd", Category: ledger.AccountCategoryChecking, Structure: ledger.AccountStructureSingle, Balance: money.Money{Minor: 10_000, Currency: "USD"}, IsAsset: true},
			},
			transactions: []ledger.Transaction{
				{ID: "income", Type: ledger.TransactionTypeIncome, OccurredAt: time.Date(2026, 8, 5, 1, 0, 0, 0, time.UTC), SourceAccountID: "checking", SourceAmount: money.Money{Minor: 100_000, Currency: "CNY"}},
				{ID: "food", Type: ledger.TransactionTypeExpense, CategoryID: "food", OccurredAt: time.Date(2026, 8, 8, 1, 0, 0, 0, time.UTC), SourceAccountID: "checking", SourceAmount: money.Money{Minor: 20_000, Currency: "CNY"}},
			},
		},
		Planner: fakePlanner{
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
		},
		Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	adapter, err := agentadapter.New(api)
	if err != nil {
		t.Fatalf("agentadapter.New: %v", err)
	}
	ctx := context.Background()
	principal := agentadapter.Principal{Kind: "test", HouseholdID: 42}

	overview, err := api.Overview(ctx, 42)
	if err != nil {
		t.Fatalf("Overview: %v", err)
	}
	assertAgentParity(t, adapter, ctx, principal, agentadapter.ToolGetHouseholdOverview, json.RawMessage(`{}`), overview)

	cashflow, err := api.Cashflow(ctx, 42, "2026-08")
	if err != nil {
		t.Fatalf("Cashflow: %v", err)
	}
	assertAgentParity(t, adapter, ctx, principal, agentadapter.ToolGetCashflow, json.RawMessage(`{"period":"2026-08"}`), cashflow)

	budgetStatus, err := api.Budget(ctx, 42, "2026-08")
	if err != nil {
		t.Fatalf("Budget: %v", err)
	}
	assertAgentParity(t, adapter, ctx, principal, agentadapter.ToolGetBudgetStatus, json.RawMessage(`{"period":"2026-08"}`), budgetStatus)

	debts, err := api.Debts(ctx, 42)
	if err != nil {
		t.Fatalf("Debts: %v", err)
	}
	assertAgentParity(t, adapter, ctx, principal, agentadapter.ToolGetDebtStatus, json.RawMessage(`{}`), debts)

	goalStatus, err := api.Goals(ctx, 42)
	if err != nil {
		t.Fatalf("Goals: %v", err)
	}
	assertAgentParity(t, adapter, ctx, principal, agentadapter.ToolGetGoalStatus, json.RawMessage(`{}`), goalStatus)

	purchaseInput := json.RawMessage(`{"amount_minor":"10000","currency":"CNY"}`)
	scenario, err := api.Scenario(ctx, server.ScenarioRequest{HouseholdID: 42, Kind: "purchase", Input: purchaseInput})
	if err != nil {
		t.Fatalf("Scenario: %v", err)
	}
	assertAgentParity(t, adapter, ctx, principal, agentadapter.ToolSimulatePurchase, purchaseInput, scenario)

	monthly, err := api.MonthlyReport(ctx, 42, "2026-07")
	if err != nil {
		t.Fatalf("MonthlyReport: %v", err)
	}
	assertAgentParity(t, adapter, ctx, principal, agentadapter.ToolGenerateMonthlyReport, json.RawMessage(`{"year":2026,"month":7}`), monthly)
}

func assertAgentParity[T any](t *testing.T, adapter *agentadapter.Service, ctx context.Context, principal agentadapter.Principal, tool agentadapter.ToolName, arguments json.RawMessage, direct T) {
	t.Helper()
	result, err := adapter.Call(ctx, principal, tool, arguments)
	if err != nil {
		t.Fatalf("Call(%s): %v", tool, err)
	}
	var got T
	if err := json.Unmarshal(result.Data, &got); err != nil {
		t.Fatalf("decode %s result: %v", tool, err)
	}
	if !reflect.DeepEqual(got, direct) {
		t.Fatalf("%s adapter result=%#v, direct=%#v", tool, got, direct)
	}
}
