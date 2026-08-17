package server

import (
	"context"
	"net/http"
)

type ReadyCheck func(context.Context) error

// WithReady adds a dependency-readiness endpoint. Apply it after WithWeb so
// the readiness wrapper can preserve the configured SPA fallback.
func WithReady(check ReadyCheck) HandlerOption {
	return func(cfg *handlerConfig) {
		cfg.web = readinessHandler{check: check, fallback: cfg.web}
	}
}

type readinessHandler struct {
	check    ReadyCheck
	fallback http.Handler
}

func (h readinessHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/readyz" {
		if h.fallback != nil {
			h.fallback.ServeHTTP(w, r)
			return
		}
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeAPIError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if h.check == nil || h.check(r.Context()) != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"service": "finance-core",
			"status":  "not_ready",
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"service": "finance-core",
		"status":  "ready",
	})
}
