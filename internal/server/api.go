package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const maxAPIRequestBytes = 1 << 20

type MoneyDTO struct {
	Minor    int64  `json:"minor,string"`
	Currency string `json:"currency"`
}

type OverviewResponse struct {
	DataAsOf        time.Time `json:"data_as_of"`
	Quality         string    `json:"quality"`
	NetWorth        MoneyDTO  `json:"net_worth"`
	Income          MoneyDTO  `json:"income"`
	Expense         MoneyDTO  `json:"expense"`
	NetCashflow     MoneyDTO  `json:"net_cashflow"`
	SavingsRate     string    `json:"savings_rate,omitempty"`
	SafeToSpend     MoneyDTO  `json:"safe_to_spend"`
	EmergencyMonths string    `json:"emergency_months,omitempty"`
	TotalDebt       MoneyDTO  `json:"total_debt"`
	GoalCount       int       `json:"goal_count"`
	Warnings        []string  `json:"warnings,omitempty"`
}

type CashflowResponse struct {
	DataAsOf    time.Time `json:"data_as_of"`
	Quality     string    `json:"quality"`
	Period      string    `json:"period"`
	Income      MoneyDTO  `json:"income"`
	Expense     MoneyDTO  `json:"expense"`
	NetCashflow MoneyDTO  `json:"net_cashflow"`
	SavingsRate string    `json:"savings_rate,omitempty"`
	Warnings    []string  `json:"warnings,omitempty"`
}

type BudgetLineResponse struct {
	Kind                string   `json:"kind"`
	ExternalCategoryRef string   `json:"external_category_ref,omitempty"`
	SemanticGroup       string   `json:"semantic_group,omitempty"`
	Planned             MoneyDTO `json:"planned"`
	Actual              MoneyDTO `json:"actual"`
	Remaining           MoneyDTO `json:"remaining"`
	Utilization         string   `json:"utilization,omitempty"`
}

type BudgetResponse struct {
	DataAsOf time.Time            `json:"data_as_of"`
	Quality  string               `json:"quality"`
	Period   string               `json:"period"`
	Currency string               `json:"currency"`
	Lines    []BudgetLineResponse `json:"lines"`
	Warnings []string             `json:"warnings,omitempty"`
}

type DebtResponse struct {
	ID                  int64    `json:"id"`
	Name                string   `json:"name"`
	Type                string   `json:"type"`
	Balance             MoneyDTO `json:"balance"`
	APR                 string   `json:"apr,omitempty"`
	RepaymentType       string   `json:"repayment_type"`
	MinimumPayment      MoneyDTO `json:"minimum_payment"`
	ScheduledPayment    MoneyDTO `json:"scheduled_payment"`
	TermRemainingMonths int32    `json:"term_remaining_months"`
	DueDay              int32    `json:"due_day"`
}

type DebtsResponse struct {
	DataAsOf time.Time      `json:"data_as_of"`
	Quality  string         `json:"quality"`
	Currency string         `json:"currency"`
	Total    MoneyDTO       `json:"total"`
	Items    []DebtResponse `json:"items"`
	Warnings []string       `json:"warnings,omitempty"`
}

type GoalResponse struct {
	ID                  int64    `json:"id"`
	Name                string   `json:"name"`
	Target              MoneyDTO `json:"target"`
	Funded              MoneyDTO `json:"funded"`
	TargetDate          string   `json:"target_date"`
	Priority            int32    `json:"priority"`
	Flexibility         string   `json:"flexibility"`
	MonthlyContribution MoneyDTO `json:"monthly_contribution"`
	RequiredMonthly     MoneyDTO `json:"required_monthly"`
	Status              string   `json:"status"`
}

type GoalsResponse struct {
	DataAsOf time.Time      `json:"data_as_of"`
	Quality  string         `json:"quality"`
	Items    []GoalResponse `json:"items"`
	Warnings []string       `json:"warnings,omitempty"`
}

type ScenarioRequest struct {
	HouseholdID int64           `json:"household_id"`
	Kind        string          `json:"kind"`
	Input       json.RawMessage `json:"input"`
}

type ScenarioResponse struct {
	Kind     string          `json:"kind"`
	Result   json.RawMessage `json:"result"`
	Warnings []string        `json:"warnings,omitempty"`
}

type AdvisorRequest struct {
	HouseholdID   int64  `json:"household_id"`
	Question      string `json:"question"`
	RequireTool   bool   `json:"require_tool,omitempty"`
	RequireReview bool   `json:"require_review,omitempty"`
}

type AdvisorResponse struct {
	Text        string   `json:"text,omitempty"`
	Reviewed    bool     `json:"reviewed"`
	Review      string   `json:"review,omitempty"`
	Blocked     bool     `json:"blocked"`
	BlockReason string   `json:"block_reason,omitempty"`
	Warnings    []string `json:"warnings,omitempty"`
}

