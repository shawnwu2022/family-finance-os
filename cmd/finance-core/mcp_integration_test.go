package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shawnwu2022/family-finance-os/internal/agentadapter"
	"github.com/shawnwu2022/family-finance-os/internal/config"
	"github.com/shawnwu2022/family-finance-os/internal/report"
	appserver "github.com/shawnwu2022/family-finance-os/internal/server"
	"github.com/shawnwu2022/family-finance-os/internal/store"
	storesqlc "github.com/shawnwu2022/family-finance-os/internal/store/sqlc"
)

func TestBuildMCPHandlerIntegration(t *testing.T) {
	pool := openMCPIntegrationPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	queries := storesqlc.New(pool)
	household, err := queries.CreateHousehold(ctx, storesqlc.CreateHouseholdParams{
		Name:         "mcp-startup-test",
		BaseCurrency: "CNY",
		Timezone:     "Asia/Shanghai",
	})
	if err != nil {
		t.Fatalf("CreateHousehold: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM agent_tool_audits WHERE household_id = $1`, household.ID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM households WHERE id = $1`, household.ID)
	})

	tokenPath := filepath.Join(t.TempDir(), "mcp-token")
	if err := os.WriteFile(tokenPath, []byte("integration-mcp-token\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg := config.MCPConfig{
		Enabled:           true,
		TokenFile:         tokenPath,
		HouseholdID:       household.ID,
		RequestTimeout:    2 * time.Second,
		MaxConcurrent:     4,
		RequestsPerMinute: 60,
		MaxBodyBytes:      262144,
	}
	backend := &mcpIntegrationBackend{asOf: time.Date(2026, 8, 18, 10, 30, 0, 0, time.UTC)}
	handler, err := buildMCPHandler(ctx, cfg, pool, backend)
	if err != nil {
		t.Fatalf("buildMCPHandler: %v", err)
	}

	httpServer := httptest.NewServer(handler)
	defer httpServer.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "mcp-integration-client", Version: "1.0.0"}, &mcp.ClientOptions{
		Capabilities: &mcp.ClientCapabilities{},
	})
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint: httpServer.URL,
		HTTPClient: &http.Client{Transport: bearerRoundTripper{
			token: "integration-mcp-token",
			base:  http.DefaultTransport,
		}},
	}, nil)
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	defer session.Close()

	listed, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(listed.Tools) != 9 {
		t.Fatalf("tool count=%d want 9", len(listed.Tools))
	}
	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      string(agentadapter.ToolGetHouseholdOverview),
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if result.IsError || result.StructuredContent == nil {
		t.Fatalf("CallTool result=%#v", result)
	}
	if backend.lastHouseholdID != household.ID || backend.overviewCalls != 1 {
		t.Fatalf("backend scope calls=%d household=%d want 1/%d", backend.overviewCalls, backend.lastHouseholdID, household.ID)
	}

	var auditCount int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM agent_tool_audits
		WHERE household_id = $1 AND tool_name = $2 AND status = 'success'
	`, household.ID, string(agentadapter.ToolGetHouseholdOverview)).Scan(&auditCount); err != nil {
		t.Fatalf("query agent audit: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("successful agent audit count=%d want 1", auditCount)
	}
}

func TestBuildMCPHandlerRejectsMissingHouseholdIntegration(t *testing.T) {
	pool := openMCPIntegrationPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tokenPath := filepath.Join(t.TempDir(), "mcp-token")
	if err := os.WriteFile(tokenPath, []byte("integration-mcp-token"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	_, err := buildMCPHandler(ctx, config.MCPConfig{
		Enabled:           true,
		TokenFile:         tokenPath,
		HouseholdID:       9_000_000_000,
		RequestTimeout:    2 * time.Second,
		MaxConcurrent:     4,
		RequestsPerMinute: 60,
		MaxBodyBytes:      262144,
	}, pool, &mcpIntegrationBackend{})
	if err == nil {
		t.Fatal("buildMCPHandler accepted nonexistent household")
	}
}

type bearerRoundTripper struct {
	token string
	base  http.RoundTripper
}

func (r bearerRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	clone := request.Clone(request.Context())
	clone.Header = request.Header.Clone()
	clone.Header.Set("Authorization", "Bearer "+r.token)
	return r.base.RoundTrip(clone)
}

type mcpIntegrationBackend struct {
	asOf            time.Time
	overviewCalls   int
	lastHouseholdID int64
}

func (b *mcpIntegrationBackend) Overview(_ context.Context, householdID int64) (appserver.OverviewResponse, error) {
	b.overviewCalls++
	b.lastHouseholdID = householdID
	return appserver.OverviewResponse{
		DataAsOf: b.asOf,
		Quality:  "good",
		NetWorth: appserver.MoneyDTO{Minor: 12345, Currency: "CNY"},
	}, nil
}

func (*mcpIntegrationBackend) Cashflow(context.Context, int64, string) (appserver.CashflowResponse, error) {
	return appserver.CashflowResponse{}, nil
}

func (*mcpIntegrationBackend) Budget(context.Context, int64, string) (appserver.BudgetResponse, error) {
	return appserver.BudgetResponse{}, nil
}

func (*mcpIntegrationBackend) Debts(context.Context, int64) (appserver.DebtsResponse, error) {
	return appserver.DebtsResponse{}, nil
}

func (*mcpIntegrationBackend) Goals(context.Context, int64) (appserver.GoalsResponse, error) {
	return appserver.GoalsResponse{}, nil
}

func (*mcpIntegrationBackend) SafeToSpend(context.Context, int64) (appserver.SafeToSpendResponse, error) {
	return appserver.SafeToSpendResponse{}, nil
}

func (*mcpIntegrationBackend) SimulateGoal(context.Context, int64, int64, int64) (appserver.GoalSimulationResponse, error) {
	return appserver.GoalSimulationResponse{}, nil
}

func (*mcpIntegrationBackend) Scenario(context.Context, appserver.ScenarioRequest) (appserver.ScenarioResponse, error) {
	return appserver.ScenarioResponse{}, nil
}

func (*mcpIntegrationBackend) MonthlyReport(context.Context, int64, string) (report.MonthlyReport, error) {
	return report.MonthlyReport{}, nil
}

func openMCPIntegrationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	host := os.Getenv("TEST_POSTGRES_HOST")
	if host == "" {
		t.Skip("TEST_POSTGRES_HOST is not set")
	}
	portRaw := os.Getenv("TEST_POSTGRES_PORT")
	if portRaw == "" {
		portRaw = "5432"
	}
	port, err := strconv.ParseUint(portRaw, 10, 16)
	if err != nil {
		t.Fatalf("parse TEST_POSTGRES_PORT: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pool, err := store.OpenPostgres(ctx, config.DatabaseConfig{
		Host: host, Port: uint16(port), Name: os.Getenv("TEST_POSTGRES_DB"),
		User: os.Getenv("TEST_POSTGRES_USER"), Password: os.Getenv("TEST_POSTGRES_PASSWORD"), SSLMode: "disable",
	})
	if err != nil {
		t.Fatalf("OpenPostgres: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}
