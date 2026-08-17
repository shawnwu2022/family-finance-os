package report

import (
	"errors"
	"time"

	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

const (
	KindMonthly    = "monthly"
	JobNameMonthly = "monthly_report"
)

var (
	ErrInvalidPeriod = errors.New("invalid report period")
	ErrMixedCurrency = errors.New("report metrics contain mixed currencies")
)

type MonthlyMetrics struct {
	Income          money.Money
	Expense         money.Money
	NetCashflow     money.Money
	NetWorth        money.Money
	SafeToSpend     money.Money
	TotalDebt       money.Money
	SavingsRate     string
	EmergencyMonths string
}

type MonthlyInput struct {
	Period      string
	DataAsOf    time.Time
	GeneratedAt time.Time
	Quality     string
	Metrics     MonthlyMetrics
	Warnings    []string
	Narrative   string
}

type MonthlyReport struct {
	Kind        string
	Period      string
	DataAsOf    time.Time
	GeneratedAt time.Time
	Quality     string
	Metrics     MonthlyMetrics
	Warnings    []string
	Narrative   string
}

func GenerateMonthly(input MonthlyInput) (MonthlyReport, error) {
	parsed, err := time.Parse("2006-01", input.Period)
	if err != nil || parsed.Format("2006-01") != input.Period {
		return MonthlyReport{}, ErrInvalidPeriod
	}
	if mixedCurrencies(input.Metrics) {
		return MonthlyReport{}, ErrMixedCurrency
	}
	return MonthlyReport{
		Kind:        KindMonthly,
		Period:      input.Period,
		DataAsOf:    input.DataAsOf,
		GeneratedAt: input.GeneratedAt,
		Quality:     input.Quality,
		Metrics:     input.Metrics,
		Warnings:    append([]string(nil), input.Warnings...),
		Narrative:   input.Narrative,
	}, nil
}

func mixedCurrencies(metrics MonthlyMetrics) bool {
	values := []money.Money{
		metrics.Income,
		metrics.Expense,
		metrics.NetCashflow,
		metrics.NetWorth,
		metrics.SafeToSpend,
		metrics.TotalDebt,
	}
	currency := ""
	for _, value := range values {
		if value.Currency == "" {
			continue
		}
		if currency == "" {
			currency = value.Currency
			continue
		}
		if value.Currency != currency {
			return true
		}
	}
	return false
}
