package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	financeauth "github.com/shawnwu2022/family-finance-os/internal/auth"
)

func TestAuthLoginRateLimitsBeforeSixthPasswordVerification(t *testing.T) {
	t.Parallel()
	beginCalls := 0
	fake := authenticatedFake()
	fake.beginLogin = func(context.Context, string, string, time.Time) (financeauth.LoginResult, error) {
		beginCalls++
		return financeauth.LoginResult{}, financeauth.ErrInvalidCredentials
	}
	handler := NewHandler(WithAPI(authTestFinanceAPI{}), WithBrowserAuth(fake))

	for attempt := 1; attempt <= 6; attempt++ {
		body, _ := json.Marshal(map[string]string{"username": "Owner", "password": "wrong"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
		req.RemoteAddr = "203.0.113.10:49152"
		req.Host = "finance.example.test"
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", "http://finance.example.test")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if attempt <= 5 {
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("attempt=%d status=%d want=401 body=%s", attempt, rec.Code, rec.Body.String())
			}
			continue
		}
		if rec.Code != http.StatusTooManyRequests {
			t.Fatalf("attempt=6 status=%d want=429 body=%s", rec.Code, rec.Body.String())
		}
		if got := rec.Header().Get("Retry-After"); got != "300" {
			t.Fatalf("Retry-After=%q want=300", got)
		}
	}
	if beginCalls != 5 {
		t.Fatalf("BeginLogin calls=%d want=5", beginCalls)
	}
}
