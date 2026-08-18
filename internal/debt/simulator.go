package debt

import (
	"fmt"

	"github.com/cockroachdb/apd/v3"
	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

const maxSimulationMonths = 1200

var debtContext = func() *apd.Context {
	ctx := apd.BaseContext.WithPrecision(34)
	ctx.Rounding = apd.RoundHalfUp
	return ctx
}()

func SimulateDebt(debt DebtContract, extraMonthly money.Money) (DebtSimulation, error) {
	return simulateDebt(debt, extraMonthly, 0)
}

func SimulateOneTimeExtraPayment(debt DebtContract, extraPayment money.Money) (DebtSimulation, error) {
	return simulateDebt(debt, extraPayment, debt.PrepaymentRestrictedMonths+1)
}

func simulateDebt(debt DebtContract, extra money.Money, oneTimeMonth int) (DebtSimulation, error) {
	currency, err := validateDebtForSimulation(debt, extra)
	if err != nil {
		return DebtSimulation{}, err
	}

	result := DebtSimulation{
		Payments:      []DebtPayment{},
		TotalInterest: money.Money{Currency: currency},
		TotalFees:     money.Money{Currency: currency},
	}
	if debt.Balance.Minor == 0 {
		return result, nil
	}

	monthlyRate, err := monthlyRate(debt.APR)
	if err != nil {
		return DebtSimulation{}, err
	}

	var fixedScheduledMinor int64
	var equalPrincipalMinor int64
	switch debt.RepaymentType {
	case DebtRepaymentAnnuity:
		fixedScheduledMinor, err = annuityPaymentMinor(debt.Balance.Minor, debt.TermRemainingMonths, monthlyRate)
	case DebtRepaymentEqualPrincipal:
		equalPrincipalMinor, err = divideAndRoundMinor(debt.Balance.Minor, int64(debt.TermRemainingMonths))
	case DebtRepaymentRevolving, DebtRepaymentCustom:
		// The configured payment is evaluated each month.
	default:
		return DebtSimulation{}, fmt.Errorf("%w: repayment type %q", ErrInvalidDebt, debt.RepaymentType)
	}
	if err != nil {
		return DebtSimulation{}, err
	}

	balance := debt.Balance
	limit := debt.TermRemainingMonths
	if debt.RepaymentType == DebtRepaymentRevolving || debt.RepaymentType == DebtRepaymentCustom {
		limit = maxSimulationMonths
	}

	for month := 1; month <= limit && balance.Minor > 0; month++ {
		monthExtra := extra
		if oneTimeMonth > 0 && month != oneTimeMonth {
			monthExtra = money.Money{Currency: currency}
		}
		payment, err := simulateMonth(debt, month, balance, monthlyRate, fixedScheduledMinor, equalPrincipalMinor, monthExtra)
		if err != nil {
			return DebtSimulation{}, err
		}
		result.Payments = append(result.Payments, payment)
		result.TotalInterest, err = result.TotalInterest.Add(payment.Interest)
		if err != nil {
			return DebtSimulation{}, fmt.Errorf("sum debt interest: %w", err)
		}
		result.TotalFees, err = result.TotalFees.Add(payment.PrepaymentFee)
		if err != nil {
			return DebtSimulation{}, fmt.Errorf("sum debt fees: %w", err)
		}
		balance = payment.ClosingBalance
	}

	if balance.Minor != 0 {
		return DebtSimulation{}, ErrNonAmortizingDebt
	}
	result.PayoffMonths = len(result.Payments)
	return result, nil
}

func simulateMonth(debt DebtContract, month int, opening money.Money, monthlyRate *apd.Decimal, annuityMinor, equalPrincipalMinor int64, extraMonthly money.Money) (DebtPayment, error) {
	interestMinor, err := multiplyAndRoundMinor(opening.Minor, monthlyRate)
	if err != nil {
		return DebtPayment{}, err
	}
	scheduledPrincipalMinor, scheduledPaymentMinor, err := scheduledAmounts(debt, month, opening.Minor, interestMinor, annuityMinor, equalPrincipalMinor)
	if err != nil {
		return DebtPayment{}, err
	}

	remainingAfterScheduled := opening.Minor - scheduledPrincipalMinor
	extraMinor := int64(0)
	if month > debt.PrepaymentRestrictedMonths && extraMonthly.Minor > 0 && remainingAfterScheduled > 0 {
		extraMinor = minInt64(extraMonthly.Minor, remainingAfterScheduled)
	}
	feeMinor := int64(0)
	if extraMinor > 0 && debt.PrepaymentFeeRate != nil && !debt.PrepaymentFeeRate.IsZero() {
		feeMinor, err = multiplyAndRoundMinor(extraMinor, debt.PrepaymentFeeRate)
		if err != nil {
			return DebtPayment{}, err
		}
	}

	currency := opening.Currency
	return DebtPayment{
		Month:              month,
		OpeningBalance:     opening,
		Interest:           money.Money{Minor: interestMinor, Currency: currency},
		ScheduledPayment:   money.Money{Minor: scheduledPaymentMinor, Currency: currency},
		ScheduledPrincipal: money.Money{Minor: scheduledPrincipalMinor, Currency: currency},
		ExtraPrincipal:     money.Money{Minor: extraMinor, Currency: currency},
		PrepaymentFee:      money.Money{Minor: feeMinor, Currency: currency},
		ClosingBalance:     money.Money{Minor: remainingAfterScheduled - extraMinor, Currency: currency},
	}, nil
}
