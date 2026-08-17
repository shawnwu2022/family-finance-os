package appapi

import (
	"context"
	"testing"
	"time"

	"github.com/shawnwu2022/family-finance-os/internal/budget"
	"github.com/shawnwu2022/family-finance-os/internal/household"
	"github.com/shawnwu2022/family-finance-os/internal/ledger"
	"github.com/shawnwu2022/family-finance-os/internal/report"
	"github.com/shawnwu2022/family-finance-os/internal/scheduler"
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

	api, err := New(Dependencies{Ledger: book, Planner: planner, Now: func() time.Time { return now }})
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
	if got.Metrics.Income.Currency != "CNY" || got.Narrative != "" {
		t.Fatalf("report currency/narrative = %#v", got)
	}
}

func TestReportsMapsPersistedMonthlyJobRuns(t *testing.T) {
	now := time.Date(2026, time.August, 17, 3, 30, 0, 0, time.UTC)
	reader := &fakeReportRunReader{records: []scheduler.RunRecord{
		{
			ID: 11,
			Key: scheduler.RunKey{
				HouseholdID:  42,
				JobName:      report.JobNameMonthly,
				ScheduledFor: time.Date(2026, time.August, 1, 3, 0, 0, 0, time.FixedZone("CST", 8*60*60)),
				Period:       "2026-07",
			},
			Status:    scheduler.RunSucceeded,
			StartedAt: time.Date(2026, time.July, 31, 19, 0, 1, 0, time.UTC),
		},
	}}
	api, err := New(Dependencies{
		Ledger: fakeLedger{},
		Planner: fakePlanner{profile: household.Profile{
			Household: household.Household{ID: 42, BaseCurrency: "CNY", Timezone: "Asia/Shanghai"},
		}},
		ReportRuns: reader,
		Now:        func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	got, err := api.Reports(context.Background(), 42)
	if err != nil {
		t.Fatalf("Reports: %v", err)
	}
	if reader.householdID != 42 || reader.jobName != report.JobNameMonthly {
		t.Fatalf("reader query = household %d job %q", reader.householdID, reader.jobName)
	}
	if len(got.Items) != 1 {
		t.Fatalf("reports = %#v", got)
	}
	item := got.Items[0]
	if item.ID != 11 || item.Period != "2026-07" || item.Kind != report.KindMonthly || item.Status != string(scheduler.RunSucceeded) {
		t.Fatalf("report summary = %#v", item)
	}
}

type fakeReportRunReader struct {
	records     []scheduler.RunRecord
	householdID int64
	jobName     string
}

func (f *fakeReportRunReader) List(_ context.Context, householdID int64, jobName string) ([]scheduler.RunRecord, error) {
	f.householdID = householdID
	f.jobName = jobName
	return append([]scheduler.RunRecord(nil), f.records...), nil
}
