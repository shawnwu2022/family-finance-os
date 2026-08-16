package portfolio

import (
	"fmt"

	"github.com/cockroachdb/apd/v3"
	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

var allocationContext = apd.Context{
	Precision:   34,
	MaxExponent: apd.MaxExponent,
	MinExponent: apd.MinExponent,
	Traps:       apd.DefaultTraps,
	Rounding:    apd.RoundHalfEven,
}

func Summarize(input SummaryInput) (Summary, error) {
	total := money.Money{Currency: input.ReportingCurrency}
	byClass := make(map[AssetClass]Allocation)
	warnings := make([]Warning, 0)

	for _, valuation := range input.Valuations {
		if !valuation.Class.valid() {
			return Summary{}, fmt.Errorf("%w: %q", ErrInvalidAssetClass, valuation.Class)
		}
		if valuation.Value.Minor < 0 {
			return Summary{}, fmt.Errorf("%w: %s", ErrNegativeValuation, valuation.ID)
		}
		if valuation.Value.Currency != input.ReportingCurrency {
			return Summary{}, fmt.Errorf("valuation %s: %w", valuation.ID, money.ErrCurrencyMismatch)
		}

		var err error
		total, err = total.Add(valuation.Value)
		if err != nil {
			return Summary{}, fmt.Errorf("add portfolio total: %w", err)
		}

		allocation := byClass[valuation.Class]
		if allocation.Value.Currency == "" {
			allocation.Value.Currency = input.ReportingCurrency
		}
		allocation.Value, err = allocation.Value.Add(valuation.Value)
		if err != nil {
			return Summary{}, fmt.Errorf("add %s allocation: %w", valuation.Class, err)
		}
		byClass[valuation.Class] = allocation

		if valuation.SourceCurrency != input.ReportingCurrency {
			switch {
			case valuation.FXAsOf == nil:
				warnings = append(warnings, Warning{Code: WarningFXMissing, ValuationID: valuation.ID})
			case input.FXStaleAfter > 0 && input.AsOf.Sub(*valuation.FXAsOf) > input.FXStaleAfter:
				warnings = append(warnings, Warning{Code: WarningFXStale, ValuationID: valuation.ID})
			}
		}
	}

	if total.Minor > 0 {
		denominator := apd.New(total.Minor, 0)
		for class, allocation := range byClass {
			share := new(apd.Decimal)
			if _, err := allocationContext.Quo(share, apd.New(allocation.Value.Minor, 0), denominator); err != nil {
				return Summary{}, fmt.Errorf("calculate %s allocation share: %w", class, err)
			}
			allocation.Share = share
			byClass[class] = allocation
		}
	}

	return Summary{Total: total, ByClass: byClass, Warnings: warnings}, nil
}
