package agentadapter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/shawnwu2022/family-finance-os/internal/report"
	"github.com/shawnwu2022/family-finance-os/internal/server"
)

type Principal struct {
	Kind        string
	HouseholdID int64
}

type FinanceBackend interface {
	Overview(context.Context, int64) (server.OverviewResponse, error)
	Cashflow(context.Context, int64, string) (server.CashflowResponse, error)
	Budget(context.Context, int64, string) (server.BudgetResponse, error)
	Debts(context.Context, int64) (server.DebtsResponse, error)
	Goals(context.Context, int64) (server.GoalsResponse, error)
	SafeToSpend(context.Context, int64) (server.SafeToSpendResponse, error)
	SpendingAnalysis(context.Context, int64, string, int) (server.SpendingAnalysisResponse, error)
	AssetAllocation(context.Context, int64) (server.AssetAllocationResponse, error)
	SimulateExtraDebtPayment(context.Context, int64, int64, int64) (server.DebtExtraPaymentSimulationResponse, error)
	SimulateGoal(context.Context, int64, int64, int64) (server.GoalSimulationResponse, error)
	Scenario(context.Context, server.ScenarioRequest) (server.ScenarioResponse, error)
	MonthlyReport(context.Context, int64, string) (report.MonthlyReport, error)
}

type Result struct {
	Data     json.RawMessage `json:"data"`
	AsOf     *time.Time      `json:"as_of,omitempty"`
	Quality  string          `json:"quality,omitempty"`
	Warnings []string        `json:"warnings,omitempty"`
	AuditID  string          `json:"audit_id,omitempty"`
}

type Service struct {
	backend FinanceBackend
}

var (
	periodPattern      = regexp.MustCompile(`^[0-9]{4}-(0[1-9]|1[0-2])$`)
	amountMinorPattern = regexp.MustCompile(`^[0-9]+$`)
)

func New(backend FinanceBackend) (*Service, error) {
	if backend == nil {
		return nil, fmt.Errorf("agentadapter: backend is required")
	}
	return &Service{backend: backend}, nil
}

func (s *Service) Definitions() []ToolDefinition {
	return definitions()
}

