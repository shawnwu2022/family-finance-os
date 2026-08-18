package mcpadapter

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSecureHTTPHandlerEnforcesPerProcessRateLimit(t *testing.T) {
	calls := 0
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusNoContent)
	})
	opts := testSecurityOptions()
	opts.RequestsPerMinute = 2
	current := time.Date(2026, 8, 18, 10, 0, 30, 0, time.UTC)
	handler, err := newSecureHTTPHandlerWithClock(next, opts, func() time.Time { return current })
	if err != nil {
		t.Fatalf("newSecureHTTPHandlerWithClock: %v", err)
	}

	for i := 1; i <= 2; i++ {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, authenticatedSecurityRequest(http.MethodPost, ""))
		if response.Code != http.StatusNoContent {
			t.Fatalf("request %d status=%d want 204 body=%q", i, response.Code, response.Body.String())
		}
	}

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, authenticatedSecurityRequest(http.MethodPost, ""))
	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("third status=%d want 429 body=%q", response.Code, response.Body.String())
	}
	assertSecurityErrorCode(t, response, "busy")
	if calls != 2 {
		t.Fatalf("downstream calls=%d want 2", calls)
	}

	current = current.Add(time.Minute)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, authenticatedSecurityRequest(http.MethodPost, ""))
	if response.Code != http.StatusNoContent {
		t.Fatalf("new-window status=%d want 204 body=%q", response.Code, response.Body.String())
	}
	if calls != 3 {
		t.Fatalf("downstream calls=%d want 3 after window reset", calls)
	}
}
