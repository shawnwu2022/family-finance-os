package budget

import (
	"errors"
	"fmt"

	"github.com/cockroachdb/apd/v3"
	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

var ErrNegativeEmergencyInput = errors.New("emergency fund inputs must be non-negative")

type DecimalResult struct {
	Value      *apd.Decimal
	Applicable bool
}

var emergencyContext = apd.BaseContext.WithPrecision(34)

func CalculateEmergencyMonths(liquid, essentialMonthly money.Money) (DecimalResult, error) {
	if liquid.Currency != essentialMonthly.Currency {
		return DecimalResult{}, money.ErrCurrencyMismatch
	}
	if liquid.Minor < 0 || essentialMonthly.Minor < 0 {
		return DecimalResult{}, ErrNegativeEmergencyInput
	}
	if essentialMonthly.Minor == 0 {
		return DecimalResult{Applicable: false}, nil
	}

	months := new(apd.Decimal)
	_, err := emergencyContext.Quo(
		months,
		apd.New(liquid.Minor, 0),
		apd.New(essentialMonthly.Minor, 0),
	)
	if err != nil {
		return DecimalResult{}, fmt.Errorf("calculate emergency months: %w", err)
	}
	return DecimalResult{Value: months, Applicable: true}, nil
}
