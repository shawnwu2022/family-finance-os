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
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"service": "finance-core",
			"status":  "ok",
		})
	})
	registerFinanceAPI(mux, cfg.api)
	if cfg.web != nil {
		mux.Handle("/", cfg.web)
	}
	return mux
}
