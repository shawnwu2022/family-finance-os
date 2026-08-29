package server

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	financeauth "github.com/shawnwu2022/family-finance-os/internal/auth"
)

type fakeHouseholdMemberAuth struct {
	fakeBrowserAuth
	list    func(context.Context, int64) ([]financeauth.HouseholdMember, error)
	create  func(context.Context, int64, financeauth.CreateHouseholdMemberInput) (financeauth.HouseholdMember, error)
	update  func(context.Context, int64, int64, financeauth.Role, time.Time) (financeauth.HouseholdMember, error)
	disable func(context.Context, int64, int64, time.Time) (financeauth.HouseholdMember, error)
	enable  func(context.Context, int64, int64, time.Time) (financeauth.HouseholdMember, error)
}

func (f fakeHouseholdMemberAuth) ListHouseholdMembers(ctx context.Context, householdID int64) ([]financeauth.HouseholdMember, error) {
	if f.list == nil {
		return nil, nil
	}
	return f.list(ctx, householdID)
}
func (f fakeHouseholdMemberAuth) CreateHouseholdMember(ctx context.Context, householdID int64, input financeauth.CreateHouseholdMemberInput) (financeauth.HouseholdMember, error) {
	if f.create == nil {
		return financeauth.HouseholdMember{}, nil
	}
	return f.create(ctx, householdID, input)
}
func (f fakeHouseholdMemberAuth) UpdateHouseholdMemberRole(ctx context.Context, householdID, userID int64, role financeauth.Role, now time.Time) (financeauth.HouseholdMember, error) {
	if f.update == nil {
		return financeauth.HouseholdMember{}, nil
	}
	return f.update(ctx, householdID, userID, role, now)
}
func (f fakeHouseholdMemberAuth) DisableHouseholdMember(ctx context.Context, householdID, userID int64, now time.Time) (financeauth.HouseholdMember, error) {
	if f.disable == nil {
		return financeauth.HouseholdMember{}, nil
	}
	return f.disable(ctx, householdID, userID, now)
}
func (f fakeHouseholdMemberAuth) EnableHouseholdMember(ctx context.Context, householdID, userID int64, now time.Time) (financeauth.HouseholdMember, error) {
	if f.enable == nil {
		return financeauth.HouseholdMember{}, nil
	}
	return f.enable(ctx, householdID, userID, now)
}

func householdAuthForRole(role financeauth.Role) fakeHouseholdMemberAuth {
	return fakeHouseholdMemberAuth{fakeBrowserAuth: roleAuthenticatedFake(role)}
}

