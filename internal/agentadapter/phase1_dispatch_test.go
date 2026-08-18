package agentadapter

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/shawnwu2022/family-finance-os/internal/server"
)

func TestCallDispatchesSafeToSpendAndGoalSimulation(t *testing.T) {
	asOf := time.Date(2026, 8, 18, 6, 0, 0, 0, time.UTC)
	backend := &phaseOneBackend{
		fakeBackend: &fakeBackend{},
		safe: server.SafeToSpendResponse{
			DataAsOf: asOf,
			Quality:  "partial",
			Period:   "2026-08",
			Amount:   server.MoneyDTO{Minor: 55_000, Currency: "CNY"},
			Warnings: []string{"source_partial"},
		},
		goal: server.GoalSimulationResponse{
			DataAsOf:            asOf,
			Quality:             "good",
			GoalID:              7,
			MonthlyContribution: server.MoneyDTO{Minor: 20_000, Currency: "CNY"},
			Status:              "on_track",
			Warnings:            []string{"goal_warning"},
		},
	}
	service, err := New(backend)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	principal := Principal{Kind: "test", HouseholdID: 42}

	safe, err := service.Call(context.Background(), principal, ToolGetSafeToSpend, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("safe-to-spend: %v", err)
	}
	if backend.safeHouseholdID != 42 {
		t.Fatalf("safe household=%d want 42", backend.safeHouseholdID)
	}
	if safe.AsOf == nil || !safe.AsOf.Equal(asOf) || safe.Quality != "partial" || !reflect.DeepEqual(safe.Warnings, []string{"source_partial"}) {
		t.Fatalf("safe metadata=%#v", safe)
	}
	var safeData server.SafeToSpendResponse
	if err := json.Unmarshal(safe.Data, &safeData); err != nil {
		t.Fatalf("decode safe data: %v", err)
	}
	if !reflect.DeepEqual(safeData, backend.safe) {
		t.Fatalf("safe data=%#v want %#v", safeData, backend.safe)
	}

	goal, err := service.Call(context.Background(), principal, ToolSimulateGoal, json.RawMessage(`{"goal_id":7,"monthly_contribution_minor":"20000"}`))
	if err != nil {
		t.Fatalf("simulate goal: %v", err)
	}
	if backend.goalHouseholdID != 42 || backend.goalID != 7 || backend.goalContribution != 20_000 {
		t.Fatalf("goal dispatch=%d/%d/%d", backend.goalHouseholdID, backend.goalID, backend.goalContribution)
	}
	if goal.AsOf == nil || !goal.AsOf.Equal(asOf) || goal.Quality != "good" || !reflect.DeepEqual(goal.Warnings, []string{"goal_warning"}) {
		t.Fatalf("goal metadata=%#v", goal)
	}
	var goalData server.GoalSimulationResponse
	if err := json.Unmarshal(goal.Data, &goalData); err != nil {
		t.Fatalf("decode goal data: %v", err)
	}
	if !reflect.DeepEqual(goalData, backend.goal) {
		t.Fatalf("goal data=%#v want %#v", goalData, backend.goal)
	}
}

func TestPhaseOneToolsRejectScopeOverrideAndInvalidGoalContribution(t *testing.T) {
	backend := &phaseOneBackend{fakeBackend: &fakeBackend{}}
	service, err := New(backend)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	principal := Principal{Kind: "test", HouseholdID: 42}

	cases := []struct {
		name ToolName
		args json.RawMessage
	}{
		{ToolGetSafeToSpend, json.RawMessage(`{"household_id":99}`)},
		{ToolSimulateGoal, json.RawMessage(`{"goal_id":7,"monthly_contribution_minor":"20000","household_id":99}`)},
		{ToolSimulateGoal, json.RawMessage(`{"goal_id":0,"monthly_contribution_minor":"20000"}`)},
		{ToolSimulateGoal, json.RawMessage(`{"goal_id":7,"monthly_contribution_minor":"9223372036854775808"}`)},
	}
	for _, tc := range cases {
		if _, err := service.Call(context.Background(), principal, tc.name, tc.args); !IsCode(err, CodeInvalidArgument) {
			t.Fatalf("%s args=%s error=%v, want %s", tc.name, tc.args, err, CodeInvalidArgument)
		}
	}
	if backend.safeCalls != 0 || backend.goalCalls != 0 {
		t.Fatalf("backend called for rejected input: safe=%d goal=%d", backend.safeCalls, backend.goalCalls)
	}
}

type phaseOneBackend struct {
	*fakeBackend
	safe server.SafeToSpendResponse
	goal server.GoalSimulationResponse

	safeCalls          int
	safeHouseholdID    int64
	goalCalls          int
	goalHouseholdID    int64
	goalID             int64
	goalContribution   int64
}

func (b *phaseOneBackend) SafeToSpend(_ context.Context, householdID int64) (server.SafeToSpendResponse, error) {
	b.safeCalls++
	b.safeHouseholdID = householdID
	return b.safe, nil
}

func (b *phaseOneBackend) SimulateGoal(_ context.Context, householdID, goalID, monthlyContributionMinor int64) (server.GoalSimulationResponse, error) {
	b.goalCalls++
	b.goalHouseholdID = householdID
	b.goalID = goalID
	b.goalContribution = monthlyContributionMinor
	return b.goal, nil
}
