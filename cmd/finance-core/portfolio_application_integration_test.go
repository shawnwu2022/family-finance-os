package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shawnwu2022/family-finance-os/internal/agentadapter"
	financeauth "github.com/shawnwu2022/family-finance-os/internal/auth"
	"github.com/shawnwu2022/family-finance-os/internal/config"
	"github.com/shawnwu2022/family-finance-os/internal/server"
	storesqlc "github.com/shawnwu2022/family-finance-os/internal/store/sqlc"
)

func TestBuildApplicationHandlerWiresPortfolioSnapshotsIntegration(t *testing.T) {
	pool := openMCPIntegrationPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	household, err := storesqlc.New(pool).CreateHousehold(ctx, storesqlc.CreateHouseholdParams{
		Name:         "portfolio-application-test",
		BaseCurrency: "CNY",
		Timezone:     "Asia/Shanghai",
	})
	if err != nil {
		t.Fatalf("CreateHousehold: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO household_policies (household_id, liquidity_floor_minor, currency)
		VALUES ($1, 0, 'CNY')
	`, household.ID); err != nil {
		t.Fatalf("create household policy: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM agent_tool_audits WHERE household_id = $1`, household.ID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM households WHERE id = $1`, household.ID)
	})

	ledgerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("ledger Authorization=%q", got)
		}
		if got := r.Header.Get("X-Timezone-Name"); got != "Asia/Shanghai" {
			t.Errorf("ledger timezone=%q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/accounts/list.json":
			_, _ = w.Write([]byte(`{"success":true,"result":[]}`))
		case "/api/v1/transactions/list.json":
			_, _ = w.Write([]byte(`{"success":true,"result":{"items":[],"nextTimeSequenceId":0,"totalCount":0}}`))
		default:
			t.Errorf("unexpected ledger path %q", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ledgerServer.Close()

	const mcpToken = "portfolio-mcp-token-high-entropy-fixture-2026"
	tokenPath := filepath.Join(t.TempDir(), "mcp-token")
	if err := os.WriteFile(tokenPath, []byte(mcpToken), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	cfg := applicationMCPTestConfig(t)
	cfg.Ledger.BaseURL = ledgerServer.URL + "/api/v1"
	cfg.MCP = config.MCPConfig{
		Enabled:           true,
		TokenFile:         tokenPath,
		HouseholdID:       household.ID,
		RequestTimeout:    3 * time.Second,
		MaxConcurrent:     4,
		RequestsPerMinute: 60,
		MaxBodyBytes:      262144,
	}

	handler, cleanup, err := buildApplicationHandler(ctx, cfg)
	if err != nil {
		t.Fatalf("buildApplicationHandler: %v", err)
	}
	defer cleanup()

	sessionToken, csrfToken := createPortfolioBrowserSession(t, ctx, pool, household.ID)

	assetURL := fmt.Sprintf("/api/v1/portfolio/assets/property:home?household_id=%d", household.ID)
	body := `{"name":"Home","asset_class":"property","value_minor":"100000","currency":"CNY","source_currency":"CNY","valuation_as_of":"2026-08-18T12:00:00Z","source_kind":"manual"}`
	put := httptest.NewRecorder()
	putRequest := httptest.NewRequest(http.MethodPut, assetURL, strings.NewReader(body))
	putRequest.AddCookie(&http.Cookie{Name: server.SessionCookieName, Value: sessionToken})
	putRequest.Header.Set("X-CSRF-Token", csrfToken)
	handler.ServeHTTP(put, putRequest)
	if put.Code != http.StatusOK {
		t.Fatalf("portfolio PUT status=%d body=%s", put.Code, put.Body.String())
	}

	get := httptest.NewRecorder()
	getRequest := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/portfolio/assets?household_id=%d", household.ID), nil)
	getRequest.AddCookie(&http.Cookie{Name: server.SessionCookieName, Value: sessionToken})
	handler.ServeHTTP(get, getRequest)
	if get.Code != http.StatusOK {
		t.Fatalf("portfolio GET status=%d body=%s", get.Code, get.Body.String())
	}
	if !bytes.Contains(get.Body.Bytes(), []byte(`"asset_ref":"property:home"`)) || !bytes.Contains(get.Body.Bytes(), []byte(`"value_minor":"100000"`)) {
		t.Fatalf("portfolio GET body=%s", get.Body.String())
	}

	httpServer := httptest.NewServer(handler)
	defer httpServer.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "portfolio-application-client", Version: "1.0.0"}, &mcp.ClientOptions{
		Capabilities: &mcp.ClientCapabilities{},
	})
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint: httpServer.URL + "/mcp",
		HTTPClient: &http.Client{Transport: bearerRoundTripper{
			token: mcpToken,
			base:  http.DefaultTransport,
		}},
	}, nil)
	if err != nil {
		t.Fatalf("connect MCP: %v", err)
	}
	defer session.Close()

	listed, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(listed.Tools) != 12 {
		t.Fatalf("tool count=%d want 12", len(listed.Tools))
	}
	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      string(agentadapter.ToolGetAssetAllocation),
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("CallTool get_asset_allocation: %v", err)
	}
	if result.IsError || result.StructuredContent == nil {
		t.Fatalf("CallTool result=%#v", result)
	}
	encoded, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("marshal structured allocation: %v", err)
	}
	if !bytes.Contains(encoded, []byte(`"class":"property"`)) || !bytes.Contains(encoded, []byte(`"minor":"100000"`)) {
		t.Fatalf("allocation=%s", encoded)
	}
}

func createPortfolioBrowserSession(t *testing.T, ctx context.Context, pool *pgxpool.Pool, householdID int64) (string, string) {
	t.Helper()
	store := financeauth.NewPostgresStore(pool)
	username := fmt.Sprintf("portfolio-browser-%d", householdID)
	user, _, err := store.CreateOrGetAdminUser(ctx, financeauth.CreateAdminUserParams{
		Username:           username,
		NormalizedUsername: username,
		PasswordHash:       "test-only-not-used-for-session",
		HouseholdID:        householdID,
	})
	if err != nil {
		t.Fatalf("create browser auth user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM auth_sessions WHERE user_id = $1`, user.ID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM auth_users WHERE id = $1`, user.ID)
	})

	secretBox, err := financeauth.NewSecretBox([]byte(applicationTestAuthKey))
	if err != nil {
		t.Fatalf("NewSecretBox: %v", err)
	}
	const csrfToken = "portfolio-browser-csrf-token"
	csrfCiphertext, err := secretBox.Seal([]byte(csrfToken))
	if err != nil {
		t.Fatalf("encrypt CSRF token: %v", err)
	}
	sessionToken := fmt.Sprintf("portfolio-browser-session-%d", householdID)
	sessionHash := financeauth.HashOpaqueToken(sessionToken)
	csrfHash := financeauth.HashOpaqueToken(csrfToken)
	now := time.Now().UTC()
	if err := store.CreateSession(ctx, financeauth.SessionRecord{
		TokenHash:           sessionHash[:],
		UserID:              user.ID,
		CSRFTokenHash:       csrfHash[:],
		CSRFTokenCiphertext: csrfCiphertext,
		CreatedAt:           now,
		LastSeenAt:          now,
		ExpiresAt:           now.Add(time.Hour),
	}); err != nil {
		t.Fatalf("create browser auth session: %v", err)
	}
	return sessionToken, csrfToken
}
