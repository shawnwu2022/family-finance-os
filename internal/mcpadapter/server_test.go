package mcpadapter

import (
	"context"
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shawnwu2022/family-finance-os/internal/agentadapter"
	"github.com/shawnwu2022/family-finance-os/internal/report"
	appserver "github.com/shawnwu2022/family-finance-os/internal/server"
)

func TestNewServerRejectsInvalidConfiguration(t *testing.T) {
	audited := newTestAuditedService(t)
	tests := []struct {
		name    string
		service *agentadapter.AuditedService
		opts    ServerOptions
	}{
		{
			name:    "missing audited service",
			service: nil,
			opts:    ServerOptions{Name: "family-finance-os", Version: "v2-test", Principal: agentadapter.Principal{Kind: "mcp", HouseholdID: 42}},
		},
		{
			name:    "missing implementation name",
			service: audited,
			opts:    ServerOptions{Version: "v2-test", Principal: agentadapter.Principal{Kind: "mcp", HouseholdID: 42}},
		},
		{
			name:    "missing implementation version",
			service: audited,
			opts:    ServerOptions{Name: "family-finance-os", Principal: agentadapter.Principal{Kind: "mcp", HouseholdID: 42}},
		},
		{
			name:    "missing principal kind",
			service: audited,
			opts:    ServerOptions{Name: "family-finance-os", Version: "v2-test", Principal: agentadapter.Principal{HouseholdID: 42}},
		},
		{
			name:    "invalid household scope",
			service: audited,
			opts:    ServerOptions{Name: "family-finance-os", Version: "v2-test", Principal: agentadapter.Principal{Kind: "mcp", HouseholdID: 0}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewServer(tc.service, tc.opts)
			if err == nil {
				t.Fatal("NewServer accepted invalid configuration")
			}
		})
	}
}

func TestNewServerRegistersExactlyAuditedReadyTools(t *testing.T) {
	audited := newTestAuditedService(t)
	server, err := NewServer(audited, ServerOptions{
		Name:      "family-finance-os",
		Version:   "v2-test",
		Principal: agentadapter.Principal{Kind: "mcp", HouseholdID: 42},
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	session := connectInMemoryClient(t, server)
	listed, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	definitions := audited.Definitions()
	if got, want := len(listed.Tools), len(definitions); got != want || got != 10 {
		t.Fatalf("tool count=%d want=%d (READY=10)", got, want)
	}

	byName := make(map[string]agentadapter.ToolDefinition, len(definitions))
	wantNames := make([]string, 0, len(definitions))
	for _, definition := range definitions {
		name := string(definition.Name)
		byName[name] = definition
		wantNames = append(wantNames, name)
	}
	sort.Strings(wantNames)

	gotNames := make([]string, 0, len(listed.Tools))
	for _, tool := range listed.Tools {
		gotNames = append(gotNames, tool.Name)
		definition, ok := byName[tool.Name]
		if !ok {
			t.Fatalf("MCP advertised non-READY tool %q", tool.Name)
		}
		if tool.Description != definition.Description {
			t.Fatalf("tool %q description changed across adapter boundary", tool.Name)
		}
		if !reflect.DeepEqual(normalizeJSON(t, tool.InputSchema), normalizeJSON(t, definition.InputSchema)) {
			t.Fatalf("tool %q input schema differs from Agent Adapter definition", tool.Name)
		}
		schemaBytes, err := json.Marshal(tool.InputSchema)
		if err != nil {
			t.Fatalf("marshal tool %q schema: %v", tool.Name, err)
		}
		if strings.Contains(strings.ToLower(string(schemaBytes)), "household_id") {
			t.Fatalf("tool %q exposes household_id in MCP schema: %s", tool.Name, schemaBytes)
		}

		annotations := tool.Annotations
		if annotations == nil || !annotations.ReadOnlyHint || annotations.DestructiveHint == nil || *annotations.DestructiveHint || !annotations.IdempotentHint || annotations.OpenWorldHint == nil || *annotations.OpenWorldHint {
			t.Fatalf("tool %q annotations are not read-only/non-destructive/idempotent/closed-world: %#v", tool.Name, annotations)
		}
	}
	sort.Strings(gotNames)
	if !reflect.DeepEqual(gotNames, wantNames) {
		t.Fatalf("tool names=%v want=%v", gotNames, wantNames)
	}

	init := session.InitializeResult()
	if init == nil || init.Capabilities == nil || init.Capabilities.Tools == nil {
		t.Fatalf("initialize capabilities missing tools: %#v", init)
	}
	if init.Capabilities.Prompts != nil || init.Capabilities.Resources != nil {
		t.Fatalf("MCP server advertised prompts/resources unexpectedly: %#v", init.Capabilities)
	}
}

func connectInMemoryClient(t *testing.T, server *mcp.Server) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server Connect: %v", err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "mcpadapter-test", Version: "1.0.0"}, &mcp.ClientOptions{
		Capabilities: &mcp.ClientCapabilities{},
	})
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		_ = serverSession.Close()
		t.Fatalf("client Connect: %v", err)
	}
	t.Cleanup(func() {
		_ = clientSession.Close()
		_ = serverSession.Wait()
	})
	return clientSession
}

