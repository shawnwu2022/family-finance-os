package debt

import (
	"fmt"

	"github.com/cockroachdb/apd/v3"
	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

func multiplyAndRoundMinor(minor int64, rate *apd.Decimal) (int64, error) {
	value := new(apd.Decimal)
	if _, err := debtContext.Mul(value, apd.New(minor, 0), rate); err != nil {
		return 0, fmt.Errorf("multiply debt decimal: %w", err)
	}
	return roundDecimalToMinor(value)
}

func divideAndRoundMinor(minor, divisor int64) (int64, error) {
	value := new(apd.Decimal)
	if _, err := debtContext.Quo(value, apd.New(minor, 0), apd.New(divisor, 0)); err != nil {
		return 0, fmt.Errorf("divide debt decimal: %w", err)
	}
	return roundDecimalToMinor(value)
}

func roundDecimalToMinor(value *apd.Decimal) (int64, error) {
	rounded := new(apd.Decimal)
	if _, err := debtContext.Quantize(rounded, value, 0); err != nil {
		return 0, fmt.Errorf("round debt decimal: %w", err)
	}
	minor, err := rounded.Int64()
	if err != nil {
		return 0, fmt.Errorf("convert debt amount to int64: %w", err)
	}
	return minor, nil
}

func addMinorChecked(a, b int64) (int64, error) {
	result, err := (money.Money{Minor: a, Currency: "_"}).Add(money.Money{Minor: b, Currency: "_"})
	if err != nil {
		return 0, fmt.Errorf("debt amount addition: %w", err)
	}
	return result.Minor, nil
}

func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
