package scenario

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/cockroachdb/apd/v3"
	"github.com/shawnwu2022/family-finance-os/internal/analytics"
	"github.com/shawnwu2022/family-finance-os/internal/budget"
	"github.com/shawnwu2022/family-finance-os/internal/debt"
	"github.com/shawnwu2022/family-finance-os/internal/goals"
	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

func TestSimulatePurchaseComparesDeterministicImpactWithoutMutation(t *testing.T) {
	currency := "CNY"
	input := PurchaseInput{
		Purchase: money.Money{Minor: 2_000, Currency: currency},
		Cashflow: analytics.CashflowResult{
			RecognizedIncome:  money.Money{Minor: 10_000, Currency: currency},
			RecognizedExpense: money.Money{Minor: 6_000, Currency: currency},
			NetCashflow:       money.Money{Minor: 4_000, Currency: currency},
		},
		SafeToSpend: budget.SafeToSpendInput{
			LiquidDiscretionaryPool:        money.Money{Minor: 10_000, Currency: currency},
			UpcomingMandatoryExpenses:      money.Money{Minor: 2_000, Currency: currency},
			DebtCommitments:                money.Money{Minor: 1_000, Currency: currency},
			EssentialReserveUntilPeriodEnd: money.Money{Minor: 2_000, Currency: currency},
			EmergencyFundGapReserved:       money.Money{Minor: 1_000, Currency: currency},
			HardGoalContributions:          money.Money{Minor: 1_000, Currency: currency},
		},
		LiquidBalance:  money.Money{Minor: 12_000, Currency: currency},
		LiquidityFloor: money.Money{Minor: 11_000, Currency: currency},
		DebtTimeline:   &TimelineInput{BeforeMonths: 10, AfterMonths: 12},
		GoalTimeline:   &TimelineInput{BeforeMonths: 24, AfterMonths: 27},
	}
	original := input

	got, err := SimulatePurchase(input)
	if err != nil {
		t.Fatalf("SimulatePurchase: %v", err)
	}
	if !reflect.DeepEqual(input, original) {
		t.Fatal("SimulatePurchase mutated its input")
	}
	if got.Before.SafeToSpend.Amount.Minor != 3_000 || got.After.SafeToSpend.Amount.Minor != 1_000 {
		t.Fatalf("safe-to-spend before/after = %d/%d", got.Before.SafeToSpend.Amount.Minor, got.After.SafeToSpend.Amount.Minor)
	}
	if got.Before.NetCashflow.Minor != 4_000 || got.After.NetCashflow.Minor != 2_000 {
		t.Fatalf("net cashflow before/after = %d/%d", got.Before.NetCashflow.Minor, got.After.NetCashflow.Minor)
	}
	assertDecimalEqual(t, got.Before.SavingsRate, apd.New(4, -1))
	assertDecimalEqual(t, got.After.SavingsRate, apd.New(2, -1))
	if got.After.LiquidBalance.Minor != 10_000 {
		t.Fatalf("after liquid balance = %d", got.After.LiquidBalance.Minor)
	}
	if got.DebtTimeline == nil || got.DebtTimeline.DeltaMonths != 2 {
		t.Fatalf("debt timeline = %#v", got.DebtTimeline)
	}
	if got.GoalTimeline == nil || got.GoalTimeline.DeltaMonths != 3 {
		t.Fatalf("goal timeline = %#v", got.GoalTimeline)
	}
	if !containsViolation(got.Violations, ViolationLiquidityFloorBreach) {
		t.Fatalf("violations = %#v", got.Violations)
	}
}

func TestSimulateExtraDebtPaymentUsesDebtEngine(t *testing.T) {
	currency := "CNY"
	zeroAPR := apd.New(0, 0)
	contract := debt.DebtContract{
		ID:                1,
		Name:              "测试债务",
		OriginalPrincipal: money.Money{Minor: 30_000, Currency: currency},
		Balance:           money.Money{Minor: 30_000, Currency: currency},
		APR:               zeroAPR,
		RateType:          debt.DebtRateFixed,
		DueDay:            1,
		RepaymentType:     debt.DebtRepaymentCustom,
		MinimumPayment:    money.Money{Currency: currency},
		ScheduledPayment:  money.Money{Minor: 10_000, Currency: currency},
		Active:            true,
	}
	input := ExtraDebtPaymentInput{
		Debt:               contract,
		BeforeExtraMonthly: money.Money{Currency: currency},
		AfterExtraMonthly:  money.Money{Minor: 5_000, Currency: currency},
	}
	original := input

	got, err := SimulateExtraDebtPayment(input)
	if err != nil {
		t.Fatalf("SimulateExtraDebtPayment: %v", err)
	}
	if !reflect.DeepEqual(input, original) {
		t.Fatal("SimulateExtraDebtPayment mutated its input")
	}
	if got.Before.PayoffMonths != 3 || got.After.PayoffMonths != 2 || got.PayoffMonthsDelta != -1 {
		t.Fatalf("payoff before/after/delta = %d/%d/%d", got.Before.PayoffMonths, got.After.PayoffMonths, got.PayoffMonthsDelta)
	}
}

func TestSimulateBudgetChangeComparesPlanOnly(t *testing.T) {
	currency := "CNY"
	input := BudgetChangeInput{
		Line: budget.BudgetLine{
			ID:      1,
			Planned: money.Money{Minor: 10_000, Currency: currency},
			Kind:    budget.BudgetKindFlexible,
		},
		Actual:          money.Money{Minor: 8_000, Currency: currency},
		ProposedPlanned: money.Money{Minor: 12_000, Currency: currency},
	}
	original := input

	got, err := SimulateBudgetChange(input)
	if err != nil {
		t.Fatalf("SimulateBudgetChange: %v", err)
	}
	if !reflect.DeepEqual(input, original) {
		t.Fatal("SimulateBudgetChange mutated its input")
	}
	if got.Before.Remaining.Minor != 2_000 || got.After.Remaining.Minor != 4_000 || got.RemainingDelta.Minor != 2_000 {
		t.Fatalf("remaining before/after/delta = %d/%d/%d", got.Before.Remaining.Minor, got.After.Remaining.Minor, got.RemainingDelta.Minor)
	}
}

