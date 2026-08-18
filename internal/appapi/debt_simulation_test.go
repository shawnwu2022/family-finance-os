package appapi

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cockroachdb/apd/v3"
	"github.com/shawnwu2022/family-finance-os/internal/debt"
	"github.com/shawnwu2022/family-finance-os/internal/household"
	"github.com/shawnwu2022/family-finance-os/internal/ledger"
	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

func TestSimulateExtraDebtPaymentUsesOneTimeDebtPrimitiveWithoutLedger(t *testing.T) {
	now := time.Date(2026, 8, 18, 11, 30, 0, 0, time.UTC)
	contract := debt.DebtContract{
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
		ScheduledPayment:           money.Money{Minor: 0, Currency: "CNY"},
		PrepaymentFeeRate:          mustDebtDecimal(t, "0.01"),
		PrepaymentRestrictedMonths: 2,
		Active:                     true,
	}
	planner := debtSimulationPlanner{
		fakePlanner: fakePlanner{profile: household.Profile{Household: household.Household{ID: 42, BaseCurrency: "CNY", Timezone: "Asia/Shanghai"}}},
		contract:    contract,
	}
	api, err := New(Dependencies{Ledger: failingDebtSimulationLedger{}, Planner: planner, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	const requestedMinor int64 = 200_000
	got, err := api.SimulateExtraDebtPayment(context.Background(), 42, 7, requestedMinor)
	if err != nil {
		t.Fatalf("SimulateExtraDebtPayment: %v", err)
	}

	baseline, err := debt.SimulateDebt(contract, money.Money{Currency: "CNY"})
	if err != nil {
		t.Fatalf("baseline SimulateDebt: %v", err)
	}
	scenario, err := debt.SimulateOneTimeExtraPayment(contract, money.Money{Minor: requestedMinor, Currency: "CNY"})
	if err != nil {
		t.Fatalf("scenario SimulateOneTimeExtraPayment: %v", err)
	}
	interestSaved, err := baseline.TotalInterest.Sub(scenario.TotalInterest)
	if err != nil {
		t.Fatal(err)
	}
	feeDelta, err := scenario.TotalFees.Sub(baseline.TotalFees)
	if err != nil {
		t.Fatal(err)
	}
	netSavings, err := interestSaved.Sub(feeDelta)
	if err != nil {
		t.Fatal(err)
	}
	appliedMonth := 0
	appliedExtra := money.Money{Currency: "CNY"}
	for _, payment := range scenario.Payments {
		if payment.ExtraPrincipal.Minor > 0 {
			appliedMonth = payment.Month
			appliedExtra = payment.ExtraPrincipal
			break
		}
	}

	if !got.DataAsOf.Equal(now) || got.Quality != "good" || got.DebtID != 7 {
		t.Fatalf("metadata=%#v", got)
	}
	if got.RepaymentAssumption != "keep_scheduled_payment" {
		t.Fatalf("repayment assumption=%q", got.RepaymentAssumption)
	}
	if got.RequestedExtra.Minor != requestedMinor || got.RequestedExtra.Currency != "CNY" || got.AppliedExtra.Minor != appliedExtra.Minor || got.AppliedMonth != appliedMonth {
		t.Fatalf("requested/applied=%#v", got)
	}
	if got.BaselinePayoffMonths != baseline.PayoffMonths || got.SimulatedPayoffMonths != scenario.PayoffMonths || got.MonthsSaved != baseline.PayoffMonths-scenario.PayoffMonths {
		t.Fatalf("payoff comparison=%#v", got)
	}
	if got.BaselineInterest.Minor != baseline.TotalInterest.Minor || got.SimulatedInterest.Minor != scenario.TotalInterest.Minor || got.InterestSaved.Minor != interestSaved.Minor {
		t.Fatalf("interest comparison=%#v", got)
	}
	if got.PrepaymentFees.Minor != scenario.TotalFees.Minor || got.NetSavings.Minor != netSavings.Minor {
		t.Fatalf("fee/net savings=%#v", got)
	}
}

func TestSimulateExtraDebtPaymentRejectsInvalidInputAndMissingDebt(t *testing.T) {
	planner := debtSimulationPlanner{
		fakePlanner: fakePlanner{profile: household.Profile{Household: household.Household{ID: 42, BaseCurrency: "CNY", Timezone: "Asia/Shanghai"}}},
		contract: debt.DebtContract{
			ID: 7, Name: "loan", Balance: money.Money{Minor: 10_000, Currency: "CNY"}, OriginalPrincipal: money.Money{Minor: 10_000, Currency: "CNY"},
			APR: mustDebtDecimal(t, "0.05"), RateType: debt.DebtRateFixed, TermRemainingMonths: 12, DueDay: 20,
			RepaymentType: debt.DebtRepaymentAnnuity, MinimumPayment: money.Money{Currency: "CNY"}, ScheduledPayment: money.Money{Currency: "CNY"}, Active: true,
		},
	}
	api, err := New(Dependencies{Ledger: failingDebtSimulationLedger{}, Planner: planner})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := api.SimulateExtraDebtPayment(context.Background(), 42, 0, 100); err == nil {
		t.Fatal("zero debt id accepted")
	}
	if _, err := api.SimulateExtraDebtPayment(context.Background(), 42, 7, 0); err == nil {
		t.Fatal("zero extra payment accepted")
	}
	if _, err := api.SimulateExtraDebtPayment(context.Background(), 42, 999, 100); !errors.Is(err, ErrDebtNotFound) {
		t.Fatalf("missing debt error=%v want ErrDebtNotFound", err)
	}
}

type debtSimulationPlanner struct {
	fakePlanner
	contract debt.DebtContract
}

func (p debtSimulationPlanner) DebtContract(_ context.Context, householdID, debtID int64) (debt.DebtContract, error) {
	if householdID != p.profile.Household.ID || debtID != p.contract.ID || !p.contract.Active {
		return debt.DebtContract{}, ErrDebtNotFound
	}
	return p.contract, nil
}

type failingDebtSimulationLedger struct{}

func (failingDebtSimulationLedger) ListAccounts(context.Context) ([]ledger.Account, error) {
	return nil, errors.New("ledger must not be called for debt simulation")
}
func (failingDebtSimulationLedger) ListCategories(context.Context) ([]ledger.Category, error) {
	return nil, errors.New("ledger must not be called for debt simulation")
}
func (failingDebtSimulationLedger) ListTransactions(context.Context, ledger.TransactionQuery) ([]ledger.Transaction, error) {
	return nil, errors.New("ledger must not be called for debt simulation")
}

func mustDebtDecimal(t *testing.T, raw string) *apd.Decimal {
	t.Helper()
	value, _, err := apd.NewFromString(raw)
	if err != nil {
		t.Fatalf("parse decimal %q: %v", raw, err)
	}
	return value
}
