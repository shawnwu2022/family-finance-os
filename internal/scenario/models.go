package scenario

import (
	"errors"

	"github.com/cockroachdb/apd/v3"
	"github.com/shawnwu2022/family-finance-os/internal/analytics"
	"github.com/shawnwu2022/family-finance-os/internal/budget"
	"github.com/shawnwu2022/family-finance-os/internal/debt"
	"github.com/shawnwu2022/family-finance-os/internal/goals"
	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

var ErrInvalidScenario = errors.New("invalid scenario input")

type TimelineInput struct {
	BeforeMonths int
	AfterMonths  int
}

type TimelineImpact struct {
	BeforeMonths int
	AfterMonths  int
	DeltaMonths  int
}

type ViolationCode string

const (
	ViolationSafeToSpendDeficit   ViolationCode = "safe_to_spend_deficit"
	ViolationLiquidityFloorBreach ViolationCode = "liquidity_floor_breach"
)

type PurchaseInput struct {
	Purchase       money.Money
	Cashflow       analytics.CashflowResult
	SafeToSpend    budget.SafeToSpendInput
	LiquidBalance  money.Money
	LiquidityFloor money.Money
	DebtTimeline   *TimelineInput
	GoalTimeline   *TimelineInput
}

type PurchaseMetrics struct {
	SafeToSpend   budget.SafeToSpendResult
	SavingsRate   *apd.Decimal
	LiquidBalance money.Money
	NetCashflow   money.Money
}

type PurchaseResult struct {
	Before           PurchaseMetrics
	After            PurchaseMetrics
	SafeToSpendDelta money.Money
	NetCashflowDelta money.Money
	DebtTimeline     *TimelineImpact
	GoalTimeline     *TimelineImpact
	Violations       []ViolationCode
}

type ExtraDebtPaymentInput struct {
	Debt               debt.DebtContract
	BeforeExtraMonthly money.Money
	AfterExtraMonthly  money.Money
}

type ExtraDebtPaymentResult struct {
	Before            debt.DebtSimulation
	After             debt.DebtSimulation
	PayoffMonthsDelta int
	InterestDelta     money.Money
	FeeDelta          money.Money
}

type BudgetChangeInput struct {
	Line            budget.BudgetLine
	Actual          money.Money
	ProposedPlanned money.Money
}

type BudgetChangeResult struct {
	Before         budget.BudgetLineMetrics
	After          budget.BudgetLineMetrics
	RemainingDelta money.Money
}

type SavingsChangeInput struct {
	Cashflow         analytics.CashflowResult
	BeforeCommitment money.Money
	AfterCommitment  money.Money
}

type SavingsChangeResult struct {
	BeforeAvailable money.Money
	AfterAvailable  money.Money
	AvailableDelta  money.Money
}

type IncomeDropInput struct {
	Cashflow analytics.CashflowResult
	Drop     money.Money
}

type IncomeDropResult struct {
	Before            analytics.CashflowResult
	After             analytics.CashflowResult
	BeforeSavingsRate *apd.Decimal
	AfterSavingsRate  *apd.Decimal
}

type GoalScenarioInput struct {
	Before goals.GoalProjectionInput
	After  goals.GoalProjectionInput
}

type GoalScenarioResult struct {
	Before               goals.GoalProjection
	After                goals.GoalProjection
	RequiredMonthlyDelta money.Money
	GapAtTargetDelta     money.Money
}
