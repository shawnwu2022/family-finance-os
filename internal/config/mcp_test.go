package config

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestLoadMCPDisabledByDefault(t *testing.T) {
	t.Parallel()

	cfg, err := Load(mapGetenv(requiredDatabaseEnv()))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.MCP.Enabled {
		t.Fatal("MCP must be disabled by default")
	}
}

func TestLoadMCPEnabledWithExplicitSettings(t *testing.T) {
	t.Parallel()

	values := requiredDatabaseEnv()
	values["MCP_ENABLED"] = "true"
	values["MCP_TOKEN_FILE"] = "/run/secrets/custom-mcp-token"
	values["MCP_HOUSEHOLD_ID"] = "42"
	values["MCP_ALLOWED_ORIGINS"] = "https://trusted.example, https://ops.example:8443"
	values["MCP_REQUEST_TIMEOUT"] = "9s"
	values["MCP_MAX_CONCURRENT"] = "3"
	values["MCP_REQUESTS_PER_MINUTE"] = "25"
	values["MCP_MAX_BODY_BYTES"] = "131072"

	cfg, err := Load(mapGetenv(values))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.MCP.Enabled {
		t.Fatal("MCP should be enabled")
	}
	if cfg.MCP.TokenFile != "/run/secrets/custom-mcp-token" || cfg.MCP.HouseholdID != 42 {
		t.Fatalf("MCP token/scope = %#v", cfg.MCP)
	}
	if !reflect.DeepEqual(cfg.MCP.AllowedOrigins, []string{"https://trusted.example", "https://ops.example:8443"}) {
		t.Fatalf("AllowedOrigins = %#v", cfg.MCP.AllowedOrigins)
	}
	if cfg.MCP.RequestTimeout != 9*time.Second || cfg.MCP.MaxConcurrent != 3 || cfg.MCP.RequestsPerMinute != 25 || cfg.MCP.MaxBodyBytes != 131072 {
		t.Fatalf("MCP limits = %#v", cfg.MCP)
	}
}

func TestLoadMCPEnabledUsesSafeDefaults(t *testing.T) {
	t.Parallel()

	values := requiredDatabaseEnv()
	values["MCP_ENABLED"] = "true"
	values["MCP_HOUSEHOLD_ID"] = "42"

	cfg, err := Load(mapGetenv(values))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.MCP.TokenFile != "/run/secrets/finance-mcp-token" {
		t.Fatalf("TokenFile = %q", cfg.MCP.TokenFile)
	}
	if cfg.MCP.RequestTimeout != 15*time.Second || cfg.MCP.MaxConcurrent != 4 || cfg.MCP.RequestsPerMinute != 60 || cfg.MCP.MaxBodyBytes != 262144 {
		t.Fatalf("MCP defaults = %#v", cfg.MCP)
	}
	if len(cfg.MCP.AllowedOrigins) != 0 {
		t.Fatalf("AllowedOrigins = %#v, want empty", cfg.MCP.AllowedOrigins)
	}
}

func TestLoadMCPRejectsInvalidSettings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		key   string
		value string
		want  string
	}{
		{name: "invalid enabled boolean", key: "MCP_ENABLED", value: "maybe", want: "MCP_ENABLED"},
		{name: "missing household", key: "MCP_HOUSEHOLD_ID", value: "", want: "MCP_HOUSEHOLD_ID"},
		{name: "zero household", key: "MCP_HOUSEHOLD_ID", value: "0", want: "MCP_HOUSEHOLD_ID"},
		{name: "invalid household", key: "MCP_HOUSEHOLD_ID", value: "abc", want: "MCP_HOUSEHOLD_ID"},
		{name: "invalid origin", key: "MCP_ALLOWED_ORIGINS", value: "https://trusted.example/path", want: "MCP_ALLOWED_ORIGINS"},
		{name: "invalid timeout", key: "MCP_REQUEST_TIMEOUT", value: "later", want: "MCP_REQUEST_TIMEOUT"},
		{name: "zero timeout", key: "MCP_REQUEST_TIMEOUT", value: "0s", want: "MCP_REQUEST_TIMEOUT"},
		{name: "zero concurrency", key: "MCP_MAX_CONCURRENT", value: "0", want: "MCP_MAX_CONCURRENT"},
		{name: "invalid concurrency", key: "MCP_MAX_CONCURRENT", value: "many", want: "MCP_MAX_CONCURRENT"},
		{name: "zero rate", key: "MCP_REQUESTS_PER_MINUTE", value: "0", want: "MCP_REQUESTS_PER_MINUTE"},
		{name: "invalid rate", key: "MCP_REQUESTS_PER_MINUTE", value: "fast", want: "MCP_REQUESTS_PER_MINUTE"},
		{name: "zero body limit", key: "MCP_MAX_BODY_BYTES", value: "0", want: "MCP_MAX_BODY_BYTES"},
		{name: "invalid body limit", key: "MCP_MAX_BODY_BYTES", value: "huge", want: "MCP_MAX_BODY_BYTES"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			values := requiredDatabaseEnv()
			values["MCP_ENABLED"] = "true"
			values["MCP_HOUSEHOLD_ID"] = "42"
			values[tc.key] = tc.value
			if tc.key == "MCP_ENABLED" {
				values["MCP_HOUSEHOLD_ID"] = "42"
			}

			_, err := Load(mapGetenv(values))
			if err == nil {
				t.Fatalf("Load accepted %s=%q", tc.key, tc.value)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %q, want mention of %s", err, tc.want)
			}
		})
	}
}

func TestLoadMCPDisabledIgnoresMissingMCPSecretsAndScope(t *testing.T) {
	t.Parallel()

	values := requiredDatabaseEnv()
	values["MCP_ENABLED"] = "false"
	values["MCP_TOKEN_FILE"] = ""
	values["MCP_HOUSEHOLD_ID"] = ""

	if _, err := Load(mapGetenv(values)); err != nil {
		t.Fatalf("disabled MCP added startup prerequisite: %v", err)
	}
}

func requiredDatabaseEnv() map[string]string {
	return map[string]string{
		"DB_NAME":     "finance",
		"DB_USER":     "finance_app",
		"DB_PASSWORD": "secret",
	}
}
