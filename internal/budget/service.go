package budget

import (
	"errors"
	"fmt"

	"github.com/cockroachdb/apd/v3"
	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

var ErrNegativePlanned = errors.New("planned budget amount must be non-negative")

var utilizationContext = apd.BaseContext.WithPrecision(34)

type BudgetLineMetrics struct {
	Planned     money.Money
	Actual      money.Money
	Remaining   money.Money
	Utilization *apd.Decimal
}

func CalculateBudgetLine(line BudgetLine, actual money.Money) (BudgetLineMetrics, error) {
	if line.Planned.Minor < 0 {
		return BudgetLineMetrics{}, ErrNegativePlanned
	}

	remaining, err := line.Planned.Sub(actual)
	if err != nil {
		return BudgetLineMetrics{}, fmt.Errorf("calculate budget remaining: %w", err)
	}

	result := BudgetLineMetrics{
		Planned:   line.Planned,
		Actual:    actual,
		Remaining: remaining,
	}
	if line.Planned.Minor == 0 {
		return result, nil
	}

	utilization := new(apd.Decimal)
	_, err = utilizationContext.Quo(
		utilization,
		apd.New(actual.Minor, 0),
		apd.New(line.Planned.Minor, 0),
	)
	if err != nil {
		return BudgetLineMetrics{}, fmt.Errorf("calculate budget utilization: %w", err)
	}
	result.Utilization = utilization
	return result, nil
}
