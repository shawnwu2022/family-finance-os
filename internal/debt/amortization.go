package debt

import (
	"fmt"

	"github.com/cockroachdb/apd/v3"
)

func annuityPaymentMinor(principal int64, months int, monthlyRate *apd.Decimal) (int64, error) {
	if monthlyRate.IsZero() {
		return divideAndRoundMinor(principal, int64(months))
	}

	one := apd.New(1, 0)
	onePlusRate := new(apd.Decimal)
	if _, err := debtContext.Add(onePlusRate, one, monthlyRate); err != nil {
		return 0, fmt.Errorf("annuity rate factor: %w", err)
	}
	factor := apd.New(1, 0)
	for range months {
		if _, err := debtContext.Mul(factor, factor, onePlusRate); err != nil {
			return 0, fmt.Errorf("annuity compound factor: %w", err)
		}
	}

	numerator := new(apd.Decimal)
	if _, err := debtContext.Mul(numerator, apd.New(principal, 0), monthlyRate); err != nil {
		return 0, fmt.Errorf("annuity numerator: %w", err)
	}
	if _, err := debtContext.Mul(numerator, numerator, factor); err != nil {
		return 0, fmt.Errorf("annuity numerator factor: %w", err)
	}
	denominator := new(apd.Decimal)
	if _, err := debtContext.Sub(denominator, factor, one); err != nil {
		return 0, fmt.Errorf("annuity denominator: %w", err)
	}
	payment := new(apd.Decimal)
	if _, err := debtContext.Quo(payment, numerator, denominator); err != nil {
		return 0, fmt.Errorf("annuity payment: %w", err)
	}
	return roundDecimalToMinor(payment)
}

func scheduledAmounts(debt DebtContract, month int, openingMinor, interestMinor, annuityMinor, equalPrincipalMinor int64) (principalMinor, paymentMinor int64, err error) {
	switch debt.RepaymentType {
	case DebtRepaymentAnnuity:
		if month == debt.TermRemainingMonths {
			paymentMinor, err = addMinorChecked(openingMinor, interestMinor)
			return openingMinor, paymentMinor, err
		}
		principalMinor = annuityMinor - interestMinor
		if principalMinor < 0 {
			return 0, 0, ErrNonAmortizingDebt
		}
		principalMinor = minInt64(principalMinor, openingMinor)
		paymentMinor, err = addMinorChecked(principalMinor, interestMinor)
		return principalMinor, paymentMinor, err

	case DebtRepaymentEqualPrincipal:
		if month == debt.TermRemainingMonths {
			principalMinor = openingMinor
		} else {
			principalMinor = minInt64(equalPrincipalMinor, openingMinor)
		}
		paymentMinor, err = addMinorChecked(principalMinor, interestMinor)
		return principalMinor, paymentMinor, err

	case DebtRepaymentRevolving:
		totalDue, addErr := addMinorChecked(openingMinor, interestMinor)
		if addErr != nil {
			return 0, 0, addErr
		}
		paymentMinor = minInt64(debt.MinimumPayment.Minor, totalDue)
		principalMinor = paymentMinor - interestMinor
		if principalMinor <= 0 {
			return 0, 0, ErrNonAmortizingDebt
		}
		principalMinor = minInt64(principalMinor, openingMinor)
		paymentMinor, err = addMinorChecked(principalMinor, interestMinor)
		return principalMinor, paymentMinor, err

	case DebtRepaymentCustom:
		totalDue, addErr := addMinorChecked(openingMinor, interestMinor)
		if addErr != nil {
			return 0, 0, addErr
		}
		paymentMinor = minInt64(debt.ScheduledPayment.Minor, totalDue)
		principalMinor = paymentMinor - interestMinor
		if principalMinor <= 0 {
			return 0, 0, ErrNonAmortizingDebt
		}
		principalMinor = minInt64(principalMinor, openingMinor)
		paymentMinor, err = addMinorChecked(principalMinor, interestMinor)
		return principalMinor, paymentMinor, err
	default:
		return 0, 0, ErrInvalidDebt
	}
}
