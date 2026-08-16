package debt

import (
	"fmt"

	"github.com/cockroachdb/apd/v3"
	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

func validateDebtForSimulation(debt DebtContract, extra money.Money) (string, error) {
	currency := debt.Balance.Currency
	if currency == "" || debt.OriginalPrincipal.Currency != currency || debt.MinimumPayment.Currency != currency || debt.ScheduledPayment.Currency != currency || extra.Currency != currency {
		return "", money.ErrCurrencyMismatch
	}
	if debt.Balance.Minor < 0 || debt.OriginalPrincipal.Minor < 0 || debt.MinimumPayment.Minor < 0 || debt.ScheduledPayment.Minor < 0 || extra.Minor < 0 {
		return "", ErrInvalidDebt
	}
	if debt.APR == nil {
		return "", ErrAPRRequired
	}
	if debt.APR.Cmp(apd.New(0, 0)) < 0 {
		return "", ErrInvalidDebt
	}
	if debt.PrepaymentFeeRate != nil && debt.PrepaymentFeeRate.Cmp(apd.New(0, 0)) < 0 {
		return "", ErrInvalidDebt
	}
	if debt.PrepaymentRestrictedMonths < 0 || debt.DueDay < 1 || debt.DueDay > 31 {
		return "", ErrInvalidDebt
	}
	switch debt.RateType {
	case DebtRateFixed, DebtRateLPRSpread, DebtRateOtherVariable:
	default:
		return "", ErrInvalidDebt
	}
	switch debt.RepaymentType {
	case DebtRepaymentAnnuity, DebtRepaymentEqualPrincipal:
		if debt.TermRemainingMonths <= 0 {
			return "", ErrInvalidDebt
		}
	case DebtRepaymentRevolving:
		if debt.MinimumPayment.Minor <= 0 {
			return "", ErrInvalidDebt
		}
	case DebtRepaymentCustom:
		if debt.ScheduledPayment.Minor <= 0 {
			return "", ErrInvalidDebt
		}
	default:
		return "", ErrInvalidDebt
	}
	return currency, nil
}

func monthlyRate(apr *apd.Decimal) (*apd.Decimal, error) {
	monthly := new(apd.Decimal)
	if _, err := debtContext.Quo(monthly, apr, apd.New(12, 0)); err != nil {
		return nil, fmt.Errorf("calculate monthly APR: %w", err)
	}
	return monthly, nil
}
