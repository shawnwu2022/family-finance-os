package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	financeauth "github.com/shawnwu2022/family-finance-os/internal/auth"
)

type authTestFinanceAPI struct{}

func (authTestFinanceAPI) Dashboard(context.Context, int64, string) (DashboardResponse, error) {
	return DashboardResponse{}, nil
}
func (authTestFinanceAPI) Overview(context.Context, int64) (OverviewResponse, error) {
	return OverviewResponse{}, nil
}
func (authTestFinanceAPI) Cashflow(context.Context, int64, string) (CashflowResponse, error) {
	return CashflowResponse{}, nil
}
func (authTestFinanceAPI) Budget(context.Context, int64, string) (BudgetResponse, error) {
	return BudgetResponse{}, nil
}
func (authTestFinanceAPI) Debts(context.Context, int64) (DebtsResponse, error) {
	return DebtsResponse{}, nil
}
func (authTestFinanceAPI) Goals(context.Context, int64) (GoalsResponse, error) {
	return GoalsResponse{}, nil
}
func (authTestFinanceAPI) Scenario(context.Context, ScenarioRequest) (ScenarioResponse, error) {
	return ScenarioResponse{}, nil
}
func (authTestFinanceAPI) Advisor(context.Context, AdvisorRequest) (AdvisorResponse, error) {
	return AdvisorResponse{}, nil
}
func (authTestFinanceAPI) Reports(context.Context, int64) (ReportsResponse, error) {
	return ReportsResponse{}, nil
}

type fakeBrowserAuth struct {
	beginLogin        func(context.Context, string, string, time.Time) (financeauth.LoginResult, error)
	confirmEnrollment func(context.Context, string, string, time.Time) (financeauth.SessionIssue, error)
	verify            func(context.Context, string, string, bool, time.Time) (financeauth.SessionIssue, error)
	authenticate      func(context.Context, string, time.Time) (financeauth.SessionIdentity, error)
	logout            func(context.Context, string, time.Time) error
}

func (f fakeBrowserAuth) BeginLogin(ctx context.Context, username, password string, now time.Time) (financeauth.LoginResult, error) {
	if f.beginLogin == nil {
		return financeauth.LoginResult{}, financeauth.ErrInvalidCredentials
	}
	return f.beginLogin(ctx, username, password, now)
}
func (f fakeBrowserAuth) ConfirmEnrollment(ctx context.Context, challenge, code string, now time.Time) (financeauth.SessionIssue, error) {
	if f.confirmEnrollment == nil {
		return financeauth.SessionIssue{}, financeauth.ErrInvalidSecondFactor
	}
	return f.confirmEnrollment(ctx, challenge, code, now)
}
func (f fakeBrowserAuth) VerifySecondFactor(ctx context.Context, challenge, code string, recovery bool, now time.Time) (financeauth.SessionIssue, error) {
	if f.verify == nil {
		return financeauth.SessionIssue{}, financeauth.ErrInvalidSecondFactor
	}
	return f.verify(ctx, challenge, code, recovery, now)
}
func (f fakeBrowserAuth) AuthenticateSession(ctx context.Context, token string, now time.Time) (financeauth.SessionIdentity, error) {
	if f.authenticate == nil {
		return financeauth.SessionIdentity{}, financeauth.ErrUnauthenticated
	}
	identity, err := f.authenticate(ctx, token, now)
	if err == nil && identity.Role == "" {
		identity.Role = financeauth.RoleOwner
	}
	return identity, err
}
func (f fakeBrowserAuth) Logout(ctx context.Context, token string, now time.Time) error {
	if f.logout == nil {
		return nil
	}
	return f.logout(ctx, token, now)
}

