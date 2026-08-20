package agentadapter

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"

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

func TestCallPreservesOverviewMetadataAndBusinessPayload(t *testing.T) {
	asOf := time.Date(2026, 8, 18, 1, 2, 3, 0, time.UTC)
	source := server.OverviewResponse{
		DataAsOf: asOf,
		Quality:  "partial",
		NetWorth: server.MoneyDTO{Minor: 12345, Currency: "CNY"},
		Warnings: []string{"source_partial"},
	}
	backend := &fakeBackend{overview: source}
	service, err := New(backend)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	result, err := service.Call(context.Background(), Principal{Kind: "test", HouseholdID: 42}, ToolGetHouseholdOverview, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if result.AsOf == nil || !result.AsOf.Equal(asOf) || result.Quality != "partial" {
		t.Fatalf("metadata=%#v", result)
	}
	if !reflect.DeepEqual(result.Warnings, []string{"source_partial"}) {
		t.Fatalf("warnings=%#v", result.Warnings)
	}
	var got server.OverviewResponse
	if err := json.Unmarshal(result.Data, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !reflect.DeepEqual(got, source) {
		t.Fatalf("data=%#v want %#v", got, source)
	}
	result.Warnings[0] = "mutated"
	if backend.overview.Warnings[0] != "source_partial" {
		t.Fatalf("backend warnings mutated: %#v", backend.overview.Warnings)
	}
	if result.AuditID != "" {
		t.Fatalf("audit id must be empty before audit wiring")
	}
}

func TestCallPreservesCashflowMetadataAndBusinessPayload(t *testing.T) {
	asOf := time.Date(2026, 8, 18, 4, 5, 6, 0, time.UTC)
	source := server.CashflowResponse{
		DataAsOf: asOf,
		Quality:  "stale",
		Period:   "2026-08",
		Income:   server.MoneyDTO{Minor: 20000, Currency: "CNY"},
		Warnings: []string{"ledger_stale"},
	}
	backend := &fakeBackend{cashflow: source}
	service, _ := New(backend)
	result, err := service.Call(context.Background(), Principal{Kind: "test", HouseholdID: 42}, ToolGetCashflow, json.RawMessage(`{"period":"2026-08"}`))
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if result.AsOf == nil || !result.AsOf.Equal(asOf) || result.Quality != "stale" || !reflect.DeepEqual(result.Warnings, source.Warnings) {
		t.Fatalf("metadata=%#v", result)
	}
	var got server.CashflowResponse
	if err := json.Unmarshal(result.Data, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !reflect.DeepEqual(got, source) {
		t.Fatalf("data=%#v want %#v", got, source)
	}
}

func TestCallPreservesScenarioWarningsWithoutFabricatingFreshness(t *testing.T) {
	source := server.ScenarioResponse{
		Kind:     "purchase",
		Result:   json.RawMessage(`{"decision":"defer"}`),
		Warnings: []string{"liquidity_floor"},
	}
	backend := &fakeBackend{scenario: source}
	service, _ := New(backend)
	result, err := service.Call(context.Background(), Principal{Kind: "test", HouseholdID: 42}, ToolSimulatePurchase, json.RawMessage(`{"amount_minor":"10000","currency":"CNY"}`))
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if result.AsOf != nil || result.Quality != "" || !reflect.DeepEqual(result.Warnings, source.Warnings) {
		t.Fatalf("metadata=%#v", result)
	}
	var got server.ScenarioResponse
	if err := json.Unmarshal(result.Data, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !reflect.DeepEqual(got, source) {
		t.Fatalf("data=%#v want %#v", got, source)
	}
}

func TestCallPreservesMonthlyReportMetadataAndBusinessPayload(t *testing.T) {
	asOf := time.Date(2026, 7, 31, 23, 59, 59, 0, time.UTC)
	source := report.MonthlyReport{
		Kind:     report.KindMonthly,
		Period:   "2026-07",
		DataAsOf: asOf,
		Quality:  "partial",
		Warnings: []string{"month_partial"},
	}
	backend := &fakeBackend{monthly: source}
	service, _ := New(backend)
	result, err := service.Call(context.Background(), Principal{Kind: "test", HouseholdID: 42}, ToolGenerateMonthlyReport, json.RawMessage(`{"year":2026,"month":7}`))
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if result.AsOf == nil || !result.AsOf.Equal(asOf) || result.Quality != "partial" || !reflect.DeepEqual(result.Warnings, source.Warnings) {
		t.Fatalf("metadata=%#v", result)
	}
	var got report.MonthlyReport
	if err := json.Unmarshal(result.Data, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !reflect.DeepEqual(got, source) {
		t.Fatalf("data=%#v want %#v", got, source)
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

func (f *fakeBackend) Overview(_ context.Context, householdID int64) (server.OverviewResponse, error) {
	f.overviewCalls++
	f.overviewHouseholdID = householdID
	return f.overview, f.err
}

func (f *fakeBackend) Cashflow(_ context.Context, householdID int64, period string) (server.CashflowResponse, error) {
	f.cashflowCalls++
	f.cashflowHouseholdID = householdID
	f.cashflowPeriod = period
	return f.cashflow, f.err
}

func (f *fakeBackend) Budget(_ context.Context, householdID int64, period string) (server.BudgetResponse, error) {
	f.budgetCalls++
	f.budgetHouseholdID = householdID
	f.budgetPeriod = period
	return f.budget, f.err
}

func (f *fakeBackend) Debts(_ context.Context, householdID int64) (server.DebtsResponse, error) {
	f.debtsCalls++
	f.debtsHouseholdID = householdID
	return f.debts, f.err
}

func (f *fakeBackend) Goals(_ context.Context, householdID int64) (server.GoalsResponse, error) {
	f.goalsCalls++
	f.goalsHouseholdID = householdID
	return f.goals, f.err
}

func (f *fakeBackend) Scenario(_ context.Context, request server.ScenarioRequest) (server.ScenarioResponse, error) {
	f.scenarioCalls++
	f.scenarioRequest = request
	f.scenarioRequest.Input = cloneRaw(request.Input)
	return f.scenario, f.err
}

func (f *fakeBackend) MonthlyReport(_ context.Context, householdID int64, period string) (report.MonthlyReport, error) {
	f.reportCalls++
	f.reportHouseholdID = householdID
	f.reportPeriod = period
	return f.monthly, f.err
}

func (f *fakeBackend) totalCalls() int {
	return f.overviewCalls + f.cashflowCalls + f.budgetCalls + f.debtsCalls + f.goalsCalls + f.scenarioCalls + f.reportCalls
}