func TestHouseholdMemberManagementIsOwnerOnly(t *testing.T) {
	for _, role := range []financeauth.Role{financeauth.RoleEditor, financeauth.RoleViewer} {
		t.Run(string(role), func(t *testing.T) {
			handler := NewHandler(WithAPI(authTestFinanceAPI{}), WithBrowserAuth(householdAuthForRole(role)))
			req := authenticatedRequest(http.MethodGet, "/api/v1/household/members", nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusForbidden || !bytes.Contains(rec.Body.Bytes(), []byte(`"code":"insufficient_role"`)) {
				t.Fatalf("role=%s status=%d body=%s", role, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestTrustedProxyConfigurationPreservesHouseholdMemberRoutes(t *testing.T) {
	auth := householdAuthForRole(financeauth.RoleOwner)
	handler := NewHandler(
		WithAPI(authTestFinanceAPI{}),
		WithBrowserAuth(auth),
		WithTrustedProxyCIDR("172.30.0.10/32"),
	)
	req := authenticatedRequest(http.MethodGet, "/api/v1/household/members", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want=200 body=%s", rec.Code, rec.Body.String())
	}
}

func TestOwnerCanListCreateAndChangeHouseholdMembers(t *testing.T) {
	auth := householdAuthForRole(financeauth.RoleOwner)
	auth.list = func(_ context.Context, householdID int64) ([]financeauth.HouseholdMember, error) {
		if householdID != 7 {
			t.Fatalf("householdID=%d want=7", householdID)
		}
		return []financeauth.HouseholdMember{{UserID: 11, Username: "owner", Role: financeauth.RoleOwner, TOTPEnrolled: true}}, nil
	}
	auth.create = func(_ context.Context, householdID int64, input financeauth.CreateHouseholdMemberInput) (financeauth.HouseholdMember, error) {
		if householdID != 7 || input.Username != "partner" || input.Password != "correct horse battery staple" || input.Role != financeauth.RoleEditor {
			t.Fatalf("unexpected create: household=%d input=%#v", householdID, input)
		}
		return financeauth.HouseholdMember{UserID: 12, Username: "partner", Role: financeauth.RoleEditor}, nil
	}
	auth.update = func(_ context.Context, householdID, userID int64, role financeauth.Role, _ time.Time) (financeauth.HouseholdMember, error) {
		if householdID != 7 || userID != 12 || role != financeauth.RoleViewer {
			t.Fatalf("unexpected update: household=%d user=%d role=%s", householdID, userID, role)
		}
		return financeauth.HouseholdMember{UserID: 12, Username: "partner", Role: financeauth.RoleViewer}, nil
	}
	handler := NewHandler(WithAPI(authTestFinanceAPI{}), WithBrowserAuth(auth))

	listReq := authenticatedRequest(http.MethodGet, "/api/v1/household/members", nil)
	listRec := httptest.NewRecorder()
	handler.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK || !bytes.Contains(listRec.Body.Bytes(), []byte(`"role":"owner"`)) {
		t.Fatalf("list status=%d body=%s", listRec.Code, listRec.Body.String())
	}

	createReq := authenticatedRequest(http.MethodPost, "/api/v1/household/members", []byte(`{"username":"partner","password":"correct horse battery staple","role":"editor"}`))
	createRec := httptest.NewRecorder()
	handler.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated || !bytes.Contains(createRec.Body.Bytes(), []byte(`"user_id":12`)) {
		t.Fatalf("create status=%d body=%s", createRec.Code, createRec.Body.String())
	}

	updateReq := authenticatedRequest(http.MethodPatch, "/api/v1/household/members/12", []byte(`{"role":"viewer"}`))
	updateRec := httptest.NewRecorder()
	handler.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusOK || !bytes.Contains(updateRec.Body.Bytes(), []byte(`"role":"viewer"`)) {
		t.Fatalf("update status=%d body=%s", updateRec.Code, updateRec.Body.String())
	}
}

func TestLastOwnerConflictIsStable(t *testing.T) {
	auth := householdAuthForRole(financeauth.RoleOwner)
	auth.disable = func(context.Context, int64, int64, time.Time) (financeauth.HouseholdMember, error) {
		return financeauth.HouseholdMember{}, financeauth.ErrLastOwner
	}
	handler := NewHandler(WithAPI(authTestFinanceAPI{}), WithBrowserAuth(auth))
	req := authenticatedRequest(http.MethodDelete, "/api/v1/household/members/11", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict || !bytes.Contains(rec.Body.Bytes(), []byte(`"code":"last_owner_required"`)) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestViewerMutationStillRequiresCSRFFirst(t *testing.T) {
	handler := NewHandler(WithAPI(rbacPortfolioAPI{}), WithBrowserAuth(roleAuthenticatedFake(financeauth.RoleViewer)))
	req := authenticatedRequest(http.MethodDelete, "/api/v1/portfolio/assets/fund", nil)
	req.Header.Del("X-CSRF-Token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden || !bytes.Contains(rec.Body.Bytes(), []byte(`"code":"invalid_csrf"`)) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHouseholdMemberNotFoundMapsTo404(t *testing.T) {
	auth := householdAuthForRole(financeauth.RoleOwner)
	auth.update = func(context.Context, int64, int64, financeauth.Role, time.Time) (financeauth.HouseholdMember, error) {
		return financeauth.HouseholdMember{}, financeauth.ErrNotFound
	}
	handler := NewHandler(WithAPI(authTestFinanceAPI{}), WithBrowserAuth(auth))
	req := authenticatedRequest(http.MethodPatch, "/api/v1/household/members/999", []byte(`{"role":"viewer"}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound || !bytes.Contains(rec.Body.Bytes(), []byte(`"code":"not_found"`)) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestOwnerCanEnableDisabledHouseholdMember(t *testing.T) {
	auth := householdAuthForRole(financeauth.RoleOwner)
	auth.enable = func(_ context.Context, householdID, userID int64, _ time.Time) (financeauth.HouseholdMember, error) {
		if householdID != 7 || userID != 12 {
			t.Fatalf("unexpected enable: household=%d user=%d", householdID, userID)
		}
		return financeauth.HouseholdMember{UserID: 12, Username: "partner", Role: financeauth.RoleViewer}, nil
	}
	handler := NewHandler(WithAPI(authTestFinanceAPI{}), WithBrowserAuth(auth))
	req := authenticatedRequest(http.MethodPost, "/api/v1/household/members/12", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte(`"disabled":false`)) || !bytes.Contains(rec.Body.Bytes(), []byte(`"user_id":12`)) {
		t.Fatalf("enable status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestEnableHouseholdMemberNotFoundMapsTo404(t *testing.T) {
	auth := householdAuthForRole(financeauth.RoleOwner)
	auth.enable = func(context.Context, int64, int64, time.Time) (financeauth.HouseholdMember, error) {
		return financeauth.HouseholdMember{}, financeauth.ErrNotFound
	}
	handler := NewHandler(WithAPI(authTestFinanceAPI{}), WithBrowserAuth(auth))
	req := authenticatedRequest(http.MethodPost, "/api/v1/household/members/999", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound || !bytes.Contains(rec.Body.Bytes(), []byte(`"code":"not_found"`)) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHouseholdMemberBackendFailureDoesNotLeakDetails(t *testing.T) {
	auth := householdAuthForRole(financeauth.RoleOwner)
	auth.list = func(context.Context, int64) ([]financeauth.HouseholdMember, error) {
		return nil, errors.New("database details")
	}
	handler := NewHandler(WithAPI(authTestFinanceAPI{}), WithBrowserAuth(auth))
	req := authenticatedRequest(http.MethodGet, "/api/v1/household/members", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError || bytes.Contains(rec.Body.Bytes(), []byte("database details")) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
