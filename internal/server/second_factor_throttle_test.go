package server

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	financeauth "github.com/shawnwu2022/family-finance-os/internal/auth"
)

func TestSecondFactorVerificationIsRateLimited(t *testing.T) {
	fake := authenticatedFake()
	fake.verify = func(context.Context, string, string, bool, time.Time) (financeauth.SessionIssue, error) {
		return financeauth.SessionIssue{}, financeauth.ErrInvalidSecondFactor
	}
	handler := NewHandler(WithAPI(authTestFinanceAPI{}), WithBrowserAuth(fake))

	for attempt := 1; attempt <= 6; attempt++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/verify", bytes.NewBufferString(`{"challenge":"challenge-token","code":"000000"}`))
		req.RemoteAddr = "203.0.113.10:4242"
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		want := http.StatusUnauthorized
		if attempt == 6 {
			want = http.StatusTooManyRequests
		}
		if rec.Code != want {
			t.Fatalf("attempt=%d status=%d want=%d body=%s", attempt, rec.Code, want, rec.Body.String())
		}
	}
}

func TestTOTPEnrollmentConfirmationIsRateLimited(t *testing.T) {
	fake := authenticatedFake()
	fake.confirmEnrollment = func(context.Context, string, string, time.Time) (financeauth.SessionIssue, error) {
		return financeauth.SessionIssue{}, financeauth.ErrInvalidSecondFactor
	}
	handler := NewHandler(WithAPI(authTestFinanceAPI{}), WithBrowserAuth(fake))

	for attempt := 1; attempt <= 6; attempt++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/totp/enroll/confirm", bytes.NewBufferString(`{"challenge":"enrollment-challenge","code":"000000"}`))
		req.RemoteAddr = "203.0.113.11:4242"
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		want := http.StatusUnauthorized
		if attempt == 6 {
			want = http.StatusTooManyRequests
		}
		if rec.Code != want {
			t.Fatalf("attempt=%d status=%d want=%d body=%s", attempt, rec.Code, want, rec.Body.String())
		}
	}
}
