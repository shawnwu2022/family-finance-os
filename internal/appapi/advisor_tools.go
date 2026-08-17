package appapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/shawnwu2022/family-finance-os/internal/advisor"
	"github.com/shawnwu2022/family-finance-os/internal/requestscope"
	"github.com/shawnwu2022/family-finance-os/internal/server"
)

var ErrHouseholdScopeRequired = errors.New("household scope is required")

type noToolInput struct{}

type periodToolInput struct {
	Period string `json:"period"`
}

type purchaseToolInput struct {
	AmountMinor string `json:"amount_minor"`
	Currency    string `json:"currency"`
}

var (
	noToolInputSchema = json.RawMessage(`{"type":"object","additionalProperties":false}`)
	periodInputSchema = json.RawMessage(`{"type":"object","properties":{"period":{"type":"string","pattern":"^[0-9]{4}-(0[1-9]|1[0-2])$"}},"required":["period"],"additionalProperties":false}`)
	purchaseInputSchema = json.RawMessage(`{"type":"object","properties":{"amount_minor":{"type":"string","pattern":"^[0-9]+$"},"currency":{"type":"string","minLength":3,"maxLength":3}},"required":["amount_minor","currency"],"additionalProperties":false}`)
)

func withHouseholdID(ctx context.Context, householdID int64) context.Context {
	return requestscope.WithHouseholdID(ctx, householdID)
}

func householdIDFromContext(ctx context.Context) (int64, error) {
	householdID, ok := requestscope.HouseholdID(ctx)
	if !ok {
		return 0, ErrHouseholdScopeRequired
	}
	return householdID, nil
}

func (a *API) AdvisorRegistry() (*advisor.Registry, error) {
	if a == nil {
		return nil, ErrInvalidDependencies
	}
	return advisor.NewRegistry(
		advisor.NewTypedTool(
			advisor.ToolNameGetOverview,
			"Get the current deterministic household finance overview.",
			noToolInputSchema,
			func(ctx context.Context, _ noToolInput) (server.OverviewResponse, error) {
				householdID, err := householdIDFromContext(ctx)
				if err != nil {
					return server.OverviewResponse{}, err
				}
				return a.Overview(ctx, householdID)
			},
		),
		advisor.NewTypedTool(
			advisor.ToolNameGetCashflow,
			"Get deterministic household cashflow for one YYYY-MM period.",
			periodInputSchema,
			func(ctx context.Context, input periodToolInput) (server.CashflowResponse, error) {
				householdID, err := householdIDFromContext(ctx)
				if err != nil {
					return server.CashflowResponse{}, err
				}
				return a.Cashflow(ctx, householdID, input.Period)
			},
		),
		advisor.NewTypedTool(
			advisor.ToolNameGetBudgetStatus,
			"Get deterministic budget status for one YYYY-MM period.",
			periodInputSchema,
			func(ctx context.Context, input periodToolInput) (server.BudgetResponse, error) {
				householdID, err := householdIDFromContext(ctx)
				if err != nil {
					return server.BudgetResponse{}, err
				}
				return a.Budget(ctx, householdID, input.Period)
			},
		),
		advisor.NewTypedTool(
			advisor.ToolNameGetDebtPlan,
			"Get current household debt balances, rates and scheduled payments.",
			noToolInputSchema,
			func(ctx context.Context, _ noToolInput) (server.DebtsResponse, error) {
				householdID, err := householdIDFromContext(ctx)
				if err != nil {
					return server.DebtsResponse{}, err
				}
				return a.Debts(ctx, householdID)
			},
		),
		advisor.NewTypedTool(
			advisor.ToolNameGetGoalStatus,
			"Get deterministic household goal progress and required monthly contributions.",
			noToolInputSchema,
			func(ctx context.Context, _ noToolInput) (server.GoalsResponse, error) {
				householdID, err := householdIDFromContext(ctx)
				if err != nil {
					return server.GoalsResponse{}, err
				}
				return a.Goals(ctx, householdID)
			},
		),
		advisor.NewTypedTool(
			advisor.ToolNameSimulatePurchase,
			"Simulate a purchase against deterministic cashflow, liquidity and safe-to-spend constraints.",
			purchaseInputSchema,
			func(ctx context.Context, input purchaseToolInput) (server.ScenarioResponse, error) {
				householdID, err := householdIDFromContext(ctx)
				if err != nil {
					return server.ScenarioResponse{}, err
				}
				raw, err := json.Marshal(input)
				if err != nil {
					return server.ScenarioResponse{}, fmt.Errorf("encode purchase tool input: %w", err)
				}
				return a.Scenario(ctx, server.ScenarioRequest{HouseholdID: householdID, Kind: "purchase", Input: raw})
			},
		),
	)
}
