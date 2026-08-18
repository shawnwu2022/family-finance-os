package appapi

import (
	"context"
	"fmt"

	"github.com/shawnwu2022/family-finance-os/internal/server"
)

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