func authenticatedFake() fakeBrowserAuth {
	return fakeBrowserAuth{
		authenticate: func(_ context.Context, token string, _ time.Time) (financeauth.SessionIdentity, error) {
			if token != "valid-session" {
				return financeauth.SessionIdentity{}, financeauth.ErrUnauthenticated
			}
			return financeauth.SessionIdentity{UserID: 11, Username: "owner", HouseholdID: 7, CSRFToken: "csrf-token"}, nil
		},
	}
}

func TestProtectedFinanceAPIRejectsMissingSessionEvenWithBearer(t *testing.T) {
	handler := NewHandler(WithAPI(authTestFinanceAPI{}), WithBrowserAuth(authenticatedFake()))
	for _, bearer := range []string{"", "Bearer mcp-token"} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard?period=2026-08", nil)
		if bearer != "" {
			req.Header.Set("Authorization", bearer)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("bearer=%q status=%d want=401 body=%s", bearer, rec.Code, rec.Body.String())
		}
	}
}

func TestAuthenticatedHouseholdCannotBeOverriddenByClient(t *testing.T) {
	handler := NewHandler(WithAPI(authTestFinanceAPI{}), WithBrowserAuth(authenticatedFake()))

	queryReq := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard?household_id=8&period=2026-08", nil)
	queryReq.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "valid-session"})
	queryRec := httptest.NewRecorder()
	handler.ServeHTTP(queryRec, queryReq)
	if queryRec.Code != http.StatusBadRequest {
		t.Fatalf("query override status=%d want=400 body=%s", queryRec.Code, queryRec.Body.String())
	}

	body := []byte(`{"household_id":8,"question":"test","require_tool":true}`)
	bodyReq := httptest.NewRequest(http.MethodPost, "/api/v1/advisor", bytes.NewReader(body))
	bodyReq.Host = "finance.example.test"
	bodyReq.Header.Set("Content-Type", "application/json")
	bodyReq.Header.Set("Origin", "http://finance.example.test")
	bodyReq.Header.Set("X-CSRF-Token", "csrf-token")
	bodyReq.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "valid-session"})
	bodyRec := httptest.NewRecorder()
	handler.ServeHTTP(bodyRec, bodyReq)
	if bodyRec.Code != http.StatusBadRequest {
		t.Fatalf("body override status=%d want=400 body=%s", bodyRec.Code, bodyRec.Body.String())
	}
}

func TestUnsafeFinanceRequestRequiresSessionCSRF(t *testing.T) {
	handler := NewHandler(WithAPI(authTestFinanceAPI{}), WithBrowserAuth(authenticatedFake()))
	body := []byte(`{"question":"test","require_tool":true}`)
	cases := []struct {
		name string
		csrf string
		want int
	}{
		{name: "missing", want: http.StatusForbidden},
		{name: "wrong", csrf: "wrong", want: http.StatusForbidden},
		{name: "valid", csrf: "csrf-token", want: http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/advisor", bytes.NewReader(body))
			req.Host = "finance.example.test"
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Origin", "http://finance.example.test")
			req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "valid-session"})
			if tc.csrf != "" {
				req.Header.Set("X-CSRF-Token", tc.csrf)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status=%d want=%d body=%s", rec.Code, tc.want, rec.Body.String())
			}
		})
	}
}

