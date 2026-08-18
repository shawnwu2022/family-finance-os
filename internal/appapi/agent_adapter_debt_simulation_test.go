package appapi

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/shawnwu2022/family-finance-os/internal/agentadapter"
	"github.com/shawnwu2022/family-finance-os/internal/debt"
	"github.com/shawnwu2022/family-finance-os/internal/household"
	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

func TestAgentAdapterExtraDebtPaymentParity(t *testing.T) {
	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	planner := debtSimulationPlanner{
		fakePlanner: fakePlanner{profile: household.Profile{Household: household.Household{ID: 42, BaseCurrency: "CNY", Timezone: "Asia/Shanghai"}}},
		contract: debt.DebtContract{
			ID:                         7,
			Name:                       "房贷",
			OriginalPrincipal:          money.Money{Minor: 1_000_000, Currency: "CNY"},
			Balance:                    money.Money{Minor: 800_000, Currency: "CNY"},
			APR:                        mustDebtDecimal(t, "0.048"),
			RateType:                   debt.DebtRateFixed,
			TermRemainingMonths:        24,
			DueDay:                     20,
			RepaymentType:              debt.DebtRepaymentAnnuity,
			MinimumPayment:             money.Money{Minor: 10_000, Currency: "CNY"},
			ScheduledPayment:           money.Money{Currency: "CNY"},
			PrepaymentFeeRate:          mustDebtDecimal(t, "0.01"),
			PrepaymentRestrictedMonths: 2,
			Active:                     true,
		},
	}
	api, err := New(Dependencies{Ledger: failingDebtSimulationLedger{}, Planner: planner, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	adapter, err := agentadapter.New(api)
	if err != nil {
		t.Fatalf("agentadapter.New: %v", err)
	}
	ctx := context.Background()
	direct, err := api.SimulateExtraDebtPayment(ctx, 42, 7, 200_000)
	if err != nil {
		t.Fatalf("SimulateExtraDebtPayment: %v", err)
	}
	assertAgentParity(
		t,
		adapter,
		ctx,
		agentadapter.Principal{Kind: "test", HouseholdID: 42},
		agentadapter.ToolSimulateExtraDebtPayment,
		json.RawMessage(`{"debt_id":7,"amount_minor":"200000"}`),
		direct,
	)
}
