package mcpadapter

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type SecurityOptions struct {
	Token             []byte
	AllowedOrigins    []string
	RequestTimeout    time.Duration
	MaxConcurrent     int
	RequestsPerMinute int
	MaxBodyBytes      int64
}

type securityError struct {
	ErrorCode string `json:"error_code"`
	Message   string `json:"message"`
}

func NewSecureHTTPHandler(next http.Handler, opts SecurityOptions) (http.Handler, error) {
	if next == nil {
		return nil, fmt.Errorf("mcpadapter: secure HTTP handler is required")
	}
	if len(opts.Token) == 0 {
		return nil, fmt.Errorf("mcpadapter: MCP bearer token is required")
	}
	if opts.RequestTimeout <= 0 {
		return nil, fmt.Errorf("mcpadapter: MCP request timeout must be positive")
	}
	if opts.MaxConcurrent <= 0 {
		return nil, fmt.Errorf("mcpadapter: MCP max concurrency must be positive")
	}
	if opts.RequestsPerMinute <= 0 {
		return nil, fmt.Errorf("mcpadapter: MCP request rate must be positive")
	}
	if opts.MaxBodyBytes <= 0 {
		return nil, fmt.Errorf("mcpadapter: MCP request body limit must be positive")
	}
	for _, origin := range opts.AllowedOrigins {
		if _, err := canonicalOrigin(origin); err != nil {
			return nil, fmt.Errorf("mcpadapter: invalid trusted origin: %w", err)
		}
	}

	expected := sha256.Sum256(append([]byte(nil), opts.Token...))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !authorizedBearer(r, expected) {
			w.Header().Set("WWW-Authenticate", "Bearer")
			writeSecurityError(w, http.StatusUnauthorized, "unauthorized", "MCP bearer token is invalid")
			return
		}
		next.ServeHTTP(w, r)
	}), nil
}

func authorizedBearer(r *http.Request, expected [sha256.Size]byte) bool {
	values := r.Header.Values("Authorization")
	if len(values) != 1 {
		return false
	}
	parts := strings.Fields(values[0])
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return false
	}
	presented := sha256.Sum256([]byte(parts[1]))
	return subtle.ConstantTimeCompare(expected[:], presented[:]) == 1
}

func canonicalOrigin(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.EqualFold(raw, "null") {
		return "", fmt.Errorf("origin is required")
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parse origin: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("origin scheme must be http or https")
	}
	if parsed.Host == "" || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("origin must contain only scheme and authority")
	}
	return parsed.Scheme + "://" + parsed.Host, nil
}

func writeSecurityError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(securityError{ErrorCode: code, Message: message})
}
