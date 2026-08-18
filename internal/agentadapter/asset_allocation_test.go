package agentadapter

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/shawnwu2022/family-finance-os/internal/server"
)

func TestAssetAllocationToolUsesServerScopedHouseholdAndPreservesResult(t *testing.T) {
	asOf := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	backend := &assetAllocationBackend{
		fakeBackend: &fakeBackend{},
		value: server.AssetAllocationResponse{
			DataAsOf: asOf,
			Quality:  "partial",
			Currency: "CNY",
			Total:    server.MoneyDTO{Minor: 100_000, Currency: "CNY"},
			Items: []server.AssetAllocationItemResponse{
				{Class: "cash", Value: server.MoneyDTO{Minor: 40_000, Currency: "CNY"}, Share: "0.4"},
				{Class: "other", Value: server.MoneyDTO{Minor: 60_000, Currency: "CNY"}, Share: "0.6"},
			},
			Warnings: []string{"investment holdings unavailable"},
		},
	}
	service, err := New(backend)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	principal := Principal{Kind: "test", HouseholdID: 42}

	if _, err := service.Call(context.Background(), principal, ToolGetAssetAllocation, json.RawMessage(`{"household_id":99}`)); !IsCode(err, CodeInvalidArgument) {
		t.Fatalf("scope override error=%v, want %s", err, CodeInvalidArgument)
	}
	if backend.calls != 0 {
		t.Fatalf("backend called for rejected scope override: %d", backend.calls)
	}

	result, err := service.Call(context.Background(), principal, ToolGetAssetAllocation, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if backend.calls != 1 || backend.householdID != 42 {
		t.Fatalf("backend calls/scope=%d/%d want 1/42", backend.calls, backend.householdID)
	}
	if result.AsOf == nil || !result.AsOf.Equal(asOf) || result.Quality != "partial" || !reflect.DeepEqual(result.Warnings, backend.value.Warnings) {
		t.Fatalf("metadata=%#v", result)
	}
	var got server.AssetAllocationResponse
	if err := json.Unmarshal(result.Data, &got); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if !reflect.DeepEqual(got, backend.value) {
		t.Fatalf("data=%#v want %#v", got, backend.value)
	}
}

type assetAllocationBackend struct {
	*fakeBackend
	value       server.AssetAllocationResponse
	calls       int
	householdID int64
}

func (b *assetAllocationBackend) AssetAllocation(_ context.Context, householdID int64) (server.AssetAllocationResponse, error) {
	b.calls++
	b.householdID = householdID
	return b.value, nil
}
