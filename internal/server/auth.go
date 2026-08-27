package server

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"
	"time"

	financeauth "github.com/shawnwu2022/family-finance-os/internal/auth"
	"github.com/shawnwu2022/family-finance-os/internal/requestscope"
)

const SessionCookieName = "__Host-finance_session"

// BrowserAuth is the application-owned browser authentication boundary. MCP
// authentication remains a separate bearer-token path and does not implement it.
type BrowserAuth interface {
	BeginLogin(context.Context, string, string, time.Time) (financeauth.LoginResult, error)
	ConfirmEnrollment(context.Context, string, string, time.Time) (financeauth.SessionIssue, error)
	VerifySecondFactor(context.Context, string, string, bool, time.Time) (financeauth.SessionIssue, error)
	AuthenticateSession(context.Context, string, time.Time) (financeauth.SessionIdentity, error)
	Logout(context.Context, string, time.Time) error
}

type authLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type authChallengeRequest struct {
	Challenge string `json:"challenge"`
	Code      string `json:"code"`
	Recovery  bool   `json:"recovery,omitempty"`
}

type authLoginResponse struct {
	Challenge  string `json:"challenge"`
	Step       string `json:"step"`
	TOTPSecret string `json:"totp_secret,omitempty"`
	OTPAuthURI string `json:"otpauth_uri,omitempty"`
}

type authSessionResponse struct {
	Authenticated bool     `json:"authenticated"`
	Username      string   `json:"username,omitempty"`
	HouseholdID   int64    `json:"household_id,omitempty"`
	CSRFToken     string   `json:"csrf_token,omitempty"`
	RecoveryCodes []string `json:"recovery_codes,omitempty"`
}

func registerBrowserAuth(mux *http.ServeMux, auth BrowserAuth) {
	if mux == nil || auth == nil {
		return
	}
	loginLimiter := newLoginThrottle(5, 5*time.Minute, 4096)

	mux.HandleFunc("GET /api/v1/auth/session", func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(SessionCookieName)
		if err != nil || strings.TrimSpace(cookie.Value) == "" {
			writeJSON(w, http.StatusOK, authSessionResponse{Authenticated: false})
			return
		}
		identity, err := auth.AuthenticateSession(r.Context(), cookie.Value, time.Now().UTC())
		if err != nil {
			writeJSON(w, http.StatusOK, authSessionResponse{Authenticated: false})
			return
		}
		writeJSON(w, http.StatusOK, authSessionResponse{
			Authenticated: true,
			Username:      identity.Username,
			HouseholdID:   identity.HouseholdID,
			CSRFToken:     identity.CSRFToken,
		})
	})

	mux.HandleFunc("POST /api/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		var request authLoginRequest
		if !decodeStrictJSON(w, r, &request) {
			return
		}
		request.Username = strings.TrimSpace(request.Username)
		normalizedUsername := strings.ToLower(request.Username)
		remoteHost := loginRemoteHostForAuth(r, auth)
		now := time.Now().UTC()
		if !loginLimiter.Allow(remoteHost, normalizedUsername, now) {
			w.Header().Set("Retry-After", "300")
			writeAPIError(w, http.StatusTooManyRequests, "rate_limited", "too many authentication attempts")
			return
		}
		result, err := auth.BeginLogin(r.Context(), request.Username, request.Password, now)
		if err != nil {
			if errors.Is(err, financeauth.ErrInvalidCredentials) {
				loginLimiter.RecordFailure(remoteHost, normalizedUsername, now)
				writeAPIError(w, http.StatusUnauthorized, "invalid_credentials", "invalid credentials")
				return
			}
			writeAPIError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}
		loginLimiter.RecordSuccess(remoteHost, normalizedUsername)
		writeJSON(w, http.StatusOK, authLoginResponse{
			Challenge:  result.ChallengeToken,
			Step:       result.Step,
			TOTPSecret: result.TOTPSecret,
			OTPAuthURI: result.OTPAuthURI,
		})
	})

	mux.HandleFunc("POST /api/v1/auth/totp/enroll/confirm", func(w http.ResponseWriter, r *http.Request) {
		var request authChallengeRequest
		if !decodeStrictJSON(w, r, &request) {
			return
		}
		request.Challenge = strings.TrimSpace(request.Challenge)
		request.Code = strings.TrimSpace(request.Code)
		if request.Challenge == "" || request.Code == "" {
			writeAPIError(w, http.StatusUnauthorized, "invalid_second_factor", "invalid second factor")
			return
		}
		issue, err := auth.ConfirmEnrollment(r.Context(), request.Challenge, request.Code, time.Now().UTC())
		if err != nil {
			writeSecondFactorError(w, err)
			return
		}
		writeSessionIssue(w, issue)
	})

	mux.HandleFunc("POST /api/v1/auth/verify", func(w http.ResponseWriter, r *http.Request) {
		var request authChallengeRequest
		if !decodeStrictJSON(w, r, &request) {
			return
		}
		request.Challenge = strings.TrimSpace(request.Challenge)
		request.Code = strings.TrimSpace(request.Code)
		if request.Challenge == "" || request.Code == "" {
			writeAPIError(w, http.StatusUnauthorized, "invalid_second_factor", "invalid second factor")
			return
		}
		issue, err := auth.VerifySecondFactor(r.Context(), request.Challenge, request.Code, request.Recovery, time.Now().UTC())
		if err != nil {
			writeSecondFactorError(w, err)
			return
		}
		writeSessionIssue(w, issue)
	})

	logout := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(SessionCookieName)
		if err != nil || strings.TrimSpace(cookie.Value) == "" {
			writeAPIError(w, http.StatusUnauthorized, "unauthenticated", "authentication required")
			return
		}
		if err := auth.Logout(r.Context(), cookie.Value, time.Now().UTC()); err != nil {
			writeAPIError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}
		clearSessionCookie(w)
		writeNoContent(w)
	})
	mux.Handle("POST /api/v1/auth/logout", requireBrowserAuth(auth, logout))
}

