package appapi

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/shawnwu2022/family-finance-os/internal/agentadapter"
	"github.com/shawnwu2022/family-finance-os/internal/budget"
	"github.com/shawnwu2022/family-finance-os/internal/goals"
	"github.com/shawnwu2022/family-finance-os/internal/household"
	"github.com/shawnwu2022/family-finance-os/internal/ledger"
	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

func TestAgentAdapterPhaseOneDeterministicParity(t *testing.T) {
	now := time.Date(2026, 8, 17, 3, 30, 0, 0, time.UTC)
	api, err := New(Dependencies{
		Ledger: fakeLedger{
			accounts: []ledger.Account{
				{ID: "checking", Category: ledger.AccountCategoryChecking, Structure: ledger.AccountStructureSingle, Balance: money.Money{Minor: 100_000, Currency: "CNY"}, IsAsset: true},
			},
			transactions: []ledger.Transaction{
				{ID: "income", Type: ledger.TransactionTypeIncome, OccurredAt: time.Date(2026, 8, 5, 1, 0, 0, 0, time.UTC), SourceAccountID: "checking", SourceAmount: money.Money{Minor: 100_000, Currency: "CNY"}},
			},
		},
		Planner: fakePlanner{
			profile: household.Profile{
				Household: household.Household{ID: 42, Name: "测试家庭", BaseCurrency: "CNY", Timezone: "Asia/Shanghai"},
				Policy:    household.HouseholdPolicy{HouseholdID: 42, LiquidityFloor: money.Money{Minor: 20_000, Currency: "CNY"}},
			},
			plan: budget.BudgetPlan{ID: 1, HouseholdID: 42, Period: "2026-08", Currency: "CNY"},
			goals: []goals.FinancialGoal{
				{ID: 7, HouseholdID: 42, Name: "教育金", Target: money.Money{Minor: 120_000, Currency: "CNY"}, Funded: money.Money{Minor: 20_000, Currency: "CNY"}, TargetDate: time.Date(2027, 6, 1, 0, 0, 0, 0, time.UTC), Priority: 1, Flexibility: goals.GoalFlexibilityHard, MonthlyContribution: money.Money{Minor: 5_000, Currency: "CNY"}, Active: true},
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

	safe, err := api.SafeToSpend(ctx, 42)
	if err != nil {
		t.Fatalf("SafeToSpend: %v", err)
	}
	assertAgentParity(t, adapter, ctx, principal, agentadapter.ToolGetSafeToSpend, json.RawMessage(`{}`), safe)

	goal, err := api.SimulateGoal(ctx, 42, 7, 20_000)
	if err != nil {
		t.Fatalf("SimulateGoal: %v", err)
	}
	assertAgentParity(t, adapter, ctx, principal, agentadapter.ToolSimulateGoal, json.RawMessage(`{"goal_id":7,"monthly_contribution_minor":"20000"}`), goal)
}
