package config

import (
	"net/url"
	"strings"
	"testing"
)

func TestLoadDefaultsAndMapsRuntimeSettings(t *testing.T) {
	t.Parallel()

	cfg, err := Load(mapGetenv(map[string]string{
		"DB_NAME":            "finance",
		"DB_USER":            "finance_app",
		"DB_PASSWORD":        "secret",
		"EBK_API_TOKEN":      "ledger-token",
		"LLM_BASE_URL":       "https://llm.example.com/v1",
		"LLM_API_KEY":        "llm-key",
		"LLM_FAST_MODEL":     "fast-model",
		"LLM_PLANNER_MODEL":  "planner-model",
		"LLM_REVIEWER_MODEL": "reviewer-model",
	}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.ListenAddr != ":8000" {
		t.Fatalf("ListenAddr = %q, want :8000", cfg.ListenAddr)
	}
	if cfg.Timezone != "Asia/Shanghai" {
		t.Fatalf("Timezone = %q, want Asia/Shanghai", cfg.Timezone)
	}
	if cfg.Database.Host != "postgres" || cfg.Database.Port != 5432 {
		t.Fatalf("Database host/port = %s/%d, want postgres/5432", cfg.Database.Host, cfg.Database.Port)
	}
	if cfg.Database.SSLMode != "disable" {
		t.Fatalf("Database SSLMode = %q, want disable", cfg.Database.SSLMode)
	}
	if cfg.Ledger.BaseURL != "http://ezbookkeeping:8080/api/v1" {
		t.Fatalf("Ledger BaseURL = %q", cfg.Ledger.BaseURL)
	}
	if cfg.Ledger.APIToken != "ledger-token" {
		t.Fatalf("Ledger APIToken = %q", cfg.Ledger.APIToken)
	}
	if cfg.LLM.BaseURL != "https://llm.example.com/v1" || cfg.LLM.APIKey != "llm-key" {
		t.Fatalf("LLM config = %#v", cfg.LLM)
	}
	if cfg.LLM.FastModel != "fast-model" || cfg.LLM.PlannerModel != "planner-model" || cfg.LLM.ReviewerModel != "reviewer-model" {
		t.Fatalf("LLM models = %#v", cfg.LLM)
	}
}

func TestLoadRequiresDatabaseCredentials(t *testing.T) {
	t.Parallel()

	required := []string{"DB_NAME", "DB_USER", "DB_PASSWORD"}
	complete := map[string]string{
		"DB_NAME":     "finance",
		"DB_USER":     "finance_app",
		"DB_PASSWORD": "secret",
	}

	for _, missing := range required {
		missing := missing
		t.Run(missing, func(t *testing.T) {
			values := map[string]string{}
			for key, value := range complete {
				values[key] = value
			}
			delete(values, missing)

			_, err := Load(mapGetenv(values))
			if err == nil {
				t.Fatalf("Load succeeded without %s", missing)
			}
			if !strings.Contains(err.Error(), missing) {
				t.Fatalf("error = %q, want mention of %s", err, missing)
			}
		})
	}
}

func TestDatabaseURLPreservesSpecialCharacterPassword(t *testing.T) {
	t.Parallel()

	password := `p@ss:/word?#[]%+`
	cfg, err := Load(mapGetenv(map[string]string{
		"DB_HOST":     "db.internal",
		"DB_PORT":     "6432",
		"DB_NAME":     "finance data",
		"DB_USER":     "finance:user",
		"DB_PASSWORD": password,
		"DB_SSLMODE":  "require",
	}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	dbURL := cfg.Database.URL()
	parsed, err := url.Parse(dbURL.String())
	if err != nil {
		t.Fatalf("parse URL: %v", err)
	}
	gotPassword, ok := parsed.User.Password()
	if !ok {
		t.Fatal("password missing from URL userinfo")
	}
	if gotPassword != password {
		t.Fatalf("password round trip = %q, want %q", gotPassword, password)
	}
	if parsed.User.Username() != "finance:user" {
		t.Fatalf("username = %q", parsed.User.Username())
	}
	if parsed.Host != "db.internal:6432" {
		t.Fatalf("host = %q", parsed.Host)
	}
	if parsed.Path != "/finance data" {
		t.Fatalf("path = %q", parsed.Path)
	}
	if parsed.Query().Get("sslmode") != "require" {
		t.Fatalf("sslmode = %q", parsed.Query().Get("sslmode"))
	}
}

func TestLoadRejectsInvalidDatabasePort(t *testing.T) {
	t.Parallel()

	_, err := Load(mapGetenv(map[string]string{
		"DB_PORT":     "70000",
		"DB_NAME":     "finance",
		"DB_USER":     "finance_app",
		"DB_PASSWORD": "secret",
	}))
	if err == nil {
		t.Fatal("Load succeeded with invalid DB_PORT")
	}
	if !strings.Contains(err.Error(), "DB_PORT") {
		t.Fatalf("error = %q, want mention of DB_PORT", err)
	}
}

func mapGetenv(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}
