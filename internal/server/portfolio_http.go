package server

import (
	"context"
	"net/http"
	"strings"
)

// PortfolioFinanceAPI extends the core Finance API with household-scoped
// portfolio snapshot mutations. Keeping the extension explicit lets existing
// read-only wrappers remain narrow while production appapi.API supplies both.
type PortfolioFinanceAPI interface {
	FinanceAPI
	ListPortfolioAssets(context.Context, int64) (PortfolioAssetsResponse, error)
	UpsertPortfolioAsset(context.Context, int64, string, PortfolioAssetUpsertRequest) (PortfolioAssetResponse, error)
	DeletePortfolioAsset(context.Context, int64, string) error
}

func registerPortfolioFinanceAPI(mux *http.ServeMux, api FinanceAPI) {
	portfolioAPI, ok := api.(PortfolioFinanceAPI)
	if !ok {
		return
	}

	mux.HandleFunc("GET /api/v1/portfolio/assets", func(w http.ResponseWriter, r *http.Request) {
		householdID, ok := parseHouseholdID(w, r)
		if !ok {
			return
		}
		response, err := portfolioAPI.ListPortfolioAssets(r.Context(), householdID)
		writeBackendResult(w, response, err)
	})
	mux.HandleFunc("PUT /api/v1/portfolio/assets/{asset_ref}", func(w http.ResponseWriter, r *http.Request) {
		householdID, ok := parseHouseholdID(w, r)
		if !ok {
			return
		}
		assetRef := strings.TrimSpace(r.PathValue("asset_ref"))
		if assetRef == "" {
			writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid request")
			return
		}
		var request PortfolioAssetUpsertRequest
		if !decodeStrictJSON(w, r, &request) {
			return
		}
		response, err := portfolioAPI.UpsertPortfolioAsset(r.Context(), householdID, assetRef, request)
		writeBackendResult(w, response, err)
	})
	mux.HandleFunc("DELETE /api/v1/portfolio/assets/{asset_ref}", func(w http.ResponseWriter, r *http.Request) {
		householdID, ok := parseHouseholdID(w, r)
		if !ok {
			return
		}
		assetRef := strings.TrimSpace(r.PathValue("asset_ref"))
		if assetRef == "" {
			writeAPIError(w, http.StatusBadRequest, "invalid_request", "invalid request")
			return
		}
		if err := portfolioAPI.DeletePortfolioAsset(r.Context(), householdID, assetRef); err != nil {
			writeAPIError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}
		writeNoContent(w)
	})
}

func writeNoContent(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusNoContent)
}
