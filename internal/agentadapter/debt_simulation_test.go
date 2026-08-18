package agentadapter

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/shawnwu2022/family-finance-os/internal/server"
)

func TestCallSimulateExtraDebtPaymentUsesPrincipalScopeAndPreservesResult(t *testing.T) {
	asOf := time.Date(2026, 8, 18, 11, 45, 0, 0, time.UTC)
	source := server.DebtExtraPaymentSimulationResponse{
		DataAsOf:            asOf,
		Quality:             "good",
		DebtID:              7,
		RepaymentAssumption: "keep_scheduled_payment",
		RequestedExtra:      server.MoneyDTO{Minor: 200000, Currency: "CNY"},
		AppliedExtra:        server.MoneyDTO{Minor: 200000, Currency: "CNY"},
		AppliedMonth:        3,
		MonthsSaved:         5,
		InterestSaved:       server.MoneyDTO{Minor: 12345, Currency: "CNY"},
		PrepaymentFees:      server.MoneyDTO{Minor: 2000, Currency: "CNY"},
		NetSavings:          server.MoneyDTO{Minor: 10345, Currency: "CNY"},
		Warnings:            []string{"contract_restriction_applied"},
	}
	backend := &debtSimulationBackend{fakeBackend: &fakeBackend{}, response: source}
	service, err := New(backend)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	result, err := service.Call(context.Background(), Principal{Kind: "test", HouseholdID: 42}, ToolSimulateExtraDebtPayment, json.RawMessage(`{"debt_id":7,"amount_minor":"200000"}`))
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if backend.calls != 1 || backend.householdID != 42 || backend.debtID != 7 || backend.amountMinor != 200000 {
		t.Fatalf("backend call=%d scope/debt/amount=%d/%d/%d", backend.calls, backend.householdID, backend.debtID, backend.amountMinor)
	}
	if result.AsOf == nil || !result.AsOf.Equal(asOf) || result.Quality != "good" || !reflect.DeepEqual(result.Warnings, source.Warnings) {
		t.Fatalf("metadata=%#v", result)
	}
	var got server.DebtExtraPaymentSimulationResponse
	if err := json.Unmarshal(result.Data, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !reflect.DeepEqual(got, source) {
		t.Fatalf("data=%#v want %#v", got, source)
	}
}

func TestCallSimulateExtraDebtPaymentRejectsScopeInjectionAndInvalidAmount(t *testing.T) {
	backend := &debtSimulationBackend{fakeBackend: &fakeBackend{}}
	service, err := New(backend)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	principal := Principal{Kind: "test", HouseholdID: 42}

	tests := []json.RawMessage{
		json.RawMessage(`{"debt_id":7,"amount_minor":"200000","household_id":99}`),
		json.RawMessage(`{"debt_id":0,"amount_minor":"200000"}`),
		json.RawMessage(`{"debt_id":7,"amount_minor":"0"}`),
		json.RawMessage(`{"debt_id":7,"amount_minor":"-1"}`),
		json.RawMessage(`{"debt_id":7,"amount_minor":"999999999999999999999999"}`),
	}
	for _, raw := range tests {
		if _, err := service.Call(context.Background(), principal, ToolSimulateExtraDebtPayment, raw); !IsCode(err, CodeInvalidArgument) {
			t.Fatalf("arguments=%s error=%v want %s", raw, err, CodeInvalidArgument)
		}
	}
	if backend.calls != 0 {
		t.Fatalf("backend called %d times", backend.calls)
	}
}

func TestExtraDebtPaymentSchemaIsScopedAndStrict(t *testing.T) {
	for _, definition := range definitions() {
		if definition.Name != ToolSimulateExtraDebtPayment {
			continue
		}
		if bytes.Contains(definition.InputSchema, []byte("household_id")) || bytes.Contains(definition.InputSchema, []byte("currency")) {
			t.Fatalf("schema exposes server-side fields: %s", definition.InputSchema)
		}
		var schema map[string]any
		if err := json.Unmarshal(definition.InputSchema, &schema); err != nil {
			t.Fatal(err)
		}
		properties, _ := schema["properties"].(map[string]any)
		if len(properties) != 2 || properties["debt_id"] == nil || properties["amount_minor"] == nil {
			t.Fatalf("schema properties=%#v", properties)
		}
		return
	}
	t.Fatal("simulate_extra_debt_payment definition is missing")
}

type debtSimulationBackend struct {
	*fakeBackend
	response    server.DebtExtraPaymentSimulationResponse
	calls       int
	householdID int64
	debtID      int64
	amountMinor int64
}

func (b *debtSimulationBackend) SimulateExtraDebtPayment(_ context.Context, householdID, debtID, amountMinor int64) (server.DebtExtraPaymentSimulationResponse, error) {
	b.calls++
	b.householdID = householdID
	b.debtID = debtID
	b.amountMinor = amountMinor
	return b.response, b.err
}
