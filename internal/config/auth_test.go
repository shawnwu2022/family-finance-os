package config

import "testing"

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
