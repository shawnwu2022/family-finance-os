package mcpadapter

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shawnwu2022/family-finance-os/internal/agentadapter"
	appserver "github.com/shawnwu2022/family-finance-os/internal/server"
)

func TestStreamableHTTPToolExecutionUsesConfiguredSecurityTimeout(t *testing.T) {
	backend := &timeoutProbeBackend{deadlineObserved: make(chan bool, 1)}
	recorder := &recordingAuditRecorder{startID: 91}
	service, err := agentadapter.New(backend)
	if err != nil {
		t.Fatalf("agentadapter.New: %v", err)
	}
	audited, err := agentadapter.NewAudited(service, recorder, time.Now)
	if err != nil {
		t.Fatalf("agentadapter.NewAudited: %v", err)
	}
	server, err := NewServer(audited, ServerOptions{
		Name:      "family-finance-os",
		Version:   "v2-timeout-test",
		Principal: agentadapter.Principal{Kind: "mcp", HouseholdID: 42},
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	transport, err := NewHTTPHandler(server)
	if err != nil {
		t.Fatalf("NewHTTPHandler: %v", err)
	}
	security := testSecurityOptions()
	security.RequestTimeout = 25 * time.Millisecond
	handler, err := NewSecureHTTPHandler(transport, security)
	if err != nil {
		t.Fatalf("NewSecureHTTPHandler: %v", err)
	}

	httpServer := httptest.NewServer(handler)
	defer httpServer.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "timeout-test-client", Version: "1.0.0"}, &mcp.ClientOptions{
		Capabilities: &mcp.ClientCapabilities{},
	})
	httpClient := &http.Client{Transport: bearerRoundTripper{base: http.DefaultTransport}}
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{
		Endpoint:   httpServer.URL,
		HTTPClient: httpClient,
	}, nil)
	if err != nil {
		t.Fatalf("client Connect: %v", err)
	}
	defer session.Close()

	callCtx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	started := time.Now()
	result, err := session.CallTool(callCtx, &mcp.CallToolParams{
		Name:      string(agentadapter.ToolGetHouseholdOverview),
		Arguments: map[string]any{},
	})
	elapsed := time.Since(started)
	if err != nil {
		t.Fatalf("CallTool protocol error after %s: %v", elapsed, err)
	}
	if !result.IsError {
		t.Fatalf("CallTool result=%#v want timeout tool error", result)
	}
	assertToolErrorCode(t, result, agentadapter.CodeTimeout)
	if elapsed >= 250*time.Millisecond {
		t.Fatalf("tool timeout elapsed=%s want server-side timeout well before client deadline", elapsed)
	}

	select {
	case observed := <-backend.deadlineObserved:
		if !observed {
			t.Fatal("Finance backend tool context had no deadline")
		}
	default:
		t.Fatal("Finance backend did not record tool context deadline state")
	}
	if len(recorder.failures) != 1 || recorder.failures[0].ErrorCode != agentadapter.CodeTimeout {
		t.Fatalf("audit failures=%#v want one timeout", recorder.failures)
	}
	if len(recorder.successes) != 0 {
		t.Fatalf("audit successes=%#v want none", recorder.successes)
	}
}

type timeoutProbeBackend struct {
	stubFinanceBackend
	deadlineObserved chan bool
}

func (b *timeoutProbeBackend) Overview(ctx context.Context, _ int64) (appserver.OverviewResponse, error) {
	_, hasDeadline := ctx.Deadline()
	select {
	case b.deadlineObserved <- hasDeadline:
	default:
	}
	<-ctx.Done()
	if !errors.Is(ctx.Err(), context.DeadlineExceeded) && !errors.Is(ctx.Err(), context.Canceled) {
		return appserver.OverviewResponse{}, errors.New("tool context ended without cancellation")
	}
	return appserver.OverviewResponse{}, ctx.Err()
}

type bearerRoundTripper struct {
	base http.RoundTripper
}

func (t bearerRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	clone := request.Clone(request.Context())
	clone.Header = request.Header.Clone()
	clone.Header.Set("Authorization", "Bearer correct-horse-battery-staple")
	return t.base.RoundTrip(clone)
}
