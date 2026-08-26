package report

import (
	"testing"
	"time"

	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

func TestContentHashIsDeterministicAndCoversMetrics(t *testing.T) {
	monthly := MonthlyReport{
		Kind: KindMonthly, Period: "2026-07", DataAsOf: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		GeneratedAt: time.Date(2026, 8, 1, 3, 0, 0, 0, time.UTC), Quality: "good",
		Metrics: MonthlyMetrics{NetWorth: money.Money{Minor: 100_000, Currency: "CNY"}},
	}
	first, err := ContentHash(monthly)
	if err != nil {
		t.Fatalf("ContentHash: %v", err)
	}
	second, err := ContentHash(monthly)
	if err != nil {
		t.Fatalf("ContentHash again: %v", err)
	}
	if len(first) != 64 || first != second {
		t.Fatalf("hashes = %q and %q", first, second)
	}
	monthly.Metrics.NetWorth.Minor++
	changed, err := ContentHash(monthly)
	if err != nil {
		t.Fatalf("ContentHash changed: %v", err)
	}
	if changed == first {
		t.Fatal("metric change did not change content hash")
	}
}
