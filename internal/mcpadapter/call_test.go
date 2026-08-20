package mcpadapter

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shawnwu2022/family-finance-os/internal/agentadapter"
	appserver "github.com/shawnwu2022/family-finance-os/internal/server"
)

func TestCallToolUsesAuditedScopedServiceAndReturnsStructuredResult(t *testing.T) {
	asOf := time.Date(2026, 8, 18, 9, 30, 0, 0, time.UTC)
	backend := &recordingFinanceBackend{
		overview: appserver.OverviewResponse{
			DataAsOf: asOf,
			Quality:  "good",
			NetWorth: appserver.MoneyDTO{Minor: 12345, Currency: "CNY"},
			Warnings: []string{"fixture-warning"},
		},
	}
	recorder := &recordingAuditRecorder{startID: 35}
	audited := newAuditedCallService(t, backend, recorder)
	server, err := NewServer(audited, ServerOptions{
		Name:      "family-finance-os",
		Version:   "v2-test",
		Principal: agentadapter.Principal{Kind: "mcp", HouseholdID: 42},
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	session := connectInMemoryClient(t, server)
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      string(agentadapter.ToolGetHouseholdOverview),
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if result.IsError {
		t.Fatalf("CallTool returned tool error: %#v", result.Content)
	}
	if backend.overviewCalls != 1 || backend.lastHouseholdID != 42 {
		t.Fatalf("overview calls=%d household=%d want 1/42", backend.overviewCalls, backend.lastHouseholdID)
	}

	if len(recorder.attempts) != 1 {
		t.Fatalf("audit attempts=%d want 1", len(recorder.attempts))
	}
	attempt := recorder.attempts[0]
	if attempt.PrincipalKind != "mcp" || attempt.HouseholdID != 42 || attempt.Protocol != "mcp" {
		t.Fatalf("audit scope/protocol=%#v", attempt)
	}
	if attempt.ProtocolVersion != session.InitializeResult().ProtocolVersion {
		t.Fatalf("audit protocol version=%q want %q", attempt.ProtocolVersion, session.InitializeResult().ProtocolVersion)
	}
	if attempt.ClientName != "mcpadapter-test" || attempt.ClientVersion != "1.0.0" {
		t.Fatalf("audit client=%q/%q", attempt.ClientName, attempt.ClientVersion)
	}
	if len(recorder.successes) != 1 || len(recorder.failures) != 0 {
		t.Fatalf("audit completions successes=%d failures=%d", len(recorder.successes), len(recorder.failures))
	}

	text := singleTextContent(t, result)
	var textValue any
	if err := json.Unmarshal([]byte(text), &textValue); err != nil {
		t.Fatalf("text content is not JSON: %v; content=%q", err, text)
	}
	if !reflect.DeepEqual(normalizeJSON(t, result.StructuredContent), normalizeJSON(t, textValue)) {
		t.Fatalf("structured content differs from text fallback: structured=%#v text=%#v", result.StructuredContent, textValue)
	}

	var wire struct {
		Data struct {
			Quality  string   `json:"quality"`
			Warnings []string `json:"warnings"`
		} `json:"data"`
		AsOf     string   `json:"as_of"`
		Quality  string   `json:"quality"`
		Warnings []string `json:"warnings"`
		AuditID  string   `json:"audit_id"`
	}
	if err := json.Unmarshal([]byte(text), &wire); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if wire.Quality != "good" || wire.Data.Quality != "good" || wire.AsOf == "" || wire.AuditID == "" {
		t.Fatalf("result metadata missing: %#v", wire)
	}
	if !reflect.DeepEqual(wire.Warnings, []string{"fixture-warning"}) || !reflect.DeepEqual(wire.Data.Warnings, []string{"fixture-warning"}) {
		t.Fatalf("warnings=%v data.warnings=%v", wire.Warnings, wire.Data.Warnings)
	}
}

func TestCallToolRejectsHouseholdInjectionWithoutRescoping(t *testing.T) {
	backend := &recordingFinanceBackend{}
	recorder := &recordingAuditRecorder{startID: 36}
	audited := newAuditedCallService(t, backend, recorder)
	server, err := NewServer(audited, ServerOptions{
		Name:      "family-finance-os",
		Version:   "v2-test",
		Principal: agentadapter.Principal{Kind: "mcp", HouseholdID: 42},
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	session := connectInMemoryClient(t, server)
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name: string(agentadapter.ToolGetHouseholdOverview),
		Arguments: map[string]any{
			"household_id": 999,
		},
	})
	if err != nil {
		t.Fatalf("CallTool protocol error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("household injection unexpectedly succeeded: %#v", result)
	}
	if backend.overviewCalls != 0 {
		t.Fatalf("backend executed after household injection: calls=%d household=%d", backend.overviewCalls, backend.lastHouseholdID)
	}
	if len(recorder.attempts) != 1 || recorder.attempts[0].HouseholdID != 42 {
		t.Fatalf("audit attempts=%#v", recorder.attempts)
	}
	if len(recorder.failures) != 1 || recorder.failures[0].ErrorCode != agentadapter.CodeInvalidArgument {
		t.Fatalf("audit failures=%#v", recorder.failures)
	}
	assertToolErrorCode(t, result, agentadapter.CodeInvalidArgument)
}

func TestCallToolDoesNotLeakBackendErrorText(t *testing.T) {
	backend := &recordingFinanceBackend{overviewErr: errors.New("postgres password=DO_NOT_LEAK")}
	recorder := &recordingAuditRecorder{startID: 37}
	audited := newAuditedCallService(t, backend, recorder)
	server, err := NewServer(audited, ServerOptions{
		Name:      "family-finance-os",
		Version:   "v2-test",
		Principal: agentadapter.Principal{Kind: "mcp", HouseholdID: 42},
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	session := connectInMemoryClient(t, server)
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      string(agentadapter.ToolGetHouseholdOverview),
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("CallTool protocol error: %v", err)
	}
	if !result.IsError {
		t.Fatalf("backend error unexpectedly succeeded: %#v", result)
	}
	assertToolErrorCode(t, result, agentadapter.CodeDataUnavailable)
	wire, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	if strings.Contains(string(wire), "DO_NOT_LEAK") || strings.Contains(string(wire), "password") {
		t.Fatalf("backend error leaked to MCP client: %s", wire)
	}
	if len(recorder.failures) != 1 || recorder.failures[0].ErrorCode != agentadapter.CodeDataUnavailable {
		t.Fatalf("audit failures=%#v", recorder.failures)
	}
}

func newAuditedCallService(t *testing.T, backend *recordingFinanceBackend, recorder *recordingAuditRecorder) *agentadapter.AuditedService {
	t.Helper()
	service, err := agentadapter.New(backend)
	if err != nil {
		t.Fatalf("agentadapter.New: %v", err)
	}
	start := time.Date(2026, 8, 18, 9, 0, 0, 0, time.UTC)
	clockCalls := 0
	now := func() time.Time {
		value := start.Add(time.Duration(clockCalls) * 25 * time.Millisecond)
		clockCalls++
		return value
	}
	audited, err := agentadapter.NewAudited(service, recorder, now)
	if err != nil {
		t.Fatalf("agentadapter.NewAudited: %v", err)
	}
	return audited
}

func singleTextContent(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	if len(result.Content) != 1 {
		t.Fatalf("content count=%d want 1: %#v", len(result.Content), result.Content)
	}
	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("content type=%T want *mcp.TextContent", result.Content[0])
	}
	return text.Text
}

func assertToolErrorCode(t *testing.T, result *mcp.CallToolResult, want agentadapter.ErrorCode) {
	t.Helper()
	text := singleTextContent(t, result)
	var payload struct {
		ErrorCode agentadapter.ErrorCode `json:"error_code"`
		Message   string                 `json:"message"`
	}
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		t.Fatalf("decode tool error: %v; content=%q", err, text)
	}
	if payload.ErrorCode != want || strings.TrimSpace(payload.Message) == "" {
		t.Fatalf("tool error=%#v want code=%q with safe message", payload, want)
	}
}

type recordingFinanceBackend struct {
	stubFinanceBackend
	overview        appserver.OverviewResponse
	overviewErr     error
	overviewCalls   int
	lastHouseholdID int64
}

func (b *recordingFinanceBackend) Overview(_ context.Context, householdID int64) (appserver.OverviewResponse, error) {
	b.overviewCalls++
	b.lastHouseholdID = householdID
	return b.overview, b.overviewErr
}

type recordingAuditRecorder struct {
	startID   int64
	attempts  []agentadapter.AuditAttempt
	successes []agentadapter.AuditSuccess
	failures  []agentadapter.AuditFailure
}

func (r *recordingAuditRecorder) Start(_ context.Context, attempt agentadapter.AuditAttempt) (int64, error) {
	r.attempts = append(r.attempts, attempt)
	return r.startID, nil
}

func (r *recordingAuditRecorder) CompleteSuccess(_ context.Context, _ int64, success agentadapter.AuditSuccess) error {
	r.successes = append(r.successes, success)
	return nil
}

func (r *recordingAuditRecorder) CompleteFailure(_ context.Context, _ int64, failure agentadapter.AuditFailure) error {
	r.failures = append(r.failures, failure)
	return nil
}