func TestSessionCookieAttributesAndSessionEndpoint(t *testing.T) {
	fake := authenticatedFake()
	fake.verify = func(context.Context, string, string, bool, time.Time) (financeauth.SessionIssue, error) {
		return financeauth.SessionIssue{SessionToken: "valid-session", CSRFToken: "csrf-token"}, nil
	}
	handler := NewHandler(WithAPI(authTestFinanceAPI{}), WithBrowserAuth(fake))
	body := strings.NewReader(`{"challenge":"challenge","code":"123456","recovery":false}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/verify", body)
	req.Host = "finance.example.test"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://finance.example.test")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("verify status=%d body=%s", rec.Code, rec.Body.String())
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies=%d want=1", len(cookies))
	}
	cookie := cookies[0]
	if cookie.Name != SessionCookieName || cookie.Value != "valid-session" || !cookie.Secure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Path != "/" || cookie.Domain != "" {
		t.Fatalf("session cookie = %#v", cookie)
	}

	sessionReq := httptest.NewRequest(http.MethodGet, "/api/v1/auth/session", nil)
	sessionReq.AddCookie(cookie)
	sessionRec := httptest.NewRecorder()
	handler.ServeHTTP(sessionRec, sessionReq)
	if sessionRec.Code != http.StatusOK {
		t.Fatalf("session status=%d body=%s", sessionRec.Code, sessionRec.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(sessionRec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode session response: %v", err)
	}
	if got["authenticated"] != true || got["csrf_token"] != "csrf-token" || got["role"] != "owner" {
		t.Fatalf("session response = %#v", got)
	}
}

func TestBrowserCookieDoesNotAuthenticateMCP(t *testing.T) {
	mcp := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer mcp-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	handler := NewHandler(WithAPI(authTestFinanceAPI{}), WithBrowserAuth(authenticatedFake()), WithMCP(mcp))

	cookieOnly := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	cookieOnly.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "valid-session"})
	cookieRec := httptest.NewRecorder()
	handler.ServeHTTP(cookieRec, cookieOnly)
	if cookieRec.Code != http.StatusUnauthorized {
		t.Fatalf("cookie-only MCP status=%d want=401", cookieRec.Code)
	}

	bearer := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	bearer.Header.Set("Authorization", "Bearer mcp-token")
	bearerRec := httptest.NewRecorder()
	handler.ServeHTTP(bearerRec, bearer)
	if bearerRec.Code != http.StatusNoContent {
		t.Fatalf("bearer MCP status=%d want=204", bearerRec.Code)
	}
}

func TestHealthzRemainsPublicMinimalAndSecurityHeadersAreApplicationNative(t *testing.T) {
	handler := NewHandler(WithAPI(authTestFinanceAPI{}), WithBrowserAuth(authenticatedFake()))
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want=200", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "household") || strings.Contains(rec.Body.String(), "database") {
		t.Fatalf("health body discloses details: %s", rec.Body.String())
	}
	if got := rec.Header().Get("X-Frame-Options"); got != "DENY" {
		t.Fatalf("X-Frame-Options=%q want DENY", got)
	}
	if got := rec.Header().Get("Content-Security-Policy"); !strings.Contains(got, "frame-ancestors 'none'") {
		t.Fatalf("CSP=%q", got)
	}
}

func TestAuthEndpointUsesGenericCredentialFailure(t *testing.T) {
	fake := authenticatedFake()
	fake.beginLogin = func(context.Context, string, string, time.Time) (financeauth.LoginResult, error) {
		return financeauth.LoginResult{}, financeauth.ErrInvalidCredentials
	}
	handler := NewHandler(WithAPI(authTestFinanceAPI{}), WithBrowserAuth(fake))
	for _, username := range []string{"missing", "owner"} {
		body, _ := json.Marshal(map[string]string{"username": username, "password": "wrong"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
		req.Host = "finance.example.test"
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", "http://finance.example.test")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("username=%q status=%d body=%s", username, rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "invalid_credentials") || strings.Contains(rec.Body.String(), username) {
			t.Fatalf("username=%q leaked detail: %s", username, rec.Body.String())
		}
	}
}

func TestAuthMiddlewareMapsBackendFailureToUnauthenticated(t *testing.T) {
	fake := authenticatedFake()
	fake.authenticate = func(context.Context, string, time.Time) (financeauth.SessionIdentity, error) {
		return financeauth.SessionIdentity{}, errors.New("database unavailable")
	}
	handler := NewHandler(WithAPI(authTestFinanceAPI{}), WithBrowserAuth(fake))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard?period=2026-08", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "valid-session"})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want=401 body=%s", rec.Code, rec.Body.String())
	}
}
