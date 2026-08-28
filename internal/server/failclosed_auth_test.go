package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandlerWithoutBrowserAuthFailsClosed(t *testing.T) {
	handler := NewHandler(WithAPI(authTestFinanceAPI{}))

	protected := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard?period=2026-08", nil)
	protectedRec := httptest.NewRecorder()
	handler.ServeHTTP(protectedRec, protected)
	if protectedRec.Code != http.StatusUnauthorized {
		t.Fatalf("protected API without BrowserAuth status=%d want=401 body=%s", protectedRec.Code, protectedRec.Body.String())
	}

	health := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	healthRec := httptest.NewRecorder()
	handler.ServeHTTP(healthRec, health)
	if healthRec.Code != http.StatusOK {
		t.Fatalf("health without BrowserAuth status=%d want=200 body=%s", healthRec.Code, healthRec.Body.String())
	}
}
