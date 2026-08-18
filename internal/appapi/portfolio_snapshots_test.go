package appapi

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/shawnwu2022/family-finance-os/internal/household"
	"github.com/shawnwu2022/family-finance-os/internal/portfolio"
	"github.com/shawnwu2022/family-finance-os/internal/server"
	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

func TestPortfolioAssetCRUDCanonicalizesScopesAndMapsDTOs(t *testing.T) {
	ctx := context.Background()
	asOf := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	store := &fakeAssetSnapshotStore{list: []portfolio.AssetSnapshot{
		{
			AssetRef:       "property:home",
			Name:           "自住房",
			Class:          portfolio.AssetClassProperty,
			Value:          money.Money{Minor: 5_000_000, Currency: "CNY"},
			SourceCurrency: "CNY",
			ValuationAsOf:  asOf,
			SourceKind:     portfolio.SnapshotSourceManual,
		},
		{
			AssetRef:         "brokerage:510300",
			Name:             "沪深300 ETF",
			Class:            portfolio.AssetClassFund,
			Value:            money.Money{Minor: 1_250_000, Currency: "CNY"},
			SourceCurrency:   "CNY",
			ValuationAsOf:    asOf,
			SourceAccountRef: "brokerage-1",
			SourceKind:       portfolio.SnapshotSourceManual,
		},
	}}
	api := newPortfolioTestAPI(t, store)

	got, err := api.UpsertPortfolioAsset(ctx, 42, " brokerage:510300 ", server.PortfolioAssetUpsertRequest{
		Name:             " 沪深300 ETF ",
		AssetClass:       " FUND ",
		ValueMinor:       1_250_000,
		Currency:         " cny ",
		SourceCurrency:   " cny ",
		ValuationAsOf:    asOf,
		SourceAccountRef: " brokerage-1 ",
		SourceKind:       " MANUAL ",
	})
	if err != nil {
		t.Fatalf("UpsertPortfolioAsset: %v", err)
	}
	if store.upsertCalls != 1 || store.lastHouseholdID != 42 {
		t.Fatalf("store upsert calls/scope=%d/%d want 1/42", store.upsertCalls, store.lastHouseholdID)
	}
	wantSnapshot := portfolio.AssetSnapshot{
		AssetRef:         "brokerage:510300",
		Name:             "沪深300 ETF",
		Class:            portfolio.AssetClassFund,
		Value:            money.Money{Minor: 1_250_000, Currency: "CNY"},
		SourceCurrency:   "CNY",
		ValuationAsOf:    asOf,
		SourceAccountRef: "brokerage-1",
		SourceKind:       portfolio.SnapshotSourceManual,
	}
	if !reflect.DeepEqual(store.lastSnapshot, wantSnapshot) {
		t.Fatalf("stored snapshot=%#v want %#v", store.lastSnapshot, wantSnapshot)
	}
	if got.AssetRef != wantSnapshot.AssetRef || got.Name != wantSnapshot.Name || got.AssetClass != "fund" ||
		got.ValueMinor != 1_250_000 || got.Currency != "CNY" || got.SourceCurrency != "CNY" ||
		got.SourceAccountRef != "brokerage-1" || got.SourceKind != "manual" || !got.ValuationAsOf.Equal(asOf) {
		t.Fatalf("response=%#v", got)
	}
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if !containsJSONFragment(encoded, `"value_minor":"1250000"`) {
		t.Fatalf("encoded response=%s want string-encoded value_minor", encoded)
	}

	listed, err := api.ListPortfolioAssets(ctx, 42)
	if err != nil {
		t.Fatalf("ListPortfolioAssets: %v", err)
	}
	if store.listCalls != 1 || store.lastHouseholdID != 42 {
		t.Fatalf("store list calls/scope=%d/%d want 1/42", store.listCalls, store.lastHouseholdID)
	}
	if len(listed.Items) != 2 || listed.Items[0].AssetRef != "brokerage:510300" || listed.Items[1].AssetRef != "property:home" {
		t.Fatalf("listed items=%#v want stable asset_ref ordering", listed.Items)
	}

	if err := api.DeletePortfolioAsset(ctx, 42, " brokerage:510300 "); err != nil {
		t.Fatalf("DeletePortfolioAsset: %v", err)
	}
	if store.deleteCalls != 1 || store.lastHouseholdID != 42 || store.lastAssetRef != "brokerage:510300" {
		t.Fatalf("delete calls/scope/ref=%d/%d/%q", store.deleteCalls, store.lastHouseholdID, store.lastAssetRef)
	}
}

