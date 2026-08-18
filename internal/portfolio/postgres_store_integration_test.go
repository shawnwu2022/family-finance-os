package portfolio

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shawnwu2022/family-finance-os/internal/config"
	householdpkg "github.com/shawnwu2022/family-finance-os/internal/household"
	"github.com/shawnwu2022/family-finance-os/internal/store"
	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

func TestPostgresStoreAssetSnapshotRoundTripAndHouseholdIsolationIntegration(t *testing.T) {
	pool := openPortfolioIntegrationPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	homeService := householdpkg.NewService(pool)
	homeA, err := homeService.CreateHousehold(ctx, householdpkg.NewHousehold{Name: "组合测试家庭 A", BaseCurrency: "CNY", Timezone: "Asia/Shanghai"})
	if err != nil {
		t.Fatalf("CreateHousehold A: %v", err)
	}
	homeB, err := homeService.CreateHousehold(ctx, householdpkg.NewHousehold{Name: "组合测试家庭 B", BaseCurrency: "CNY", Timezone: "Asia/Shanghai"})
	if err != nil {
		t.Fatalf("CreateHousehold B: %v", err)
	}

	store := NewPostgresStore(pool)
	asOf := time.Date(2026, 8, 18, 12, 34, 56, 123456000, time.UTC)
	fxAsOf := asOf.Add(-2 * time.Hour)
	sharedRef := "brokerage:shared"

	a := AssetSnapshot{
		AssetRef:         sharedRef,
		Name:             "A 沪深300 ETF",
		Class:            AssetClassFund,
		Value:            money.Money{Minor: 1_250_000, Currency: "CNY"},
		SourceCurrency:   "CNY",
		ValuationAsOf:    asOf,
		SourceAccountRef: "brokerage-a",
		SourceKind:       SnapshotSourceManual,
	}
	b := AssetSnapshot{
		AssetRef:         sharedRef,
		Name:             "B 美股基金",
		Class:            AssetClassFund,
		Value:            money.Money{Minor: 900_000, Currency: "CNY"},
		SourceCurrency:   "USD",
		ValuationAsOf:    asOf,
		FXAsOf:           &fxAsOf,
		SourceAccountRef: "brokerage-b",
		SourceKind:       SnapshotSourceImport,
	}

	gotA, err := store.UpsertAssetSnapshot(ctx, homeA.ID, a)
	if err != nil {
		t.Fatalf("UpsertAssetSnapshot A: %v", err)
	}
	assertSnapshotRoundTrip(t, gotA, a)
	gotB, err := store.UpsertAssetSnapshot(ctx, homeB.ID, b)
	if err != nil {
		t.Fatalf("UpsertAssetSnapshot B: %v", err)
	}
	assertSnapshotRoundTrip(t, gotB, b)

	secondA := AssetSnapshot{
		AssetRef:       "property:home",
		Name:           "自住房",
		Class:          AssetClassProperty,
		Value:          money.Money{Minor: 5_000_000, Currency: "CNY"},
		SourceCurrency: "CNY",
		ValuationAsOf:  asOf.Add(time.Minute),
		SourceKind:     SnapshotSourceManual,
	}
	if _, err := store.UpsertAssetSnapshot(ctx, homeA.ID, secondA); err != nil {
		t.Fatalf("UpsertAssetSnapshot second A: %v", err)
	}

	listA, err := store.ListAssetSnapshots(ctx, homeA.ID)
	if err != nil {
		t.Fatalf("ListAssetSnapshots A: %v", err)
	}
	if len(listA) != 2 || listA[0].AssetRef != sharedRef || listA[1].AssetRef != secondA.AssetRef {
		t.Fatalf("list A=%#v want stable asset_ref ordering", listA)
	}
	listB, err := store.ListAssetSnapshots(ctx, homeB.ID)
	if err != nil {
		t.Fatalf("ListAssetSnapshots B: %v", err)
	}
	if len(listB) != 1 {
		t.Fatalf("list B len=%d want 1", len(listB))
	}
	assertSnapshotRoundTrip(t, listB[0], b)

	a.Name = "A 沪深300 ETF 更新"
	a.Value.Minor = 1_300_000
	updatedA, err := store.UpsertAssetSnapshot(ctx, homeA.ID, a)
	if err != nil {
		t.Fatalf("update A: %v", err)
	}
	assertSnapshotRoundTrip(t, updatedA, a)
	listB, err = store.ListAssetSnapshots(ctx, homeB.ID)
	if err != nil {
		t.Fatalf("ListAssetSnapshots B after A update: %v", err)
	}
	assertSnapshotRoundTrip(t, listB[0], b)

	if err := store.DeleteAssetSnapshot(ctx, homeA.ID, sharedRef); err != nil {
		t.Fatalf("DeleteAssetSnapshot A: %v", err)
	}
	listA, err = store.ListAssetSnapshots(ctx, homeA.ID)
	if err != nil {
		t.Fatalf("ListAssetSnapshots A after delete: %v", err)
	}
	if len(listA) != 1 || listA[0].AssetRef != secondA.AssetRef {
		t.Fatalf("list A after delete=%#v", listA)
	}
	listB, err = store.ListAssetSnapshots(ctx, homeB.ID)
	if err != nil {
		t.Fatalf("ListAssetSnapshots B after A delete: %v", err)
	}
	if len(listB) != 1 || listB[0].AssetRef != sharedRef {
		t.Fatalf("A delete affected B: %#v", listB)
	}

	if err := store.DeleteAssetSnapshot(ctx, homeA.ID, "missing"); err != nil {
		t.Fatalf("idempotent delete missing: %v", err)
	}
}

