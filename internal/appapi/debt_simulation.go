package appapi

import (
	"context"
	"errors"
	"fmt"

	"github.com/shawnwu2022/family-finance-os/internal/debt"
	"github.com/shawnwu2022/family-finance-os/internal/server"
	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

var ErrDebtSimulationUnavailable = errors.New("debt simulation is unavailable")

type debtContractProvider interface {
	DebtContract(context.Context, int64, int64) (debt.DebtContract, error)
}

func (a *API) SimulateExtraDebtPayment(ctx context.Context, householdID, debtID, amountMinor int64) (server.DebtExtraPaymentSimulationResponse, error) {
	if a == nil || householdID <= 0 || debtID <= 0 || amountMinor <= 0 {
		return server.DebtExtraPaymentSimulationResponse{}, fmt.Errorf("invalid extra debt payment simulation request")
	}
	provider, ok := a.planner.(debtContractProvider)
	if !ok {
		return server.DebtExtraPaymentSimulationResponse{}, ErrDebtSimulationUnavailable
	}
	contract, err := provider.DebtContract(ctx, householdID, debtID)
	if err != nil {
		return server.DebtExtraPaymentSimulationResponse{}, err
	}
	currency := contract.Balance.Currency
	requested := money.Money{Minor: amountMinor, Currency: currency}

	baseline, err := debt.SimulateDebt(contract, money.Money{Currency: currency})
	if err != nil {
		return server.DebtExtraPaymentSimulationResponse{}, fmt.Errorf("simulate baseline debt payoff: %w", err)
	}
	scenario, err := debt.SimulateOneTimeExtraPayment(contract, requested)
	if err != nil {
		return server.DebtExtraPaymentSimulationResponse{}, fmt.Errorf("simulate one-time extra debt payment: %w", err)
	}

	interestSaved, err := baseline.TotalInterest.Sub(scenario.TotalInterest)
	if err != nil {
		return server.DebtExtraPaymentSimulationResponse{}, fmt.Errorf("calculate debt interest savings: %w", err)
	}
	feeDelta, err := scenario.TotalFees.Sub(baseline.TotalFees)
	if err != nil {
		return server.DebtExtraPaymentSimulationResponse{}, fmt.Errorf("calculate debt prepayment fee delta: %w", err)
	}
	netSavings, err := interestSaved.Sub(feeDelta)
	if err != nil {
		return server.DebtExtraPaymentSimulationResponse{}, fmt.Errorf("calculate debt net savings: %w", err)
	}

	appliedExtra := money.Money{Currency: currency}
	appliedMonth := 0
	for _, payment := range scenario.Payments {
		if payment.ExtraPrincipal.Minor > 0 {
			appliedExtra = payment.ExtraPrincipal
			appliedMonth = payment.Month
			break
		}
	}

	return server.DebtExtraPaymentSimulationResponse{
		DataAsOf:              a.now().UTC(),
		Quality:               "good",
		DebtID:                contract.ID,
		RepaymentAssumption:   "keep_scheduled_payment",
		RequestedExtra:        moneyDTO(requested),
		AppliedExtra:          moneyDTO(appliedExtra),
		AppliedMonth:          appliedMonth,
		BaselinePayoffMonths:  baseline.PayoffMonths,
		SimulatedPayoffMonths: scenario.PayoffMonths,
		MonthsSaved:           baseline.PayoffMonths - scenario.PayoffMonths,
		BaselineInterest:      moneyDTO(baseline.TotalInterest),
		SimulatedInterest:     moneyDTO(scenario.TotalInterest),
		InterestSaved:         moneyDTO(interestSaved),
		PrepaymentFees:        moneyDTO(scenario.TotalFees),
		NetSavings:            moneyDTO(netSavings),
	}, nil
}
