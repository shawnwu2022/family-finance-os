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

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shawnwu2022/family-finance-os/internal/config"
	storesqlc "github.com/shawnwu2022/family-finance-os/internal/store/sqlc"
)

const applicationTestAuthKey = "0123456789abcdef0123456789abcdef"

func TestBuildApplicationHandlerMountsMCPOnlyWhenEnabledIntegration(t *testing.T) {
	pool := openMCPIntegrationPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	household, err := storesqlc.New(pool).CreateHousehold(ctx, storesqlc.CreateHouseholdParams{
		Name:         "application-mcp-test",
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

	const token = "application-mcp-token-high-entropy-fixture-2026"
	tokenPath := filepath.Join(t.TempDir(), "mcp-token")
	if err := os.WriteFile(tokenPath, []byte(token), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg := applicationMCPTestConfig(t)
	cfg.MCP = config.MCPConfig{
		Enabled:           true,
		TokenFile:         tokenPath,
		HouseholdID:       household.ID,
		RequestTimeout:    2 * time.Second,
		MaxConcurrent:     4,
		RequestsPerMinute: 60,
		MaxBodyBytes:      262144,
	}
	handler, cleanup, err := buildApplicationHandler(ctx, cfg)
	if err != nil {
		t.Fatalf("buildApplicationHandler: %v", err)
	}
	if cleanup == nil {
		t.Fatal("cleanup is nil")
	}
	defer cleanup()

	httpServer := httptest.NewServer(handler)
	defer httpServer.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "application-mcp-client", Version: "1.0.0"}, &mcp.ClientOptions{
		Capabilities: &mcp.ClientCapabilities{},
	})
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint: httpServer.URL + "/mcp",
		HTTPClient: &http.Client{Transport: bearerRoundTripper{
			token: token,
			base:  http.DefaultTransport,
		}},
	}, nil)
	if err != nil {
		t.Fatalf("connect enabled MCP endpoint: %v", err)
	}
	defer session.Close()
	listed, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(listed.Tools) != 12 {
		t.Fatalf("tool count=%d want 12", len(listed.Tools))
	}
}

func applicationMCPTestConfig(t *testing.T) config.Config {
	t.Helper()
	portRaw := os.Getenv("TEST_POSTGRES_PORT")
	if portRaw == "" {
		portRaw = "5432"
	}
	port, err := strconv.ParseUint(portRaw, 10, 16)
	if err != nil {
		t.Fatalf("parse TEST_POSTGRES_PORT: %v", err)
	}
	authKeyPath := filepath.Join(t.TempDir(), "finance-auth-key")
	if err := os.WriteFile(authKeyPath, []byte(applicationTestAuthKey), 0o600); err != nil {
		t.Fatalf("write auth key: %v", err)
	}
	return config.Config{
		ListenAddr: ":8000",
		Timezone:   "Asia/Shanghai",
		Database: config.DatabaseConfig{
			Host:     os.Getenv("TEST_POSTGRES_HOST"),
			Port:     uint16(port),
			Name:     os.Getenv("TEST_POSTGRES_DB"),
			User:     os.Getenv("TEST_POSTGRES_USER"),
			Password: os.Getenv("TEST_POSTGRES_PASSWORD"),
			SSLMode:  "disable",
		},
		Ledger: config.LedgerConfig{
			BaseURL:  "http://ezbookkeeping.invalid:8080",
			APIToken: "test-token",
		},
		Auth: config.AuthConfig{KeyFile: authKeyPath},
	}
}
