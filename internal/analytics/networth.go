package analytics

import (
	"errors"
	"fmt"

	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

var (
	ErrNegativeValuation    = errors.New("valuation must be non-negative")
	ErrUnknownValuationKind = errors.New("unknown valuation kind")
)

type ValuationKind uint8

const (
	ValuationUnknown ValuationKind = iota
	ValuationAsset
	ValuationLiability
)

type Valuation struct {
	Kind  ValuationKind
	Value money.Money
}

type NetWorthResult struct {
	TotalAssets      money.Money
	TotalLiabilities money.Money
	NetWorth         money.Money
}

func CalculateNetWorth(valuations []Valuation, currency string) (NetWorthResult, error) {
	assets := money.Money{Currency: currency}
	liabilities := money.Money{Currency: currency}

	for _, valuation := range valuations {
		if valuation.Value.Minor < 0 {
			return NetWorthResult{}, ErrNegativeValuation
		}

		var err error
		switch valuation.Kind {
		case ValuationAsset:
			assets, err = assets.Add(valuation.Value)
		case ValuationLiability:
			liabilities, err = liabilities.Add(valuation.Value)
		default:
			return NetWorthResult{}, ErrUnknownValuationKind
		}
		if err != nil {
			return NetWorthResult{}, fmt.Errorf("calculate net worth totals: %w", err)
		}
	}

	net, err := assets.Sub(liabilities)
	if err != nil {
		return NetWorthResult{}, fmt.Errorf("calculate net worth: %w", err)
	}
	return NetWorthResult{
		TotalAssets:      assets,
		TotalLiabilities: liabilities,
		NetWorth:         net,
	}, nil
}
