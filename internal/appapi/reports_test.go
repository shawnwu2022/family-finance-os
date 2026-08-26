package appapi

import (
	"context"
	"testing"
	"time"

	"github.com/shawnwu2022/family-finance-os/internal/budget"
	"github.com/shawnwu2022/family-finance-os/internal/household"
	"github.com/shawnwu2022/family-finance-os/internal/ledger"
	"github.com/shawnwu2022/family-finance-os/internal/report"
	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

func TestMonthlyReportUsesExplicitPeriodDeterministicSnapshot(t *testing.T) {
	now := time.Date(2026, time.August, 17, 3, 30, 0, 0, time.UTC)
	planner := fakePlanner{
		profile: household.Profile{
			Household: household.Household{ID: 42, Name: "测试家庭", BaseCurrency: "CNY", Timezone: "Asia/Shanghai"},
			Policy:    household.HouseholdPolicy{HouseholdID: 42, LiquidityFloor: money.Money{Currency: "CNY"}},
		},
		plan: budget.BudgetPlan{ID: 1, HouseholdID: 42, Period: "2026-07", Currency: "CNY"},
	}
	book := fakeLedger{
		accounts: []ledger.Account{
			{ID: "checking", Category: ledger.AccountCategoryChecking, Structure: ledger.AccountStructureSingle, Balance: money.Money{Minor: 100_000, Currency: "CNY"}, IsAsset: true},
		},
		transactions: []ledger.Transaction{
			{ID: "july-income", Type: ledger.TransactionTypeIncome, OccurredAt: time.Date(2026, 7, 5, 1, 0, 0, 0, time.UTC), SourceAccountID: "checking", SourceAmount: money.Money{Minor: 100_000, Currency: "CNY"}},
			{ID: "july-food", Type: ledger.TransactionTypeExpense, OccurredAt: time.Date(2026, 7, 8, 1, 0, 0, 0, time.UTC), SourceAccountID: "checking", SourceAmount: money.Money{Minor: 20_000, Currency: "CNY"}},
			{ID: "august-food", Type: ledger.TransactionTypeExpense, OccurredAt: time.Date(2026, 8, 8, 1, 0, 0, 0, time.UTC), SourceAccountID: "checking", SourceAmount: money.Money{Minor: 99_000, Currency: "CNY"}},
		},
	}

	artifacts := &fakeMonthlyReportStore{}
	api, err := New(Dependencies{Ledger: book, Planner: planner, Reports: artifacts, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	got, err := api.MonthlyReport(context.Background(), 42, "2026-07")
	if err != nil {
		t.Fatalf("MonthlyReport: %v", err)
	}
	if got.Kind != report.KindMonthly || got.Period != "2026-07" || !got.GeneratedAt.Equal(now) {
		t.Fatalf("report identity = %#v", got)
	}
	if got.Metrics.Income.Minor != 100_000 || got.Metrics.Expense.Minor != 20_000 || got.Metrics.NetCashflow.Minor != 80_000 {
		t.Fatalf("report cashflow metrics = %#v", got.Metrics)
	}
	if got.Metrics.NetWorth.Minor != 199_000 {
		t.Fatalf("report month-end net worth = %d, want 199000", got.Metrics.NetWorth.Minor)
	}
	wantDataAsOf := time.Date(2026, time.July, 31, 16, 0, 0, 0, time.UTC)
	if !got.DataAsOf.Equal(wantDataAsOf) {
		t.Fatalf("report data_as_of = %v, want %v", got.DataAsOf, wantDataAsOf)
	}
	if got.Metrics.Income.Currency != "CNY" || got.Narrative != "" {
		t.Fatalf("report currency/narrative = %#v", got)
	}
	if artifacts.saveCalls != 1 || artifacts.householdID != 42 || artifacts.items[0].ContentHash == "" {
		t.Fatalf("persisted report artifact = %#v", artifacts)
	}
	now = now.Add(24 * time.Hour)
	again, err := api.MonthlyReport(context.Background(), 42, "2026-07")
	if err != nil {
		t.Fatalf("MonthlyReport existing: %v", err)
	}
	if artifacts.saveCalls != 1 || !again.GeneratedAt.Equal(got.GeneratedAt) {
		t.Fatalf("existing report was regenerated: calls=%d report=%#v", artifacts.saveCalls, again)
	}
}

func TestReportingAPIMapsPersistedMonthlyReports(t *testing.T) {
	reader := &fakeMonthlyReportStore{items: []report.StoredMonthlyReport{{
		ID: 11, HouseholdID: 42, ContentHash: "aabbcc", CreatedAt: time.Date(2026, time.July, 31, 19, 0, 1, 0, time.UTC),
		Report: report.MonthlyReport{Kind: report.KindMonthly, Period: "2026-07"},
	}}}
	core, err := New(Dependencies{
		Ledger: fakeLedger{},
		Planner: fakePlanner{profile: household.Profile{
			Household: household.Household{ID: 42, BaseCurrency: "CNY", Timezone: "Asia/Shanghai"},
		}},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	api, err := NewReportingAPI(core, reader)
	if err != nil {
		t.Fatalf("NewReportingAPI: %v", err)
	}

	got, err := api.Reports(context.Background(), 42)
	if err != nil {
		t.Fatalf("Reports: %v", err)
	}
	if reader.householdID != 42 {
		t.Fatalf("reader query household = %d", reader.householdID)
	}
	if len(got.Items) != 1 {
		t.Fatalf("reports = %#v", got)
	}
	item := got.Items[0]
	if item.ID != 11 || item.Period != "2026-07" || item.Kind != report.KindMonthly || item.Status != "ready" || item.ContentHash != "aabbcc" {
		t.Fatalf("report summary = %#v", item)
	}
}

type fakeMonthlyReportStore struct {
	items       []report.StoredMonthlyReport
	householdID int64
	saveCalls   int
}

func (f *fakeMonthlyReportStore) Get(_ context.Context, householdID int64, period string) (report.StoredMonthlyReport, error) {
	f.householdID = householdID
	for _, item := range f.items {
		if item.HouseholdID == householdID && item.Report.Period == period {
			return item, nil
		}
	}
	return report.StoredMonthlyReport{}, report.ErrNotFound
}

func (f *fakeMonthlyReportStore) Save(_ context.Context, householdID int64, monthly report.MonthlyReport) (report.StoredMonthlyReport, error) {
	f.householdID = householdID
	f.saveCalls++
	hash, err := report.ContentHash(monthly)
	if err != nil {
		return report.StoredMonthlyReport{}, err
	}
	stored := report.StoredMonthlyReport{ID: int64(len(f.items) + 1), HouseholdID: householdID, ContentHash: hash, CreatedAt: monthly.GeneratedAt, Report: monthly}
	f.items = append(f.items, stored)
	return stored, nil

}

func (f *fakeMonthlyReportStore) List(_ context.Context, householdID int64) ([]report.StoredMonthlyReport, error) {
	f.householdID = householdID
	return append([]report.StoredMonthlyReport(nil), f.items...), nil
}
