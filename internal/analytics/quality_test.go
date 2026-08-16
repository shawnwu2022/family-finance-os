package analytics

import (
	"testing"
	"time"

	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

func TestEvaluateDataQuality(t *testing.T) {
	t.Parallel()

	asOf := time.Date(2026, 8, 16, 1, 0, 0, 0, time.UTC)
	staleAfter := 24 * time.Hour

	tests := []struct {
		name    string
		synced  time.Time
		unknown money.Money
		want    QualityLevel
	}{
		{
			name:    "good when fresh and complete",
			synced:  asOf.Add(-time.Hour),
			unknown: money.Money{Minor: 0, Currency: "CNY"},
			want:    QualityGood,
		},
		{
			name:    "partial when unknown monetary events exist",
			synced:  asOf.Add(-time.Hour),
			unknown: money.Money{Minor: 12_300, Currency: "CNY"},
			want:    QualityPartial,
		},
		{
			name:    "stale dominates partial",
			synced:  asOf.Add(-48 * time.Hour),
			unknown: money.Money{Minor: 12_300, Currency: "CNY"},
			want:    QualityStale,
		},
		{
			name:    "missing sync is stale",
			synced:  time.Time{},
			unknown: money.Money{Minor: 0, Currency: "CNY"},
			want:    QualityStale,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := EvaluateDataQuality(asOf, tt.synced, tt.unknown, staleAfter)
			if got.Level != tt.want {
				t.Fatalf("Level = %v, want %v", got.Level, tt.want)
			}
			if !got.AsOf.Equal(asOf) || !got.LedgerSyncedAt.Equal(tt.synced) || got.UnknownAmount != tt.unknown {
				t.Fatalf("quality context changed: %#v", got)
			}
		})
	}
}
