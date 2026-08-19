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
)

func TestPortfolioAssetsHTTPCRUDContract(t *testing.T) {
	asOf := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	fake := &portfolioHTTPFake{
		listResponse: PortfolioAssetsResponse{Items: []PortfolioAssetResponse{
			{
				AssetRef:       "property:home",
				Name:           "Home",
				AssetClass:     "property",
				ValueMinor:     500_000,
				Currency:       "CNY",
				SourceCurrency: "CNY",
				ValuationAsOf:  asOf,
				SourceKind:     "manual",
			},
		}},
		upsertResponse: PortfolioAssetResponse{
			AssetRef:       "property:home",
			Name:           "Home",
			AssetClass:     "property",
			ValueMinor:     500_000,
			Currency:       "CNY",
			SourceCurrency: "CNY",
			ValuationAsOf:  asOf,
			SourceKind:     "manual",
		},
	}
	handler := NewHandler(WithAPI(fake))

	t.Run("GET", func(t *testing.T) {
		resp := httptest.NewRecorder()
		handler.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/api/v1/portfolio/assets?household_id=42", nil))
		if resp.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", resp.Code, resp.Body.String())
		}
		if fake.listCalls != 1 || fake.lastHouseholdID != 42 {
			t.Fatalf("list calls/scope=%d/%d want 1/42", fake.listCalls, fake.lastHouseholdID)
		}
		if got := resp.Header().Get("Cache-Control"); got != "no-store" {
			t.Fatalf("Cache-Control=%q want no-store", got)
		}
		var got PortfolioAssetsResponse
		if err := json.Unmarshal(resp.Body.Bytes(), &got); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if len(got.Items) != 1 || got.Items[0].AssetRef != "property:home" || got.Items[0].ValueMinor != 500_000 {
			t.Fatalf("response=%#v", got)
		}
		if !bytes.Contains(resp.Body.Bytes(), []byte(`"value_minor":"500000"`)) {
			t.Fatalf("value_minor must remain JSON string: %s", resp.Body.String())
		}
	})

	t.Run("PUT", func(t *testing.T) {
		body := `{"name":"Home","asset_class":"property","value_minor":"500000","currency":"CNY","source_currency":"CNY","valuation_as_of":"2026-08-18T12:00:00Z","source_kind":"manual"}`
		resp := httptest.NewRecorder()
		handler.ServeHTTP(resp, httptest.NewRequest(http.MethodPut, "/api/v1/portfolio/assets/property:home?household_id=42", strings.NewReader(body)))
		if resp.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", resp.Code, resp.Body.String())
		}
		if fake.upsertCalls != 1 || fake.lastHouseholdID != 42 || fake.lastAssetRef != "property:home" {
			t.Fatalf("upsert calls/scope/ref=%d/%d/%q", fake.upsertCalls, fake.lastHouseholdID, fake.lastAssetRef)
		}
		if fake.lastRequest.Name != "Home" || fake.lastRequest.AssetClass != "property" || fake.lastRequest.ValueMinor != 500_000 || !fake.lastRequest.ValuationAsOf.Equal(asOf) {
			t.Fatalf("request=%#v", fake.lastRequest)
		}
	})

	t.Run("DELETE", func(t *testing.T) {
		resp := httptest.NewRecorder()
		handler.ServeHTTP(resp, httptest.NewRequest(http.MethodDelete, "/api/v1/portfolio/assets/property:home?household_id=42", nil))
		if resp.Code != http.StatusNoContent {
			t.Fatalf("status=%d body=%s", resp.Code, resp.Body.String())
		}
		if resp.Body.Len() != 0 {
			t.Fatalf("DELETE body=%q want empty", resp.Body.String())
		}
		if fake.deleteCalls != 1 || fake.lastHouseholdID != 42 || fake.lastAssetRef != "property:home" {
			t.Fatalf("delete calls/scope/ref=%d/%d/%q", fake.deleteCalls, fake.lastHouseholdID, fake.lastAssetRef)
		}
		if got := resp.Header().Get("Cache-Control"); got != "no-store" {
			t.Fatalf("Cache-Control=%q want no-store", got)
		}
	})
}

