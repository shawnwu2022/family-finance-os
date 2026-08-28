package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeDataQualityAPI struct {
	FinanceAPI
	gotHouseholdID int64
	gotPeriod      string
}

func (f *fakeDataQualityAPI) DataQuality(_ context.Context, householdID int64, period string) (DataQualityResponse, error) {
	f.gotHouseholdID = householdID
	f.gotPeriod = period
	return DataQualityResponse{
		Period:              period,
		Quality:             "review",
		CheckedTransactions: 3,
		IssueCount:          1,
		DuplicateGroupCount: 1,
	}, nil
}

func TestDataQualityRoute(t *testing.T) {
	api := &fakeDataQualityAPI{}
	mux := http.NewServeMux()
	registerDataQualityAPI(mux, api)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/data-quality?household_id=42&period=2026-08", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if api.gotHouseholdID != 42 || api.gotPeriod != "2026-08" {
		t.Fatalf("backend args household=%d period=%q", api.gotHouseholdID, api.gotPeriod)
	}
	var got DataQualityResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Quality != "review" || got.CheckedTransactions != 3 || got.IssueCount != 1 || got.DuplicateGroupCount != 1 {
		t.Fatalf("unexpected response: %#v", got)
	}
}

func TestDataQualityRouteRejectsInvalidPeriod(t *testing.T) {
	api := &fakeDataQualityAPI{}
	mux := http.NewServeMux()
	registerDataQualityAPI(mux, api)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/data-quality?household_id=42&period=2026-8", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if api.gotPeriod != "" {
		t.Fatalf("backend must not run for invalid period: %q", api.gotPeriod)
	}
}
