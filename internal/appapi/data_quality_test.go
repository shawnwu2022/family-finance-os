package appapi

import (
	"context"
	"testing"
	"time"

	"github.com/shawnwu2022/family-finance-os/internal/budget"
	"github.com/shawnwu2022/family-finance-os/internal/goals"
	"github.com/shawnwu2022/family-finance-os/internal/household"
	"github.com/shawnwu2022/family-finance-os/internal/ledger"
	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

type dataQualityPlanner struct{}

func (dataQualityPlanner) Profile(_ context.Context, householdID int64) (household.Profile, error) {
	return household.Profile{Household: household.Household{ID: householdID, BaseCurrency: "CNY", Timezone: "Asia/Shanghai"}}, nil
}
func (dataQualityPlanner) BudgetPlan(context.Context, int64, string) (budget.BudgetPlan, error) {
	return budget.BudgetPlan{}, nil
}
func (dataQualityPlanner) Debts(context.Context, int64) ([]DebtSnapshot, error) { return nil, nil }
func (dataQualityPlanner) Goals(context.Context, int64) ([]goals.FinancialGoal, error) { return nil, nil }

type dataQualityLedger struct {
	query ledger.TransactionQuery
}

func (l *dataQualityLedger) ListAccounts(context.Context) ([]ledger.Account, error) {
	return []ledger.Account{{ID: "cash"}}, nil
}
func (l *dataQualityLedger) ListCategories(context.Context) ([]ledger.Category, error) {
	return []ledger.Category{{ID: "food", Type: ledger.CategoryTypeExpense}}, nil
}
func (l *dataQualityLedger) ListTransactions(_ context.Context, query ledger.TransactionQuery) ([]ledger.Transaction, error) {
	l.query = query
	return []ledger.Transaction{
		{ID: "a", Type: ledger.TransactionTypeExpense, CategoryID: "food", OccurredAt: time.Date(2026, 7, 31, 15, 59, 0, 0, time.UTC), SourceAccountID: "cash", SourceAmount: money.Money{Minor: 100, Currency: "CNY"}},
		{ID: "b", Type: ledger.TransactionTypeExpense, CategoryID: "food", OccurredAt: time.Date(2026, 8, 1, 4, 0, 0, 0, time.UTC), SourceAccountID: "cash", SourceAmount: money.Money{Minor: 1280, Currency: "CNY"}, Comment: "lunch"},
		{ID: "c", Type: ledger.TransactionTypeExpense, CategoryID: "food", OccurredAt: time.Date(2026, 8, 1, 4, 2, 0, 0, time.UTC), SourceAccountID: "cash", SourceAmount: money.Money{Minor: 1280, Currency: "CNY"}, Comment: "lunch"},
	}, nil
}

func TestDataQualityUsesHouseholdTimezonePeriodAndMapsFindings(t *testing.T) {
	ledgerBackend := &dataQualityLedger{}
	api, err := New(Dependencies{Ledger: ledgerBackend, Planner: dataQualityPlanner{}})
	if err != nil {
		t.Fatal(err)
	}

	got, err := api.DataQuality(context.Background(), 7, "2026-08")
	if err != nil {
		t.Fatal(err)
	}
	wantStart := time.Date(2026, 7, 31, 16, 0, 0, 0, time.UTC)
	wantEnd := time.Date(2026, 8, 31, 16, 0, 0, 0, time.UTC)
	if !ledgerBackend.query.Start.Equal(wantStart) || !ledgerBackend.query.End.Equal(wantEnd) {
		t.Fatalf("query bounds start=%s end=%s", ledgerBackend.query.Start, ledgerBackend.query.End)
	}
	if got.Period != "2026-08" || got.Quality != "review" || got.CheckedTransactions != 2 || got.DuplicateGroupCount != 1 {
		t.Fatalf("unexpected report: %#v", got)
	}
	if len(got.DuplicateCandidates) != 1 || len(got.DuplicateCandidates[0].TransactionIDs) != 2 {
		t.Fatalf("unexpected duplicates: %#v", got.DuplicateCandidates)
	}
}
