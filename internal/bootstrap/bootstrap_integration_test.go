package bootstrap

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/shawnwu2022/family-finance-os/internal/config"
	"github.com/shawnwu2022/family-finance-os/internal/store"
)

func TestBootstrapIsIdempotentIntegration(t *testing.T) {
	cfg := integrationDatabaseConfig(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	input := Input{
		Name:     "bootstrap-integration-" + strconv.FormatInt(time.Now().UnixNano(), 10),
		Currency: "CNY", Timezone: "Asia/Shanghai", Period: "2026-08", LiquidityFloorMinor: 500_000,
	}
	first, err := Run(ctx, cfg, input)
	if err != nil {
		t.Fatalf("Run first: %v", err)
	}
	second, err := Run(ctx, cfg, input)
	if err != nil {
		t.Fatalf("Run second: %v", err)
	}
	if first != second || first.HouseholdID <= 0 || first.BudgetPlanID <= 0 {
		t.Fatalf("bootstrap results = %#v and %#v", first, second)
	}

	changed := input
	changed.LiquidityFloorMinor = 999_999
	if _, err := Run(ctx, cfg, changed); err != nil {
		t.Fatalf("Run with changed bootstrap default: %v", err)
	}
	pool, err := store.OpenPostgres(ctx, cfg)
	if err != nil {
		t.Fatalf("OpenPostgres: %v", err)
	}
	defer pool.Close()
	var liquidityFloorMinor int64
	if err := pool.QueryRow(ctx, `
		SELECT liquidity_floor_minor
		FROM household_policies
		WHERE household_id = $1
	`, first.HouseholdID).Scan(&liquidityFloorMinor); err != nil {
		t.Fatalf("read household policy: %v", err)
	}
	if liquidityFloorMinor != input.LiquidityFloorMinor {
		t.Fatalf("liquidity floor = %d, want preserved value %d", liquidityFloorMinor, input.LiquidityFloorMinor)
	}
}

func integrationDatabaseConfig(t *testing.T) config.DatabaseConfig {
	t.Helper()
	host := os.Getenv("TEST_POSTGRES_HOST")
	if host == "" {
		t.Skip("TEST_POSTGRES_HOST is not set")
	}
	port, err := strconv.ParseUint(os.Getenv("TEST_POSTGRES_PORT"), 10, 16)
	if err != nil {
		t.Fatalf("TEST_POSTGRES_PORT: %v", err)
	}
	return config.DatabaseConfig{
		Host: host, Port: uint16(port), Name: os.Getenv("TEST_POSTGRES_DB"), User: os.Getenv("TEST_POSTGRES_USER"),
		Password: os.Getenv("TEST_POSTGRES_PASSWORD"), SSLMode: "disable",
	}
}