func requireBrowserAuth(auth BrowserAuth, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth == nil || next == nil {
			writeAPIError(w, http.StatusUnauthorized, "unauthenticated", "authentication required")
			return
		}
		cookie, err := r.Cookie(SessionCookieName)
		if err != nil || strings.TrimSpace(cookie.Value) == "" {
			writeAPIError(w, http.StatusUnauthorized, "unauthenticated", "authentication required")
			return
		}
		identity, err := auth.AuthenticateSession(r.Context(), cookie.Value, time.Now().UTC())
		if err != nil || identity.UserID <= 0 || identity.HouseholdID <= 0 || strings.TrimSpace(identity.CSRFToken) == "" {
			writeAPIError(w, http.StatusUnauthorized, "unauthenticated", "authentication required")
			return
		}
		if isUnsafeAPIRequest(r) && !constantTimeEqual(strings.TrimSpace(r.Header.Get("X-CSRF-Token")), identity.CSRFToken) {
			writeAPIError(w, http.StatusForbidden, "invalid_csrf", "invalid CSRF token")
			return
		}
		ctx := requestscope.WithUserID(r.Context(), identity.UserID)
		ctx = requestscope.WithHouseholdID(ctx, identity.HouseholdID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func writeSecondFactorError(w http.ResponseWriter, err error) {
	if errors.Is(err, financeauth.ErrInvalidSecondFactor) {
		writeAPIError(w, http.StatusUnauthorized, "invalid_second_factor", "invalid second factor")
		return
	}
	writeAPIError(w, http.StatusInternalServerError, "internal_error", "internal server error")
}

func writeSessionIssue(w http.ResponseWriter, issue financeauth.SessionIssue) {
	if strings.TrimSpace(issue.SessionToken) == "" || strings.TrimSpace(issue.CSRFToken) == "" {
		writeAPIError(w, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}
	setSessionCookie(w, issue.SessionToken)
	writeJSON(w, http.StatusOK, authSessionResponse{
		Authenticated: true,
		CSRFToken:     issue.CSRFToken,
		RecoveryCodes: issue.RecoveryCodes,
	})
}

func setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
}

func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
		Expires:  time.Unix(1, 0).UTC(),
	})
}

func constantTimeEqual(left, right string) bool {
	if len(left) != len(right) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}

func applicationSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := w.Header()
		header.Set("Content-Security-Policy", "default-src 'self'; base-uri 'self'; object-src 'none'; frame-ancestors 'none'; form-action 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; manifest-src 'self'; worker-src 'self'")
		header.Set("X-Frame-Options", "DENY")
		header.Set("X-Content-Type-Options", "nosniff")
		header.Set("Referrer-Policy", "no-referrer")
		header.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=(), usb=()")
		next.ServeHTTP(w, r)
	})
}
