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

func TestAgentAdapterAssetAllocationDeterministicParity(t *testing.T) {
	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	book := fakeLedger{accounts: []ledger.Account{
		{ID: "checking", Category: ledger.AccountCategoryChecking, Structure: ledger.AccountStructureSingle, Balance: money.Money{Minor: 50_000, Currency: "CNY"}, IsAsset: true},
		{ID: "savings", Category: ledger.AccountCategorySavings, Structure: ledger.AccountStructureSingle, Balance: money.Money{Minor: 30_000, Currency: "CNY"}, IsAsset: true},
		{ID: "investment", Category: ledger.AccountCategoryInvestment, Structure: ledger.AccountStructureSingle, Balance: money.Money{Minor: 20_000, Currency: "CNY"}, IsAsset: true},
		{ID: "foreign", Category: ledger.AccountCategoryCash, Structure: ledger.AccountStructureSingle, Balance: money.Money{Minor: 10_000, Currency: "USD"}, IsAsset: true},
	}}
	planner := fakePlanner{profile: household.Profile{Household: household.Household{ID: 42, Name: "测试家庭", BaseCurrency: "CNY", Timezone: "Asia/Shanghai"}}}
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

	direct, err := api.AssetAllocation(ctx, 42)
	if err != nil {
		t.Fatalf("AssetAllocation: %v", err)
	}
	assertAgentParity(t, adapter, ctx, principal, agentadapter.ToolGetAssetAllocation, json.RawMessage(`{}`), direct)
}
