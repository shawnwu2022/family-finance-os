package budget

import (
	"fmt"

	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

type SafeToSpendInput struct {
	LiquidDiscretionaryPool        money.Money
	UpcomingMandatoryExpenses      money.Money
	DebtCommitments                money.Money
	DebtCommitmentsAlreadyReserved bool
	EssentialReserveUntilPeriodEnd money.Money
	EmergencyFundGapReserved       money.Money
	HardGoalContributions          money.Money
}

type SafeToSpendComponents struct {
	LiquidDiscretionaryPool        money.Money
	UpcomingMandatoryExpenses      money.Money
	DebtCommitments                money.Money
	EssentialReserveUntilPeriodEnd money.Money
	EmergencyFundGapReserved       money.Money
	HardGoalContributions          money.Money
}

type SafeToSpendResult struct {
	Amount     money.Money
	IsDeficit  bool
	Components SafeToSpendComponents
}

func CalculateSafeToSpend(in SafeToSpendInput) (SafeToSpendResult, error) {
	currency := in.LiquidDiscretionaryPool.Currency
	for _, value := range []money.Money{
		in.UpcomingMandatoryExpenses,
		in.DebtCommitments,
		in.EssentialReserveUntilPeriodEnd,
		in.EmergencyFundGapReserved,
		in.HardGoalContributions,
	} {
		if value.Currency != currency {
			return SafeToSpendResult{}, money.ErrCurrencyMismatch
		}
	}

	appliedDebt := in.DebtCommitments
	if in.DebtCommitmentsAlreadyReserved {
		appliedDebt = money.Money{Currency: currency}
	}

	components := SafeToSpendComponents{
		LiquidDiscretionaryPool:        in.LiquidDiscretionaryPool,
		UpcomingMandatoryExpenses:      in.UpcomingMandatoryExpenses,
		DebtCommitments:                appliedDebt,
		EssentialReserveUntilPeriodEnd: in.EssentialReserveUntilPeriodEnd,
		EmergencyFundGapReserved:       in.EmergencyFundGapReserved,
		HardGoalContributions:          in.HardGoalContributions,
	}

	amount := in.LiquidDiscretionaryPool
	for _, deduction := range []money.Money{
		components.UpcomingMandatoryExpenses,
		components.DebtCommitments,
		components.EssentialReserveUntilPeriodEnd,
		components.EmergencyFundGapReserved,
		components.HardGoalContributions,
	} {
		var err error
		amount, err = amount.Sub(deduction)
		if err != nil {
			return SafeToSpendResult{}, fmt.Errorf("calculate safe-to-spend: %w", err)
		}
	}

	return SafeToSpendResult{
		Amount:     amount,
		IsDeficit:  amount.Minor < 0,
		Components: components,
	}, nil
}
