package llm

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAICompatibleRespondUsesRoleModelAndTypedTools(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/responses" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("Authorization = %q", got)
		}
		var body struct {
			Model             string `json:"model"`
			Instructions      string `json:"instructions"`
			Input             string `json:"input"`
			Store             bool   `json:"store"`
			ParallelToolCalls bool   `json:"parallel_tool_calls"`
			Stream            bool   `json:"stream"`
			Tools             []struct {
				Type       string          `json:"type"`
				Name       string          `json:"name"`
				Parameters json.RawMessage `json:"parameters"`
				Strict     bool            `json:"strict"`
			} `json:"tools"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if body.Model != "planner-model" || body.Store || body.ParallelToolCalls || body.Stream {
			t.Fatalf("request flags/model = %#v", body)
		}
		if body.Instructions != "use finance tools" || body.Input != "这个月还能花多少？" {
			t.Fatalf("instructions/input = %q / %q", body.Instructions, body.Input)
		}
		if len(body.Tools) != 1 || body.Tools[0].Type != "function" || body.Tools[0].Name != "get_budget_status" || !body.Tools[0].Strict {
			t.Fatalf("tools = %#v", body.Tools)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":"resp_1",
			"output":[
				{"type":"message","content":[{"type":"output_text","text":"先读取预算状态。"}]},
				{"type":"function_call","call_id":"call_1","name":"get_budget_status","arguments":"{\"household_id\":1}"}
			]
		}`))
	}))
	defer server.Close()

	provider, err := NewOpenAICompatibleProvider(OpenAICompatibleConfig{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Models: ModelSet{
			Fast: "fast-model", Planner: "planner-model", Reviewer: "reviewer-model",
		},
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatalf("NewOpenAICompatibleProvider: %v", err)
	}
	response, err := provider.Respond(context.Background(), Request{
		Role:         ModelRolePlanner,
		Instructions: "use finance tools",
		Input:        "这个月还能花多少？",
		Tools: []ToolDefinition{{
			Name: "get_budget_status", Description: "read budget", Parameters: json.RawMessage(`{"type":"object"}`), Strict: true,
		}},
	})
	if err != nil {
		t.Fatalf("Respond: %v", err)
	}
	if response.ID != "resp_1" || response.Text != "先读取预算状态。" {
		t.Fatalf("response = %#v", response)
	}
	if len(response.ToolCalls) != 1 || response.ToolCalls[0].ID != "call_1" || response.ToolCalls[0].Name != "get_budget_status" || string(response.ToolCalls[0].Arguments) != `{"household_id":1}` {
		t.Fatalf("tool calls = %#v", response.ToolCalls)
	}
}

func TestOpenAICompatibleStreamParsesTextToolCallAndCompletion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/responses" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if body["model"] != "fast-model" || body["stream"] != true || body["store"] != false {
			t.Fatalf("stream request = %#v", body)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("event: response.output_text.delta\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"response.output_text.delta\",\"delta\":\"预算\"}\n\n"))
		_, _ = w.Write([]byte("event: response.output_item.done\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"response.output_item.done\",\"item\":{\"type\":\"function_call\",\"call_id\":\"call_2\",\"name\":\"get_overview\",\"arguments\":\"{}\"}}\n\n"))
		_, _ = w.Write([]byte("event: response.completed\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_2\"}}\n\n"))
	}))
	defer server.Close()

	provider, err := NewOpenAICompatibleProvider(OpenAICompatibleConfig{
		BaseURL:    server.URL + "/v1",
		APIKey:     "test-key",
		Models:     ModelSet{Fast: "fast-model"},
		HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatalf("NewOpenAICompatibleProvider: %v", err)
	}
	var events []StreamEvent
	err = provider.Stream(context.Background(), Request{Role: ModelRoleFast, Input: "概览"}, func(event StreamEvent) error {
		events = append(events, event)
		return nil
	})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("events = %#v", events)
	}
	if events[0].Type != StreamEventTextDelta || events[0].TextDelta != "预算" {
		t.Fatalf("text event = %#v", events[0])
	}
	if events[1].Type != StreamEventToolCall || events[1].ToolCall == nil || events[1].ToolCall.Name != "get_overview" {
		t.Fatalf("tool event = %#v", events[1])
	}
	if events[2].Type != StreamEventCompleted || events[2].ResponseID != "resp_2" {
		t.Fatalf("completed event = %#v", events[2])
	}
}

func TestOpenAICompatibleStreamFailsOnResponseFailureEvent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("event: response.failed\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"response.failed\",\"response\":{\"error\":{\"message\":\"upstream failed\"}}}\n\n"))
	}))
	defer server.Close()

	provider, err := NewOpenAICompatibleProvider(OpenAICompatibleConfig{BaseURL: server.URL, Models: ModelSet{Fast: "fast-model"}, HTTPClient: server.Client()})
	if err != nil {
		t.Fatalf("NewOpenAICompatibleProvider: %v", err)
	}
	err = provider.Stream(context.Background(), Request{Role: ModelRoleFast, Input: "test"}, func(StreamEvent) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "upstream failed") {
		t.Fatalf("stream error = %v", err)
	}
}

func TestOpenAICompatibleRejectsUnconfiguredModelRole(t *testing.T) {
	provider, err := NewOpenAICompatibleProvider(OpenAICompatibleConfig{BaseURL: "https://example.com/v1", Models: ModelSet{Fast: "fast-model"}})
	if err != nil {
		t.Fatalf("NewOpenAICompatibleProvider: %v", err)
	}
	_, err = provider.Respond(context.Background(), Request{Role: ModelRolePlanner, Input: "plan"})
	if !errors.Is(err, ErrModelNotConfigured) {
		t.Fatalf("model error = %v", err)
	}
}
