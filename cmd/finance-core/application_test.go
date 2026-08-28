package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	financeauth "github.com/shawnwu2022/family-finance-os/internal/auth"
	"github.com/shawnwu2022/family-finance-os/internal/config"
	"github.com/shawnwu2022/family-finance-os/internal/report"
	"github.com/shawnwu2022/family-finance-os/internal/scheduler"
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

	secretDir := t.TempDir()
	authKeyFile := filepath.Join(secretDir, "finance-auth-key")
	if err := os.WriteFile(authKeyFile, []byte("0123456789abcdef0123456789abcdef"), 0o600); err != nil {
		t.Fatalf("write auth key: %v", err)
	}
	ledgerTokenFile := filepath.Join(secretDir, "ezbookkeeping-api-token")
	if err := os.WriteFile(ledgerTokenFile, []byte("test-token"), 0o600); err != nil {
		t.Fatalf("write ledger token: %v", err)
	}
	cfg := config.Config{
		ListenAddr: ":8000",
		Timezone:   "Asia/Shanghai",
		Database: config.DatabaseConfig{
			Host:     host,
			Port:     uint16(port),
			Name:     os.Getenv("TEST_POSTGRES_DB"),
			User:     os.Getenv("TEST_POSTGRES_USER"),
			Password: os.Getenv("TEST_POSTGRES_PASSWORD"),
			SSLMode:  "disable",
		},
		Ledger: config.LedgerConfig{
			BaseURL:      "http://ezbookkeeping.invalid:8080",
			APITokenFile: ledgerTokenFile,
		},
		Auth: config.AuthConfig{KeyFile: authKeyFile},
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

	protected := httptest.NewRecorder()
	handler.ServeHTTP(protected, httptest.NewRequest(http.MethodGet, "/api/v1/overview", nil))
	if protected.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated finance API status=%d want=401 body=%s", protected.Code, protected.Body.String())
	}
}

func TestLoadBrowserAuthSecretBoxRequiresPrivateExactKey(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	valid := filepath.Join(dir, "valid")
	if err := os.WriteFile(valid, []byte("0123456789abcdef0123456789abcdef"), 0o600); err != nil {
		t.Fatalf("write valid key: %v", err)
	}
	box, err := loadBrowserAuthSecretBox(config.AuthConfig{KeyFile: valid})
	if err != nil || box == nil {
		t.Fatalf("load valid auth key: box=%v err=%v", box, err)
	}

	short := filepath.Join(dir, "short")
	if err := os.WriteFile(short, []byte("too-short"), 0o600); err != nil {
		t.Fatalf("write short key: %v", err)
	}
	if _, err := loadBrowserAuthSecretBox(config.AuthConfig{KeyFile: short}); !errors.Is(err, financeauth.ErrInvalidSecretBoxKey) {
		t.Fatalf("short auth key error=%v want ErrInvalidSecretBoxKey", err)
	}

	insecure := filepath.Join(dir, "insecure")
	if err := os.WriteFile(insecure, []byte("0123456789abcdef0123456789abcdef"), 0o644); err != nil {
		t.Fatalf("write insecure key: %v", err)
	}
	if _, err := loadBrowserAuthSecretBox(config.AuthConfig{KeyFile: insecure}); err == nil {
		t.Fatal("loadBrowserAuthSecretBox accepted group/other-readable key")
	}
}

func TestLoadLedgerAPITokenRequiresPrivateSecretFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	valid := filepath.Join(dir, "valid-token")
	if err := os.WriteFile(valid, []byte("ledger-token\n"), 0o600); err != nil {
		t.Fatalf("write valid token: %v", err)
	}
	token, err := loadLedgerAPIToken(config.LedgerConfig{APITokenFile: valid})
	if err != nil || string(token) != "ledger-token" {
		t.Fatalf("load valid ledger token: token=%q err=%v", token, err)
	}
	clear(token)

	insecure := filepath.Join(dir, "insecure-token")
	if err := os.WriteFile(insecure, []byte("ledger-token"), 0o644); err != nil {
		t.Fatalf("write insecure token: %v", err)
	}
	if _, err := loadLedgerAPIToken(config.LedgerConfig{APITokenFile: insecure}); err == nil {
		t.Fatal("loadLedgerAPIToken accepted group/other-readable token")
	}
}

func TestNewMonthlyReportJobUsesHouseholdTimezoneAndPreviousMonth(t *testing.T) {
	reporter := &fakeMonthlyReporter{}
	job, err := newMonthlyReportJob(scheduler.HouseholdScope{
		HouseholdID: 42,
		Timezone:    "Asia/Shanghai",
	}, reporter)
	if err != nil {
		t.Fatalf("newMonthlyReportJob: %v", err)
	}
	if job.Name != report.JobNameMonthly || job.HouseholdID != 42 || job.CatchUp != scheduler.CatchUpLatestOnly {
		t.Fatalf("job identity = %#v", job)
	}

	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	trigger := time.Date(2026, time.August, 1, 3, 0, 0, 0, loc)
	if got := job.Period(trigger); got != "2026-07" {
		t.Fatalf("job period = %q, want 2026-07", got)
	}
	if err := job.Run(context.Background(), trigger); err != nil {
		t.Fatalf("job Run: %v", err)
	}
	if reporter.householdID != 42 || reporter.period != "2026-07" {
		t.Fatalf("report call = household %d period %q", reporter.householdID, reporter.period)
	}
}

func TestNewMonthlyReportJobRejectsInvalidHouseholdTimezone(t *testing.T) {
	_, err := newMonthlyReportJob(scheduler.HouseholdScope{HouseholdID: 42, Timezone: "Mars/Olympus"}, &fakeMonthlyReporter{})
	if err == nil {
		t.Fatal("newMonthlyReportJob() error = nil, want invalid timezone error")
	}
}

type fakeMonthlyReporter struct {
	householdID int64
	period      string
}

func (f *fakeMonthlyReporter) MonthlyReport(_ context.Context, householdID int64, period string) (report.MonthlyReport, error) {
	f.householdID = householdID
	f.period = period
	return report.MonthlyReport{Kind: report.KindMonthly, Period: period}, nil
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
