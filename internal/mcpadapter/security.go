package mcpadapter

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
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

type fixedWindowLimiter struct {
	mu          sync.Mutex
	windowStart time.Time
	count       int
	limit       int
	now         func() time.Time
}

func NewSecureHTTPHandler(next http.Handler, opts SecurityOptions) (http.Handler, error) {
	return newSecureHTTPHandlerWithClock(next, opts, time.Now)
}

func newSecureHTTPHandlerWithClock(next http.Handler, opts SecurityOptions, now func() time.Time) (http.Handler, error) {
	if next == nil {
		return nil, fmt.Errorf("mcpadapter: secure HTTP handler is required")
	}
	if now == nil {
		return nil, fmt.Errorf("mcpadapter: security clock is required")
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

	trustedOrigins := make(map[string]struct{}, len(opts.AllowedOrigins))
	for _, origin := range opts.AllowedOrigins {
		canonical, err := canonicalOrigin(origin)
		if err != nil {
			return nil, fmt.Errorf("mcpadapter: invalid trusted origin: %w", err)
		}
		trustedOrigins[canonical] = struct{}{}
	}

	expected := sha256.Sum256(append([]byte(nil), opts.Token...))
	limiter := &fixedWindowLimiter{limit: opts.RequestsPerMinute, now: now}
	semaphore := make(chan struct{}, opts.MaxConcurrent)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !allowedRequestOrigin(r, trustedOrigins) {
			writeSecurityError(w, http.StatusForbidden, "forbidden", "MCP request origin is not allowed")
			return
		}
		if !authorizedBearer(r, expected) {
			w.Header().Set("WWW-Authenticate", "Bearer")
			writeSecurityError(w, http.StatusUnauthorized, "unauthorized", "MCP bearer token is invalid")
			return
		}
		if !limiter.allow() {
			writeSecurityError(w, http.StatusTooManyRequests, "busy", "MCP request rate limit exceeded")
			return
		}

		select {
		case semaphore <- struct{}{}:
			defer func() { <-semaphore }()
		default:
			writeSecurityError(w, http.StatusServiceUnavailable, "busy", "MCP endpoint is busy")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), opts.RequestTimeout)
		defer cancel()
		r = r.WithContext(ctx)

		if !limitRequestBody(w, r, opts.MaxBodyBytes) {
			return
		}
		next.ServeHTTP(w, r)
	}), nil
}

func (l *fixedWindowLimiter) allow() bool {
	window := l.now().UTC().Truncate(time.Minute)
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.windowStart.IsZero() || !l.windowStart.Equal(window) {
		l.windowStart = window
		l.count = 0
	}
	if l.count >= l.limit {
		return false
	}
	l.count++
	return true
}

func allowedRequestOrigin(r *http.Request, trusted map[string]struct{}) bool {
	raw := strings.TrimSpace(r.Header.Get("Origin"))
	if raw == "" {
		return true
	}
	origin, err := canonicalOrigin(raw)
	if err != nil {
		return false
	}
	if _, ok := trusted[origin]; ok {
		return true
	}
	requestOrigin, err := canonicalRequestOrigin(r)
	return err == nil && origin == requestOrigin
}

func canonicalRequestOrigin(r *http.Request) (string, error) {
	scheme := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto"))
	if scheme == "" {
		if r.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}
	if scheme != "http" && scheme != "https" {
		return "", fmt.Errorf("request scheme must be http or https")
	}
	host := strings.TrimSpace(r.Host)
	if host == "" {
		return "", fmt.Errorf("request host is required")
	}
	return strings.ToLower(scheme) + "://" + strings.ToLower(host), nil
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

func limitRequestBody(w http.ResponseWriter, r *http.Request, maxBytes int64) bool {
	if r.ContentLength > maxBytes {
		writeSecurityError(w, http.StatusRequestEntityTooLarge, "payload_too_large", "MCP request body is too large")
		return false
	}
	if r.Body == nil || r.Body == http.NoBody {
		return true
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, maxBytes+1))
	if err != nil {
		writeSecurityError(w, http.StatusBadRequest, "invalid_request", "MCP request body could not be read")
		return false
	}
	if int64(len(body)) > maxBytes {
		writeSecurityError(w, http.StatusRequestEntityTooLarge, "payload_too_large", "MCP request body is too large")
		return false
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	r.ContentLength = int64(len(body))
	return true
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
	return strings.ToLower(parsed.Scheme) + "://" + strings.ToLower(parsed.Host), nil
}

func writeSecurityError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(securityError{ErrorCode: code, Message: message})
}
