package advisor

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/shawnwu2022/family-finance-os/internal/llm"
)

type purchaseArgs struct {
	AmountMinor int64 `json:"amount_minor"`
}

type purchaseResult struct {
	SafeToSpendAfter int64 `json:"safe_to_spend_after"`
}

func TestTypedToolDecodesStrictInputAndReturnsStructuredResult(t *testing.T) {
	called := false
	tool := NewTypedTool[purchaseArgs, purchaseResult](
		ToolNameSimulatePurchase,
		"simulate a proposed purchase",
		json.RawMessage(`{"type":"object","properties":{"amount_minor":{"type":"integer"}},"required":["amount_minor"],"additionalProperties":false}`),
		func(_ context.Context, in purchaseArgs) (purchaseResult, error) {
			called = true
			if in.AmountMinor != 8_999 {
				t.Fatalf("amount = %d", in.AmountMinor)
			}
			return purchaseResult{SafeToSpendAfter: 3_001}, nil
		},
	)
	registry, err := NewRegistry(tool)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}

	definitions := registry.Definitions()
	if len(definitions) != 1 {
		t.Fatalf("definitions = %#v", definitions)
	}
	if definitions[0].Name != string(ToolNameSimulatePurchase) || !definitions[0].Strict {
		t.Fatalf("definition = %#v", definitions[0])
	}

	raw, err := registry.Invoke(context.Background(), ToolNameSimulatePurchase, json.RawMessage(`{"amount_minor":8999}`))
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if !called {
		t.Fatal("typed handler was not called")
	}
	var result purchaseResult
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result.SafeToSpendAfter != 3_001 {
		t.Fatalf("result = %#v", result)
	}

	_, err = registry.Invoke(context.Background(), ToolNameSimulatePurchase, json.RawMessage(`{"amount_minor":8999,"sql":"DROP TABLE households"}`))
	if !errors.Is(err, ErrInvalidToolInput) {
		t.Fatalf("unknown-field error = %v", err)
	}
}

func TestRegistryRejectsUnapprovedOrDuplicateTools(t *testing.T) {
	invalid := NewTypedTool[struct{}, struct{}](
		ToolName("execute_sql"),
		"must never be exposed",
		json.RawMessage(`{"type":"object","additionalProperties":false}`),
		func(context.Context, struct{}) (struct{}, error) { return struct{}{}, nil },
	)
	if _, err := NewRegistry(invalid); !errors.Is(err, ErrToolNotAllowed) {
		t.Fatalf("invalid tool error = %v", err)
	}

	valid := NewTypedTool[struct{}, struct{}](
		ToolNameGetOverview,
		"read overview",
		json.RawMessage(`{"type":"object","additionalProperties":false}`),
		func(context.Context, struct{}) (struct{}, error) { return struct{}{}, nil },
	)
	if _, err := NewRegistry(valid, valid); !errors.Is(err, ErrDuplicateTool) {
		t.Fatalf("duplicate tool error = %v", err)
	}
}

func TestRegistryNeverExposesDestructiveToolNames(t *testing.T) {
	for _, name := range AllowedToolNames() {
		switch name {
		case ToolNameGetOverview,
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
			ToolNameSimulateGoal:
		default:
			t.Fatalf("unexpected V1 finance tool %q", name)
		}
	}
}

func TestRegistryDefinitionMatchesLLMToolContract(t *testing.T) {
	tool := NewTypedTool[struct{}, struct{}](
		ToolNameGetOverview,
		"read overview",
		json.RawMessage(`{"type":"object","additionalProperties":false}`),
		func(context.Context, struct{}) (struct{}, error) { return struct{}{}, nil },
	)
	registry, err := NewRegistry(tool)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	var definition llm.ToolDefinition = registry.Definitions()[0]
	if definition.Name != "get_overview" {
		t.Fatalf("definition name = %q", definition.Name)
	}
}
