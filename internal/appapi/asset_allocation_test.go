package appapi

import (
	"context"
	"testing"
	"time"

	"github.com/shawnwu2022/family-finance-os/internal/household"
	"github.com/shawnwu2022/family-finance-os/internal/ledger"
	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

func TestAssetAllocationUsesOnlyProvableAccountClasses(t *testing.T) {
	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	book := fakeLedger{accounts: []ledger.Account{
		{ID: "cash", Category: ledger.AccountCategoryCash, Structure: ledger.AccountStructureSingle, Balance: money.Money{Minor: 10_000, Currency: "CNY"}, IsAsset: true},
		{ID: "checking", Category: ledger.AccountCategoryChecking, Structure: ledger.AccountStructureSingle, Balance: money.Money{Minor: 20_000, Currency: "CNY"}, IsAsset: true},
		{ID: "wallet", Category: ledger.AccountCategoryVirtual, Structure: ledger.AccountStructureSingle, Balance: money.Money{Minor: 5_000, Currency: "CNY"}, IsAsset: true},
		{ID: "savings", Category: ledger.AccountCategorySavings, Structure: ledger.AccountStructureSingle, Balance: money.Money{Minor: 30_000, Currency: "CNY"}, IsAsset: true},
		{ID: "cd", Category: ledger.AccountCategoryCertificateOfDeposit, Structure: ledger.AccountStructureSingle, Balance: money.Money{Minor: 40_000, Currency: "CNY"}, IsAsset: true},
		{ID: "broker", Category: ledger.AccountCategoryInvestment, Structure: ledger.AccountStructureSingle, Balance: money.Money{Minor: 50_000, Currency: "CNY"}, IsAsset: true},
		{ID: "receivable", Category: ledger.AccountCategoryReceivables, Structure: ledger.AccountStructureSingle, Balance: money.Money{Minor: 60_000, Currency: "CNY"}, IsAsset: true},
		{ID: "unknown", Category: ledger.AccountCategoryUnknown, Structure: ledger.AccountStructureSingle, Balance: money.Money{Minor: 7_000, Currency: "CNY"}, IsAsset: true},
		{ID: "usd", Category: ledger.AccountCategoryInvestment, Structure: ledger.AccountStructureSingle, Balance: money.Money{Minor: 10_000, Currency: "USD"}, IsAsset: true},
		{ID: "hidden", Category: ledger.AccountCategoryCash, Structure: ledger.AccountStructureSingle, Balance: money.Money{Minor: 10_000, Currency: "CNY"}, IsAsset: true, Hidden: true},
		{ID: "parent", Category: ledger.AccountCategorySavings, Structure: ledger.AccountStructureMultipleSubAccounts, Balance: money.Money{Minor: 99_999, Currency: "CNY"}, IsAsset: true},
		{ID: "card", Category: ledger.AccountCategoryCreditCard, Structure: ledger.AccountStructureSingle, Balance: money.Money{Minor: 20_000, Currency: "CNY"}, IsLiability: true},
	}}
	planner := fakePlanner{profile: household.Profile{Household: household.Household{ID: 42, Name: "测试家庭", BaseCurrency: "CNY", Timezone: "Asia/Shanghai"}}}
	api, err := New(Dependencies{Ledger: book, Planner: planner, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	got, err := api.AssetAllocation(context.Background(), 42)
	if err != nil {
		t.Fatalf("AssetAllocation: %v", err)
	}
	if !got.DataAsOf.Equal(now) || got.Currency != "CNY" || got.Quality != "partial" {
		t.Fatalf("metadata=%#v", got)
	}
	if got.Total.Minor != 222_000 || got.Total.Currency != "CNY" {
		t.Fatalf("total=%#v want 222000 CNY", got.Total)
	}
	if len(got.Items) != 3 {
		t.Fatalf("items=%#v want 3 classes", got.Items)
	}
	wantClasses := []string{"cash", "deposit", "other"}
	wantMinor := []int64{35_000, 70_000, 117_000}
	for i, item := range got.Items {
		if item.Class != wantClasses[i] || item.Value.Minor != wantMinor[i] || item.Value.Currency != "CNY" || item.Share == "" {
			t.Fatalf("item[%d]=%#v", i, item)
		}
	}
	for _, part := range []string{"investment", "receivables", "unknown", "USD"} {
		if !containsWarning(got.Warnings, part) {
			t.Fatalf("warnings=%#v missing %q", got.Warnings, part)
		}
	}
}