type ReportSummary struct {
	ID        int64     `json:"id"`
	Period    string    `json:"period"`
	Kind      string    `json:"kind"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

type ReportsResponse struct {
	Items []ReportSummary `json:"items"`
}

type FinanceAPI interface {
	Overview(ctx context.Context, householdID int64) (OverviewResponse, error)
	Cashflow(ctx context.Context, householdID int64, period string) (CashflowResponse, error)
	Budget(ctx context.Context, householdID int64, period string) (BudgetResponse, error)
	Debts(ctx context.Context, householdID int64) (DebtsResponse, error)
	Goals(ctx context.Context, householdID int64) (GoalsResponse, error)
	Scenario(ctx context.Context, request ScenarioRequest) (ScenarioResponse, error)
	Advisor(ctx context.Context, request AdvisorRequest) (AdvisorResponse, error)
	Reports(ctx context.Context, householdID int64) (ReportsResponse, error)
}

type handlerConfig struct {
	api FinanceAPI
	web http.Handler
	mcp http.Handler
}

type HandlerOption func(*handlerConfig)

func WithAPI(api FinanceAPI) HandlerOption { return func(cfg *handlerConfig) { cfg.api = api } }
func WithWeb(handler http.Handler) HandlerOption {
	return func(cfg *handlerConfig) { cfg.web = handler }
}
func WithMCP(handler http.Handler) HandlerOption {
	return func(cfg *handlerConfig) { cfg.mcp = handler }
}

func registerFinanceAPI(mux *http.ServeMux, api FinanceAPI) {
	if api == nil {
		return
	}
	mux.HandleFunc("GET /api/v1/overview", func(w http.ResponseWriter, r *http.Request) {
		householdID, ok := parseHouseholdID(w, r)
		if !ok {
			return
		}
		response, err := api.Overview(r.Context(), householdID)
		writeBackendResult(w, response, err)
	})
	mux.HandleFunc("GET /api/v1/cashflow", func(w http.ResponseWriter, r *http.Request) {
		householdID, ok := parseHouseholdID(w, r)
		if !ok {
			return
		}
		period, ok := parsePeriod(w, r.URL.Query().Get("period"))
		if !ok {
			return
		}
		response, err := api.Cashflow(r.Context(), householdID, period)
		writeBackendResult(w, response, err)
	})
	mux.HandleFunc("GET /api/v1/budget", func(w http.ResponseWriter, r *http.Request) {
		householdID, ok := parseHouseholdID(w, r)
		if !ok {
			return
		}
		period, ok := parsePeriod(w, r.URL.Query().Get("period"))
		if !ok {
			return
		}
		response, err := api.Budget(r.Context(), householdID, period)
		writeBackendResult(w, response, err)
	})
	mux.HandleFunc("GET /api/v1/debts", func(w http.ResponseWriter, r *http.Request) {
		householdID, ok := parseHouseholdID(w, r)
		if !ok {
			return
		}
		response, err := api.Debts(r.Context(), householdID)
		writeBackendResult(w, response, err)
	})
	mux.HandleFunc("GET /api/v1/goals", func(w http.ResponseWriter, r *http.Request) {
		householdID, ok := parseHouseholdID(w, r)
		if !ok {
			return
		}
		response, err := api.Goals(r.Context(), householdID)
		writeBackendResult(w, response, err)
	})
	mux.HandleFunc("POST /api/v1/scenarios", func(w http.ResponseWriter, r *http.Request) {
		var request ScenarioRequest
		if !decodeStrictJSON(w, r, &request) {
			return
		}
		request.Kind = strings.TrimSpace(request.Kind)
		if request.HouseholdID <= 0 || request.Kind == "" || len(request.Input) == 0 || !json.Valid(request.Input) {
			writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid request")
			return
		}
		response, err := api.Scenario(r.Context(), request)
		writeBackendResult(w, response, err)
	})
	mux.HandleFunc("POST /api/v1/advisor", func(w http.ResponseWriter, r *http.Request) {
		var request AdvisorRequest
		if !decodeStrictJSON(w, r, &request) {
			return
		}
		request.Question = strings.TrimSpace(request.Question)
		if request.HouseholdID <= 0 || request.Question == "" || len(request.Question) > 8192 {
			writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid request")
			return
		}
		response, err := api.Advisor(r.Context(), request)
		writeBackendResult(w, response, err)
	})
	mux.HandleFunc("GET /api/v1/reports", func(w http.ResponseWriter, r *http.Request) {
		householdID, ok := parseHouseholdID(w, r)
		if !ok {
			return
		}
		response, err := api.Reports(r.Context(), householdID)
		writeBackendResult(w, response, err)
	})
}

func parseHouseholdID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	raw := strings.TrimSpace(r.URL.Query().Get("household_id"))
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid request")
		return 0, false
	}
	return value, true
}

func parsePeriod(w http.ResponseWriter, raw string) (string, bool) {
	period := strings.TrimSpace(raw)
	parsed, err := time.Parse("2006-01", period)
	if err != nil || parsed.Format("2006-01") != period {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid request")
		return "", false
	}
	return period, true
}

func decodeStrictJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxAPIRequestBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid request")
		return false
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid request")
		return false
	}
	return true
}

func writeBackendResult(w http.ResponseWriter, payload any, err error) {
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

func writeAPIError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		_ = fmt.Errorf("encode HTTP response: %w", err)
	}
}
