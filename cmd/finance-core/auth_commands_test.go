package main

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/shawnwu2022/family-finance-os/internal/bootstrap"
	"github.com/shawnwu2022/family-finance-os/internal/config"
)

func TestRunAuthMaintenanceCommandsUseSecretFileAndDatabaseConfig(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	passwordFile := filepath.Join(dir, "admin-password")
	if err := os.WriteFile(passwordFile, []byte("correct horse battery staple\n"), 0o600); err != nil {
		t.Fatalf("write password file: %v", err)
	}

	var gotUsername string
	var gotPassword []byte
	var resetTOTPUsername string
	handlers := databaseCommandHandlers{
		migrate: func(context.Context, config.DatabaseConfig) error { return nil },
		bootstrap: func(context.Context, config.DatabaseConfig, bootstrap.Input) (bootstrap.Result, error) {
			return bootstrap.Result{}, nil
		},
		resetPassword: func(_ context.Context, cfg config.DatabaseConfig, username string, password []byte) error {
			if cfg.Name != "finance" || cfg.User != "finance_app" {
				t.Fatalf("reset password DB config = %#v", cfg)
			}
			gotUsername = username
			gotPassword = append([]byte(nil), password...)
			return nil
		},
		resetTOTP: func(_ context.Context, cfg config.DatabaseConfig, username string) error {
			if cfg.Name != "finance" || cfg.User != "finance_app" {
				t.Fatalf("reset TOTP DB config = %#v", cfg)
			}
			resetTOTPUsername = username
			return nil
		},
	}

	if err := runWithCommands([]string{
		"auth-reset-password", "--username", "Owner", "--password-file", passwordFile,
	}, validRuntimeGetenv(nil), nil, nil, nil, io.Discard, handlers); err != nil {
		t.Fatalf("auth-reset-password: %v", err)
	}
	if gotUsername != "Owner" || string(gotPassword) != "correct horse battery staple" {
		t.Fatalf("password reset args: username=%q password=%q", gotUsername, gotPassword)
	}

	if err := runWithCommands([]string{
		"auth-reset-totp", "--username", "Owner",
	}, validRuntimeGetenv(nil), nil, nil, nil, io.Discard, handlers); err != nil {
		t.Fatalf("auth-reset-totp: %v", err)
	}
	if resetTOTPUsername != "Owner" {
		t.Fatalf("reset TOTP username = %q", resetTOTPUsername)
	}
}

func TestReadRequiredSecretFileRejectsEmptyDirectoryAndOversized(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	empty := filepath.Join(dir, "empty")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatalf("write empty file: %v", err)
	}
	if _, err := readRequiredSecretFile(empty, 128); err == nil {
		t.Fatal("readRequiredSecretFile accepted empty secret")
	}
	if _, err := readRequiredSecretFile(dir, 128); err == nil {
		t.Fatal("readRequiredSecretFile accepted directory")
	}
	over := filepath.Join(dir, "oversized")
	if err := os.WriteFile(over, make([]byte, 129), 0o600); err != nil {
		t.Fatalf("write oversized file: %v", err)
	}
	if _, err := readRequiredSecretFile(over, 128); err == nil {
		t.Fatal("readRequiredSecretFile accepted oversized secret")
	}
}
