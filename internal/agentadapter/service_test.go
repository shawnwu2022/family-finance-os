package agentadapter

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/shawnwu2022/family-finance-os/internal/report"
	"github.com/shawnwu2022/family-finance-os/internal/server"
)

func TestCallInjectsPrincipalHouseholdAndNeverAcceptsHouseholdArgument(t *testing.T) {
	backend := &fakeBackend{overview: server.OverviewResponse{Quality: "good"}}
	service, err := New(backend)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = service.Call(context.Background(), Principal{Kind: "test", HouseholdID: 42}, ToolGetHouseholdOverview, json.RawMessage(`{"household_id":99}`))
	if !IsCode(err, CodeInvalidArgument) {
		t.Fatalf("error=%v, want %s", err, CodeInvalidArgument)
	}
	if backend.overviewCalls != 0 {
		t.Fatalf("backend called %d times", backend.overviewCalls)
	}

	_, err = service.Call(context.Background(), Principal{Kind: "test", HouseholdID: 42}, ToolGetHouseholdOverview, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if backend.overviewHouseholdID != 42 {
		t.Fatalf("household=%d want 42", backend.overviewHouseholdID)
	}
}

func TestCallRejectsInvalidPrincipalBeforeBackend(t *testing.T) {
	backend := &fakeBackend{}
	service, err := New(backend)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_, err = service.Call(context.Background(), Principal{}, ToolGetHouseholdOverview, json.RawMessage(`{}`))
	if !IsCode(err, CodeForbidden) {
		t.Fatalf("error=%v, want %s", err, CodeForbidden)
	}
	if backend.totalCalls() != 0 {
		t.Fatalf("backend was called %d times", backend.totalCalls())
	}
}

func TestCallRejectsUnknownToolAndTrailingJSON(t *testing.T) {
	backend := &fakeBackend{}
	service, err := New(backend)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	principal := Principal{Kind: "test", HouseholdID: 42}

	if _, err := service.Call(context.Background(), principal, ToolName("execute_sql"), json.RawMessage(`{}`)); !IsCode(err, CodeToolNotFound) {
		t.Fatalf("unknown tool error=%v", err)
	}
	if _, err := service.Call(context.Background(), principal, ToolGetCashflow, json.RawMessage(`{"period":"2026-08"}{}`)); !IsCode(err, CodeInvalidArgument) {
		t.Fatalf("trailing JSON error=%v", err)
	}
	if backend.totalCalls() != 0 {
		t.Fatalf("backend was called %d times", backend.totalCalls())
	}
}

func TestCallDispatchesImplementedCapabilitiesWithPrincipalHousehold(t *testing.T) {
	backend := &fakeBackend{}
	service, err := New(backend)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx := context.Background()
	principal := Principal{Kind: "test", HouseholdID: 42}

	if _, err := service.Call(ctx, principal, ToolGetHouseholdOverview, json.RawMessage(`{}`)); err != nil {
		t.Fatalf("overview: %v", err)
	}
	if backend.overviewHouseholdID != 42 {
		t.Fatalf("overview household=%d", backend.overviewHouseholdID)
	}

	if _, err := service.Call(ctx, principal, ToolGetCashflow, json.RawMessage(`{"period":"2026-08"}`)); err != nil {
		t.Fatalf("cashflow: %v", err)
	}
	if backend.cashflowHouseholdID != 42 || backend.cashflowPeriod != "2026-08" {
		t.Fatalf("cashflow scope/period=%d/%q", backend.cashflowHouseholdID, backend.cashflowPeriod)
	}

	if _, err := service.Call(ctx, principal, ToolGetBudgetStatus, json.RawMessage(`{"period":"2026-08"}`)); err != nil {
		t.Fatalf("budget: %v", err)
	}
	if backend.budgetHouseholdID != 42 || backend.budgetPeriod != "2026-08" {
		t.Fatalf("budget scope/period=%d/%q", backend.budgetHouseholdID, backend.budgetPeriod)
	}

	if _, err := service.Call(ctx, principal, ToolGetDebtStatus, json.RawMessage(`{}`)); err != nil {
		t.Fatalf("debts: %v", err)
	}
	if backend.debtsHouseholdID != 42 {
		t.Fatalf("debt household=%d", backend.debtsHouseholdID)
	}

	if _, err := service.Call(ctx, principal, ToolGetGoalStatus, json.RawMessage(`{}`)); err != nil {
		t.Fatalf("goals: %v", err)
	}
	if backend.goalsHouseholdID != 42 {
		t.Fatalf("goal household=%d", backend.goalsHouseholdID)
	}

	if _, err := service.Call(ctx, principal, ToolSimulatePurchase, json.RawMessage(`{"amount_minor":"10000","currency":"CNY"}`)); err != nil {
		t.Fatalf("purchase: %v", err)
	}
	if backend.scenarioRequest.HouseholdID != 42 || backend.scenarioRequest.Kind != "purchase" {
		t.Fatalf("scenario scope/kind=%d/%q", backend.scenarioRequest.HouseholdID, backend.scenarioRequest.Kind)
	}
	var purchase PurchaseInput
	if err := json.Unmarshal(backend.scenarioRequest.Input, &purchase); err != nil {
		t.Fatalf("decode scenario input: %v", err)
	}
	if purchase.AmountMinor != "10000" || purchase.Currency != "CNY" {
		t.Fatalf("scenario input=%#v", purchase)
	}

	if _, err := service.Call(ctx, principal, ToolGenerateMonthlyReport, json.RawMessage(`{"year":2026,"month":7}`)); err != nil {
		t.Fatalf("monthly report: %v", err)
	}
	if backend.reportHouseholdID != 42 || backend.reportPeriod != "2026-07" {
		t.Fatalf("report scope/period=%d/%q", backend.reportHouseholdID, backend.reportPeriod)
	}
}

type fakeBackend struct {
	overview server.OverviewResponse
	cashflow server.CashflowResponse
	budget   server.BudgetResponse
	debts    server.DebtsResponse
	goals    server.GoalsResponse
	scenario server.ScenarioResponse
	monthly  report.MonthlyReport
	err      error

	overviewCalls       int
	cashflowCalls       int
	budgetCalls         int
	debtsCalls          int
	goalsCalls          int
	scenarioCalls       int
	reportCalls         int
	overviewHouseholdID int64
	cashflowHouseholdID int64
	cashflowPeriod      string
	budgetHouseholdID   int64
	budgetPeriod        string
	debtsHouseholdID    int64
	goalsHouseholdID    int64
	scenarioRequest     server.ScenarioRequest
	reportHouseholdID   int64
	reportPeriod        string
}

func (f *fakeBackend) Overview(context.Context, int64) (server.OverviewResponse, error) {
	panic("use pointer receiver helper")
}

func (f *fakeBackend) Cashflow(context.Context, int64, string) (server.CashflowResponse, error) {
	panic("use pointer receiver helper")
}

func (f *fakeBackend) Budget(context.Context, int64, string) (server.BudgetResponse, error) {
	panic("use pointer receiver helper")
}

func (f *fakeBackend) Debts(context.Context, int64) (server.DebtsResponse, error) {
	panic("use pointer receiver helper")
}

func (f *fakeBackend) Goals(context.Context, int64) (server.GoalsResponse, error) {
	panic("use pointer receiver helper")
}

func (f *fakeBackend) Scenario(context.Context, server.ScenarioRequest) (server.ScenarioResponse, error) {
	panic("use pointer receiver helper")
}

func (f *fakeBackend) MonthlyReport(context.Context, int64, string) (report.MonthlyReport, error) {
	panic("use pointer receiver helper")
}

func (f *fakeBackend) totalCalls() int {
	return f.overviewCalls + f.cashflowCalls + f.budgetCalls + f.debtsCalls + f.goalsCalls + f.scenarioCalls + f.reportCalls
}