func TestPortfolioAssetsHTTPRejectsInvalidRequestsBeforeBackend(t *testing.T) {
	fake := &portfolioHTTPFake{}
	handler := NewHandler(WithAPI(fake))

	tests := []struct {
		name   string
		method string
		target string
		body   string
	}{
		{name: "missing household", method: http.MethodGet, target: "/api/v1/portfolio/assets"},
		{name: "invalid household", method: http.MethodGet, target: "/api/v1/portfolio/assets?household_id=0"},
		{name: "blank asset ref", method: http.MethodPut, target: "/api/v1/portfolio/assets/%20%20?household_id=42", body: `{"name":"Home","asset_class":"property","value_minor":"1","currency":"CNY","source_currency":"CNY","valuation_as_of":"2026-08-18T12:00:00Z","source_kind":"manual"}`},
		{name: "unknown field", method: http.MethodPut, target: "/api/v1/portfolio/assets/property:home?household_id=42", body: `{"name":"Home","asset_class":"property","value_minor":"1","currency":"CNY","source_currency":"CNY","valuation_as_of":"2026-08-18T12:00:00Z","source_kind":"manual","raw_sql":"select *"}`},
		{name: "oversized body", method: http.MethodPut, target: "/api/v1/portfolio/assets/property:home?household_id=42", body: `{"name":"` + strings.Repeat("x", maxAPIRequestBytes+1) + `"}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp := httptest.NewRecorder()
			handler.ServeHTTP(resp, httptest.NewRequest(tc.method, tc.target, strings.NewReader(tc.body)))
			if resp.Code != http.StatusBadRequest {
				t.Fatalf("status=%d body=%s", resp.Code, resp.Body.String())
			}
		})
	}
	if fake.listCalls != 0 || fake.upsertCalls != 0 || fake.deleteCalls != 0 {
		t.Fatalf("backend calls list/upsert/delete=%d/%d/%d want 0", fake.listCalls, fake.upsertCalls, fake.deleteCalls)
	}
}

func TestPortfolioAssetsHTTPDoesNotLeakBackendErrors(t *testing.T) {
	fake := &portfolioHTTPFake{listErr: errors.New("SECRET_PORTFOLIO_FAILURE")}
	handler := NewHandler(WithAPI(fake))
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/api/v1/portfolio/assets?household_id=42", nil))
	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s", resp.Code, resp.Body.String())
	}
	if strings.Contains(resp.Body.String(), "SECRET_PORTFOLIO_FAILURE") {
		t.Fatalf("backend error leaked: %s", resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "internal_error") {
		t.Fatalf("missing stable error code: %s", resp.Body.String())
	}
}

type portfolioHTTPFake struct {
	fakeFinanceAPI
	listResponse    PortfolioAssetsResponse
	listErr         error
	upsertResponse  PortfolioAssetResponse
	upsertErr       error
	deleteErr       error
	listCalls       int
	upsertCalls     int
	deleteCalls     int
	lastHouseholdID int64
	lastAssetRef    string
	lastRequest     PortfolioAssetUpsertRequest
}

func (f *portfolioHTTPFake) ListPortfolioAssets(_ context.Context, householdID int64) (PortfolioAssetsResponse, error) {
	f.listCalls++
	f.lastHouseholdID = householdID
	return f.listResponse, f.listErr
}

func (f *portfolioHTTPFake) UpsertPortfolioAsset(_ context.Context, householdID int64, assetRef string, request PortfolioAssetUpsertRequest) (PortfolioAssetResponse, error) {
	f.upsertCalls++
	f.lastHouseholdID = householdID
	f.lastAssetRef = assetRef
	f.lastRequest = request
	return f.upsertResponse, f.upsertErr
}

func (f *portfolioHTTPFake) DeletePortfolioAsset(_ context.Context, householdID int64, assetRef string) error {
	f.deleteCalls++
	f.lastHouseholdID = householdID
	f.lastAssetRef = assetRef
	return f.deleteErr
}
