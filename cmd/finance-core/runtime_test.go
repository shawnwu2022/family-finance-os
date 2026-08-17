package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shawnwu2022/family-finance-os/internal/config"
)

func TestRunWithBuilderUsesHandlerAndCleanup(t *testing.T) {
	t.Parallel()

	built := false
	cleaned := false
	builder := func(_ context.Context, cfg config.Config) (http.Handler, func(), error) {
		built = true
		if cfg.Database.Name != "finance" {
			t.Fatalf("database=%q", cfg.Database.Name)
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/dashboard" {
				http.NotFound(w, r)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		}), func() { cleaned = true }, nil
	}
	serve := func(_ string, handler http.Handler) error {
		if !built {
			t.Fatal("serve ran before builder")
		}
		resp := httptest.NewRecorder()
		handler.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/dashboard", nil))
		if resp.Code != http.StatusNoContent {
			t.Fatalf("status=%d", resp.Code)
		}
		return nil
	}

	if err := runWithBuilder([]string{"serve"}, validRuntimeGetenv(nil), serve, nil, builder); err != nil {
		t.Fatalf("runWithBuilder: %v", err)
	}
	if !cleaned {
		t.Fatal("application cleanup was not called")
	}
}

func TestRunWithBuilderStopsBeforeServeWhenBuildFails(t *testing.T) {
	t.Parallel()

	want := errors.New("build failed")
	builder := func(context.Context, config.Config) (http.Handler, func(), error) {
		return nil, nil, want
	}
	serveCalled := false
	serve := func(string, http.Handler) error {
		serveCalled = true
		return nil
	}

	err := runWithBuilder([]string{"serve"}, validRuntimeGetenv(nil), serve, nil, builder)
	if !errors.Is(err, want) {
		t.Fatalf("error=%v want build failure", err)
	}
	if serveCalled {
		t.Fatal("serve was called after application build failure")
	}
}
