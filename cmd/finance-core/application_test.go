package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/shawnwu2022/family-finance-os/internal/config"
)

func TestBuildApplicationHandlerWithoutLLMIntegration(t *testing.T) {
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

	cfg := config.Config{
		ListenAddr: ":8000",
		Timezone:   "Asia/Shanghai",
		Database: config.DatabaseConfig{
			Host: host,
			Port: uint16(port),
			Name: os.Getenv("TEST_POSTGRES_DB"),
			User: os.Getenv("TEST_POSTGRES_USER"),
			Password: os.Getenv("TEST_POSTGRES_PASSWORD"),
			SSLMode: "disable",
		},
		Ledger: config.LedgerConfig{
			BaseURL:  "http://ezbookkeeping.invalid:8080",
			Token:    "test-token",
			Timezone: "Asia/Shanghai",
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	handler, cleanup, err := buildApplicationHandler(ctx, cfg)
	if err != nil {
		t.Fatalf("buildApplicationHandler: %v", err)
	}
	if cleanup == nil {
		t.Fatal("cleanup is nil")
	}
	defer cleanup()

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if resp.Code != http.StatusOK {
		t.Fatalf("health status=%d body=%s", resp.Code, resp.Body.String())
	}
}

func TestValidateRuntimeAIConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  config.LLMConfig
		want bool
		err  bool
	}{
		{name: "disabled", cfg: config.LLMConfig{}, want: false},
		{name: "enabled", cfg: config.LLMConfig{BaseURL: "https://llm.example/v1", APIKey: "secret", FastModel: "fast", PlannerModel: "planner", ReviewerModel: "reviewer"}, want: true},
		{name: "partial", cfg: config.LLMConfig{BaseURL: "https://llm.example/v1", PlannerModel: "planner"}, err: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enabled, err := validateRuntimeAIConfig(tt.cfg)
			if tt.err {
				if err == nil {
					t.Fatal("expected config error")
				}
				return
			}
			if err != nil || enabled != tt.want {
				t.Fatalf("enabled=%v error=%v", enabled, err)
			}
		})
	}
}
