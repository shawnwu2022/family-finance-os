package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFinanceAPIOverviewContract(t *testing.T) {
	fake := &fakeFinanceAPI{
		overview: OverviewResponse{
			DataAsOf:        time.Date(2026, 8, 17, 3, 30, 0, 0, time.UTC),
			Quality:         "good",
			NetWorth:        MoneyDTO{Minor: 8_800_000, Currency: "CNY"},
			Income:          MoneyDTO{Minor: 1_800_000, Currency: "CNY"},
			Expense:         MoneyDTO{Minor: 1_100_000, Currency: "CNY"},
			NetCashflow:     MoneyDTO{Minor: 700_000, Currency: "CNY"},
			SavingsRate:     "0.3888888888888888888888888888888889",
			SafeToSpend:     MoneyDTO{Minor: 350_000, Currency: "CNY"},
			EmergencyMonths: "6.5",
			TotalDebt:       MoneyDTO{Minor: 2_000_000, Currency: "CNY"},
			GoalCount:       2,
		},
	}
	handler := newFinanceAPIUnitHandler(fake)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/overview?household_id=42", nil)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", resp.Code, resp.Body.String())
	}
	if fake.overviewHouseholdID != 42 {
		t.Fatalf("household id=%d", fake.overviewHouseholdID)
	}
	if got := resp.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control=%q want no-store", got)
	}
	if got := resp.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("Content-Type=%q", got)
	}
	raw := resp.Body.Bytes()
	if !bytes.Contains(raw, []byte(`"minor":"350000"`)) {
		t.Fatalf("Money minor must be a JSON string for browser int64 safety: %s", raw)
	}
	var got OverviewResponse
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.SafeToSpend.Minor != 350_000 || got.NetWorth.Minor != 8_800_000 || got.Quality != "good" {
		t.Fatalf("overview=%#v", got)
	}
}

func TestFinanceAPIDashboardContract(t *testing.T) {
	fake := &fakeFinanceAPI{dashboard: DashboardResponse{Cashflow: CashflowResponse{Period: "2026-08"}}}
	handler := newFinanceAPIUnitHandler(fake)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard?household_id=42&period=2026-08", nil)
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", resp.Code, resp.Body.String())
	}
	if fake.dashboardHouseholdID != 42 || fake.dashboardPeriod != "2026-08" {
		t.Fatalf("dashboard query = household %d period %q", fake.dashboardHouseholdID, fake.dashboardPeriod)
	}
	var got DashboardResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode dashboard: %v", err)
	}
	if got.Cashflow.Period != "2026-08" {
		t.Fatalf("dashboard response = %#v", got)
	}
}

func TestFinanceAPIRejectsInvalidQueryBeforeBackend(t *testing.T) {
	fake := &fakeFinanceAPI{}
	handler := newFinanceAPIUnitHandler(fake)
	for _, target := range []string{
		"/api/v1/overview",
		"/api/v1/overview?household_id=0",
		"/api/v1/cashflow?household_id=1&period=2026-13",
		"/api/v1/budget?household_id=abc&period=2026-08",
	} {
		t.Run(target, func(t *testing.T) {
			resp := httptest.NewRecorder()
			handler.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, target, nil))
			if resp.Code != http.StatusBadRequest {
				t.Fatalf("status=%d body=%s", resp.Code, resp.Body.String())
			}
		})
	}
	if fake.calls != 0 {
		t.Fatalf("backend calls=%d want 0", fake.calls)
	}
}

