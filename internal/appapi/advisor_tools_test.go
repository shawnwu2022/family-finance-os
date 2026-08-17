package appapi

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/shawnwu2022/family-finance-os/internal/advisor"
	"github.com/shawnwu2022/family-finance-os/internal/budget"
	"github.com/shawnwu2022/family-finance-os/internal/household"
	"github.com/shawnwu2022/family-finance-os/internal/ledger"
	"github.com/shawnwu2022/family-finance-os/internal/server"
	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

func TestAdvisorRegistryScopesFinanceToolsToServerSelectedHousehold(t *testing.T) {
	now := time.Date(2026, 8, 17, 3, 30, 0, 0, time.UTC)
	api, err := New(Dependencies{
		Ledger: fakeLedger{
			accounts: []ledger.Account{
				{ID: "cash", Category: ledger.AccountCategoryChecking, Structure: ledger.AccountStructureSingle, Balance: money.Money{Minor: 80_000, Currency: "CNY"}, IsAsset: true},
			},
		},
		Planner: fakePlanner{
			profile: household.Profile{
				Household: household.Household{ID: 42, BaseCurrency: "CNY", Timezone: "Asia/Shanghai"},
				Policy:    household.HouseholdPolicy{HouseholdID: 42, LiquidityFloor: money.Money{Minor: 20_000, Currency: "CNY"}},
			},
			plan: budget.BudgetPlan{ID: 1, HouseholdID: 42, Period: "2026-08", Currency: "CNY"},
		},
		Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	registry, err := api.AdvisorRegistry()
	if err != nil {
		t.Fatalf("AdvisorRegistry: %v", err)
	}

	if _, err := registry.Invoke(context.Background(), advisor.ToolNameGetOverview, json.RawMessage(`{}`)); !errors.Is(err, ErrHouseholdScopeRequired) {
		t.Fatalf("unscoped tool error=%v", err)
	}

	raw, err := registry.Invoke(withHouseholdID(context.Background(), 42), advisor.ToolNameGetOverview, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	var overview server.OverviewResponse
	if err := json.Unmarshal(raw, &overview); err != nil {
		t.Fatalf("decode overview: %v", err)
	}
	if overview.NetWorth.Minor != 80_000 {
		t.Fatalf("net worth=%d want 80000", overview.NetWorth.Minor)
	}

	definitionFound := false
	for _, definition := range registry.Definitions() {
		if definition.Name != string(advisor.ToolNameGetOverview) {
			continue
		}
		definitionFound = true
		if string(definition.Parameters) != `{"type":"object","additionalProperties":false}` {
			t.Fatalf("overview tool parameters=%s", definition.Parameters)
		}
	}
	if !definitionFound {
		t.Fatal("get_overview tool is missing")
	}
}
