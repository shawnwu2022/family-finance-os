package agentadapter

import (
	"encoding/json"
	"sort"
)

type ToolName string

const (
	ToolGetHouseholdOverview  ToolName = "get_household_overview"
	ToolGetCashflow           ToolName = "get_cashflow"
	ToolGetBudgetStatus       ToolName = "get_budget_status"
	ToolGetDebtStatus         ToolName = "get_debt_status"
	ToolGetGoalStatus         ToolName = "get_goal_status"
	ToolSimulatePurchase      ToolName = "simulate_purchase"
	ToolGenerateMonthlyReport ToolName = "generate_monthly_report"
)

type EmptyInput struct{}

type PeriodInput struct {
	Period string `json:"period"`
}

type PurchaseInput struct {
	AmountMinor string `json:"amount_minor"`
	Currency    string `json:"currency"`
}

type MonthlyReportInput struct {
	Year  int `json:"year"`
	Month int `json:"month"`
}

type ToolDefinition struct {
	Name        ToolName
	Description string
	InputSchema json.RawMessage
	ReadOnly    bool
}

var (
	emptyInputSchema         = json.RawMessage(`{"type":"object","additionalProperties":false}`)
	periodInputSchema        = json.RawMessage(`{"type":"object","properties":{"period":{"type":"string","pattern":"^[0-9]{4}-(0[1-9]|1[0-2])$"}},"required":["period"],"additionalProperties":false}`)
	purchaseInputSchema      = json.RawMessage(`{"type":"object","properties":{"amount_minor":{"type":"string","pattern":"^[0-9]+$"},"currency":{"type":"string","minLength":3,"maxLength":3}},"required":["amount_minor","currency"],"additionalProperties":false}`)
	monthlyReportInputSchema = json.RawMessage(`{"type":"object","properties":{"year":{"type":"integer","minimum":1970,"maximum":9999},"month":{"type":"integer","minimum":1,"maximum":12}},"required":["year","month"],"additionalProperties":false}`)
)

var initialDefinitions = []ToolDefinition{
	{
		Name:        ToolGetHouseholdOverview,
		Description: "Get the current deterministic Finance Core household overview. Preserve quality and warning metadata.",
		InputSchema: emptyInputSchema,
		ReadOnly:    true,
	},
	{
		Name:        ToolGetCashflow,
		Description: "Get deterministic Finance Core household cashflow for one YYYY-MM period. Preserve quality and warning metadata.",
		InputSchema: periodInputSchema,
		ReadOnly:    true,
	},
	{
		Name:        ToolGetBudgetStatus,
		Description: "Get deterministic Finance Core budget status for one YYYY-MM period. Preserve quality and warning metadata.",
		InputSchema: periodInputSchema,
		ReadOnly:    true,
	},
	{
		Name:        ToolGetDebtStatus,
		Description: "Get current deterministic Finance Core household debt status. Preserve quality and warning metadata.",
		InputSchema: emptyInputSchema,
		ReadOnly:    true,
	},
	{
		Name:        ToolGetGoalStatus,
		Description: "Get deterministic Finance Core household goal status. Preserve quality and warning metadata.",
		InputSchema: emptyInputSchema,
		ReadOnly:    true,
	},
	{
		Name:        ToolSimulatePurchase,
		Description: "Simulate a proposed purchase using deterministic Finance Core rules without persisting changes. Preserve warning metadata.",
		InputSchema: purchaseInputSchema,
		ReadOnly:    true,
	},
	{
		Name:        ToolGenerateMonthlyReport,
		Description: "Generate a deterministic Finance Core monthly report without persisting ledger changes. Preserve quality and warning metadata.",
		InputSchema: monthlyReportInputSchema,
		ReadOnly:    true,
	},
}

func definitions() []ToolDefinition {
	result := make([]ToolDefinition, len(initialDefinitions))
	for i, definition := range initialDefinitions {
		result[i] = definition
		result[i].InputSchema = cloneRaw(definition.InputSchema)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

func cloneRaw(value json.RawMessage) json.RawMessage {
	if value == nil {
		return nil
	}
	result := make(json.RawMessage, len(value))
	copy(result, value)
	return result
}
