package agentadapter

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/shawnwu2022/family-finance-os/internal/server"
)

func TestAuditedCallPreservesTimeoutWhenRequestCancelsBeforeFailureAuditCompletion(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	backend := &cancelingOverviewBackend{
		fakeBackend: &fakeBackend{err: context.DeadlineExceeded},
		cancel:      cancel,
	}
	service, err := New(backend)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	baseRecorder := &fakeAgentAuditRecorder{startID: 1}
	recorder := &activeCompletionRecorder{fakeAgentAuditRecorder: baseRecorder}
	audited, err := NewAudited(service, recorder, stepClock())
	if err != nil {
		t.Fatalf("NewAudited: %v", err)
	}

	_, err = audited.Call(ctx, Principal{Kind: "mcp", HouseholdID: 42}, testAuditMetadata(), ToolGetHouseholdOverview, json.RawMessage(`{}`))
	if !IsCode(err, CodeTimeout) {
		t.Fatalf("error=%v, want %s", err, CodeTimeout)
	}
	if baseRecorder.failureCalls != 1 || baseRecorder.failure.ErrorCode != CodeTimeout {
		t.Fatalf("failure=%#v calls=%d", baseRecorder.failure, baseRecorder.failureCalls)
	}
}

type cancelingOverviewBackend struct {
	*fakeBackend
	cancel context.CancelFunc
}

func (b *cancelingOverviewBackend) Overview(ctx context.Context, householdID int64) (server.OverviewResponse, error) {
	b.cancel()
	return b.fakeBackend.Overview(ctx, householdID)
}

type activeCompletionRecorder struct {
	*fakeAgentAuditRecorder
}

func (r *activeCompletionRecorder) CompleteFailure(ctx context.Context, id int64, failure AuditFailure) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return r.fakeAgentAuditRecorder.CompleteFailure(ctx, id, failure)
}
