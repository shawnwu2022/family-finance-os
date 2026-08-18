package mcpadapter

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSecureHTTPHandlerAppliesRequestTimeout(t *testing.T) {
	observed := make(chan error, 1)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
			observed <- r.Context().Err()
		case <-time.After(100 * time.Millisecond):
			observed <- errors.New("request context did not receive a deadline")
		}
		w.WriteHeader(http.StatusNoContent)
	})
	opts := testSecurityOptions()
	opts.RequestTimeout = 10 * time.Millisecond
	handler, err := NewSecureHTTPHandler(next, opts)
	if err != nil {
		t.Fatalf("NewSecureHTTPHandler: %v", err)
	}

	request := authenticatedSecurityRequest(http.MethodPost, "")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	select {
	case observedErr := <-observed:
		if !errors.Is(observedErr, context.DeadlineExceeded) {
			t.Fatalf("request context error=%v want deadline exceeded", observedErr)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("downstream handler did not report request context state")
	}
}
