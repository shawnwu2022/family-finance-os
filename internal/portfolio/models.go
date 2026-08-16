package portfolio

import (
	"errors"
	"time"

	"github.com/cockroachdb/apd/v3"
	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

var (
	ErrNegativeValuation = errors.New("portfolio valuation must not be negative")
	ErrInvalidAssetClass = errors.New("invalid portfolio asset class")
)

type AssetClass string

const (
	AssetClassCash        AssetClass = "cash"
	AssetClassDeposit     AssetClass = "deposit"
	AssetClassFixedIncome AssetClass = "fixed_income"
	AssetClassEquity      AssetClass = "equity"
	AssetClassFund        AssetClass = "fund"
	AssetClassGold        AssetClass = "gold"
	AssetClassProperty    AssetClass = "property"
	AssetClassOther       AssetClass = "other"
)

func (c AssetClass) valid() bool {
	switch c {
	case AssetClassCash, AssetClassDeposit, AssetClassFixedIncome, AssetClassEquity,
		AssetClassFund, AssetClassGold, AssetClassProperty, AssetClassOther:
		return true
	default:
		return false
	}
}

type Valuation struct {
	ID             string
	Name           string
	Class          AssetClass
	Value          money.Money
	SourceCurrency string
	ValuationAsOf  time.Time
	FXAsOf         *time.Time
}

type SummaryInput struct {
	ReportingCurrency string
	AsOf              time.Time
	FXStaleAfter      time.Duration
	Valuations        []Valuation
}

type Allocation struct {
	Value money.Money
	Share *apd.Decimal
}

type WarningCode string

const (
	WarningFXMissing WarningCode = "fx_missing"
	WarningFXStale   WarningCode = "fx_stale"
)

type Warning struct {
	Code        WarningCode
	ValuationID string
}

type Summary struct {
	Total    money.Money
	ByClass  map[AssetClass]Allocation
	Warnings []Warning
}
