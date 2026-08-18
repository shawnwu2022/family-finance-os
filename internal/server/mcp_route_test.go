package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMCPRouteIsAbsentUnlessExplicitlyConfigured(t *testing.T) {
	handler := NewHandler()
	request := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("POST /mcp status=%d want 404 when MCP is not configured", response.Code)
	}
}

func TestWithMCPRegistersExactRouteForStreamableHTTPMethods(t *testing.T) {
	calls := 0
	mcpHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("X-MCP-Method", r.Method)
		w.WriteHeader(http.StatusNoContent)
	})
	handler := NewHandler(WithMCP(mcpHandler))

	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			request := httptest.NewRequest(method, "/mcp", nil)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusNoContent || response.Header().Get("X-MCP-Method") != method {
				t.Fatalf("%s /mcp status=%d header=%q", method, response.Code, response.Header().Get("X-MCP-Method"))
			}
		})
	}
	if calls != 3 {
		t.Fatalf("MCP handler calls=%d want 3", calls)
	}

	request := httptest.NewRequest(http.MethodPost, "/mcp/anything", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("POST /mcp/anything status=%d want 404", response.Code)
	}
	if calls != 3 {
		t.Fatalf("MCP handler handled non-exact path; calls=%d", calls)
	}
}
