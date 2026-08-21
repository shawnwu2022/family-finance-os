package server

import (
	"mime"
	"net/http"
	"net/url"
	"strings"
)

func secureAPIRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isUnsafeAPIRequest(r) {
			if !sameOriginOrAbsent(r) {
				writeAPIError(w, http.StatusForbidden, "forbidden", "request origin is not allowed")
				return
			}
			if raw := strings.TrimSpace(r.Header.Get("Content-Type")); raw != "" {
				mediaType, _, err := mime.ParseMediaType(raw)
				if err != nil || !strings.EqualFold(mediaType, "application/json") {
					writeAPIError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", "content type must be application/json")
					return
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

func isUnsafeAPIRequest(r *http.Request) bool {
	if r == nil || r.URL == nil || !strings.HasPrefix(r.URL.Path, "/api/") {
		return false
	}
	switch r.Method {
	case http.MethodPost, http.MethodPut, http.MethodPatch:
		return true
	default:
		return false
	}
}

func sameOriginOrAbsent(r *http.Request) bool {
	rawOrigin := strings.TrimSpace(r.Header.Get("Origin"))
	if rawOrigin == "" {
		return true
	}
	origin, err := url.Parse(rawOrigin)
	if err != nil || (origin.Scheme != "http" && origin.Scheme != "https") || origin.Host == "" || origin.User != nil || origin.Path != "" || origin.RawQuery != "" || origin.Fragment != "" {
		return false
	}

	scheme := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto"))
	if scheme == "" {
		if r.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}
	if scheme != "http" && scheme != "https" {
		return false
	}
	host := strings.TrimSpace(r.Host)
	if host == "" {
		return false
	}
	return strings.EqualFold(origin.Scheme, scheme) && strings.EqualFold(origin.Host, host)
}