func TestFinanceAPIScenarioAndAdvisorStrictJSON(t *testing.T) {
	fake := &fakeFinanceAPI{
		scenario: ScenarioResponse{Kind: "purchase", Result: json.RawMessage(`{"affordable":true}`)},
		advisor:  AdvisorResponse{Text: "可以购买，但会降低安全可消费余额。", Reviewed: true, Review: "审查通过"},
	}
	handler := newFinanceAPIUnitHandler(fake)

	t.Run("scenario", func(t *testing.T) {
		body := `{"household_id":7,"kind":"purchase","input":{"amount_minor":899900,"currency":"CNY"}}`
		resp := httptest.NewRecorder()
		handler.ServeHTTP(resp, httptest.NewRequest(http.MethodPost, "/api/v1/scenarios", strings.NewReader(body)))
		if resp.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", resp.Code, resp.Body.String())
		}
		if fake.scenarioRequest.HouseholdID != 7 || fake.scenarioRequest.Kind != "purchase" || !json.Valid(fake.scenarioRequest.Input) {
			t.Fatalf("scenario request=%#v", fake.scenarioRequest)
		}
	})

	t.Run("advisor", func(t *testing.T) {
		body := `{"household_id":7,"question":"现在可以买电脑吗？","require_review":true}`
		resp := httptest.NewRecorder()
		handler.ServeHTTP(resp, httptest.NewRequest(http.MethodPost, "/api/v1/advisor", strings.NewReader(body)))
		if resp.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", resp.Code, resp.Body.String())
		}
		if fake.advisorRequest.HouseholdID != 7 || fake.advisorRequest.Question == "" || !fake.advisorRequest.RequireReview {
			t.Fatalf("advisor request=%#v", fake.advisorRequest)
		}
	})

	t.Run("unknown field rejected", func(t *testing.T) {
		body := `{"household_id":7,"question":"test","raw_sql":"select *"}`
		resp := httptest.NewRecorder()
		handler.ServeHTTP(resp, httptest.NewRequest(http.MethodPost, "/api/v1/advisor", strings.NewReader(body)))
		if resp.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", resp.Code, resp.Body.String())
		}
	})
}

func TestFinanceAPIDoesNotLeakBackendErrors(t *testing.T) {
	fake := &fakeFinanceAPI{overviewErr: errors.New("SECRET_DATABASE_FAILURE")}
	handler := newFinanceAPIUnitHandler(fake)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/api/v1/overview?household_id=1", nil))
	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s", resp.Code, resp.Body.String())
	}
	if strings.Contains(resp.Body.String(), "SECRET_DATABASE_FAILURE") {
		t.Fatalf("backend error leaked: %s", resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "internal_error") {
		t.Fatalf("missing stable error code: %s", resp.Body.String())
	}
}

type fakeFinanceAPI struct {
	calls                int
	overview             OverviewResponse
	overviewErr          error
	overviewHouseholdID  int64
	scenario             ScenarioResponse
	scenarioRequest      ScenarioRequest
	advisor              AdvisorResponse
	advisorRequest       AdvisorRequest
	dashboard            DashboardResponse
	dashboardHouseholdID int64
	dashboardPeriod      string
}

func (f *fakeFinanceAPI) Dashboard(_ context.Context, householdID int64, period string) (DashboardResponse, error) {
	f.calls++
	f.dashboardHouseholdID = householdID
	f.dashboardPeriod = period
	return f.dashboard, nil
}

func (f *fakeFinanceAPI) Overview(_ context.Context, householdID int64) (OverviewResponse, error) {
	f.calls++
	f.overviewHouseholdID = householdID
	return f.overview, f.overviewErr
}
func (f *fakeFinanceAPI) Cashflow(context.Context, int64, string) (CashflowResponse, error) {
	f.calls++
	return CashflowResponse{}, nil
}
func (f *fakeFinanceAPI) Budget(context.Context, int64, string) (BudgetResponse, error) {
	f.calls++
	return BudgetResponse{}, nil
}
func (f *fakeFinanceAPI) Debts(context.Context, int64) (DebtsResponse, error) {
	f.calls++
	return DebtsResponse{}, nil
}
func (f *fakeFinanceAPI) Goals(context.Context, int64) (GoalsResponse, error) {
	f.calls++
	return GoalsResponse{}, nil
}
func (f *fakeFinanceAPI) Scenario(_ context.Context, request ScenarioRequest) (ScenarioResponse, error) {
	f.calls++
	f.scenarioRequest = request
	return f.scenario, nil
}
func (f *fakeFinanceAPI) Advisor(_ context.Context, request AdvisorRequest) (AdvisorResponse, error) {
	f.calls++
	f.advisorRequest = request
	return f.advisor, nil
}
func (f *fakeFinanceAPI) Reports(context.Context, int64) (ReportsResponse, error) {
	f.calls++
	return ReportsResponse{}, nil
}
