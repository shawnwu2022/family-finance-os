package appapi

import (
	"context"
	"errors"
	"fmt"

	"github.com/shawnwu2022/family-finance-os/internal/goals"
	"github.com/shawnwu2022/family-finance-os/internal/server"
	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

var ErrGoalNotFound = errors.New("goal not found")

func (a *API) SafeToSpend(ctx context.Context, householdID int64) (server.SafeToSpendResponse, error) {
	profile, err := a.planner.Profile(ctx, householdID)
	if err != nil {
		return server.SafeToSpendResponse{}, fmt.Errorf("load household profile: %w", err)
	}
	period, err := periodAt(a.now(), profile.Household.Timezone)
	if err != nil {
		return server.SafeToSpendResponse{}, err
	}
	snapshot, err := a.snapshot(ctx, profile, period)
	if err != nil {
		return server.SafeToSpendResponse{}, err
	}

	components := snapshot.safeToSpend.Components
	return server.SafeToSpendResponse{
		DataAsOf:  snapshot.asOf,
		Quality:   qualityString(snapshot.quality),
		Period:    period,
		Amount:    moneyDTO(snapshot.safeToSpend.Amount),
		IsDeficit: snapshot.safeToSpend.IsDeficit,
		Components: server.SafeToSpendComponentsResponse{
			LiquidDiscretionaryPool:        moneyDTO(components.LiquidDiscretionaryPool),
			UpcomingMandatoryExpenses:      moneyDTO(components.UpcomingMandatoryExpenses),
			DebtCommitments:                moneyDTO(components.DebtCommitments),
			EssentialReserveUntilPeriodEnd: moneyDTO(components.EssentialReserveUntilPeriodEnd),
			EmergencyFundGapReserved:       moneyDTO(components.EmergencyFundGapReserved),
			HardGoalContributions:          moneyDTO(components.HardGoalContributions),
		},
		Warnings: cloneWarnings(snapshot.warnings),
	}, nil
}

func (a *API) SimulateGoal(ctx context.Context, householdID, goalID, monthlyContributionMinor int64) (server.GoalSimulationResponse, error) {
	if goalID <= 0 || monthlyContributionMinor < 0 {
		return server.GoalSimulationResponse{}, fmt.Errorf("invalid goal simulation input")
	}

	profile, err := a.planner.Profile(ctx, householdID)
	if err != nil {
		return server.GoalSimulationResponse{}, fmt.Errorf("load household profile: %w", err)
	}
	period, err := periodAt(a.now(), profile.Household.Timezone)
	if err != nil {
		return server.GoalSimulationResponse{}, err
	}
	snapshot, err := a.snapshot(ctx, profile, period)
	if err != nil {
		return server.GoalSimulationResponse{}, err
	}

	var selected goals.FinancialGoal
	found := false
	for _, goal := range snapshot.goals {
		if goal.ID == goalID && goal.Active {
			selected = goal
			found = true
			break
		}
	}
	if !found {
		return server.GoalSimulationResponse{}, ErrGoalNotFound
	}

	selected.MonthlyContribution = money.Money{Minor: monthlyContributionMinor, Currency: selected.Target.Currency}
	available := snapshot.cashflow.NetCashflow
	if available.Minor < 0 {
		available.Minor = 0
	}
	projection, err := goals.ProjectGoal(goals.GoalProjectionInput{
		Goal:             selected,
		AsOf:             snapshot.asOf,
		AvailableMonthly: available,
	})
	if err != nil {
		return server.GoalSimulationResponse{}, err
	}

	return server.GoalSimulationResponse{
		DataAsOf:            snapshot.asOf,
		Quality:             qualityString(snapshot.quality),
		GoalID:              selected.ID,
		MonthlyContribution: moneyDTO(selected.MonthlyContribution),
		MonthsRemaining:     projection.MonthsRemaining,
		RequiredMonthly:     moneyDTO(projection.RequiredMonthly),
		ProjectedFunded:     moneyDTO(projection.ProjectedFunded),
		GapAtTarget:         moneyDTO(projection.GapAtTarget),
		CapacityShortfall:   moneyDTO(projection.CapacityShortfall),
		Status:              string(projection.Status),
		Warnings:            cloneWarnings(snapshot.warnings),
	}, nil
}
