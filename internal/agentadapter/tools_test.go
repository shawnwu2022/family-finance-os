package agentadapter

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestDefinitionsExposeOnlyImplementedReadAndSimulationCapabilities(t *testing.T) {
	got := definitions()
	want := []ToolName{
		ToolGenerateMonthlyReport,
		ToolGetBudgetStatus,
		ToolGetCashflow,
		ToolGetDebtStatus,
		ToolGetGoalStatus,
		ToolGetHouseholdOverview,
		ToolGetSafeToSpend,
		ToolSimulateExtraDebtPayment,
		ToolSimulateGoal,
		ToolSimulatePurchase,
	}
	if names := gotNames(got); !reflect.DeepEqual(names, want) {
		t.Fatalf("tool names = %#v, want %#v", names, want)
	}
	for _, definition := range got {
		if !definition.ReadOnly {
			t.Fatalf("tool %q is not marked read-only/non-destructive", definition.Name)
		}
		if strings.TrimSpace(definition.Description) == "" {
			t.Fatalf("tool %q has an empty description", definition.Name)
		}
		if bytes.Contains(definition.InputSchema, []byte("household_id")) {
			t.Fatalf("tool %q exposes household_id: %s", definition.Name, definition.InputSchema)
		}
		if !json.Valid(definition.InputSchema) {
			t.Fatalf("tool %q has invalid schema: %s", definition.Name, definition.InputSchema)
		}
	}
}

func TestSchemasRejectAdditionalPropertiesByContract(t *testing.T) {
	for _, definition := range definitions() {
		var schema map[string]any
		if err := json.Unmarshal(definition.InputSchema, &schema); err != nil {
			t.Fatalf("decode %s schema: %v", definition.Name, err)
		}
		if value, ok := schema["additionalProperties"].(bool); !ok || value {
			t.Fatalf("tool %q must set additionalProperties=false", definition.Name)
		}
	}
}

func gotNames(definitions []ToolDefinition) []ToolName {
	names := make([]ToolName, 0, len(definitions))
	for _, definition := range definitions {
		names = append(names, definition.Name)
	}
	return names
}
