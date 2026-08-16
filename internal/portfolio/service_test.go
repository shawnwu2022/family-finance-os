package portfolio

import (
	"errors"
	"testing"
	"time"

	"github.com/cockroachdb/apd/v3"
	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

func TestSummarizePortfolioAllocationTotals(t *testing.T) {
	asOf := time.Date(2026, 8, 16, 0, 0, 0, 0, time.UTC)
	got, err := Summarize(SummaryInput{
		ReportingCurrency: "CNY",
		AsOf:              asOf,
		FXStaleAfter:      72 * time.Hour,
		Valuations: []Valuation{
			{ID: "cash", Name: "活期", Class: AssetClassCash, Value: money.Money{Minor: 50_000, Currency: "CNY"}, SourceCurrency: "CNY", ValuationAsOf: asOf},
			{ID: "equity", Name: "权益", Class: AssetClassEquity, Value: money.Money{Minor: 30_000, Currency: "CNY"}, SourceCurrency: "CNY", ValuationAsOf: asOf},
			{ID: "gold", Name: "黄金", Class: AssetClassGold, Value: money.Money{Minor: 20_000, Currency: "CNY"}, SourceCurrency: "CNY", ValuationAsOf: asOf},
		},
	})
	if err != nil {
		t.Fatalf("Summarize: %v", err)
	}
	if got.Total != (money.Money{Minor: 100_000, Currency: "CNY"}) {
		t.Fatalf("total=%#v", got.Total)
	}
	assertAllocation(t, got, AssetClassCash, 50_000, apd.New(5, -1))
	assertAllocation(t, got, AssetClassEquity, 30_000, apd.New(3, -1))
	assertAllocation(t, got, AssetClassGold, 20_000, apd.New(2, -1))
	if len(got.Warnings) != 0 {
		t.Fatalf("warnings=%#v", got.Warnings)
	}
}

func TestSummarizePortfolioWarnsOnForeignFXFreshness(t *testing.T) {
	asOf := time.Date(2026, 8, 16, 0, 0, 0, 0, time.UTC)
	stale := asOf.Add(-10 * 24 * time.Hour)
	got, err := Summarize(SummaryInput{
		ReportingCurrency: "CNY",
		AsOf:              asOf,
		FXStaleAfter:      72 * time.Hour,
		Valuations: []Valuation{
			{ID: "usd-stale", Name: "美股", Class: AssetClassEquity, Value: money.Money{Minor: 10_000, Currency: "CNY"}, SourceCurrency: "USD", ValuationAsOf: asOf, FXAsOf: &stale},
			{ID: "usd-missing", Name: "美元现金", Class: AssetClassCash, Value: money.Money{Minor: 5_000, Currency: "CNY"}, SourceCurrency: "USD", ValuationAsOf: asOf},
		},
	})
	if err != nil {
		t.Fatalf("Summarize: %v", err)
	}
	if !hasWarning(got.Warnings, WarningFXStale, "usd-stale") {
		t.Fatalf("missing stale FX warning: %#v", got.Warnings)
	}
	if !hasWarning(got.Warnings, WarningFXMissing, "usd-missing") {
		t.Fatalf("missing FX warning: %#v", got.Warnings)
	}
}

func TestSummarizePortfolioRejectsUnsafeInputs(t *testing.T) {
	asOf := time.Date(2026, 8, 16, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		in   SummaryInput
		want error
	}{
		{
			name: "cross currency valuation",
			in: SummaryInput{ReportingCurrency: "CNY", AsOf: asOf, Valuations: []Valuation{{ID: "x", Class: AssetClassCash, Value: money.Money{Minor: 1, Currency: "USD"}, SourceCurrency: "USD", ValuationAsOf: asOf}}},
			want: money.ErrCurrencyMismatch,
		},
		{
			name: "negative valuation",
			in: SummaryInput{ReportingCurrency: "CNY", AsOf: asOf, Valuations: []Valuation{{ID: "x", Class: AssetClassCash, Value: money.Money{Minor: -1, Currency: "CNY"}, SourceCurrency: "CNY", ValuationAsOf: asOf}}},
			want: ErrNegativeValuation,
		},
		{
			name: "unknown asset class",
			in: SummaryInput{ReportingCurrency: "CNY", AsOf: asOf, Valuations: []Valuation{{ID: "x", Class: AssetClass("mystery"), Value: money.Money{Minor: 1, Currency: "CNY"}, SourceCurrency: "CNY", ValuationAsOf: asOf}}},
			want: ErrInvalidAssetClass,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Summarize(tt.in)
			if !errors.Is(err, tt.want) {
				t.Fatalf("error=%v want %v", err, tt.want)
			}
		})
	}
}

func assertAllocation(t *testing.T, summary Summary, class AssetClass, minor int64, share *apd.Decimal) {
	t.Helper()
	allocation, ok := summary.ByClass[class]
	if !ok {
		t.Fatalf("missing allocation %q", class)
	}
	if allocation.Value.Minor != minor || allocation.Value.Currency != summary.Total.Currency {
		t.Fatalf("allocation[%q]=%#v", class, allocation)
	}
	if allocation.Share == nil || allocation.Share.Cmp(share) != 0 {
		t.Fatalf("share[%q]=%v want %v", class, allocation.Share, share)
	}
}

func hasWarning(warnings []Warning, code WarningCode, valuationID string) bool {
	for _, warning := range warnings {
		if warning.Code == code && warning.ValuationID == valuationID {
			return true
		}
	}
	return false
}
