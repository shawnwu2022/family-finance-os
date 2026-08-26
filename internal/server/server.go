package server

import (
	"encoding/json"
	"net/http"
)

func NewHandler(options ...HandlerOption) http.Handler {
	cfg := handlerConfig{}
	for _, option := range options {
		if option != nil {
			option(&cfg)
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"service": "finance-core",
			"status":  "ok",
		})
	})

	apiMux := http.NewServeMux()
	registerFinanceAPI(apiMux, cfg.api)
	registerPortfolioFinanceAPI(apiMux, cfg.api)
	if cfg.auth != nil {
		registerBrowserAuth(mux, cfg.auth)
		mux.Handle("/api/v1/", requireBrowserAuth(cfg.auth, apiMux))
	} else {
		mux.Handle("/api/v1/", apiMux)
	}
	if cfg.mcp != nil {
		mux.Handle("/mcp", cfg.mcp)
	}
	if cfg.web != nil {
		mux.Handle("/", cfg.web)
	}
	return applicationSecurityHeaders(secureAPIRequests(mux))
}
