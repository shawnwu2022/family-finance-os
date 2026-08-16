package scenario

import (
	"fmt"

	"github.com/shawnwu2022/family-finance-os/internal/analytics"
	"github.com/shawnwu2022/family-finance-os/internal/budget"
	"github.com/shawnwu2022/family-finance-os/internal/debt"
	"github.com/shawnwu2022/family-finance-os/internal/goals"
	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

func SimulatePurchase(input PurchaseInput) (PurchaseResult, error) {
	if input.Purchase.Minor < 0 {
		return PurchaseResult{}, ErrInvalidScenario
	}
	beforeCashflow, err := validatedCashflow(input.Cashflow)
	if err != nil {
		return PurchaseResult{}, err
	}
	currency := beforeCashflow.RecognizedIncome.Currency
	if err := requireCurrency(currency, input.Purchase, input.LiquidBalance, input.LiquidityFloor); err != nil {
		return PurchaseResult{}, err
	}

	beforeSafe, err := budget.CalculateSafeToSpend(input.SafeToSpend)
	if err != nil {
		return PurchaseResult{}, fmt.Errorf("before safe-to-spend: %w", err)
	}
	if beforeSafe.Amount.Currency != currency {
		return PurchaseResult{}, money.ErrCurrencyMismatch
	}

	afterSafeInput := input.SafeToSpend
	afterSafeInput.LiquidDiscretionaryPool, err = afterSafeInput.LiquidDiscretionaryPool.Sub(input.Purchase)
	if err != nil {
		return PurchaseResult{}, fmt.Errorf("purchase safe-to-spend pool: %w", err)
	}
	afterSafe, err := budget.CalculateSafeToSpend(afterSafeInput)
	if err != nil {
		return PurchaseResult{}, fmt.Errorf("after safe-to-spend: %w", err)
	}

	afterExpense, err := beforeCashflow.RecognizedExpense.Add(input.Purchase)
	if err != nil {
		return PurchaseResult{}, fmt.Errorf("purchase expense: %w", err)
	}
	afterNet, err := beforeCashflow.RecognizedIncome.Sub(afterExpense)
	if err != nil {
		return PurchaseResult{}, fmt.Errorf("purchase net cashflow: %w", err)
	}
	afterCashflow := analytics.CashflowResult{
		RecognizedIncome:  beforeCashflow.RecognizedIncome,
		RecognizedExpense: afterExpense,
		NetCashflow:       afterNet,
	}

	afterLiquid, err := input.LiquidBalance.Sub(input.Purchase)
	if err != nil {
		return PurchaseResult{}, fmt.Errorf("purchase liquid balance: %w", err)
	}
	beforeRate, err := savingsRate(beforeCashflow)
	if err != nil {
		return PurchaseResult{}, err
	}
	afterRate, err := savingsRate(afterCashflow)
	if err != nil {
		return PurchaseResult{}, err
	}
	safeDelta, err := afterSafe.Amount.Sub(beforeSafe.Amount)
	if err != nil {
		return PurchaseResult{}, err
	}
	netDelta, err := afterCashflow.NetCashflow.Sub(beforeCashflow.NetCashflow)
	if err != nil {
		return PurchaseResult{}, err
	}

	result := PurchaseResult{
		Before: PurchaseMetrics{
			SafeToSpend:   beforeSafe,
			SavingsRate:   beforeRate,
			LiquidBalance: input.LiquidBalance,
			NetCashflow:   beforeCashflow.NetCashflow,
		},
		After: PurchaseMetrics{
			SafeToSpend:   afterSafe,
			SavingsRate:   afterRate,
			LiquidBalance: afterLiquid,
			NetCashflow:   afterCashflow.NetCashflow,
		},
		SafeToSpendDelta: safeDelta,
		NetCashflowDelta: netDelta,
		Violations:       make([]ViolationCode, 0, 2),
	}
	if afterSafe.IsDeficit {
		result.Violations = append(result.Violations, ViolationSafeToSpendDeficit)
	}
	if afterLiquid.Minor < input.LiquidityFloor.Minor {
		result.Violations = append(result.Violations, ViolationLiquidityFloorBreach)
	}
	if input.DebtTimeline != nil {
		impact, err := timelineImpact(*input.DebtTimeline)
		if err != nil {
			return PurchaseResult{}, err
		}
		result.DebtTimeline = &impact
	}
	if input.GoalTimeline != nil {
		impact, err := timelineImpact(*input.GoalTimeline)
		if err != nil {
			return PurchaseResult{}, err
		}
		result.GoalTimeline = &impact
	}
	return result, nil
}

func SimulateExtraDebtPayment(input ExtraDebtPaymentInput) (ExtraDebtPaymentResult, error) {
	before, err := debt.SimulateDebt(input.Debt, input.BeforeExtraMonthly)
	if err != nil {
		return ExtraDebtPaymentResult{}, fmt.Errorf("before debt simulation: %w", err)
	}
	after, err := debt.SimulateDebt(input.Debt, input.AfterExtraMonthly)
	if err != nil {
		return ExtraDebtPaymentResult{}, fmt.Errorf("after debt simulation: %w", err)
	}
	interestDelta, err := after.TotalInterest.Sub(before.TotalInterest)
	if err != nil {
		return ExtraDebtPaymentResult{}, err
	}
	feeDelta, err := after.TotalFees.Sub(before.TotalFees)
	if err != nil {
		return ExtraDebtPaymentResult{}, err
	}
	return ExtraDebtPaymentResult{
		Before:            before,
		After:             after,
		PayoffMonthsDelta: after.PayoffMonths - before.PayoffMonths,
		InterestDelta:     interestDelta,
		FeeDelta:          feeDelta,
	}, nil
}

func SimulateBudgetChange(input BudgetChangeInput) (BudgetChangeResult, error) {
	before, err := budget.CalculateBudgetLine(input.Line, input.Actual)
	if err != nil {
		return BudgetChangeResult{}, fmt.Errorf("before budget simulation: %w", err)
	}
	afterLine := input.Line
	afterLine.Planned = input.ProposedPlanned
	after, err := budget.CalculateBudgetLine(afterLine, input.Actual)
	if err != nil {
		return BudgetChangeResult{}, fmt.Errorf("after budget simulation: %w", err)
	}
	delta, err := after.Remaining.Sub(before.Remaining)
	if err != nil {
		return BudgetChangeResult{}, err
	}
	return BudgetChangeResult{Before: before, After: after, RemainingDelta: delta}, nil
}

func SimulateSavingsChange(input SavingsChangeInput) (SavingsChangeResult, error) {
	cashflow, err := validatedCashflow(input.Cashflow)
	if err != nil {
		return SavingsChangeResult{}, err
	}
	if input.BeforeCommitment.Minor < 0 || input.AfterCommitment.Minor < 0 {
		return SavingsChangeResult{}, ErrInvalidScenario
	}
	currency := cashflow.NetCashflow.Currency
	if err := requireCurrency(currency, input.BeforeCommitment, input.AfterCommitment); err != nil {
		return SavingsChangeResult{}, err
	}
	before, err := cashflow.NetCashflow.Sub(input.BeforeCommitment)
	if err != nil {
		return SavingsChangeResult{}, err
	}
	after, err := cashflow.NetCashflow.Sub(input.AfterCommitment)
	if err != nil {
		return SavingsChangeResult{}, err
	}
	delta, err := after.Sub(before)
	if err != nil {
		return SavingsChangeResult{}, err
	}
	return SavingsChangeResult{BeforeAvailable: before, AfterAvailable: after, AvailableDelta: delta}, nil
}

func SimulateIncomeDrop(input IncomeDropInput) (IncomeDropResult, error) {
	before, err := validatedCashflow(input.Cashflow)
	if err != nil {
		return IncomeDropResult{}, err
	}
	if input.Drop.Minor < 0 || input.Drop.Minor > before.RecognizedIncome.Minor {
		return IncomeDropResult{}, ErrInvalidScenario
	}
	if err := requireCurrency(before.RecognizedIncome.Currency, input.Drop); err != nil {
		return IncomeDropResult{}, err
	}
	afterIncome, err := before.RecognizedIncome.Sub(input.Drop)
	if err != nil {
		return IncomeDropResult{}, err
	}
	afterNet, err := afterIncome.Sub(before.RecognizedExpense)
	if err != nil {
		return IncomeDropResult{}, err
	}
	after := analytics.CashflowResult{
		RecognizedIncome:  afterIncome,
		RecognizedExpense: before.RecognizedExpense,
		NetCashflow:       afterNet,
	}
	beforeRate, err := savingsRate(before)
	if err != nil {
		return IncomeDropResult{}, err
	}
	afterRate, err := savingsRate(after)
	if err != nil {
		return IncomeDropResult{}, err
	}
	return IncomeDropResult{Before: before, After: after, BeforeSavingsRate: beforeRate, AfterSavingsRate: afterRate}, nil
}

func SimulateGoal(input GoalScenarioInput) (GoalScenarioResult, error) {
	before, err := goals.ProjectGoal(input.Before)
	if err != nil {
		return GoalScenarioResult{}, fmt.Errorf("before goal simulation: %w", err)
	}
	after, err := goals.ProjectGoal(input.After)
	if err != nil {
		return GoalScenarioResult{}, fmt.Errorf("after goal simulation: %w", err)
	}
	requiredDelta, err := after.RequiredMonthly.Sub(before.RequiredMonthly)
	if err != nil {
		return GoalScenarioResult{}, err
	}
	gapDelta, err := after.GapAtTarget.Sub(before.GapAtTarget)
	if err != nil {
		return GoalScenarioResult{}, err
	}
	return GoalScenarioResult{
		Before:               before,
		After:                after,
		RequiredMonthlyDelta: requiredDelta,
		GapAtTargetDelta:     gapDelta,
	}, nil
}
