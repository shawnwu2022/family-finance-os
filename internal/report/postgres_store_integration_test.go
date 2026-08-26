package report

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/shawnwu2022/family-finance-os/internal/config"
	"github.com/shawnwu2022/family-finance-os/internal/store"
	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

func TestPostgresStorePersistsImmutableMonthlyArtifactIntegration(t *testing.T) {
	cfg := reportIntegrationDatabaseConfig(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	pool, err := store.OpenPostgres(ctx, cfg)
	if err != nil {
		t.Fatalf("OpenPostgres: %v", err)
	}
	defer pool.Close()
	var householdID int64
	if err := pool.QueryRow(ctx, `
		INSERT INTO households (name, base_currency, timezone)
		VALUES ($1, 'CNY', 'Asia/Shanghai')
		RETURNING id
	`, "report-integration-"+strconv.FormatInt(time.Now().UnixNano(), 10)).Scan(&householdID); err != nil {
		t.Fatalf("insert household: %v", err)
	}
	monthly := MonthlyReport{
		Kind: KindMonthly, Period: "2026-07", DataAsOf: time.Date(2026, 7, 31, 16, 0, 0, 0, time.UTC),
		GeneratedAt: time.Date(2026, 8, 1, 3, 0, 0, 0, time.UTC), Quality: "good",
		Metrics: MonthlyMetrics{NetWorth: money.Money{Minor: 100_000, Currency: "CNY"}},
	}
	reports := NewPostgresStore(pool)
	first, err := reports.Save(ctx, householdID, monthly)
	if err != nil {
		t.Fatalf("Save first: %v", err)
	}
	monthly.Metrics.NetWorth.Minor = 200_000
	second, err := reports.Save(ctx, householdID, monthly)
	if err != nil {
		t.Fatalf("Save second: %v", err)
	}
	if first.ID != second.ID || first.ContentHash != second.ContentHash || second.Report.Metrics.NetWorth.Minor != 100_000 {
		t.Fatalf("immutable artifacts = %#v and %#v", first, second)
	}
	items, err := reports.List(ctx, householdID)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(items) != 1 || items[0].ID != first.ID {
		t.Fatalf("items = %#v", items)
	}
}

func reportIntegrationDatabaseConfig(t *testing.T) config.DatabaseConfig {
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
