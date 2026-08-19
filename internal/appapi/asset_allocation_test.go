package appapi

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/shawnwu2022/family-finance-os/internal/household"
	"github.com/shawnwu2022/family-finance-os/internal/ledger"
	"github.com/shawnwu2022/family-finance-os/internal/portfolio"
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

func TestAssetAllocationMergesExplicitSnapshotsWithoutDoubleCounting(t *testing.T) {
	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	fxAsOf := now.Add(-time.Hour)
	book := fakeLedger{accounts: []ledger.Account{
		{ID: "checking", Name: "Checking", Category: ledger.AccountCategoryChecking, Structure: ledger.AccountStructureSingle, Balance: money.Money{Minor: 10_000, Currency: "CNY"}, IsAsset: true},
		{ID: "broker-covered", Name: "Covered broker", Category: ledger.AccountCategoryInvestment, Structure: ledger.AccountStructureSingle, Balance: money.Money{Minor: 50_000, Currency: "CNY"}, IsAsset: true},
		{ID: "broker-uncovered", Name: "Uncovered broker", Category: ledger.AccountCategoryInvestment, Structure: ledger.AccountStructureSingle, Balance: money.Money{Minor: 10_000, Currency: "CNY"}, IsAsset: true},
	}}
	snapshots := &fakeAssetSnapshotStore{list: []portfolio.AssetSnapshot{
		{
			AssetRef:         "broker:equity",
			Name:             "Equity position",
			Class:            portfolio.AssetClassEquity,
			Value:            money.Money{Minor: 20_000, Currency: "CNY"},
			SourceCurrency:   "CNY",
			ValuationAsOf:    now,
			SourceAccountRef: "broker-covered",
			SourceKind:       portfolio.SnapshotSourceManual,
		},
		{
			AssetRef:         "broker:fund",
			Name:             "Fund position",
			Class:            portfolio.AssetClassFund,
			Value:            money.Money{Minor: 30_000, Currency: "CNY"},
			SourceCurrency:   "CNY",
			ValuationAsOf:    now,
			SourceAccountRef: "broker-covered",
			SourceKind:       portfolio.SnapshotSourceImport,
		},
		{
			AssetRef:       "property:home",
			Name:           "Home",
			Class:          portfolio.AssetClassProperty,
			Value:          money.Money{Minor: 20_000, Currency: "CNY"},
			SourceCurrency: "CNY",
			ValuationAsOf:  now,
			SourceKind:     portfolio.SnapshotSourceManual,
		},
		{
			AssetRef:       "gold:converted",
			Name:           "Gold",
			Class:          portfolio.AssetClassGold,
			Value:          money.Money{Minor: 10_000, Currency: "CNY"},
			SourceCurrency: "USD",
			ValuationAsOf:  now,
			FXAsOf:         &fxAsOf,
			SourceKind:     portfolio.SnapshotSourceManual,
		},
		{
			AssetRef:         "foreign-position",
			Name:             "Foreign position",
			Class:            portfolio.AssetClassEquity,
			Value:            money.Money{Minor: 99_000, Currency: "USD"},
			SourceCurrency:   "USD",
			ValuationAsOf:    now,
			SourceAccountRef: "broker-uncovered",
			SourceKind:       portfolio.SnapshotSourceManual,
		},
	}}
	planner := fakePlanner{profile: household.Profile{Household: household.Household{ID: 42, Name: "测试家庭", BaseCurrency: "CNY", Timezone: "Asia/Shanghai"}}}
	api, err := New(Dependencies{Ledger: book, Planner: planner, Portfolio: snapshots, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	got, err := api.AssetAllocation(context.Background(), 42)
	if err != nil {
		t.Fatalf("AssetAllocation: %v", err)
	}
	if snapshots.listCalls != 1 || snapshots.lastHouseholdID != 42 {
		t.Fatalf("snapshot list calls/scope=%d/%d want 1/42", snapshots.listCalls, snapshots.lastHouseholdID)
	}
	if !got.DataAsOf.Equal(now) || got.Currency != "CNY" || got.Quality != "partial" {
		t.Fatalf("metadata=%#v", got)
	}
	if got.Total.Minor != 100_000 || got.Total.Currency != "CNY" {
		t.Fatalf("total=%#v want 100000 CNY", got.Total)
	}
	wantClasses := []string{"cash", "equity", "fund", "gold", "other", "property"}
	wantMinor := []int64{10_000, 20_000, 30_000, 10_000, 10_000, 20_000}
	wantShare := []string{
		"0.1000000000000000000000000000000000",
		"0.2000000000000000000000000000000000",
		"0.3000000000000000000000000000000000",
		"0.1000000000000000000000000000000000",
		"0.1000000000000000000000000000000000",
		"0.2000000000000000000000000000000000",
	}
	if len(got.Items) != len(wantClasses) {
		t.Fatalf("items=%#v want %d classes", got.Items, len(wantClasses))
	}
	for i, item := range got.Items {
		if item.Class != wantClasses[i] || item.Value.Minor != wantMinor[i] || item.Value.Currency != "CNY" || item.Share != wantShare[i] {
			t.Fatalf("item[%d]=%#v want class=%s minor=%d share=%s", i, item, wantClasses[i], wantMinor[i], wantShare[i])
		}
	}
	for _, part := range []string{"broker-uncovered", "foreign-position", "USD"} {
		if !containsWarning(got.Warnings, part) {
			t.Fatalf("warnings=%#v missing %q", got.Warnings, part)
		}
	}
	for _, warning := range got.Warnings {
		if strings.Contains(warning, "broker-covered") {
			t.Fatalf("covered account leaked fallback warning: %#v", got.Warnings)
		}
	}
}
