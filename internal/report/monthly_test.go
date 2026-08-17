package report

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

func TestGenerateMonthlyKeepsDeterministicMetricsIndependentFromNarrative(t *testing.T) {
	dataAsOf := time.Date(2026, time.August, 1, 0, 5, 0, 0, time.UTC)
	generatedAt := time.Date(2026, time.August, 1, 0, 10, 0, 0, time.UTC)
	metrics := MonthlyMetrics{
		Income:          money.Money{Minor: 3500000, Currency: "CNY"},
		Expense:         money.Money{Minor: 2100000, Currency: "CNY"},
		NetCashflow:     money.Money{Minor: 1400000, Currency: "CNY"},
		NetWorth:        money.Money{Minor: 52000000, Currency: "CNY"},
		SafeToSpend:     money.Money{Minor: 680000, Currency: "CNY"},
		TotalDebt:       money.Money{Minor: 18000000, Currency: "CNY"},
		SavingsRate:     "0.4",
		EmergencyMonths: "5.25",
	}

	base, err := GenerateMonthly(MonthlyInput{
		Period:      "2026-07",
		DataAsOf:    dataAsOf,
		GeneratedAt: generatedAt,
		Quality:     "good",
		Metrics:     metrics,
		Warnings:    []string{"ledger reconciled through month end"},
	})
	if err != nil {
		t.Fatalf("GenerateMonthly() error = %v", err)
	}
	withNarrative, err := GenerateMonthly(MonthlyInput{
		Period:      "2026-07",
		DataAsOf:    dataAsOf,
		GeneratedAt: generatedAt,
		Quality:     "good",
		Metrics:     metrics,
		Warnings:    []string{"ledger reconciled through month end"},
		Narrative:   "LLM explanation that must not change numeric facts.",
	})
	if err != nil {
		t.Fatalf("GenerateMonthly() with narrative error = %v", err)
	}

	if base.Kind != KindMonthly || base.Period != "2026-07" {
		t.Fatalf("unexpected identity: kind=%q period=%q", base.Kind, base.Period)
	}
	if !base.DataAsOf.Equal(dataAsOf) || !base.GeneratedAt.Equal(generatedAt) {
		t.Fatalf("unexpected timestamps: data_as_of=%v generated_at=%v", base.DataAsOf, base.GeneratedAt)
	}
	if !reflect.DeepEqual(base.Metrics, metrics) {
		t.Fatalf("metrics changed: got %#v want %#v", base.Metrics, metrics)
	}
	if !reflect.DeepEqual(withNarrative.Metrics, base.Metrics) {
		t.Fatalf("narrative altered deterministic metrics: got %#v want %#v", withNarrative.Metrics, base.Metrics)
	}
	if base.Narrative != "" || withNarrative.Narrative == "" {
		t.Fatalf("unexpected narrative handling: base=%q enhanced=%q", base.Narrative, withNarrative.Narrative)
	}
}

func TestGenerateMonthlyRejectsMixedCurrencyMetrics(t *testing.T) {
	_, err := GenerateMonthly(MonthlyInput{
		Period:      "2026-07",
		DataAsOf:    time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC),
		GeneratedAt: time.Date(2026, time.August, 1, 0, 1, 0, 0, time.UTC),
		Quality:     "good",
		Metrics: MonthlyMetrics{
			Income:      money.Money{Minor: 10000, Currency: "CNY"},
			Expense:     money.Money{Minor: 5000, Currency: "USD"},
			NetCashflow: money.Money{Minor: 5000, Currency: "CNY"},
			NetWorth:    money.Money{Currency: "CNY"},
			SafeToSpend: money.Money{Currency: "CNY"},
			TotalDebt:   money.Money{Currency: "CNY"},
		},
	})
	if !errors.Is(err, ErrMixedCurrency) {
		t.Fatalf("GenerateMonthly() error = %v, want ErrMixedCurrency", err)
	}
}

func TestGenerateMonthlyRejectsInvalidPeriod(t *testing.T) {
	_, err := GenerateMonthly(MonthlyInput{Period: "2026-7"})
	if !errors.Is(err, ErrInvalidPeriod) {
		t.Fatalf("GenerateMonthly() error = %v, want ErrInvalidPeriod", err)
	}
}
