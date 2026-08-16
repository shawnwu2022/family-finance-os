package goals

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/cockroachdb/apd/v3"
	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

var (
	ErrInvalidGoal                = errors.New("invalid financial goal")
	ErrGoalAssumptionsUnsupported = errors.New("non-zero goal planning assumptions are not supported in V1")
)

func ProjectGoal(input GoalProjectionInput) (GoalProjection, error) {
	goal := input.Goal
	if err := validateGoalFacts(goal); err != nil {
		return GoalProjection{}, err
	}
	if input.AvailableMonthly.Currency != goal.Target.Currency {
		return GoalProjection{}, money.ErrCurrencyMismatch
	}
	if hasNonZeroAssumptions(input.Assumptions) {
		return GoalProjection{}, ErrGoalAssumptionsUnsupported
	}

	currency := goal.Target.Currency
	zero := money.Money{Currency: currency}
	months := calendarMonthsRemaining(input.AsOf, goal.TargetDate)
	gap, err := goal.Target.Sub(goal.Funded)
	if err != nil {
		return GoalProjection{}, fmt.Errorf("calculate goal funded gap: %w", err)
	}
	if gap.Minor <= 0 {
		return GoalProjection{
			MonthsRemaining:   months,
			RequiredMonthly:   zero,
			ProjectedFunded:   goal.Funded,
			GapAtTarget:       zero,
			CapacityShortfall: zero,
			Status:            GoalStatusCompleted,
		}, nil
	}

	if months == 0 {
		return GoalProjection{
			MonthsRemaining:   0,
			RequiredMonthly:   zero,
			ProjectedFunded:   goal.Funded,
			GapAtTarget:       gap,
			CapacityShortfall: zero,
			Status:            GoalStatusInfeasible,
		}, nil
	}

	requiredMinor := gap.Minor / int64(months)
	if gap.Minor%int64(months) != 0 {
		requiredMinor++
	}
	required := money.Money{Minor: requiredMinor, Currency: currency}

	contributionTotal, err := multiplyMoney(goal.MonthlyContribution, months)
	if err != nil {
		return GoalProjection{}, fmt.Errorf("project goal contributions: %w", err)
	}
	projectedFunded, err := goal.Funded.Add(contributionTotal)
	if err != nil {
		return GoalProjection{}, fmt.Errorf("project goal funded amount: %w", err)
	}
	projectedGap, err := goal.Target.Sub(projectedFunded)
	if err != nil {
		return GoalProjection{}, fmt.Errorf("project goal target gap: %w", err)
	}
	if projectedGap.Minor < 0 {
		projectedGap = zero
	}

	projection := GoalProjection{
		MonthsRemaining:   months,
		RequiredMonthly:   required,
		ProjectedFunded:   projectedFunded,
		GapAtTarget:       projectedGap,
		CapacityShortfall: zero,
		Status:            GoalStatusBehind,
	}

	if input.AvailableMonthly.Minor < required.Minor {
		shortfall, err := required.Sub(input.AvailableMonthly)
		if err != nil {
			return GoalProjection{}, fmt.Errorf("calculate goal capacity shortfall: %w", err)
		}
		projection.CapacityShortfall = shortfall
		projection.Status = GoalStatusConflicting
		return projection, nil
	}
	if goal.MonthlyContribution.Minor >= required.Minor {
		projection.Status = GoalStatusOnTrack
	}
	return projection, nil
}

func validateGoalFacts(goal FinancialGoal) error {
	if strings.TrimSpace(goal.Name) == "" || goal.Priority <= 0 || !goal.Flexibility.valid() || goal.TargetDate.IsZero() {
		return ErrInvalidGoal
	}
	currency := goal.Target.Currency
	if currency == "" || goal.Funded.Currency != currency || goal.MonthlyContribution.Currency != currency {
		return money.ErrCurrencyMismatch
	}
	if goal.Target.Minor < 0 || goal.Funded.Minor < 0 || goal.MonthlyContribution.Minor < 0 {
		return ErrInvalidGoal
	}
	return nil
}

func hasNonZeroAssumptions(assumptions *GoalPlanningAssumptions) bool {
	if assumptions == nil {
		return false
	}
	zero := apd.New(0, 0)
	return (assumptions.ExpectedReturnAnnual != nil && assumptions.ExpectedReturnAnnual.Cmp(zero) != 0) ||
		(assumptions.InflationAnnual != nil && assumptions.InflationAnnual.Cmp(zero) != 0)
}

func calendarMonthsRemaining(asOf, target time.Time) int {
	if !target.After(asOf) {
		return 0
	}
	months := (target.Year()-asOf.Year())*12 + int(target.Month()-asOf.Month())
	if target.Day() > asOf.Day() {
		months++
	}
	if months < 1 {
		return 1
	}
	return months
}

func multiplyMoney(value money.Money, count int) (money.Money, error) {
	if count < 0 {
		return money.Money{}, ErrInvalidGoal
	}
	if count == 0 || value.Minor == 0 {
		return money.Money{Currency: value.Currency}, nil
	}
	if value.Minor > 0 && int64(count) > math.MaxInt64/value.Minor {
		return money.Money{}, money.ErrOverflow
	}
	return money.Money{Minor: value.Minor * int64(count), Currency: value.Currency}, nil
}
