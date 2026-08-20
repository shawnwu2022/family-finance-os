package agentadapter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/shawnwu2022/family-finance-os/internal/report"
	"github.com/shawnwu2022/family-finance-os/internal/server"
)

func TestCallRejectsHouseholdOverrideForEveryTool(t *testing.T) {
	cases := []struct {
		name ToolName
		args json.RawMessage
	}{
		{ToolGetHouseholdOverview, json.RawMessage(`{"household_id":99}`)},
		{ToolGetCashflow, json.RawMessage(`{"period":"2026-08","household_id":99}`)},
		{ToolGetBudgetStatus, json.RawMessage(`{"period":"2026-08","household_id":99}`)},
		{ToolGetDebtStatus, json.RawMessage(`{"household_id":99}`)},
		{ToolGetGoalStatus, json.RawMessage(`{"household_id":99}`)},
		{ToolSimulatePurchase, json.RawMessage(`{"amount_minor":"10000","currency":"CNY","household_id":99}`)},
		{ToolGenerateMonthlyReport, json.RawMessage(`{"year":2026,"month":7,"household_id":99}`)},
	}
	for _, tc := range cases {
		t.Run(string(tc.name), func(t *testing.T) {
			backend := &fakeBackend{}
			service, err := New(backend)
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			_, err = service.Call(context.Background(), Principal{Kind: "test", HouseholdID: 42}, tc.name, tc.args)
			if !IsCode(err, CodeInvalidArgument) {
				t.Fatalf("error=%v, want %s", err, CodeInvalidArgument)
			}
			if backend.totalCalls() != 0 {
				t.Fatalf("backend called %d times", backend.totalCalls())
			}
		})
	}
}

func TestCallDoesNotExposeBackendErrorText(t *testing.T) {
	sensitive := errors.New("postgres password=secret-token")
	backend := &fakeBackend{err: sensitive}
	service, err := New(backend)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_, err = service.Call(context.Background(), Principal{Kind: "test", HouseholdID: 42}, ToolGetHouseholdOverview, json.RawMessage(`{}`))
	if !IsCode(err, CodeDataUnavailable) {
		t.Fatalf("error=%v, want %s", err, CodeDataUnavailable)
	}
	if strings.Contains(err.Error(), "secret-token") || strings.Contains(err.Error(), "password") {
		t.Fatalf("external error leaked backend text: %q", err.Error())
	}
	if !errors.Is(err, sensitive) {
		t.Fatalf("underlying error is not retained for server-side inspection")
	}
}

func TestCallPropagatesCancellationToBackend(t *testing.T) {
	backend := &cancellationBackend{}
	service, err := New(backend)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = service.Call(ctx, Principal{Kind: "test", HouseholdID: 42}, ToolGetHouseholdOverview, json.RawMessage(`{}`))
	if !backend.sawCanceled {
		t.Fatalf("backend did not observe cancelled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error=%v, want wrapped context.Canceled", err)
	}
}

func TestCallCanonicalizesPurchaseInputBeforeBackend(t *testing.T) {
	backend := &fakeBackend{}
	service, err := New(backend)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	arguments := json.RawMessage(`{"currency":"CNY","amount_minor":"10000"}`)
	if _, err := service.Call(context.Background(), Principal{Kind: "test", HouseholdID: 42}, ToolSimulatePurchase, arguments); err != nil {
		t.Fatalf("Call: %v", err)
	}
	for i := range arguments {
		arguments[i] = 'x'
	}
	var captured PurchaseInput
	if err := json.Unmarshal(backend.scenarioRequest.Input, &captured); err != nil {
		t.Fatalf("decode captured input: %v", err)
	}
	if captured.AmountMinor != "10000" || captured.Currency != "CNY" {
		t.Fatalf("captured=%#v", captured)
	}
	if string(backend.scenarioRequest.Input) != `{"amount_minor":"10000","currency":"CNY"}` {
		t.Fatalf("backend input is not canonical typed JSON: %s", backend.scenarioRequest.Input)
	}
}

