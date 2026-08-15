package money

import (
	"errors"
	"math"
)

var (
	ErrCurrencyMismatch = errors.New("money currency mismatch")
	ErrOverflow         = errors.New("money arithmetic overflow")
)

type Money struct {
	Minor    int64
	Currency string
}

func (m Money) Add(other Money) (Money, error) {
	if m.Currency != other.Currency {
		return Money{}, ErrCurrencyMismatch
	}
	if (other.Minor > 0 && m.Minor > math.MaxInt64-other.Minor) ||
		(other.Minor < 0 && m.Minor < math.MinInt64-other.Minor) {
		return Money{}, ErrOverflow
	}
	return Money{Minor: m.Minor + other.Minor, Currency: m.Currency}, nil
}

func (m Money) Sub(other Money) (Money, error) {
	if m.Currency != other.Currency {
		return Money{}, ErrCurrencyMismatch
	}
	if (other.Minor > 0 && m.Minor < math.MinInt64+other.Minor) ||
		(other.Minor < 0 && m.Minor > math.MaxInt64+other.Minor) {
		return Money{}, ErrOverflow
	}
	return Money{Minor: m.Minor - other.Minor, Currency: m.Currency}, nil
}
