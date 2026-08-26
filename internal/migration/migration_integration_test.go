package migration

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/shawnwu2022/family-finance-os/internal/config"
)

func TestEmbeddedMigrationsAreIdempotentIntegration(t *testing.T) {
	host := os.Getenv("TEST_POSTGRES_HOST")
	if host == "" {
		t.Skip("TEST_POSTGRES_HOST is not set")
	}
	port, err := strconv.ParseUint(os.Getenv("TEST_POSTGRES_PORT"), 10, 16)
	if err != nil {
		t.Fatalf("TEST_POSTGRES_PORT: %v", err)
	}
	cfg := config.DatabaseConfig{
		Host: host, Port: uint16(port), Name: os.Getenv("TEST_POSTGRES_DB"), User: os.Getenv("TEST_POSTGRES_USER"),
		Password: os.Getenv("TEST_POSTGRES_PASSWORD"), SSLMode: "disable",
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := Run(ctx, cfg); err != nil {
		t.Fatalf("Run first: %v", err)
	}
	if err := Run(ctx, cfg); err != nil {
		t.Fatalf("Run second: %v", err)
	}
}
