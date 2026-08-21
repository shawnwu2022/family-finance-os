package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFinanceAPIRejectsNonJSONContentTypeBeforeBackend(t *testing.T) {
	fake := &fakeFinanceAPI{}
	handler := NewHandler(WithAPI(fake))
	body := `{"household_id":7,"question":"trigger advisor"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/advisor", strings.NewReader(body))
	req.Header.Set("Content-Type", "text/plain")
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status=%d body=%s want %d", resp.Code, resp.Body.String(), http.StatusUnsupportedMediaType)
	}
	if fake.calls != 0 {
		t.Fatalf("backend calls=%d want 0", fake.calls)
	}
}

func TestFinanceAPIRejectsCrossOriginUnsafeRequestBeforeBackend(t *testing.T) {
	fake := &fakeFinanceAPI{}
	handler := NewHandler(WithAPI(fake))
	body := `{"household_id":7,"question":"trigger advisor"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/advisor", strings.NewReader(body))
	req.Host = "finance.example.com"
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("Origin", "https://evil.example")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s want %d", resp.Code, resp.Body.String(), http.StatusForbidden)
	}
	if fake.calls != 0 {
		t.Fatalf("backend calls=%d want 0", fake.calls)
	}
}

func TestFinanceAPIAcceptsSameOriginJSONRequest(t *testing.T) {
	fake := &fakeFinanceAPI{advisor: AdvisorResponse{Text: "ok"}}
	handler := NewHandler(WithAPI(fake))
	body := `{"household_id":7,"question":"normal request"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/advisor", strings.NewReader(body))
	req.Host = "finance.example.com"
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("Origin", "https://finance.example.com")
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s want %d", resp.Code, resp.Body.String(), http.StatusOK)
	}
	if fake.calls != 1 {
		t.Fatalf("backend calls=%d want 1", fake.calls)
	}
}