func (s *Service) Call(ctx context.Context, principal Principal, name ToolName, arguments json.RawMessage) (Result, error) {
	if strings.TrimSpace(principal.Kind) == "" || principal.HouseholdID <= 0 {
		return Result{}, adapterError(CodeForbidden, "agent principal is not permitted", nil)
	}

	switch name {
	case ToolGetHouseholdOverview:
		if _, err := decodeStrict[EmptyInput](arguments); err != nil {
			return Result{}, err
		}
		value, err := s.backend.Overview(ctx, principal.HouseholdID)
		asOf := value.DataAsOf
		return encodeBackendResult(value, err, &asOf, value.Quality, value.Warnings)

	case ToolGetCashflow:
		input, err := decodeStrict[PeriodInput](arguments)
		if err != nil {
			return Result{}, err
		}
		if !periodPattern.MatchString(input.Period) {
			return Result{}, adapterError(CodeInvalidArgument, "period must use YYYY-MM", nil)
		}
		value, err := s.backend.Cashflow(ctx, principal.HouseholdID, input.Period)
		asOf := value.DataAsOf
		return encodeBackendResult(value, err, &asOf, value.Quality, value.Warnings)

	case ToolGetSpendingAnalysis:
		input, err := decodeStrict[SpendingAnalysisInput](arguments)
		if err != nil {
			return Result{}, err
		}
		if !periodPattern.MatchString(input.Period) || input.ComparePeriods < 0 || input.ComparePeriods > 12 {
			return Result{}, adapterError(CodeInvalidArgument, "period or compare_periods is invalid", nil)
		}
		value, backendErr := s.backend.SpendingAnalysis(ctx, principal.HouseholdID, input.Period, input.ComparePeriods)
		asOf := value.DataAsOf
		return encodeBackendResult(value, backendErr, &asOf, value.Quality, value.Warnings)

	case ToolGetBudgetStatus:
		input, err := decodeStrict[PeriodInput](arguments)
		if err != nil {
			return Result{}, err
		}
		if !periodPattern.MatchString(input.Period) {
			return Result{}, adapterError(CodeInvalidArgument, "period must use YYYY-MM", nil)
		}
		value, err := s.backend.Budget(ctx, principal.HouseholdID, input.Period)
		asOf := value.DataAsOf
		return encodeBackendResult(value, err, &asOf, value.Quality, value.Warnings)

	case ToolGetSafeToSpend:
		if _, err := decodeStrict[EmptyInput](arguments); err != nil {
			return Result{}, err
		}
		value, err := s.backend.SafeToSpend(ctx, principal.HouseholdID)
		asOf := value.DataAsOf
		return encodeBackendResult(value, err, &asOf, value.Quality, value.Warnings)

	case ToolGetDebtStatus:
		if _, err := decodeStrict[EmptyInput](arguments); err != nil {
			return Result{}, err
		}
		value, err := s.backend.Debts(ctx, principal.HouseholdID)
		asOf := value.DataAsOf
		return encodeBackendResult(value, err, &asOf, value.Quality, value.Warnings)

	case ToolSimulateExtraDebtPayment:
		input, err := decodeStrict[DebtExtraPaymentInput](arguments)
		if err != nil {
			return Result{}, err
		}
		if input.DebtID <= 0 || !amountMinorPattern.MatchString(input.AmountMinor) {
			return Result{}, adapterError(CodeInvalidArgument, "debt_id and amount_minor are invalid", nil)
		}
		amountMinor, err := strconv.ParseInt(input.AmountMinor, 10, 64)
		if err != nil || amountMinor <= 0 {
			return Result{}, adapterError(CodeInvalidArgument, "amount_minor must be a positive supported integer", err)
		}
		value, backendErr := s.backend.SimulateExtraDebtPayment(ctx, principal.HouseholdID, input.DebtID, amountMinor)
		asOf := value.DataAsOf
		return encodeBackendResult(value, backendErr, &asOf, value.Quality, value.Warnings)

	case ToolSimulatePurchase:
		input, err := decodeStrict[PurchaseInput](arguments)
		if err != nil {
			return Result{}, err
		}
		if !amountMinorPattern.MatchString(input.AmountMinor) {
			return Result{}, adapterError(CodeInvalidArgument, "amount_minor must contain only decimal digits", nil)
		}
		if len(input.Currency) != 3 {
			return Result{}, adapterError(CodeInvalidArgument, "currency must be a three-character code", nil)
		}
		canonical, err := json.Marshal(input)
		if err != nil {
			return Result{}, adapterError(CodeInternal, "tool arguments could not be encoded", err)
		}
		value, backendErr := s.backend.Scenario(ctx, server.ScenarioRequest{
			HouseholdID: principal.HouseholdID,
			Kind:        "purchase",
			Input:       canonical,
		})
		return encodeBackendResult(value, backendErr, nil, "", value.Warnings)

	case ToolGetGoalStatus:
		if _, err := decodeStrict[EmptyInput](arguments); err != nil {
			return Result{}, err
		}
		value, err := s.backend.Goals(ctx, principal.HouseholdID)
		asOf := value.DataAsOf
		return encodeBackendResult(value, err, &asOf, value.Quality, value.Warnings)

	case ToolSimulateGoal:
		input, err := decodeStrict[GoalSimulationInput](arguments)
		if err != nil {
			return Result{}, err
		}
		if input.GoalID <= 0 || !amountMinorPattern.MatchString(input.MonthlyContributionMinor) {
			return Result{}, adapterError(CodeInvalidArgument, "goal_id and monthly_contribution_minor are invalid", nil)
		}
		monthlyContributionMinor, err := strconv.ParseInt(input.MonthlyContributionMinor, 10, 64)
		if err != nil {
			return Result{}, adapterError(CodeInvalidArgument, "monthly_contribution_minor is outside the supported range", err)
		}
		value, backendErr := s.backend.SimulateGoal(ctx, principal.HouseholdID, input.GoalID, monthlyContributionMinor)
		asOf := value.DataAsOf
		return encodeBackendResult(value, backendErr, &asOf, value.Quality, value.Warnings)

	case ToolGetAssetAllocation:
		if _, err := decodeStrict[EmptyInput](arguments); err != nil {
			return Result{}, err
		}
		value, backendErr := s.backend.AssetAllocation(ctx, principal.HouseholdID)
		asOf := value.DataAsOf
		return encodeBackendResult(value, backendErr, &asOf, value.Quality, value.Warnings)

	case ToolGenerateMonthlyReport:
		input, err := decodeStrict[MonthlyReportInput](arguments)
		if err != nil {
			return Result{}, err
		}
		if input.Year < 1970 || input.Year > 9999 || input.Month < 1 || input.Month > 12 {
			return Result{}, adapterError(CodeInvalidArgument, "year or month is outside the supported range", nil)
		}
		period := fmt.Sprintf("%04d-%02d", input.Year, input.Month)
		value, backendErr := s.backend.MonthlyReport(ctx, principal.HouseholdID, period)
		asOf := value.DataAsOf
		return encodeBackendResult(value, backendErr, &asOf, value.Quality, value.Warnings)

	default:
		return Result{}, adapterError(CodeToolNotFound, "requested tool is not available", nil)
	}
}

func decodeStrict[T any](raw json.RawMessage) (T, error) {
	var value T
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return value, adapterError(CodeInvalidArgument, "tool arguments must be one JSON object", nil)
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return value, adapterError(CodeInvalidArgument, "tool arguments are invalid", err)
	}

	var trailing any
	err := decoder.Decode(&trailing)
	if err == nil {
		return value, adapterError(CodeInvalidArgument, "tool arguments must contain exactly one JSON value", nil)
	}
	if err != io.EOF {
		return value, adapterError(CodeInvalidArgument, "tool arguments contain invalid trailing data", err)
	}
	return value, nil
}

func encodeBackendResult[T any](value T, backendErr error, asOf *time.Time, quality string, warnings []string) (Result, error) {
	if errors.Is(backendErr, context.DeadlineExceeded) || errors.Is(backendErr, context.Canceled) {
		return Result{}, adapterError(CodeTimeout, "tool execution timed out", backendErr)
	}
	if backendErr != nil {
		return Result{}, adapterError(CodeDataUnavailable, "finance data is unavailable", backendErr)
	}
	data, err := json.Marshal(value)
	if err != nil {
		return Result{}, adapterError(CodeInternal, "finance result could not be encoded", err)
	}
	return Result{
		Data:     data,
		AsOf:     cloneTime(asOf),
		Quality:  quality,
		Warnings: cloneWarnings(warnings),
	}, nil
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func cloneWarnings(values []string) []string {
	return append([]string(nil), values...)
}

func adapterError(code ErrorCode, message string, err error) *Error {
	return &Error{Code: code, Message: message, Err: err}
}
