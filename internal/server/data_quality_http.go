package server

import (
	"context"
	"net/http"
)

type DataQualityAPI interface {
	DataQuality(ctx context.Context, householdID int64, period string) (DataQualityResponse, error)
}

func registerDataQualityAPI(mux *http.ServeMux, api FinanceAPI) {
	dataQualityAPI, ok := api.(DataQualityAPI)
	if !ok {
		return
	}
	mux.HandleFunc("GET /api/v1/data-quality", func(w http.ResponseWriter, r *http.Request) {
		householdID, ok := parseHouseholdID(w, r)
		if !ok {
			return
		}
		period, ok := parsePeriod(w, r.URL.Query().Get("period"))
		if !ok {
			return
		}
		response, err := dataQualityAPI.DataQuality(r.Context(), householdID, period)
		writeBackendResult(w, response, err)
	})
}
