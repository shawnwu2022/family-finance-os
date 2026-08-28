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

type rbacPortfolioAPI struct{ authTestFinanceAPI }

func (rbacPortfolioAPI) ListPortfolioAssets(context.Context, int64) (PortfolioAssetsResponse, error) {
	return PortfolioAssetsResponse{}, nil
}
func (rbacPortfolioAPI) UpsertPortfolioAsset(context.Context, int64, string, PortfolioAssetUpsertRequest) (PortfolioAssetResponse, error) {
	return PortfolioAssetResponse{}, nil
}
func (rbacPortfolioAPI) DeletePortfolioAsset(context.Context, int64, string) error { return nil }

func roleAuthenticatedFake(role financeauth.Role) fakeBrowserAuth {
	return fakeBrowserAuth{
		authenticate: func(_ context.Context, token string, _ time.Time) (financeauth.SessionIdentity, error) {
			if token != "valid-session" {
				return financeauth.SessionIdentity{}, financeauth.ErrUnauthenticated
			}
			return financeauth.SessionIdentity{
				UserID:      11,
				Username:    "member",
				HouseholdID: 7,
				Role:        role,
				CSRFToken:   "csrf-token",
			}, nil
		},
	}
}

func authenticatedRequest(method, target string, body []byte) *http.Request {
	req := httptest.NewRequest(method, target, bytes.NewReader(body))
	req.Host = "finance.example.test"
	req.Header.Set("Origin", "http://finance.example.test")
	req.Header.Set("X-CSRF-Token", "csrf-token")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "valid-session"})
	return req
}

func TestViewerCanReadAndSimulateButCannotPersistFinanceChanges(t *testing.T) {
	handler := NewHandler(WithAPI(rbacPortfolioAPI{}), WithBrowserAuth(roleAuthenticatedFake(financeauth.RoleViewer)))

	readReq := authenticatedRequest(http.MethodGet, "/api/v1/portfolio/assets", nil)
	readRec := httptest.NewRecorder()
	handler.ServeHTTP(readRec, readReq)
	if readRec.Code != http.StatusOK {
		t.Fatalf("viewer read status=%d body=%s", readRec.Code, readRec.Body.String())
	}

	advisorReq := authenticatedRequest(http.MethodPost, "/api/v1/advisor", []byte(`{"question":"Can I afford this?","require_tool":true}`))
	advisorRec := httptest.NewRecorder()
	handler.ServeHTTP(advisorRec, advisorReq)
	if advisorRec.Code != http.StatusOK {
		t.Fatalf("viewer advisor status=%d body=%s", advisorRec.Code, advisorRec.Body.String())
	}

	writeReq := authenticatedRequest(http.MethodPut, "/api/v1/portfolio/assets/fund", []byte(`{"name":"Fund","category":"fund","value":{"minor":"10000","currency":"CNY"},"value_as_of":"2026-08-28T00:00:00Z"}`))
	writeRec := httptest.NewRecorder()
	handler.ServeHTTP(writeRec, writeReq)
	if writeRec.Code != http.StatusForbidden {
		t.Fatalf("viewer mutation status=%d want=403 body=%s", writeRec.Code, writeRec.Body.String())
	}
	if !bytes.Contains(writeRec.Body.Bytes(), []byte(`"code":"insufficient_role"`)) {
		t.Fatalf("viewer mutation returned wrong error: %s", writeRec.Body.String())
	}
}

func TestEditorCanPersistFinanceChanges(t *testing.T) {
	handler := NewHandler(WithAPI(rbacPortfolioAPI{}), WithBrowserAuth(roleAuthenticatedFake(financeauth.RoleEditor)))
	writeReq := authenticatedRequest(http.MethodPut, "/api/v1/portfolio/assets/fund", []byte(`{"name":"Fund","category":"fund","value":{"minor":"10000","currency":"CNY"},"value_as_of":"2026-08-28T00:00:00Z"}`))
	writeRec := httptest.NewRecorder()
	handler.ServeHTTP(writeRec, writeReq)
	if writeRec.Code != http.StatusOK {
		t.Fatalf("editor mutation status=%d body=%s", writeRec.Code, writeRec.Body.String())
	}
}
