package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReadinessEndpointUsesConfiguredCheckWithoutLeakingErrors(t *testing.T) {
	t.Run("ready", func(t *testing.T) {
		handler := NewHandler(WithReady(func(context.Context) error { return nil }))
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/readyz", nil))
		if response.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
		}
		if response.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("Cache-Control=%q", response.Header().Get("Cache-Control"))
		}
	})

	t.Run("not ready", func(t *testing.T) {
		handler := NewHandler(WithReady(func(context.Context) error {
			return errors.New("postgres password=super-secret")
		}))
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/readyz", nil))
		if response.Code != http.StatusServiceUnavailable {
			t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
		}
		if strings.Contains(response.Body.String(), "super-secret") || strings.Contains(response.Body.String(), "password") {
			t.Fatalf("readiness leaked dependency error: %s", response.Body.String())
		}
	})
}
