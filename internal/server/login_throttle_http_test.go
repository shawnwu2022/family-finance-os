package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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
		rec := performFailedLogin(handler, "203.0.113.10:49152", "", "Owner")
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

func TestAuthLoginTrustedProxySeparatesForwardedClientIPBuckets(t *testing.T) {
	t.Parallel()
	beginCalls := 0
	fake := authenticatedFake()
	fake.beginLogin = func(context.Context, string, string, time.Time) (financeauth.LoginResult, error) {
		beginCalls++
		return financeauth.LoginResult{}, financeauth.ErrInvalidCredentials
	}
	handler := NewHandler(
		WithAPI(authTestFinanceAPI{}),
		WithBrowserAuth(fake),
		WithTrustedProxyCIDR("172.30.0.10/32"),
	)

	for attempt := 1; attempt <= 5; attempt++ {
		rec := performFailedLogin(handler, "172.30.0.10:41000", "198.51.100.10", "Owner-A")
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("client A attempt=%d status=%d want=401 body=%s", attempt, rec.Code, rec.Body.String())
		}
	}

	rec := performFailedLogin(handler, "172.30.0.10:41001", "198.51.100.11", "Owner-B")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("client B status=%d want=401 body=%s", rec.Code, rec.Body.String())
	}
	if beginCalls != 6 {
		t.Fatalf("BeginLogin calls=%d want=6", beginCalls)
	}
}

func TestAuthLoginUntrustedPeerCannotRotateForwardedIPToEvadeBucket(t *testing.T) {
	t.Parallel()
	beginCalls := 0
	fake := authenticatedFake()
	fake.beginLogin = func(context.Context, string, string, time.Time) (financeauth.LoginResult, error) {
		beginCalls++
		return financeauth.LoginResult{}, financeauth.ErrInvalidCredentials
	}
	handler := NewHandler(
		WithAPI(authTestFinanceAPI{}),
		WithBrowserAuth(fake),
		WithTrustedProxyCIDR("172.30.0.10/32"),
	)

	for attempt := 1; attempt <= 6; attempt++ {
		forwarded := fmt.Sprintf("198.51.100.%d", attempt)
		username := fmt.Sprintf("Owner-%d", attempt)
		rec := performFailedLogin(handler, "203.0.113.50:49152", forwarded, username)
		if attempt <= 5 {
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("attempt=%d status=%d want=401 body=%s", attempt, rec.Code, rec.Body.String())
			}
			continue
		}
		if rec.Code != http.StatusTooManyRequests {
			t.Fatalf("attempt=6 status=%d want=429 body=%s", rec.Code, rec.Body.String())
		}
	}
	if beginCalls != 5 {
		t.Fatalf("BeginLogin calls=%d want=5", beginCalls)
	}
}

func TestAuthLoginTrustedProxyMalformedForwardedIPFallsBackToPeer(t *testing.T) {
	t.Parallel()
	beginCalls := 0
	fake := authenticatedFake()
	fake.beginLogin = func(context.Context, string, string, time.Time) (financeauth.LoginResult, error) {
		beginCalls++
		return financeauth.LoginResult{}, financeauth.ErrInvalidCredentials
	}
	handler := NewHandler(
		WithAPI(authTestFinanceAPI{}),
		WithBrowserAuth(fake),
		WithTrustedProxyCIDR("172.30.0.10/32"),
	)

	for attempt := 1; attempt <= 6; attempt++ {
		username := fmt.Sprintf("Malformed-%d", attempt)
		rec := performFailedLogin(handler, "172.30.0.10:41000", "not-an-ip", username)
		if attempt <= 5 {
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("attempt=%d status=%d want=401 body=%s", attempt, rec.Code, rec.Body.String())
			}
			continue
		}
		if rec.Code != http.StatusTooManyRequests {
			t.Fatalf("attempt=6 status=%d want=429 body=%s", rec.Code, rec.Body.String())
		}
	}
	if beginCalls != 5 {
		t.Fatalf("BeginLogin calls=%d want=5", beginCalls)
	}
}

func performFailedLogin(handler http.Handler, remoteAddr, forwardedFor, username string) *httptest.ResponseRecorder {
	body, _ := json.Marshal(map[string]string{"username": username, "password": "wrong"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.RemoteAddr = remoteAddr
	req.Host = "finance.example.test"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://finance.example.test")
	if forwardedFor != "" {
		req.Header.Set("X-Forwarded-For", forwardedFor)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}
