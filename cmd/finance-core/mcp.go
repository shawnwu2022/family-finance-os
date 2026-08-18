package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
	"unicode"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shawnwu2022/family-finance-os/internal/agentadapter"
	"github.com/shawnwu2022/family-finance-os/internal/audit"
	"github.com/shawnwu2022/family-finance-os/internal/config"
	"github.com/shawnwu2022/family-finance-os/internal/mcpadapter"
	storesqlc "github.com/shawnwu2022/family-finance-os/internal/store/sqlc"
)

const maxMCPTokenFileBytes = 4096

func buildMCPHandler(ctx context.Context, cfg config.MCPConfig, pool *pgxpool.Pool, backend agentadapter.FinanceBackend) (http.Handler, error) {
	if !cfg.Enabled {
		return nil, fmt.Errorf("MCP is disabled")
	}
	if pool == nil {
		return nil, fmt.Errorf("MCP postgres pool is required")
	}
	if backend == nil {
		return nil, fmt.Errorf("MCP finance backend is required")
	}
	if cfg.HouseholdID <= 0 {
		return nil, fmt.Errorf("MCP household ID must be positive")
	}
	if _, err := storesqlc.New(pool).GetHousehold(ctx, cfg.HouseholdID); err != nil {
		return nil, fmt.Errorf("validate MCP household scope %d: %w", cfg.HouseholdID, err)
	}

	token, err := loadMCPToken(cfg.TokenFile)
	if err != nil {
		return nil, err
	}
	base, err := agentadapter.New(backend)
	if err != nil {
		return nil, fmt.Errorf("configure MCP agent adapter: %w", err)
	}
	audited, err := agentadapter.NewAudited(base, audit.NewAgentPostgresRecorder(pool), time.Now)
	if err != nil {
		return nil, fmt.Errorf("configure MCP agent audit: %w", err)
	}
	mcpServer, err := mcpadapter.NewServer(audited, mcpadapter.ServerOptions{
		Name:      "family-finance-os",
		Version:   "v2",
		Principal: agentadapter.Principal{Kind: "mcp", HouseholdID: cfg.HouseholdID},
	})
	if err != nil {
		return nil, fmt.Errorf("configure MCP server: %w", err)
	}
	transport, err := mcpadapter.NewHTTPHandler(mcpServer)
	if err != nil {
		return nil, fmt.Errorf("configure MCP Streamable HTTP: %w", err)
	}
	secure, err := mcpadapter.NewSecureHTTPHandler(transport, mcpadapter.SecurityOptions{
		Token:             token,
		AllowedOrigins:    cfg.AllowedOrigins,
		RequestTimeout:    cfg.RequestTimeout,
		MaxConcurrent:     cfg.MaxConcurrent,
		RequestsPerMinute: cfg.RequestsPerMinute,
		MaxBodyBytes:      cfg.MaxBodyBytes,
	})
	if err != nil {
		return nil, fmt.Errorf("configure MCP security: %w", err)
	}
	return secure, nil
}

func loadMCPToken(path string) ([]byte, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("MCP token file path is required")
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open MCP token file %q: %w", path, err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat MCP token file %q: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("MCP token file %q must be a regular file", path)
	}
	if info.Size() > maxMCPTokenFileBytes {
		return nil, fmt.Errorf("MCP token file %q exceeds %d bytes", path, maxMCPTokenFileBytes)
	}

	raw, err := io.ReadAll(io.LimitReader(file, maxMCPTokenFileBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read MCP token file %q: %w", path, err)
	}
	if len(raw) > maxMCPTokenFileBytes {
		return nil, fmt.Errorf("MCP token file %q exceeds %d bytes", path, maxMCPTokenFileBytes)
	}

	token := bytes.TrimSpace(raw)
	if len(token) == 0 {
		return nil, fmt.Errorf("MCP token file %q is empty", path)
	}
	if strings.IndexFunc(string(token), unicode.IsSpace) >= 0 {
		return nil, fmt.Errorf("MCP token file %q contains whitespace", path)
	}
	return append([]byte(nil), token...), nil
}
