package appapi

import (
	"context"
	"testing"
	"time"

	"github.com/shawnwu2022/family-finance-os/internal/advisor"
	"github.com/shawnwu2022/family-finance-os/internal/analytics"
	"github.com/shawnwu2022/family-finance-os/internal/budget"
	"github.com/shawnwu2022/family-finance-os/internal/goals"
	"github.com/shawnwu2022/family-finance-os/internal/household"
	"github.com/shawnwu2022/family-finance-os/internal/ledger"
	"github.com/shawnwu2022/family-finance-os/internal/server"
	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

func TestAdvisorBindsHouseholdScopeOutsideModelInput(t *testing.T) {
	now := time.Date(2026, 8, 17, 4, 0, 0, 0, time.UTC)
	runner := &recordingScopedAdvisor{}
	planner := scopedAdvisorPlanner{
		profile: household.Profile{
			Household: household.Household{ID: 77, Name: "Scoped", BaseCurrency: "CNY", Timezone: "Asia/Shanghai"},
			Policy:    household.HouseholdPolicy{HouseholdID: 77, LiquidityFloor: money.Money{Currency: "CNY"}},
		},
		plan: budget.BudgetPlan{ID: 1, HouseholdID: 77, Period: "2026-08", Currency: "CNY"},
	}
	api, err := New(Dependencies{
		Ledger:  scopedAdvisorLedger{},
		Planner: planner,
		Advisor: runner,
		Now:     func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	response, err := api.Advisor(context.Background(), server.AdvisorRequest{
		HouseholdID:   77,
		Question:      "现在的现金流安全吗？",
		RequireTool:   true,
		RequireReview: true,
	})
	if err != nil {
		t.Fatalf("Advisor: %v", err)
	}
	if runner.householdID != 77 {
		t.Fatalf("advisor household scope=%d want 77", runner.householdID)
	}
	if runner.request.Question == "" || !runner.request.RequireTool || !runner.request.RequireReview {
		t.Fatalf("advisor request=%#v", runner.request)
	}
	if response.Text != "scoped advice" {
		t.Fatalf("response=%#v", response)
	}
}

type recordingScopedAdvisor struct {
	householdID int64
	request     advisor.AdviceRequest
}

func (r *recordingScopedAdvisor) Advise(_ context.Context, householdID int64, request advisor.AdviceRequest) (advisor.AdviceResult, error) {
	r.householdID = householdID
	r.request = request
	return advisor.AdviceResult{Text: "scoped advice"}, nil
}

type scopedAdvisorLedger struct{}

func (scopedAdvisorLedger) ListAccounts(context.Context) ([]ledger.Account, error) { return nil, nil }
func (scopedAdvisorLedger) ListCategories(context.Context) ([]ledger.Category, error) { return nil, nil }
func (scopedAdvisorLedger) ListTransactions(context.Context, ledger.TransactionQuery) ([]ledger.Transaction, error) {
	return nil, nil
}

type scopedAdvisorPlanner struct {
	profile household.Profile
	plan    budget.BudgetPlan
}

func (p scopedAdvisorPlanner) Profile(context.Context, int64) (household.Profile, error) { return p.profile, nil }
func (p scopedAdvisorPlanner) BudgetPlan(context.Context, int64, string) (budget.BudgetPlan, error) {
	return p.plan, nil
}
func (scopedAdvisorPlanner) Debts(context.Context, int64) ([]DebtSnapshot, error) { return nil, nil }
func (scopedAdvisorPlanner) Goals(context.Context, int64) ([]goals.FinancialGoal, error) { return nil, nil }

var _ = analytics.QualityGood
