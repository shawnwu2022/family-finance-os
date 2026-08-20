package agentadapter

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/shawnwu2022/family-finance-os/internal/server"
)

func TestAuditedCallFailsClosedBeforeToolWhenAuditStartFails(t *testing.T) {
	backend := &fakeBackend{overview: server.OverviewResponse{Quality: "good"}}
	service, err := New(backend)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	recorder := &fakeAgentAuditRecorder{startErr: errors.New("audit database unavailable")}
	audited, err := NewAudited(service, recorder, time.Now)
	if err != nil {
		t.Fatalf("NewAudited: %v", err)
	}

	_, err = audited.Call(context.Background(), Principal{Kind: "mcp", HouseholdID: 42}, testAuditMetadata(), ToolGetHouseholdOverview, json.RawMessage(`{}`))
	if !IsCode(err, CodeAuditUnavailable) {
		t.Fatalf("error=%v, want %s", err, CodeAuditUnavailable)
	}
	if backend.totalCalls() != 0 {
		t.Fatalf("backend executed despite missing audit attempt: calls=%d", backend.totalCalls())
	}
}

func TestAuditedCallWithholdsSuccessfulResultWhenCompletionAuditFails(t *testing.T) {
	backend := &fakeBackend{overview: server.OverviewResponse{Quality: "good"}}
	service, _ := New(backend)
	recorder := &fakeAgentAuditRecorder{startID: 35, successErr: errors.New("completion write failed")}
	audited, _ := NewAudited(service, recorder, stepClock())

	result, err := audited.Call(context.Background(), Principal{Kind: "mcp", HouseholdID: 42}, testAuditMetadata(), ToolGetHouseholdOverview, json.RawMessage(`{}`))
	if !IsCode(err, CodeAuditUnavailable) {
		t.Fatalf("error=%v, want %s", err, CodeAuditUnavailable)
	}
	if len(result.Data) != 0 || result.AuditID != "" {
		t.Fatalf("business result disclosed after audit completion failure: %#v", result)
	}
	if backend.overviewCalls != 1 || recorder.successCalls != 1 {
		t.Fatalf("backend/success calls=%d/%d", backend.overviewCalls, recorder.successCalls)
	}
}

func TestAuditedCallReturnsAuditIDOnlyAfterSuccessfulCompletion(t *testing.T) {
	backend := &fakeBackend{overview: server.OverviewResponse{Quality: "good"}}
	service, _ := New(backend)
	recorder := &fakeAgentAuditRecorder{startID: 35}
	audited, _ := NewAudited(service, recorder, stepClock())

	result, err := audited.Call(context.Background(), Principal{Kind: "mcp", HouseholdID: 42}, testAuditMetadata(), ToolGetHouseholdOverview, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if result.AuditID != "audit_z" || len(result.Data) == 0 {
		t.Fatalf("result=%#v", result)
	}
	if recorder.attempt.HouseholdID != 42 || recorder.attempt.ToolName != ToolGetHouseholdOverview || recorder.attempt.Protocol != "mcp" {
		t.Fatalf("attempt=%#v", recorder.attempt)
	}
	if !isLowerSHA256(recorder.attempt.InputSHA256) || !isLowerSHA256(recorder.success.OutputSHA256) {
		t.Fatalf("hashes=%q/%q", recorder.attempt.InputSHA256, recorder.success.OutputSHA256)
	}
	if recorder.success.DurationMS < 0 {
		t.Fatalf("duration=%d", recorder.success.DurationMS)
	}
}

func TestAuditedCallRecordsStableBaseFailureAndHidesRawError(t *testing.T) {
	sensitive := errors.New("postgres password=DO_NOT_LEAK")
	backend := &fakeBackend{err: sensitive}
	service, _ := New(backend)
	recorder := &fakeAgentAuditRecorder{startID: 1}
	audited, _ := NewAudited(service, recorder, stepClock())

	_, err := audited.Call(context.Background(), Principal{Kind: "mcp", HouseholdID: 42}, testAuditMetadata(), ToolGetHouseholdOverview, json.RawMessage(`{}`))
	if !IsCode(err, CodeDataUnavailable) {
		t.Fatalf("error=%v, want %s", err, CodeDataUnavailable)
	}
	if strings.Contains(err.Error(), "DO_NOT_LEAK") || strings.Contains(err.Error(), "password") {
		t.Fatalf("external error leaked backend text: %q", err.Error())
	}
	if recorder.failureCalls != 1 || recorder.failure.ErrorCode != CodeDataUnavailable {
		t.Fatalf("failure=%#v calls=%d", recorder.failure, recorder.failureCalls)
	}
}

func TestAuditedCallReturnsAuditUnavailableWhenFailureCompletionCannotPersist(t *testing.T) {
	backend := &fakeBackend{err: errors.New("backend failed")}
	service, _ := New(backend)
	recorder := &fakeAgentAuditRecorder{startID: 1, failureErr: errors.New("audit failure write failed")}
	audited, _ := NewAudited(service, recorder, stepClock())

	_, err := audited.Call(context.Background(), Principal{Kind: "mcp", HouseholdID: 42}, testAuditMetadata(), ToolGetHouseholdOverview, json.RawMessage(`{}`))
	if !IsCode(err, CodeAuditUnavailable) {
		t.Fatalf("error=%v, want %s", err, CodeAuditUnavailable)
	}
}

func TestAuditInputSHA256CanonicalizesValidJSONKeyOrder(t *testing.T) {
	first := auditInputSHA256(json.RawMessage(`{"goal_id":7,"monthly_contribution_minor":"20000"}`))
	second := auditInputSHA256(json.RawMessage(`{"monthly_contribution_minor":"20000","goal_id":7}`))
	if first != second || !isLowerSHA256(first) {
		t.Fatalf("canonical hashes=%q/%q", first, second)
	}
	invalid := auditInputSHA256(json.RawMessage(`{"goal_id":`))
	if invalid == first || !isLowerSHA256(invalid) {
		t.Fatalf("invalid-json hash=%q", invalid)
	}
}

func testAuditMetadata() CallMetadata {
	return CallMetadata{Protocol: "mcp", ProtocolVersion: "test-version", ClientName: "test-client", ClientVersion: "1"}
}

type fakeAgentAuditRecorder struct {
	startID    int64
	startErr   error
	successErr error
	failureErr error

	attempt      AuditAttempt
	success      AuditSuccess
	failure      AuditFailure
	successCalls int
	failureCalls int
}

func (r *fakeAgentAuditRecorder) Start(_ context.Context, attempt AuditAttempt) (int64, error) {
	r.attempt = attempt
	return r.startID, r.startErr
}

func (r *fakeAgentAuditRecorder) CompleteSuccess(_ context.Context, _ int64, success AuditSuccess) error {
	r.successCalls++
	r.success = success
	return r.successErr
}

func (r *fakeAgentAuditRecorder) CompleteFailure(_ context.Context, _ int64, failure AuditFailure) error {
	r.failureCalls++
	r.failure = failure
	return r.failureErr
}

func stepClock() func() time.Time {
	current := time.Date(2026, 8, 18, 5, 0, 0, 0, time.UTC)
	return func() time.Time {
		value := current
		current = current.Add(25 * time.Millisecond)
		return value
	}
}

func isLowerSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, r := range value {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}
