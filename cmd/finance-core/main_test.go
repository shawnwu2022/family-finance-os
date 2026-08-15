package main

import (
	"net/http"
	"testing"
)

func TestRunServeUsesDefaultListenAddress(t *testing.T) {
	t.Parallel()

	called := false
	serve := func(addr string, handler http.Handler) error {
		called = true
		if addr != ":8000" {
			t.Fatalf("addr = %q, want :8000", addr)
		}
		if handler == nil {
			t.Fatal("handler is nil")
		}
		return nil
	}

	if err := run([]string{"serve"}, validRuntimeGetenv(nil), serve, nil); err != nil {
		t.Fatalf("run serve: %v", err)
	}
	if !called {
		t.Fatal("serve function was not called")
	}
}

func TestRunServeRejectsMissingDatabaseConfig(t *testing.T) {
	t.Parallel()

	serve := func(string, http.Handler) error {
		return nil
	}

	err := run([]string{"serve"}, func(string) string { return "" }, serve, nil)
	if err == nil {
		t.Fatal("run serve succeeded without database credentials")
	}
}

func TestRunServeUsesConfiguredListenAddress(t *testing.T) {
	t.Parallel()

	serve := func(addr string, _ http.Handler) error {
		if addr != "127.0.0.1:9000" {
			t.Fatalf("addr = %q, want 127.0.0.1:9000", addr)
		}
		return nil
	}

	if err := run([]string{"serve"}, validRuntimeGetenv(map[string]string{
		"FINANCE_LISTEN_ADDR": "127.0.0.1:9000",
	}), serve, nil); err != nil {
		t.Fatalf("run serve: %v", err)
	}
}

func validRuntimeGetenv(overrides map[string]string) func(string) string {
	values := map[string]string{
		"DB_NAME":     "finance",
		"DB_USER":     "finance_app",
		"DB_PASSWORD": "secret",
	}
	for key, value := range overrides {
		values[key] = value
	}
	return func(key string) string { return values[key] }
}

func TestRunHealthcheckUsesConfiguredURL(t *testing.T) {
	t.Parallel()

	called := false
	check := func(url string) error {
		called = true
		if url != "http://finance-core:8000/healthz" {
			t.Fatalf("url = %q", url)
		}
		return nil
	}

	getenv := func(key string) string {
		if key == "FINANCE_HEALTHCHECK_URL" {
			return "http://finance-core:8000/healthz"
		}
		return ""
	}

	if err := run([]string{"healthcheck"}, getenv, nil, check); err != nil {
		t.Fatalf("run healthcheck: %v", err)
	}
	if !called {
		t.Fatal("check function was not called")
	}
}

func TestRunRejectsUnknownCommand(t *testing.T) {
	t.Parallel()

	if err := run([]string{"unknown"}, func(string) string { return "" }, nil, nil); err == nil {
		t.Fatal("expected error for unknown command")
	}
}
