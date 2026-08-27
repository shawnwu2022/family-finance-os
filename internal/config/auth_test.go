package config

import (
	"reflect"
	"strings"
	"testing"
)

func TestLoadAuthSecretPathsHaveSafeDefaultsAndAllowPathOverrides(t *testing.T) {
	base := map[string]string{
		"DB_NAME":     "finance",
		"DB_USER":     "finance_app",
		"DB_PASSWORD": "secret",
	}
	getenv := func(key string) string { return base[key] }
	cfg, err := Load(getenv)
	if err != nil {
		t.Fatalf("Load defaults: %v", err)
	}
	if cfg.Auth.KeyFile != "/run/secrets/finance-auth-key" {
		t.Fatalf("Auth.KeyFile = %q", cfg.Auth.KeyFile)
	}
	if cfg.Auth.AdminUsername != "finance" {
		t.Fatalf("Auth.AdminUsername = %q", cfg.Auth.AdminUsername)
	}
	if cfg.Auth.AdminPasswordFile != "/run/secrets/finance-admin-password" {
		t.Fatalf("Auth.AdminPasswordFile = %q", cfg.Auth.AdminPasswordFile)
	}

	base["FINANCE_AUTH_KEY_FILE"] = "/etc/family-finance/auth-key"
	base["FINANCE_ADMIN_USERNAME"] = "owner"
	base["FINANCE_ADMIN_PASSWORD_FILE"] = "/etc/family-finance/admin-password"
	cfg, err = Load(getenv)
	if err != nil {
		t.Fatalf("Load overrides: %v", err)
	}
	if cfg.Auth.KeyFile != "/etc/family-finance/auth-key" || cfg.Auth.AdminUsername != "owner" || cfg.Auth.AdminPasswordFile != "/etc/family-finance/admin-password" {
		t.Fatalf("Auth overrides = %#v", cfg.Auth)
	}
}

func TestLoadFinanceTrustedProxyCIDRDefaultsToTrustNoProxy(t *testing.T) {
	cfg, err := Load(mapGetenv(map[string]string{
		"DB_NAME":     "finance",
		"DB_USER":     "finance_app",
		"DB_PASSWORD": "secret",
	}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := authTrustedProxyCIDR(t, cfg.Auth); got != "" {
		t.Fatalf("Auth.TrustedProxyCIDR = %q, want empty", got)
	}
}

func TestLoadFinanceTrustedProxyCIDRAcceptsExplicitProxyNetwork(t *testing.T) {
	cfg, err := Load(mapGetenv(map[string]string{
		"DB_NAME":                    "finance",
		"DB_USER":                    "finance_app",
		"DB_PASSWORD":                "secret",
		"FINANCE_TRUSTED_PROXY_CIDR": "172.30.0.10/32",
	}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := authTrustedProxyCIDR(t, cfg.Auth); got != "172.30.0.10/32" {
		t.Fatalf("Auth.TrustedProxyCIDR = %q", got)
	}
}

func TestLoadFinanceTrustedProxyCIDRRejectsInvalidNetwork(t *testing.T) {
	_, err := Load(mapGetenv(map[string]string{
		"DB_NAME":                    "finance",
		"DB_USER":                    "finance_app",
		"DB_PASSWORD":                "secret",
		"FINANCE_TRUSTED_PROXY_CIDR": "not-a-cidr",
	}))
	if err == nil || !strings.Contains(err.Error(), "FINANCE_TRUSTED_PROXY_CIDR") {
		t.Fatalf("invalid trusted proxy error = %v", err)
	}
}

func authTrustedProxyCIDR(t *testing.T, auth AuthConfig) string {
	t.Helper()
	field := reflect.ValueOf(auth).FieldByName("TrustedProxyCIDR")
	if !field.IsValid() || field.Kind() != reflect.String {
		t.Fatal("AuthConfig.TrustedProxyCIDR is missing")
	}
	return field.String()
}
