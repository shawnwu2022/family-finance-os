package mcpadapter

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewSecureHTTPHandlerRejectsInvalidOptions(t *testing.T) {
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	valid := testSecurityOptions()
	tests := []struct {
		name string
		next http.Handler
		opts SecurityOptions
	}{
		{name: "nil handler", next: nil, opts: valid},
		{name: "empty token", next: next, opts: func() SecurityOptions { v := valid; v.Token = nil; return v }()},
		{name: "zero timeout", next: next, opts: func() SecurityOptions { v := valid; v.RequestTimeout = 0; return v }()},
		{name: "zero concurrency", next: next, opts: func() SecurityOptions { v := valid; v.MaxConcurrent = 0; return v }()},
		{name: "zero rate", next: next, opts: func() SecurityOptions { v := valid; v.RequestsPerMinute = 0; return v }()},
		{name: "zero body limit", next: next, opts: func() SecurityOptions { v := valid; v.MaxBodyBytes = 0; return v }()},
		{name: "malformed trusted origin", next: next, opts: func() SecurityOptions { v := valid; v.AllowedOrigins = []string{"not-an-origin"}; return v }()},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewSecureHTTPHandler(tc.next, tc.opts); err == nil {
				t.Fatal("NewSecureHTTPHandler accepted invalid options")
			}
		})
	}
}

func TestSecureHTTPHandlerRequiresBearerOnEveryMethod(t *testing.T) {
	calls := 0
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusNoContent)
	})
	handler, err := NewSecureHTTPHandler(next, testSecurityOptions())
	if err != nil {
		t.Fatalf("NewSecureHTTPHandler: %v", err)
	}

	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodDelete} {
		t.Run(method+" missing", func(t *testing.T) {
			assertUnauthorized(t, handler, method, nil)
		})
		t.Run(method+" basic", func(t *testing.T) {
			assertUnauthorized(t, handler, method, []string{"Basic Zm9vOmJhcg=="})
		})
		t.Run(method+" wrong bearer", func(t *testing.T) {
			assertUnauthorized(t, handler, method, []string{"Bearer wrong"})
		})
		t.Run(method+" duplicate authorization", func(t *testing.T) {
			assertUnauthorized(t, handler, method, []string{"Bearer correct-horse-battery-staple", "Bearer correct-horse-battery-staple"})
		})
	}

	request := httptest.NewRequest(http.MethodPost, "https://finance.example/mcp", strings.NewReader(`{"jsonrpc":"2.0"}`))
	request.Header.Set("Authorization", "Bearer correct-horse-battery-staple")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("authorized status=%d want %d body=%q", response.Code, http.StatusNoContent, response.Body.String())
	}
	if calls != 1 {
		t.Fatalf("next calls=%d want 1", calls)
	}
}

func TestSecureHTTPHandlerDoesNotAcceptQueryTokenOrLeakWrongBearer(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	handler, err := NewSecureHTTPHandler(next, testSecurityOptions())
	if err != nil {
		t.Fatalf("NewSecureHTTPHandler: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "https://finance.example/mcp?access_token=correct-horse-battery-staple", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("query token status=%d want 401", response.Code)
	}

	secret := "postgres-password-DO_NOT_LEAK"
	request = httptest.NewRequest(http.MethodPost, "https://finance.example/mcp", nil)
	request.Header.Set("Authorization", "Bearer "+secret)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("wrong token status=%d want 401", response.Code)
	}
	if strings.Contains(response.Body.String(), secret) || strings.Contains(response.Body.String(), "DO_NOT_LEAK") {
		t.Fatalf("wrong bearer leaked in response: %q", response.Body.String())
	}
}

