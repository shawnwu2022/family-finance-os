package agentadapter

import (
	"encoding/json"
	"sort"
)

type ToolName string

const (
	ToolGetHouseholdOverview     ToolName = "get_household_overview"
	ToolGetCashflow              ToolName = "get_cashflow"
	ToolGetBudgetStatus          ToolName = "get_budget_status"
	ToolGetDebtStatus            ToolName = "get_debt_status"
	ToolGetGoalStatus            ToolName = "get_goal_status"
	ToolGetSafeToSpend           ToolName = "get_safe_to_spend"
	ToolGetSpendingAnalysis      ToolName = "get_spending_analysis"
	ToolSimulateExtraDebtPayment ToolName = "simulate_extra_debt_payment"
	ToolSimulateGoal             ToolName = "simulate_goal"
	ToolSimulatePurchase         ToolName = "simulate_purchase"
	ToolGenerateMonthlyReport    ToolName = "generate_monthly_report"
)

type EmptyInput struct{}

type PeriodInput struct {
	Period string `json:"period"`
}

type SpendingAnalysisInput struct {
	Period         string `json:"period"`
	ComparePeriods int    `json:"compare_periods"`
}

type PurchaseInput struct {
	AmountMinor string `json:"amount_minor"`
	Currency    string `json:"currency"`
}

type DebtExtraPaymentInput struct {
	DebtID      int64  `json:"debt_id"`
	AmountMinor string `json:"amount_minor"`
}

type GoalSimulationInput struct {
	GoalID                   int64  `json:"goal_id"`
	MonthlyContributionMinor string `json:"monthly_contribution_minor"`
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
	emptyInputSchema            = json.RawMessage(`{"type":"object","additionalProperties":false}`)
	periodInputSchema           = json.RawMessage(`{"type":"object","properties":{"period":{"type":"string","pattern":"^[0-9]{4}-(0[1-9]|1[0-2])$"}},"required":["period"],"additionalProperties":false}`)
	spendingAnalysisInputSchema = json.RawMessage(`{"type":"object","properties":{"period":{"type":"string","pattern":"^[0-9]{4}-(0[1-9]|1[0-2])$"},"compare_periods":{"type":"integer","minimum":0,"maximum":12}},"required":["period","compare_periods"],"additionalProperties":false}`)
	purchaseInputSchema         = json.RawMessage(`{"type":"object","properties":{"amount_minor":{"type":"string","pattern":"^[0-9]+$"},"currency":{"type":"string","minLength":3,"maxLength":3}},"required":["amount_minor","currency"],"additionalProperties":false}`)
	debtExtraPaymentInputSchema = json.RawMessage(`{"type":"object","properties":{"debt_id":{"type":"integer","minimum":1},"amount_minor":{"type":"string","pattern":"^[1-9][0-9]*$"}},"required":["debt_id","amount_minor"],"additionalProperties":false}`)
	goalSimulationInputSchema   = json.RawMessage(`{"type":"object","properties":{"goal_id":{"type":"integer","minimum":1},"monthly_contribution_minor":{"type":"string","pattern":"^[0-9]+$"}},"required":["goal_id","monthly_contribution_minor"],"additionalProperties":false}`)
	monthlyReportInputSchema    = json.RawMessage(`{"type":"object","properties":{"year":{"type":"integer","minimum":1970,"maximum":9999},"month":{"type":"integer","minimum":1,"maximum":12}},"required":["year","month"],"additionalProperties":false}`)
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
		Name:        ToolGetSafeToSpend,
		Description: "Get the current deterministic Finance Core safe-to-spend result and its components. Preserve quality and warning metadata.",
		InputSchema: emptyInputSchema,
		ReadOnly:    true,
	},
	{
		Name:        ToolGetSpendingAnalysis,
		Description: "Get deterministic net household spending for one YYYY-MM period and zero to twelve prior complete calendar months, aggregated by ledger category without merchant inference. Preserve quality and warning metadata.",
		InputSchema: spendingAnalysisInputSchema,
		ReadOnly:    true,
	},
	{
		Name:        ToolSimulateExtraDebtPayment,
		Description: "Simulate one one-time extra principal payment on an existing household debt at the first contractually eligible month while keeping the existing scheduled-payment rule. Uses deterministic Finance Core debt rules, including prepayment restrictions and fees, and persists no changes.",
		InputSchema: debtExtraPaymentInputSchema,
		ReadOnly:    true,
	},
	{
		Name:        ToolSimulateGoal,
		Description: "Simulate a monthly contribution for an existing household goal using deterministic Finance Core projection rules without persisting changes. Preserve quality and warning metadata.",
		InputSchema: goalSimulationInputSchema,
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
