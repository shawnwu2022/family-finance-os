package scenario

import (
	"fmt"

	"github.com/cockroachdb/apd/v3"
	"github.com/shawnwu2022/family-finance-os/internal/analytics"
	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

var ratioContext = apd.BaseContext.WithPrecision(34)

func validatedCashflow(input analytics.CashflowResult) (analytics.CashflowResult, error) {
	currency := input.RecognizedIncome.Currency
	if currency == "" || input.RecognizedExpense.Currency != currency || input.NetCashflow.Currency != currency {
		return analytics.CashflowResult{}, money.ErrCurrencyMismatch
	}
	net, err := input.RecognizedIncome.Sub(input.RecognizedExpense)
	if err != nil {
		return analytics.CashflowResult{}, err
	}
	if net != input.NetCashflow {
		return analytics.CashflowResult{}, ErrInvalidScenario
	}
	return input, nil
}

func savingsRate(cashflow analytics.CashflowResult) (*apd.Decimal, error) {
	if cashflow.RecognizedIncome.Minor <= 0 {
		return nil, nil
	}
	numerator := cashflow.NetCashflow.Minor
	if numerator < 0 {
		numerator = 0
	}
	rate := new(apd.Decimal)
	if _, err := ratioContext.Quo(rate, apd.New(numerator, 0), apd.New(cashflow.RecognizedIncome.Minor, 0)); err != nil {
		return nil, fmt.Errorf("calculate savings rate: %w", err)
	}
	return rate, nil
}

func requireCurrency(currency string, values ...money.Money) error {
	if currency == "" {
		return money.ErrCurrencyMismatch
	}
	for _, value := range values {
		if value.Currency != currency {
			return money.ErrCurrencyMismatch
		}
	}
	return nil
}

func timelineImpact(input TimelineInput) (TimelineImpact, error) {
	if input.BeforeMonths < 0 || input.AfterMonths < 0 {
		return TimelineImpact{}, ErrInvalidScenario
	}
	return TimelineImpact{
		BeforeMonths: input.BeforeMonths,
		AfterMonths:  input.AfterMonths,
		DeltaMonths:  input.AfterMonths - input.BeforeMonths,
	}, nil
}