func TestPortfolioSnapshotDatabaseConstraintsIntegration(t *testing.T) {
	pool := openPortfolioIntegrationPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	homeService := householdpkg.NewService(pool)
	home, err := homeService.CreateHousehold(ctx, householdpkg.NewHousehold{Name: "组合约束家庭", BaseCurrency: "CNY", Timezone: "Asia/Shanghai"})
	if err != nil {
		t.Fatalf("CreateHousehold: %v", err)
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO portfolio_asset_snapshots (
			household_id, asset_ref, name, asset_class, value_minor, currency,
			source_currency, valuation_as_of, source_kind
		) VALUES ($1, 'bad', 'invalid', 'crypto', 100, 'CNY', 'CNY', CURRENT_TIMESTAMP, 'manual')
	`, home.ID)
	if err == nil {
		t.Fatal("database accepted unsupported asset_class")
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO portfolio_asset_snapshots (
			household_id, asset_ref, name, asset_class, value_minor, currency,
			source_currency, valuation_as_of, source_kind
		) VALUES ($1, 'bad-fx', 'invalid fx', 'fund', 100, 'CNY', 'USD', CURRENT_TIMESTAMP, 'manual')
	`, home.ID)
	if err == nil {
		t.Fatal("database accepted foreign source currency without fx_as_of")
	}
}

func assertSnapshotRoundTrip(t *testing.T, got, want AssetSnapshot) {
	t.Helper()
	if got.AssetRef != want.AssetRef || got.Name != want.Name || got.Class != want.Class || got.Value != want.Value ||
		got.SourceCurrency != want.SourceCurrency || got.SourceAccountRef != want.SourceAccountRef || got.SourceKind != want.SourceKind {
		t.Fatalf("snapshot=%#v want %#v", got, want)
	}
	if !got.ValuationAsOf.Equal(want.ValuationAsOf) {
		t.Fatalf("valuation_as_of=%v want %v", got.ValuationAsOf, want.ValuationAsOf)
	}
	if (got.FXAsOf == nil) != (want.FXAsOf == nil) {
		t.Fatalf("fx_as_of=%v want %v", got.FXAsOf, want.FXAsOf)
	}
	if got.FXAsOf != nil && !got.FXAsOf.Equal(*want.FXAsOf) {
		t.Fatalf("fx_as_of=%v want %v", got.FXAsOf, want.FXAsOf)
	}
}

func openPortfolioIntegrationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	host := os.Getenv("TEST_POSTGRES_HOST")
	if host == "" {
		t.Skip("TEST_POSTGRES_HOST is not set")
	}
	portRaw := os.Getenv("TEST_POSTGRES_PORT")
	if portRaw == "" {
		portRaw = "5432"
	}
	port, err := strconv.ParseUint(portRaw, 10, 16)
	if err != nil {
		t.Fatalf("parse TEST_POSTGRES_PORT: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pool, err := store.OpenPostgres(ctx, config.DatabaseConfig{
		Host: host, Port: uint16(port), Name: os.Getenv("TEST_POSTGRES_DB"),
		User: os.Getenv("TEST_POSTGRES_USER"), Password: os.Getenv("TEST_POSTGRES_PASSWORD"), SSLMode: "disable",
	})
	if err != nil {
		t.Fatalf("OpenPostgres: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}
