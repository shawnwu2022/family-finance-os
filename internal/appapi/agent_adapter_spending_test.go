package appapi

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/shawnwu2022/family-finance-os/internal/agentadapter"
	"github.com/shawnwu2022/family-finance-os/internal/household"
	"github.com/shawnwu2022/family-finance-os/internal/ledger"
	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

func TestAgentAdapterSpendingAnalysisDeterministicParity(t *testing.T) {
	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	book := &spendingLedger{
		accounts: []ledger.Account{{ID: "checking", Category: ledger.AccountCategoryChecking, Structure: ledger.AccountStructureSingle, IsAsset: true}},
		categories: []ledger.Category{
			{ID: "food", Name: "餐饮", Type: ledger.CategoryTypeExpense},
			{ID: "housing", Name: "住房", Type: ledger.CategoryTypeExpense},
		},
		transactions: []ledger.Transaction{
			{ID: "aug-food", Type: ledger.TransactionTypeExpense, CategoryID: "food", OccurredAt: time.Date(2026, 8, 8, 1, 0, 0, 0, time.UTC), SourceAccountID: "checking", SourceAmount: money.Money{Minor: 10_000, Currency: "CNY"}},
			{ID: "aug-housing", Type: ledger.TransactionTypeExpense, CategoryID: "housing", OccurredAt: time.Date(2026, 8, 10, 1, 0, 0, 0, time.UTC), SourceAccountID: "checking", SourceAmount: money.Money{Minor: 20_000, Currency: "CNY"}},
			{ID: "jul-food", Type: ledger.TransactionTypeExpense, CategoryID: "food", OccurredAt: time.Date(2026, 7, 10, 1, 0, 0, 0, time.UTC), SourceAccountID: "checking", SourceAmount: money.Money{Minor: 5_000, Currency: "CNY"}},
		},
	}
	planner := &spendingPlanner{profile: household.Profile{Household: household.Household{ID: 42, Name: "测试家庭", BaseCurrency: "CNY", Timezone: "Asia/Shanghai"}}}
	api, err := New(Dependencies{Ledger: book, Planner: planner, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	adapter, err := agentadapter.New(api)
	if err != nil {
		t.Fatalf("agentadapter.New: %v", err)
	}
	ctx := context.Background()
	principal := agentadapter.Principal{Kind: "test", HouseholdID: 42}

	direct, err := api.SpendingAnalysis(ctx, 42, "2026-08", 1)
	if err != nil {
		t.Fatalf("SpendingAnalysis: %v", err)
	}
	assertAgentParity(t, adapter, ctx, principal, agentadapter.ToolGetSpendingAnalysis, json.RawMessage(`{"period":"2026-08","compare_periods":1}`), direct)
}
