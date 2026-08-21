package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenAICompatibleProviderRejectsInsecureHTTPBaseURL(t *testing.T) {
	_, err := NewOpenAICompatibleProvider(OpenAICompatibleConfig{
		BaseURL: "http://llm.example.com/v1",
		APIKey:  "test-key",
		Models:  ModelSet{Fast: "fast-model"},
	})
	if err == nil {
		t.Fatal("NewOpenAICompatibleProvider accepted insecure HTTP base URL")
	}
}

func TestOpenAICompatibleProviderRejectsHTTPSRedirectDowngrade(t *testing.T) {
	plaintextReached := make(chan struct{}, 1)
	plaintext := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		plaintextReached <- struct{}{}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"plaintext","output":[]}`))
	}))
	defer plaintext.Close()

	tlsUpstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, plaintext.URL+"/v1/responses", http.StatusTemporaryRedirect)
	}))
	defer tlsUpstream.Close()

	provider, err := NewOpenAICompatibleProvider(OpenAICompatibleConfig{
		BaseURL:    tlsUpstream.URL,
		APIKey:     "test-key",
		Models:     ModelSet{Fast: "fast-model"},
		HTTPClient: tlsUpstream.Client(),
	})
	if err != nil {
		t.Fatalf("NewOpenAICompatibleProvider: %v", err)
	}
	if _, err := provider.Respond(context.Background(), Request{Role: ModelRoleFast, Input: "test"}); err == nil {
		t.Fatal("provider followed HTTPS redirect downgrade to plaintext HTTP")
	}
	select {
	case <-plaintextReached:
		t.Fatal("plaintext redirect target received the LLM request")
	default:
	}
}