func TestSimulateSavingsChangePreservesCashflowFacts(t *testing.T) {
	currency := "CNY"
	input := SavingsChangeInput{
		Cashflow: analytics.CashflowResult{
			RecognizedIncome:  money.Money{Minor: 10_000, Currency: currency},
			RecognizedExpense: money.Money{Minor: 6_000, Currency: currency},
			NetCashflow:       money.Money{Minor: 4_000, Currency: currency},
		},
		BeforeCommitment: money.Money{Minor: 1_000, Currency: currency},
		AfterCommitment:  money.Money{Minor: 2_000, Currency: currency},
	}
	got, err := SimulateSavingsChange(input)
	if err != nil {
		t.Fatalf("SimulateSavingsChange: %v", err)
	}
	if got.BeforeAvailable.Minor != 3_000 || got.AfterAvailable.Minor != 2_000 || got.AvailableDelta.Minor != -1_000 {
		t.Fatalf("available before/after/delta = %d/%d/%d", got.BeforeAvailable.Minor, got.AfterAvailable.Minor, got.AvailableDelta.Minor)
	}
}

func TestSimulateIncomeDropRecomputesCashflowAndSavingsRate(t *testing.T) {
	currency := "CNY"
	input := IncomeDropInput{
		Cashflow: analytics.CashflowResult{
			RecognizedIncome:  money.Money{Minor: 10_000, Currency: currency},
			RecognizedExpense: money.Money{Minor: 6_000, Currency: currency},
			NetCashflow:       money.Money{Minor: 4_000, Currency: currency},
		},
		Drop: money.Money{Minor: 3_000, Currency: currency},
	}
	got, err := SimulateIncomeDrop(input)
	if err != nil {
		t.Fatalf("SimulateIncomeDrop: %v", err)
	}
	if got.After.RecognizedIncome.Minor != 7_000 || got.After.NetCashflow.Minor != 1_000 {
		t.Fatalf("after cashflow = %#v", got.After)
	}
	assertDecimalEqual(t, got.BeforeSavingsRate, apd.New(4, -1))
	if got.AfterSavingsRate == nil || got.AfterSavingsRate.Cmp(got.BeforeSavingsRate) >= 0 {
		t.Fatalf("after savings rate = %v, before = %v", got.AfterSavingsRate, got.BeforeSavingsRate)
	}
}

func TestSimulateGoalComparesGoalEngineResults(t *testing.T) {
	currency := "CNY"
	asOf := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	target := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	baseGoal := goals.FinancialGoal{
		Name:                "年度储蓄",
		Target:              money.Money{Minor: 120_000, Currency: currency},
		Funded:              money.Money{Currency: currency},
		TargetDate:          target,
		Priority:            1,
		Flexibility:         goals.GoalFlexibilityHard,
		MonthlyContribution: money.Money{Minor: 10_000, Currency: currency},
		Active:              true,
	}
	afterGoal := baseGoal
	afterGoal.Funded = money.Money{Minor: 60_000, Currency: currency}
	input := GoalScenarioInput{
		Before: goals.GoalProjectionInput{Goal: baseGoal, AsOf: asOf, AvailableMonthly: money.Money{Minor: 20_000, Currency: currency}},
		After:  goals.GoalProjectionInput{Goal: afterGoal, AsOf: asOf, AvailableMonthly: money.Money{Minor: 20_000, Currency: currency}},
	}
	got, err := SimulateGoal(input)
	if err != nil {
		t.Fatalf("SimulateGoal: %v", err)
	}
	if got.Before.RequiredMonthly.Minor != 10_000 || got.After.RequiredMonthly.Minor != 5_000 || got.RequiredMonthlyDelta.Minor != -5_000 {
		t.Fatalf("required monthly before/after/delta = %d/%d/%d", got.Before.RequiredMonthly.Minor, got.After.RequiredMonthly.Minor, got.RequiredMonthlyDelta.Minor)
	}
}

func TestScenarioRejectsUnsafeInputs(t *testing.T) {
	currency := "CNY"
	_, err := SimulatePurchase(PurchaseInput{Purchase: money.Money{Minor: -1, Currency: currency}})
	if !errors.Is(err, ErrInvalidScenario) {
		t.Fatalf("negative purchase error = %v", err)
	}

	_, err = SimulateIncomeDrop(IncomeDropInput{
		Cashflow: analytics.CashflowResult{
			RecognizedIncome:  money.Money{Minor: 1_000, Currency: currency},
			RecognizedExpense: money.Money{Currency: currency},
			NetCashflow:       money.Money{Minor: 1_000, Currency: currency},
		},
		Drop: money.Money{Minor: 2_000, Currency: currency},
	})
	if !errors.Is(err, ErrInvalidScenario) {
		t.Fatalf("income drop error = %v", err)
	}
}

func assertDecimalEqual(t *testing.T, got, want *apd.Decimal) {
	t.Helper()
	if got == nil || got.Cmp(want) != 0 {
		t.Fatalf("decimal = %v, want %v", got, want)
	}
}

func containsViolation(violations []ViolationCode, code ViolationCode) bool {
	for _, violation := range violations {
		if violation == code {
			return true
		}
	}
	return false
}
