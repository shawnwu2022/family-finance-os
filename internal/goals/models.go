package goals

import (
	"time"

	"github.com/cockroachdb/apd/v3"
	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

type GoalFlexibility string

const (
	GoalFlexibilityHard     GoalFlexibility = "hard"
	GoalFlexibilityFlexible GoalFlexibility = "flexible"
)

func (f GoalFlexibility) valid() bool {
	return f == GoalFlexibilityHard || f == GoalFlexibilityFlexible
}

type GoalStatus string

const (
	GoalStatusCompleted   GoalStatus = "completed"
	GoalStatusOnTrack     GoalStatus = "on_track"
	GoalStatusBehind      GoalStatus = "behind"
	GoalStatusConflicting GoalStatus = "conflicting"
	GoalStatusInfeasible  GoalStatus = "infeasible"
)

type FinancialGoal struct {
	ID                  int64
	HouseholdID         int64
	Name                string
	Target              money.Money
	Funded              money.Money
	TargetDate          time.Time
	Priority            int32
	Flexibility         GoalFlexibility
	MonthlyContribution money.Money
	Active              bool
}

type NewFinancialGoal struct {
	HouseholdID         int64
	Name                string
	Target              money.Money
	Funded              money.Money
	TargetDate          time.Time
	Priority            int32
	Flexibility         GoalFlexibility
	MonthlyContribution money.Money
	Active              bool
}

// GoalPlanningAssumptions is deliberately separate from FinancialGoal facts.
// V1 zero-return projections reject non-zero assumptions rather than silently
// using optimistic return or inflation estimates.
type GoalPlanningAssumptions struct {
	ExpectedReturnAnnual *apd.Decimal
	InflationAnnual      *apd.Decimal
}

type GoalProjectionInput struct {
	Goal             FinancialGoal
	AsOf             time.Time
	AvailableMonthly money.Money
	Assumptions      *GoalPlanningAssumptions
}

type GoalProjection struct {
	MonthsRemaining   int
	RequiredMonthly   money.Money
	ProjectedFunded   money.Money
	GapAtTarget       money.Money
	CapacityShortfall money.Money
	Status            GoalStatus
}