func TestUpsertPortfolioAssetRejectsInvalidFactsBeforePersistence(t *testing.T) {
	store := &fakeAssetSnapshotStore{}
	api := newPortfolioTestAPI(t, store)

	_, err := api.UpsertPortfolioAsset(context.Background(), 42, "asset-1", server.PortfolioAssetUpsertRequest{
		Name:           "Invalid",
		AssetClass:     "crypto",
		ValueMinor:     100,
		Currency:       "CNY",
		SourceCurrency: "CNY",
		ValuationAsOf:  time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC),
		SourceKind:     "manual",
	})
	if err == nil {
		t.Fatal("UpsertPortfolioAsset error=nil want validation error")
	}
	if store.upsertCalls != 0 {
		t.Fatalf("store called for invalid request: %d", store.upsertCalls)
	}
}

func TestPortfolioAssetCRUDRequiresConfiguredStore(t *testing.T) {
	api := newPortfolioTestAPI(t, nil)
	if _, err := api.ListPortfolioAssets(context.Background(), 42); !errors.Is(err, ErrPortfolioUnavailable) {
		t.Fatalf("ListPortfolioAssets error=%v want ErrPortfolioUnavailable", err)
	}
}

func newPortfolioTestAPI(t *testing.T, store AssetSnapshotStore) *API {
	t.Helper()
	api, err := New(Dependencies{
		Ledger: fakeLedger{},
		Planner: fakePlanner{profile: household.Profile{Household: household.Household{
			ID: 42, Name: "测试家庭", BaseCurrency: "CNY", Timezone: "Asia/Shanghai",
		}}},
		Portfolio: store,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return api
}

type fakeAssetSnapshotStore struct {
	list            []portfolio.AssetSnapshot
	listErr         error
	upsertErr       error
	deleteErr       error
	listCalls       int
	upsertCalls     int
	deleteCalls     int
	lastHouseholdID int64
	lastAssetRef    string
	lastSnapshot    portfolio.AssetSnapshot
}

func (f *fakeAssetSnapshotStore) ListAssetSnapshots(_ context.Context, householdID int64) ([]portfolio.AssetSnapshot, error) {
	f.listCalls++
	f.lastHouseholdID = householdID
	return append([]portfolio.AssetSnapshot(nil), f.list...), f.listErr
}

func (f *fakeAssetSnapshotStore) UpsertAssetSnapshot(_ context.Context, householdID int64, snapshot portfolio.AssetSnapshot) (portfolio.AssetSnapshot, error) {
	f.upsertCalls++
	f.lastHouseholdID = householdID
	f.lastSnapshot = snapshot
	return snapshot, f.upsertErr
}

func (f *fakeAssetSnapshotStore) DeleteAssetSnapshot(_ context.Context, householdID int64, assetRef string) error {
	f.deleteCalls++
	f.lastHouseholdID = householdID
	f.lastAssetRef = assetRef
	return f.deleteErr
}

func containsJSONFragment(data []byte, fragment string) bool {
	return len(data) >= len(fragment) && string(data) != "" && containsString(string(data), fragment)
}

func containsString(value, fragment string) bool {
	for i := 0; i+len(fragment) <= len(value); i++ {
		if value[i:i+len(fragment)] == fragment {
			return true
		}
	}
	return false
}