func TestSecureHTTPHandlerOriginPolicy(t *testing.T) {
	calls := 0
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusNoContent)
	})
	handler, err := NewSecureHTTPHandler(next, testSecurityOptions())
	if err != nil {
		t.Fatalf("NewSecureHTTPHandler: %v", err)
	}

	allowed := []struct {
		name   string
		method string
		origin string
		host   string
		proto  string
	}{
		{name: "non-browser no origin", method: http.MethodPost},
		{name: "same origin post", method: http.MethodPost, origin: "https://finance.example", host: "finance.example", proto: "https"},
		{name: "same origin get", method: http.MethodGet, origin: "https://finance.example", host: "finance.example", proto: "https"},
		{name: "trusted cross origin", method: http.MethodPost, origin: "https://trusted.example", host: "finance.example", proto: "https"},
	}
	for _, tc := range allowed {
		t.Run(tc.name, func(t *testing.T) {
			request := authenticatedSecurityRequest(tc.method, tc.origin)
			if tc.host != "" {
				request.Host = tc.host
			}
			if tc.proto != "" {
				request.Header.Set("X-Forwarded-Proto", tc.proto)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusNoContent {
				t.Fatalf("status=%d want 204 body=%q", response.Code, response.Body.String())
			}
		})
	}

	rejected := []struct {
		name   string
		method string
		origin string
	}{
		{name: "null post", method: http.MethodPost, origin: "null"},
		{name: "malformed post", method: http.MethodPost, origin: "://bad"},
		{name: "untrusted post", method: http.MethodPost, origin: "https://evil.example"},
		{name: "untrusted get", method: http.MethodGet, origin: "https://evil.example"},
		{name: "wrong scheme", method: http.MethodPost, origin: "http://finance.example"},
	}
	for _, tc := range rejected {
		t.Run(tc.name, func(t *testing.T) {
			before := calls
			request := authenticatedSecurityRequest(tc.method, tc.origin)
			request.Host = "finance.example"
			request.Header.Set("X-Forwarded-Proto", "https")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusForbidden {
				t.Fatalf("status=%d want 403 body=%q", response.Code, response.Body.String())
			}
			if calls != before {
				t.Fatalf("next invoked for rejected origin %q", tc.origin)
			}
			assertSecurityErrorCode(t, response, "forbidden")
		})
	}
}

func TestSecureHTTPHandlerRejectsOversizedBodyBeforeDispatch(t *testing.T) {
	calls := 0
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusNoContent)
	})
	opts := testSecurityOptions()
	opts.MaxBodyBytes = 8
	handler, err := NewSecureHTTPHandler(next, opts)
	if err != nil {
		t.Fatalf("NewSecureHTTPHandler: %v", err)
	}

	tests := []struct {
		name string
		body interface{ Read([]byte) (int, error) }
	}{
		{name: "known content length", body: strings.NewReader("123456789")},
		{name: "unknown content length", body: &unknownLengthReader{reader: strings.NewReader("123456789")}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			before := calls
			request := httptest.NewRequest(http.MethodPost, "https://finance.example/mcp", tc.body)
			request.Header.Set("Authorization", "Bearer correct-horse-battery-staple")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusRequestEntityTooLarge {
				t.Fatalf("status=%d want 413 body=%q", response.Code, response.Body.String())
			}
			if calls != before {
				t.Fatalf("next invoked for oversized body")
			}
			if strings.Contains(response.Body.String(), "123456789") {
				t.Fatalf("request body leaked in response: %q", response.Body.String())
			}
			assertSecurityErrorCode(t, response, "payload_too_large")
		})
	}
}

type unknownLengthReader struct {
	reader *strings.Reader
}

func (r *unknownLengthReader) Read(p []byte) (int, error) {
	return r.reader.Read(p)
}

func authenticatedSecurityRequest(method, origin string) *http.Request {
	request := httptest.NewRequest(method, "https://finance.example/mcp", nil)
	request.Header.Set("Authorization", "Bearer correct-horse-battery-staple")
	if origin != "" {
		request.Header.Set("Origin", origin)
	}
	return request
}

func assertUnauthorized(t *testing.T, handler http.Handler, method string, authorization []string) {
	t.Helper()
	request := httptest.NewRequest(method, "https://finance.example/mcp", nil)
	for _, value := range authorization {
		request.Header.Add("Authorization", value)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401 body=%q", response.Code, response.Body.String())
	}
	if got := response.Header().Get("WWW-Authenticate"); got != "Bearer" {
		t.Fatalf("WWW-Authenticate=%q want Bearer", got)
	}
	assertSecurityErrorCode(t, response, "unauthorized")
}

func assertSecurityErrorCode(t *testing.T, response *httptest.ResponseRecorder, want string) {
	t.Helper()
	var payload struct {
		ErrorCode string `json:"error_code"`
		Message   string `json:"message"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode error response: %v body=%q", err, response.Body.String())
	}
	if payload.ErrorCode != want || strings.TrimSpace(payload.Message) == "" {
		t.Fatalf("payload=%#v want error_code=%q", payload, want)
	}
}

func testSecurityOptions() SecurityOptions {
	return SecurityOptions{
		Token:             []byte("correct-horse-battery-staple"),
		AllowedOrigins:    []string{"https://trusted.example"},
		RequestTimeout:    15 * time.Second,
		MaxConcurrent:     4,
		RequestsPerMinute: 60,
		MaxBodyBytes:      262144,
	}
}
