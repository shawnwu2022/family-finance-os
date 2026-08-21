package llm

import "testing"

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