func normalizeJSON(t *testing.T, value any) any {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal JSON value: %v", err)
	}
	var normalized any
	if err := json.Unmarshal(encoded, &normalized); err != nil {
		t.Fatalf("unmarshal JSON value: %v", err)
	}
	return normalized
}

func newTestAuditedService(t *testing.T) *agentadapter.AuditedService {
	t.Helper()
	service, err := agentadapter.New(&stubFinanceBackend{})
	if err != nil {
		t.Fatalf("agentadapter.New: %v", err)
	}
	audited, err := agentadapter.NewAudited(service, &stubAuditRecorder{}, time.Now)
	if err != nil {
		t.Fatalf("agentadapter.NewAudited: %v", err)
	}
	return audited
}

type stubAuditRecorder struct{}

func (*stubAuditRecorder) Start(context.Context, agentadapter.AuditAttempt) (int64, error) {
	return 1, nil
}

func (*stubAuditRecorder) CompleteSuccess(context.Context, int64, agentadapter.AuditSuccess) error {
	return nil
}

func (*stubAuditRecorder) CompleteFailure(context.Context, int64, agentadapter.AuditFailure) error {
	return nil
}

type stubFinanceBackend struct{}

func (*stubFinanceBackend) Overview(context.Context, int64) (appserver.OverviewResponse, error) {
	return appserver.OverviewResponse{}, nil
}

func (*stubFinanceBackend) Cashflow(context.Context, int64, string) (appserver.CashflowResponse, error) {
	return appserver.CashflowResponse{}, nil
}

func (*stubFinanceBackend) Budget(context.Context, int64, string) (appserver.BudgetResponse, error) {
	return appserver.BudgetResponse{}, nil
}

func (*stubFinanceBackend) Debts(context.Context, int64) (appserver.DebtsResponse, error) {
	return appserver.DebtsResponse{}, nil
}

func (*stubFinanceBackend) Goals(context.Context, int64) (appserver.GoalsResponse, error) {
	return appserver.GoalsResponse{}, nil
}

func (*stubFinanceBackend) SafeToSpend(context.Context, int64) (appserver.SafeToSpendResponse, error) {
	return appserver.SafeToSpendResponse{}, nil
}

func (*stubFinanceBackend) SimulateGoal(context.Context, int64, int64, int64) (appserver.GoalSimulationResponse, error) {
	return appserver.GoalSimulationResponse{}, nil
}

func (*stubFinanceBackend) Scenario(context.Context, appserver.ScenarioRequest) (appserver.ScenarioResponse, error) {
	return appserver.ScenarioResponse{}, nil
}

func (*stubFinanceBackend) MonthlyReport(context.Context, int64, string) (report.MonthlyReport, error) {
	return report.MonthlyReport{}, nil
}
