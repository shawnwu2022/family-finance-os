package appapi

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/shawnwu2022/family-finance-os/internal/agentadapter"
	"github.com/shawnwu2022/family-finance-os/internal/household"
	"github.com/shawnwu2022/family-finance-os/internal/ledger"
	"github.com/shawnwu2022/family-finance-os/internal/portfolio"
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
	snapshots := &fakeAssetSnapshotStore{list: []portfolio.AssetSnapshot{
		{
			AssetRef:         "investment:equity",
			Name:             "Equity position",
			Class:            portfolio.AssetClassEquity,
			Value:            money.Money{Minor: 20_000, Currency: "CNY"},
			SourceCurrency:   "CNY",
			ValuationAsOf:    now,
			SourceAccountRef: "investment",
			SourceKind:       portfolio.SnapshotSourceManual,
		},
		{
			AssetRef:       "property:home",
			Name:           "Home",
			Class:          portfolio.AssetClassProperty,
			Value:          money.Money{Minor: 10_000, Currency: "CNY"},
			SourceCurrency: "CNY",
			ValuationAsOf:  now,
			SourceKind:     portfolio.SnapshotSourceManual,
		},
	}}
	planner := fakePlanner{profile: household.Profile{Household: household.Household{ID: 42, Name: "测试家庭", BaseCurrency: "CNY", Timezone: "Asia/Shanghai"}}}
	api, err := New(Dependencies{Ledger: book, Planner: planner, Portfolio: snapshots, Now: func() time.Time { return now }})
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
	if direct.Total.Minor != 110_000 || direct.Total.Currency != "CNY" {
		t.Fatalf("direct total=%#v want 110000 CNY", direct.Total)
	}
	wantClasses := []string{"cash", "deposit", "equity", "property"}
	if len(direct.Items) != len(wantClasses) {
		t.Fatalf("direct items=%#v", direct.Items)
	}
	for i, class := range wantClasses {
		if direct.Items[i].Class != class {
			t.Fatalf("direct item[%d]=%#v want class %s", i, direct.Items[i], class)
		}
	}
	assertAgentParity(t, adapter, ctx, principal, agentadapter.ToolGetAssetAllocation, json.RawMessage(`{}`), direct)
}
