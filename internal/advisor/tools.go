package advisor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"

	"github.com/shawnwu2022/family-finance-os/internal/llm"
)

var (
	ErrInvalidToolInput      = errors.New("invalid finance tool input")
	ErrInvalidToolDefinition = errors.New("invalid finance tool definition")
	ErrToolNotAllowed        = errors.New("finance tool is not allowed")
	ErrDuplicateTool         = errors.New("duplicate finance tool")
	ErrToolNotFound          = errors.New("finance tool not found")
)

type ToolName string

const (
	ToolNameGetOverview              ToolName = "get_overview"
	ToolNameGetCashflow              ToolName = "get_cashflow"
	ToolNameGetBudgetStatus          ToolName = "get_budget_status"
	ToolNameGetDebtPlan              ToolName = "get_debt_plan"
	ToolNameGetGoalStatus            ToolName = "get_goal_status"
	ToolNameGetPortfolioAllocation   ToolName = "get_portfolio_allocation"
	ToolNameSimulatePurchase         ToolName = "simulate_purchase"
	ToolNameSimulateDebtPayment      ToolName = "simulate_debt_payment"
	ToolNameSimulateBudgetChange     ToolName = "simulate_budget_change"
	ToolNameSimulateSavingsChange    ToolName = "simulate_savings_change"
	ToolNameSimulateIncomeDrop       ToolName = "simulate_income_drop"
	ToolNameSimulateGoal             ToolName = "simulate_goal"
)

var allowedToolNames = []ToolName{
	ToolNameGetOverview,
	ToolNameGetCashflow,
	ToolNameGetBudgetStatus,
	ToolNameGetDebtPlan,
	ToolNameGetGoalStatus,
	ToolNameGetPortfolioAllocation,
	ToolNameSimulatePurchase,
	ToolNameSimulateDebtPayment,
	ToolNameSimulateBudgetChange,
	ToolNameSimulateSavingsChange,
	ToolNameSimulateIncomeDrop,
	ToolNameSimulateGoal,
}

var allowedToolSet = func() map[ToolName]struct{} {
	result := make(map[ToolName]struct{}, len(allowedToolNames))
	for _, name := range allowedToolNames {
		result[name] = struct{}{}
	}
	return result
}()

func AllowedToolNames() []ToolName {
	result := make([]ToolName, len(allowedToolNames))
	copy(result, allowedToolNames)
	return result
}

type Tool interface {
	toolName() ToolName
	definition() llm.ToolDefinition
	invoke(context.Context, json.RawMessage) (json.RawMessage, error)
	valid() error
}

type typedTool[I any, O any] struct {
	name        ToolName
	description string
	parameters  json.RawMessage
	handler     func(context.Context, I) (O, error)
}

func NewTypedTool[I any, O any](
	name ToolName,
	description string,
	parameters json.RawMessage,
	handler func(context.Context, I) (O, error),
) Tool {
	return &typedTool[I, O]{
		name:        name,
		description: description,
		parameters:  cloneRaw(parameters),
		handler:     handler,
	}
}

func (t *typedTool[I, O]) toolName() ToolName {
	return t.name
}

func (t *typedTool[I, O]) definition() llm.ToolDefinition {
	return llm.ToolDefinition{
		Name:        string(t.name),
		Description: t.description,
		Parameters:  cloneRaw(t.parameters),
		Strict:      true,
	}
}

func (t *typedTool[I, O]) valid() error {
	if _, ok := allowedToolSet[t.name]; !ok {
		return fmt.Errorf("%w: %q", ErrToolNotAllowed, t.name)
	}
	if t.handler == nil || len(t.parameters) == 0 || !json.Valid(t.parameters) {
		return fmt.Errorf("%w: %q", ErrInvalidToolDefinition, t.name)
	}
	return nil
}

func (t *typedTool[I, O]) invoke(ctx context.Context, raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("%w: %s: empty JSON", ErrInvalidToolInput, t.name)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var input I
	if err := decoder.Decode(&input); err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrInvalidToolInput, t.name, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("%w: %s: multiple JSON values", ErrInvalidToolInput, t.name)
		}
		return nil, fmt.Errorf("%w: %s: trailing JSON: %v", ErrInvalidToolInput, t.name, err)
	}

	output, err := t.handler(ctx, input)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(output)
	if err != nil {
		return nil, fmt.Errorf("encode finance tool %s result: %w", t.name, err)
	}
	return encoded, nil
}

type Registry struct {
	tools map[ToolName]Tool
}

func NewRegistry(tools ...Tool) (*Registry, error) {
	registry := &Registry{tools: make(map[ToolName]Tool, len(tools))}
	for _, tool := range tools {
		if tool == nil {
			return nil, ErrInvalidToolDefinition
		}
		if err := tool.valid(); err != nil {
			return nil, err
		}
		name := tool.toolName()
		if _, exists := registry.tools[name]; exists {
			return nil, fmt.Errorf("%w: %q", ErrDuplicateTool, name)
		}
		registry.tools[name] = tool
	}
	return registry, nil
}

func (r *Registry) Definitions() []llm.ToolDefinition {
	if r == nil {
		return nil
	}
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, string(name))
	}
	sort.Strings(names)
	result := make([]llm.ToolDefinition, 0, len(names))
	for _, rawName := range names {
		result = append(result, r.tools[ToolName(rawName)].definition())
	}
	return result
}

func (r *Registry) Invoke(ctx context.Context, name ToolName, arguments json.RawMessage) (json.RawMessage, error) {
	if r == nil {
		return nil, fmt.Errorf("%w: %q", ErrToolNotFound, name)
	}
	tool, ok := r.tools[name]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrToolNotFound, name)
	}
	return tool.invoke(ctx, arguments)
}

func cloneRaw(value json.RawMessage) json.RawMessage {
	if value == nil {
		return nil
	}
	result := make(json.RawMessage, len(value))
	copy(result, value)
	return result
}
