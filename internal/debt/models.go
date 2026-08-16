package debt

import (
	"errors"

	"github.com/cockroachdb/apd/v3"
	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

var (
	ErrAPRRequired       = errors.New("APR is required")
	ErrInvalidDebt       = errors.New("invalid debt contract")
	ErrNonAmortizingDebt = errors.New("debt payment does not amortize balance")
	ErrInvalidStrategy   = errors.New("invalid payoff strategy")
)

type DebtRateType string

const (
	DebtRateFixed         DebtRateType = "fixed"
	DebtRateLPRSpread     DebtRateType = "lpr_spread"
	DebtRateOtherVariable DebtRateType = "other_variable"
)

type DebtRepaymentType string

const (
	DebtRepaymentAnnuity        DebtRepaymentType = "annuity"
	DebtRepaymentEqualPrincipal DebtRepaymentType = "equal_principal"
	DebtRepaymentRevolving      DebtRepaymentType = "revolving"
	DebtRepaymentCustom         DebtRepaymentType = "custom"
)

type DebtContract struct {
	ID                         int64
	Name                       string
	OriginalPrincipal          money.Money
	Balance                    money.Money
	APR                        *apd.Decimal
	RateType                   DebtRateType
	TermRemainingMonths        int
	DueDay                     int
	RepaymentType              DebtRepaymentType
	MinimumPayment             money.Money
	ScheduledPayment           money.Money
	PrepaymentFeeRate          *apd.Decimal
	PrepaymentRestrictedMonths int
	Revolving                  bool
	Active                     bool
}

type DebtPayment struct {
	Month              int
	OpeningBalance     money.Money
	Interest           money.Money
	ScheduledPayment   money.Money
	ScheduledPrincipal money.Money
	ExtraPrincipal     money.Money
	PrepaymentFee      money.Money
	ClosingBalance     money.Money
}

type DebtSimulation struct {
	Payments      []DebtPayment
	TotalInterest money.Money
	TotalFees     money.Money
	PayoffMonths  int
}

type PayoffStrategy string

const (
	PayoffStrategyAvalanche PayoffStrategy = "avalanche"
	PayoffStrategySnowball  PayoffStrategy = "snowball"
)

type DebtExtraAllocation struct {
	DebtID int64
	Amount money.Money
}

type PayoffPlan struct {
	Strategy           PayoffStrategy
	AvailableExtra     money.Money
	LiquidityShortfall money.Money
	Allocations        []DebtExtraAllocation
}
