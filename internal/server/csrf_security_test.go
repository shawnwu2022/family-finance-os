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
