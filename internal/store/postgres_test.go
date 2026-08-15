package store

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/shawnwu2022/family-finance-os/internal/config"
)

func TestOpenPostgresIntegration(t *testing.T) {
	host := os.Getenv("TEST_POSTGRES_HOST")
	if host == "" {
		t.Skip("TEST_POSTGRES_HOST is not set")
	}

	portRaw := os.Getenv("TEST_POSTGRES_PORT")
	if portRaw == "" {
		portRaw = "5432"
	}
	port64, err := strconv.ParseUint(portRaw, 10, 16)
	if err != nil {
		t.Fatalf("parse TEST_POSTGRES_PORT: %v", err)
	}

	cfg := config.DatabaseConfig{
		Host:     host,
		Port:     uint16(port64),
		Name:     os.Getenv("TEST_POSTGRES_DB"),
		User:     os.Getenv("TEST_POSTGRES_USER"),
		Password: os.Getenv("TEST_POSTGRES_PASSWORD"),
		SSLMode:  "disable",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := OpenPostgres(ctx, cfg)
	if err != nil {
		t.Fatalf("OpenPostgres: %v", err)
	}
	defer pool.Close()

	var one int
	if err := pool.QueryRow(ctx, "select 1").Scan(&one); err != nil {
		t.Fatalf("select 1: %v", err)
	}
	if one != 1 {
		t.Fatalf("select 1 = %d, want 1", one)
	}
}
