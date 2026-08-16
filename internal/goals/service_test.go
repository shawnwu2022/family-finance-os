package goals

import (
	"errors"
	"testing"
	"time"

	"github.com/cockroachdb/apd/v3"
	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

func TestProjectGoalZeroReturnBaseline(t *testing.T) {
	goal := baselineGoal()
	got, err := ProjectGoal(GoalProjectionInput{
		Goal:             goal,
		AsOf:             time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		AvailableMonthly: money.Money{Minor: 15_000, Currency: "CNY"},
	})
	if err != nil {
		t.Fatalf("ProjectGoal: %v", err)
	}
	if got.MonthsRemaining != 10 {
		t.Fatalf("months=%d want 10", got.MonthsRemaining)
	}
	if got.RequiredMonthly.Minor != 10_000 {
		t.Fatalf("required=%d want 10000", got.RequiredMonthly.Minor)
	}
	if got.ProjectedFunded.Minor != 120_000 || got.GapAtTarget.Minor != 0 {
		t.Fatalf("projection=%#v", got)
	}
	if got.Status != GoalStatusOnTrack {
		t.Fatalf("status=%q want on_track", got.Status)
	}
}

func TestProjectGoalBehindConfiguredContribution(t *testing.T) {
	goal := baselineGoal()
	goal.MonthlyContribution = money.Money{Minor: 6_000, Currency: "CNY"}
	got, err := ProjectGoal(GoalProjectionInput{
		Goal:             goal,
		AsOf:             time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		AvailableMonthly: money.Money{Minor: 15_000, Currency: "CNY"},
	})
	if err != nil {
		t.Fatalf("ProjectGoal: %v", err)
	}
	if got.Status != GoalStatusBehind || got.ProjectedFunded.Minor != 80_000 || got.GapAtTarget.Minor != 40_000 {
		t.Fatalf("projection=%#v", got)
	}
}

func TestProjectGoalConflictingCapacity(t *testing.T) {
	goal := baselineGoal()
	got, err := ProjectGoal(GoalProjectionInput{
		Goal:             goal,
		AsOf:             time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		AvailableMonthly: money.Money{Minor: 5_000, Currency: "CNY"},
	})
	if err != nil {
		t.Fatalf("ProjectGoal: %v", err)
	}
	if got.Status != GoalStatusConflicting || got.CapacityShortfall.Minor != 5_000 {
		t.Fatalf("projection=%#v", got)
	}
}

func TestProjectGoalCompletedAndInfeasibleStates(t *testing.T) {
	completed := baselineGoal()
	completed.Funded = money.Money{Minor: 125_000, Currency: "CNY"}
	got, err := ProjectGoal(GoalProjectionInput{
		Goal:             completed,
		AsOf:             time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		AvailableMonthly: money.Money{Currency: "CNY"},
	})
	if err != nil {
		t.Fatalf("completed ProjectGoal: %v", err)
	}
	if got.Status != GoalStatusCompleted || got.RequiredMonthly.Minor != 0 || got.GapAtTarget.Minor != 0 {
		t.Fatalf("completed=%#v", got)
	}

	past := baselineGoal()
	past.TargetDate = time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	got, err = ProjectGoal(GoalProjectionInput{
		Goal:             past,
		AsOf:             time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		AvailableMonthly: money.Money{Minor: 100_000, Currency: "CNY"},
	})
	if err != nil {
		t.Fatalf("past ProjectGoal: %v", err)
	}
	if got.Status != GoalStatusInfeasible || got.MonthsRemaining != 0 || got.GapAtTarget.Minor != 100_000 {
		t.Fatalf("past=%#v", got)
	}
}

func TestProjectGoalKeepsPlanningAssumptionsSeparateFromFacts(t *testing.T) {
	goal := baselineGoal()
	_, err := ProjectGoal(GoalProjectionInput{
		Goal:             goal,
		AsOf:             time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		AvailableMonthly: money.Money{Minor: 15_000, Currency: "CNY"},
		Assumptions: &GoalPlanningAssumptions{
			ExpectedReturnAnnual: apd.New(5, -2),
			InflationAnnual:      apd.New(2, -2),
		},
	})
	if !errors.Is(err, ErrGoalAssumptionsUnsupported) {
		t.Fatalf("error=%v want ErrGoalAssumptionsUnsupported", err)
	}
}

func TestProjectGoalRejectsCurrencyMismatch(t *testing.T) {
	goal := baselineGoal()
	_, err := ProjectGoal(GoalProjectionInput{
		Goal:             goal,
		AsOf:             time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		AvailableMonthly: money.Money{Minor: 15_000, Currency: "USD"},
	})
	if !errors.Is(err, money.ErrCurrencyMismatch) {
		t.Fatalf("error=%v want currency mismatch", err)
	}
}

func FuzzProjectGoalMoreFundingNeverRaisesRequiredMonthly(f *testing.F) {
	f.Add(uint32(120_000), uint32(20_000), uint32(10_000))
	f.Fuzz(func(t *testing.T, targetRaw, fundedRaw, extraRaw uint32) {
		target := int64(targetRaw%1_000_000) + 1
		funded := int64(fundedRaw) % (target + 1)
		extra := int64(extraRaw) % (target + 1)
		more := funded + extra
		if more > target {
			more = target
		}
		input := GoalProjectionInput{
			Goal: FinancialGoal{
				Name:                "fuzz",
				Target:              money.Money{Minor: target, Currency: "CNY"},
				Funded:              money.Money{Minor: funded, Currency: "CNY"},
				TargetDate:          time.Date(2027, 8, 1, 0, 0, 0, 0, time.UTC),
				Priority:            1,
				Flexibility:         GoalFlexibilityHard,
				MonthlyContribution: money.Money{Minor: 1, Currency: "CNY"},
				Active:              true,
			},
			AsOf:             time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
			AvailableMonthly: money.Money{Minor: target, Currency: "CNY"},
		}
		before, err := ProjectGoal(input)
		if err != nil {
			t.Fatalf("before: %v", err)
		}
		input.Goal.Funded.Minor = more
		after, err := ProjectGoal(input)
		if err != nil {
			t.Fatalf("after: %v", err)
		}
		if after.RequiredMonthly.Minor > before.RequiredMonthly.Minor {
			t.Fatalf("required increased: before=%d after=%d", before.RequiredMonthly.Minor, after.RequiredMonthly.Minor)
		}
	})
}

func baselineGoal() FinancialGoal {
	return FinancialGoal{
		ID:                  1,
		HouseholdID:         1,
		Name:                "教育金",
		Target:              money.Money{Minor: 120_000, Currency: "CNY"},
		Funded:              money.Money{Minor: 20_000, Currency: "CNY"},
		TargetDate:          time.Date(2027, 6, 1, 0, 0, 0, 0, time.UTC),
		Priority:            1,
		Flexibility:         GoalFlexibilityHard,
		MonthlyContribution: money.Money{Minor: 10_000, Currency: "CNY"},
		Active:              true,
	}
}
