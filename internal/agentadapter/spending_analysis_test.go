package agentadapter

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/shawnwu2022/family-finance-os/internal/server"
)

func TestCallDispatchesSpendingAnalysisWithPrincipalScopeAndMetadata(t *testing.T) {
	asOf := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	source := server.SpendingAnalysisResponse{
		DataAsOf: asOf,
		Quality:  "partial",
		Currency: "CNY",
		Current: server.SpendingPeriodResponse{
			Period:           "2026-08",
			Total:            server.MoneyDTO{Minor: 28_000, Currency: "CNY"},
			TransactionCount: 3,
		},
		Comparisons: []server.SpendingPeriodResponse{{Period: "2026-07", Total: server.MoneyDTO{Minor: 7_500, Currency: "CNY"}, TransactionCount: 2}},
		Warnings:    []string{"source_partial"},
	}
	backend := &spendingAnalysisBackend{fakeBackend: &fakeBackend{}, response: source}
	service, err := New(backend)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result, err := service.Call(context.Background(), Principal{Kind: "test", HouseholdID: 42}, ToolGetSpendingAnalysis, json.RawMessage(`{"period":"2026-08","compare_periods":1}`))
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if backend.calls != 1 || backend.householdID != 42 || backend.period != "2026-08" || backend.comparePeriods != 1 {
		t.Fatalf("dispatch calls/scope/input=%d/%d/%q/%d", backend.calls, backend.householdID, backend.period, backend.comparePeriods)
	}
	if result.AsOf == nil || !result.AsOf.Equal(asOf) || result.Quality != "partial" || !reflect.DeepEqual(result.Warnings, source.Warnings) {
		t.Fatalf("metadata=%#v", result)
	}
	var got server.SpendingAnalysisResponse
	if err := json.Unmarshal(result.Data, &got); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if !reflect.DeepEqual(got, source) {
		t.Fatalf("data=%#v want %#v", got, source)
	}
}

func TestSpendingAnalysisToolRejectsScopeOverrideAndInvalidArgumentsBeforeBackend(t *testing.T) {
	backend := &spendingAnalysisBackend{fakeBackend: &fakeBackend{}}
	service, err := New(backend)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	principal := Principal{Kind: "test", HouseholdID: 42}
	for _, args := range []json.RawMessage{
		json.RawMessage(`{"period":"2026-08","compare_periods":1,"household_id":99}`),
		json.RawMessage(`{"period":"2026-13","compare_periods":1}`),
		json.RawMessage(`{"period":"2026-08","compare_periods":-1}`),
		json.RawMessage(`{"period":"2026-08","compare_periods":13}`),
	} {
		if _, err := service.Call(context.Background(), principal, ToolGetSpendingAnalysis, args); !IsCode(err, CodeInvalidArgument) {
			t.Fatalf("args=%s error=%v want %s", args, err, CodeInvalidArgument)
		}
	}
	if backend.calls != 0 {
		t.Fatalf("backend called %d times for rejected input", backend.calls)
	}
}

type spendingAnalysisBackend struct {
	*fakeBackend
	response       server.SpendingAnalysisResponse
	calls          int
	householdID    int64
	period         string
	comparePeriods int
}

func (b *spendingAnalysisBackend) SpendingAnalysis(_ context.Context, householdID int64, period string, comparePeriods int) (server.SpendingAnalysisResponse, error) {
	b.calls++
	b.householdID = householdID
	b.period = period
	b.comparePeriods = comparePeriods
	return b.response, nil
}