func TestConcurrentCallsKeepPrincipalScopesIsolated(t *testing.T) {
	backend := &concurrentBackend{}
	service, err := New(backend)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	const calls = 32
	errCh := make(chan error, calls)
	var wg sync.WaitGroup
	for i := 0; i < calls; i++ {
		householdID := int64(100 + i)
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, callErr := service.Call(context.Background(), Principal{Kind: "test", HouseholdID: householdID}, ToolGetHouseholdOverview, json.RawMessage(`{}`))
			if callErr != nil {
				errCh <- fmt.Errorf("household %d: %w", householdID, callErr)
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Error(err)
	}
	if got := backend.scopes(); len(got) != calls {
		t.Fatalf("scope count=%d want %d; scopes=%v", len(got), calls, got)
	} else {
		for householdID := int64(100); householdID < 100+calls; householdID++ {
			if got[householdID] != 1 {
				t.Fatalf("household %d seen %d times; scopes=%v", householdID, got[householdID], got)
			}
		}
	}
}

type cancellationBackend struct {
	sawCanceled bool
}

func (b *cancellationBackend) Overview(ctx context.Context, _ int64) (server.OverviewResponse, error) {
	b.sawCanceled = errors.Is(ctx.Err(), context.Canceled)
	return server.OverviewResponse{}, ctx.Err()
}
func (b *cancellationBackend) Cashflow(context.Context, int64, string) (server.CashflowResponse, error) {
	return server.CashflowResponse{}, nil
}
func (b *cancellationBackend) Budget(context.Context, int64, string) (server.BudgetResponse, error) {
	return server.BudgetResponse{}, nil
}
func (b *cancellationBackend) Debts(context.Context, int64) (server.DebtsResponse, error) {
	return server.DebtsResponse{}, nil
}
func (b *cancellationBackend) Goals(context.Context, int64) (server.GoalsResponse, error) {
	return server.GoalsResponse{}, nil
}
func (b *cancellationBackend) Scenario(context.Context, server.ScenarioRequest) (server.ScenarioResponse, error) {
	return server.ScenarioResponse{}, nil
}
func (b *cancellationBackend) MonthlyReport(context.Context, int64, string) (report.MonthlyReport, error) {
	return report.MonthlyReport{}, nil
}

type concurrentBackend struct {
	mu   sync.Mutex
	seen map[int64]int
}

func (b *concurrentBackend) Overview(_ context.Context, householdID int64) (server.OverviewResponse, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.seen == nil {
		b.seen = make(map[int64]int)
	}
	b.seen[householdID]++
	return server.OverviewResponse{}, nil
}
func (b *concurrentBackend) Cashflow(context.Context, int64, string) (server.CashflowResponse, error) {
	return server.CashflowResponse{}, nil
}
func (b *concurrentBackend) Budget(context.Context, int64, string) (server.BudgetResponse, error) {
	return server.BudgetResponse{}, nil
}
func (b *concurrentBackend) Debts(context.Context, int64) (server.DebtsResponse, error) {
	return server.DebtsResponse{}, nil
}
func (b *concurrentBackend) Goals(context.Context, int64) (server.GoalsResponse, error) {
	return server.GoalsResponse{}, nil
}
func (b *concurrentBackend) Scenario(context.Context, server.ScenarioRequest) (server.ScenarioResponse, error) {
	return server.ScenarioResponse{}, nil
}
func (b *concurrentBackend) MonthlyReport(context.Context, int64, string) (report.MonthlyReport, error) {
	return report.MonthlyReport{}, nil
}
func (b *concurrentBackend) scopes() map[int64]int {
	b.mu.Lock()
	defer b.mu.Unlock()
	result := make(map[int64]int, len(b.seen))
	for householdID, count := range b.seen {
		result[householdID] = count
	}
	return result
}
