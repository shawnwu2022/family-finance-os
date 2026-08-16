package audit

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/shawnwu2022/family-finance-os/internal/llm"
)

func TestSHA256HexIsStable(t *testing.T) {
	got := SHA256Hex([]byte("abc"))
	const want = "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	if got != want {
		t.Fatalf("SHA256Hex = %q want %q", got, want)
	}
}

func TestToolExecutionStoresHashesNotRawPayloads(t *testing.T) {
	rawInput := []byte(`{"amount_minor":899900,"account":"SECRET_ACCOUNT"}`)
	rawResult := []byte(`{"safe_to_spend_minor":300100,"private_note":"SECRET_RESULT"}`)
	execution := NewToolExecution(1, "simulate_purchase", rawInput, rawResult, "")
	if execution.Sequence != 1 || execution.ToolName != "simulate_purchase" || !execution.Success {
		t.Fatalf("execution = %#v", execution)
	}
	if execution.InputSHA256 != SHA256Hex(rawInput) || execution.ResultSHA256 != SHA256Hex(rawResult) {
		t.Fatalf("hashes = %#v", execution)
	}
	encoded, err := json.Marshal(execution)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(encoded), "SECRET_ACCOUNT") || strings.Contains(string(encoded), "SECRET_RESULT") {
		t.Fatalf("audit leaked raw payload: %s", encoded)
	}
}

func TestFailedToolExecutionStoresStableErrorCodeWithoutRawError(t *testing.T) {
	rawInput := []byte(`{"household_id":1}`)
	execution := NewFailedToolExecution(2, "get_debt_plan", rawInput, "tool_execution_failed")
	if execution.Success || execution.ErrorCode != "tool_execution_failed" || execution.ResultSHA256 != "" {
		t.Fatalf("execution = %#v", execution)
	}
}

func TestAdviceRecordContainsOnlyMetadataAndHashes(t *testing.T) {
	reviewer := llm.ModelRoleReviewer
	rawQuestion := []byte("我可以提前还房贷吗？ SECRET_QUESTION")
	rawAdvice := []byte("建议先保留应急资金。 SECRET_ADVICE")
	record := AdviceRecord{
		CreatedAt:             time.Date(2026, 8, 16, 20, 0, 0, 0, time.UTC),
		ModelRole:             llm.ModelRolePlanner,
		ReviewerRole:          &reviewer,
		DataAsOf:              time.Date(2026, 8, 16, 19, 55, 0, 0, time.UTC),
		PromptTemplateVersion: "finance-advisor-v1",
		RequestSHA256:         SHA256Hex(rawQuestion),
		AdviceSHA256:          SHA256Hex(rawAdvice),
		QualityLevel:          "partial",
		Status:                AdviceStatusSuccess,
		Tools: []ToolExecution{
			NewToolExecution(1, "get_debt_plan", []byte(`{"household_id":1}`), []byte(`{"payoff_months":24}`), ""),
		},
	}
	if err := record.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, secret := range []string{"SECRET_QUESTION", "SECRET_ADVICE", "payoff_months", "household_id"} {
		if strings.Contains(string(encoded), secret) {
			t.Fatalf("audit leaked %q: %s", secret, encoded)
		}
	}
}

func TestAdviceRecordRejectsInvalidHashesOrStatus(t *testing.T) {
	record := AdviceRecord{
		ModelRole:             llm.ModelRoleFast,
		PromptTemplateVersion: "finance-advisor-v1",
		RequestSHA256:         "not-a-hash",
		AdviceSHA256:          SHA256Hex([]byte("advice")),
		QualityLevel:          "good",
		Status:                AdviceStatus("mystery"),
	}
	if err := record.Validate(); err == nil {
		t.Fatal("Validate accepted invalid audit metadata")
	}
}
