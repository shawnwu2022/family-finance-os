package advisor

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/shawnwu2022/family-finance-os/internal/analytics"
	"github.com/shawnwu2022/family-finance-os/internal/audit"
	"github.com/shawnwu2022/family-finance-os/internal/llm"
	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

type scriptedProvider struct {
	responses []llm.Response
	errors    map[int]error
	requests  []llm.Request
}

func (p *scriptedProvider) Respond(_ context.Context, request llm.Request) (llm.Response, error) {
	index := len(p.requests)
	p.requests = append(p.requests, request)
	if err := p.errors[index]; err != nil {
		return llm.Response{}, err
	}
	if index >= len(p.responses) {
		return llm.Response{}, errors.New("unexpected provider call")
	}
	return p.responses[index], nil
}

func (p *scriptedProvider) Stream(context.Context, llm.Request, llm.StreamHandler) error {
	return errors.New("Stream must not be called by V1 Advisor Service")
}

type memoryRecorder struct {
	records []audit.AdviceRecord
	err     error
}

func (r *memoryRecorder) Record(_ context.Context, record audit.AdviceRecord) (int64, error) {
	if r.err != nil {
		return 0, r.err
	}
	r.records = append(r.records, record)
	return int64(len(r.records)), nil
}

type overviewArgs struct {
	HouseholdID int64 `json:"household_id"`
}

type overviewResult struct {
	SafeToSpendMinor int64  `json:"safe_to_spend_minor"`
	Currency         string `json:"currency"`
}

func TestServiceBlocksWhenRequiredToolIsNotCalled(t *testing.T) {
	provider := &scriptedProvider{responses: []llm.Response{{ID: "resp_1", Text: "我直接猜一个数字"}}}
	recorder := &memoryRecorder{}
	service := newAdvisorTestService(t, provider, recorder, false)

	result, err := service.Advise(context.Background(), AdviceRequest{
		Question:    "这个月还能花多少？",
		Role:        llm.ModelRoleFast,
		DataQuality: goodQuality(),
		RequireTool: true,
	})
	if err != nil {
		t.Fatalf("Advise: %v", err)
	}
	if !result.Blocked || result.BlockReason != BlockReasonRequiredToolNotCalled {
		t.Fatalf("result = %#v", result)
	}
	if len(provider.requests) != 1 {
		t.Fatalf("provider calls = %d, want 1", len(provider.requests))
	}
	if len(recorder.records) != 1 || recorder.records[0].Status != audit.AdviceStatusBlocked {
		t.Fatalf("audit records = %#v", recorder.records)
	}
}

func TestServiceBlocksOnToolFailureWithoutExplanationCall(t *testing.T) {
	provider := &scriptedProvider{responses: []llm.Response{{
		ID: "resp_1",
		ToolCalls: []llm.ToolCall{{ID: "call_1", Name: string(ToolNameGetOverview), Arguments: json.RawMessage(`{"household_id":1}`)}},
	}}}
	recorder := &memoryRecorder{}
	service := newAdvisorTestService(t, provider, recorder, true)

	result, err := service.Advise(context.Background(), AdviceRequest{
		Question:    "我的财务情况怎么样？",
		Role:        llm.ModelRoleFast,
		DataQuality: goodQuality(),
		RequireTool: true,
	})
	if err != nil {
		t.Fatalf("Advise: %v", err)
	}
	if !result.Blocked || result.BlockReason != BlockReasonToolExecutionFailed {
		t.Fatalf("result = %#v", result)
	}
	if len(provider.requests) != 1 {
		t.Fatalf("provider calls = %d, want no explanation call", len(provider.requests))
	}
	if len(recorder.records) != 1 || len(recorder.records[0].Tools) != 1 {
		t.Fatalf("audit records = %#v", recorder.records)
	}
	execution := recorder.records[0].Tools[0]
	if execution.Success || execution.ErrorCode != "tool_execution_failed" || execution.ResultSHA256 != "" {
		t.Fatalf("tool execution audit = %#v", execution)
	}
	encoded, _ := json.Marshal(recorder.records[0])
	if strings.Contains(string(encoded), "SECRET_TOOL_ERROR") {
		t.Fatalf("raw tool error leaked into audit: %s", encoded)
	}
}

