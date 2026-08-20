package mcpadapter

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSecureHTTPHandlerRejectsConcurrencyOverflow(t *testing.T) {
	entered := make(chan struct{}, 2)
	release := make(chan struct{})
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		entered <- struct{}{}
		<-release
		w.WriteHeader(http.StatusNoContent)
	})
	opts := testSecurityOptions()
	opts.MaxConcurrent = 1
	opts.RequestTimeout = time.Second
	handler, err := NewSecureHTTPHandler(next, opts)
	if err != nil {
		t.Fatalf("NewSecureHTTPHandler: %v", err)
	}

	firstDone := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, authenticatedSecurityRequest(http.MethodPost, ""))
		firstDone <- response
	}()

	select {
	case <-entered:
	case <-time.After(250 * time.Millisecond):
		close(release)
		t.Fatal("first request did not enter downstream handler")
	}

	secondDone := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, authenticatedSecurityRequest(http.MethodPost, ""))
		secondDone <- response
	}()

	select {
	case response := <-secondDone:
		if response.Code != http.StatusServiceUnavailable {
			close(release)
			t.Fatalf("second status=%d want 503 body=%q", response.Code, response.Body.String())
		}
		assertSecurityErrorCode(t, response, "busy")
	case <-entered:
		close(release)
		<-secondDone
		<-firstDone
		t.Fatal("second request entered downstream handler while concurrency slot was occupied")
	case <-time.After(250 * time.Millisecond):
		close(release)
		<-secondDone
		<-firstDone
		t.Fatal("second request neither rejected nor entered downstream handler")
	}

	close(release)
	select {
	case response := <-firstDone:
		if response.Code != http.StatusNoContent {
			t.Fatalf("first status=%d want 204 body=%q", response.Code, response.Body.String())
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("first request did not complete after releasing concurrency slot")
	}
}