func TestServiceExecutesOneToolRoundThenExplanationAndPrefixesQualityNotice(t *testing.T) {
	provider := &scriptedProvider{responses: []llm.Response{
		{ID: "resp_1", ToolCalls: []llm.ToolCall{{ID: "call_1", Name: string(ToolNameGetOverview), Arguments: json.RawMessage(`{"household_id":1}`)}}},
		{ID: "resp_2", Text: "本月安全可消费约 3500 元。"},
	}}
	recorder := &memoryRecorder{}
	service := newAdvisorTestService(t, provider, recorder, false)
	quality := goodQuality()
	quality.Level = analytics.QualityPartial
	quality.UnknownAmount = money.Money{Minor: 1_200, Currency: "CNY"}

	result, err := service.Advise(context.Background(), AdviceRequest{
		Question:    "这个月还能花多少？",
		Role:        llm.ModelRoleFast,
		DataQuality: quality,
		RequireTool: true,
	})
	if err != nil {
		t.Fatalf("Advise: %v", err)
	}
	if result.Blocked || !strings.Contains(result.Text, "财务数据不完整") || !strings.Contains(result.Text, "3500") {
		t.Fatalf("result = %#v", result)
	}
	if len(provider.requests) != 2 {
		t.Fatalf("provider calls = %d, want 2", len(provider.requests))
	}
	if len(provider.requests[0].Tools) == 0 || len(provider.requests[1].Tools) != 0 {
		t.Fatalf("tool exposure first/second = %d/%d", len(provider.requests[0].Tools), len(provider.requests[1].Tools))
	}
	if len(result.ToolResults) != 1 || result.ToolResults[0].Name != ToolNameGetOverview {
		t.Fatalf("tool results = %#v", result.ToolResults)
	}
	if len(recorder.records) != 1 || recorder.records[0].Status != audit.AdviceStatusSuccess {
		t.Fatalf("audit = %#v", recorder.records)
	}
	if err := recorder.records[0].Validate(); err != nil {
		t.Fatalf("audit Validate: %v", err)
	}
}

func TestServiceMajorAdviceAddsSingleReviewerPassWithNoTools(t *testing.T) {
	provider := &scriptedProvider{responses: []llm.Response{
		{ID: "resp_1", ToolCalls: []llm.ToolCall{{ID: "call_1", Name: string(ToolNameGetOverview), Arguments: json.RawMessage(`{"household_id":1}`)}}},
		{ID: "resp_2", Text: "候选建议：优先保留应急资金。"},
		{ID: "resp_3", Text: "审查通过，但应明确数据假设。"},
	}}
	recorder := &memoryRecorder{}
	service := newAdvisorTestService(t, provider, recorder, false)

	result, err := service.Advise(context.Background(), AdviceRequest{
		Question:      "是否提前还房贷？",
		Role:          llm.ModelRolePlanner,
		DataQuality:   goodQuality(),
		RequireTool:   true,
		RequireReview: true,
	})
	if err != nil {
		t.Fatalf("Advise: %v", err)
	}
	if result.Blocked || !result.Reviewed || !strings.Contains(result.Review, "审查通过") {
		t.Fatalf("result = %#v", result)
	}
	if len(provider.requests) != 3 {
		t.Fatalf("provider calls = %d, want 3", len(provider.requests))
	}
	if provider.requests[2].Role != llm.ModelRoleReviewer || len(provider.requests[2].Tools) != 0 {
		t.Fatalf("review request = %#v", provider.requests[2])
	}
	if len(recorder.records) != 1 || recorder.records[0].ReviewerRole == nil || *recorder.records[0].ReviewerRole != llm.ModelRoleReviewer {
		t.Fatalf("audit reviewer = %#v", recorder.records)
	}
}

func newAdvisorTestService(t *testing.T, provider llm.Provider, recorder *memoryRecorder, failTool bool) *Service {
	t.Helper()
	tool := NewTypedTool[overviewArgs, overviewResult](
		ToolNameGetOverview,
		"read household overview",
		json.RawMessage(`{"type":"object","properties":{"household_id":{"type":"integer"}},"required":["household_id"],"additionalProperties":false}`),
		func(_ context.Context, input overviewArgs) (overviewResult, error) {
			if failTool {
				return overviewResult{}, errors.New("SECRET_TOOL_ERROR")
			}
			return overviewResult{SafeToSpendMinor: 350_000, Currency: "CNY"}, nil
		},
	)
	registry, err := NewRegistry(tool)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	service, err := NewService(provider, registry, recorder, DefaultPolicy())
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	return service
}

func goodQuality() analytics.DataQuality {
	return analytics.DataQuality{
		AsOf:           time.Date(2026, 8, 16, 20, 0, 0, 0, time.UTC),
		LedgerSyncedAt: time.Date(2026, 8, 16, 19, 59, 0, 0, time.UTC),
		UnknownAmount:  money.Money{Currency: "CNY"},
		Level:          analytics.QualityGood,
	}
}
