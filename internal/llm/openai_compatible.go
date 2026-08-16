package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type OpenAICompatibleConfig struct {
	BaseURL    string
	APIKey     string
	Models     ModelSet
	HTTPClient *http.Client
}

type OpenAICompatibleProvider struct {
	endpoint *url.URL
	apiKey   string
	models   ModelSet
	client   *http.Client
}

func NewOpenAICompatibleProvider(cfg OpenAICompatibleConfig) (*OpenAICompatibleProvider, error) {
	endpoint, err := responsesEndpoint(cfg.BaseURL)
	if err != nil {
		return nil, err
	}
	client := cfg.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	return &OpenAICompatibleProvider{
		endpoint: endpoint,
		apiKey:   cfg.APIKey,
		models:   cfg.Models,
		client:   client,
	}, nil
}

type responsesRequest struct {
	Model             string          `json:"model"`
	Instructions      string          `json:"instructions,omitempty"`
	Input             string          `json:"input"`
	Tools             []responsesTool `json:"tools,omitempty"`
	ToolChoice        string          `json:"tool_choice,omitempty"`
	Store             bool            `json:"store"`
	ParallelToolCalls bool            `json:"parallel_tool_calls"`
	Stream            bool            `json:"stream"`
}

type responsesTool struct {
	Type        string          `json:"type"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters"`
	Strict      bool            `json:"strict"`
}

type responsesOutputItem struct {
	Type      string `json:"type"`
	CallID    string `json:"call_id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
	Content   []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

type responsesResponse struct {
	ID     string                `json:"id"`
	Output []responsesOutputItem `json:"output"`
}

func (p *OpenAICompatibleProvider) Respond(ctx context.Context, request Request) (Response, error) {
	body, err := p.buildRequest(request, false)
	if err != nil {
		return Response{}, err
	}
	resp, err := p.do(ctx, body)
	if err != nil {
		return Response{}, err
	}
	defer resp.Body.Close()

	var wire responsesResponse
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 8<<20))
	if err := decoder.Decode(&wire); err != nil {
		return Response{}, fmt.Errorf("%w: decode response: %v", ErrProviderResponse, err)
	}
	return parseResponse(wire)
}

func (p *OpenAICompatibleProvider) Stream(ctx context.Context, request Request, handler StreamHandler) error {
	if handler == nil {
		return errorsInvalidStreamHandler()
	}
	body, err := p.buildRequest(request, true)
	if err != nil {
		return err
	}
	resp, err := p.do(ctx, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return parseResponsesSSE(resp.Body, handler)
}

func (p *OpenAICompatibleProvider) buildRequest(request Request, stream bool) ([]byte, error) {
	model, err := p.models.Resolve(request.Role)
	if err != nil {
		return nil, err
	}
	tools := make([]responsesTool, 0, len(request.Tools))
	for _, tool := range request.Tools {
		if tool.Name == "" || len(tool.Parameters) == 0 || !json.Valid(tool.Parameters) {
			return nil, fmt.Errorf("%w: invalid tool definition %q", ErrProviderResponse, tool.Name)
		}
		tools = append(tools, responsesTool{
			Type:        "function",
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  tool.Parameters,
			Strict:      tool.Strict,
		})
	}
	payload := responsesRequest{
		Model:             model,
		Instructions:      request.Instructions,
		Input:             request.Input,
		Tools:             tools,
		Store:             false,
		ParallelToolCalls: false,
		Stream:            stream,
	}
	if len(tools) > 0 {
		payload.ToolChoice = "auto"
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode Responses request: %w", err)
	}
	return body, nil
}

func (p *OpenAICompatibleProvider) do(ctx context.Context, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create Responses request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Responses request: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		defer resp.Body.Close()
		message, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		return nil, fmt.Errorf("Responses request status %d: %s", resp.StatusCode, strings.TrimSpace(string(message)))
	}
	return resp, nil
}

func responsesEndpoint(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errorsInvalidBaseURL()
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, errorsInvalidBaseURL()
	}
	path := strings.TrimSuffix(parsed.Path, "/")
	switch {
	case path == "":
		path = "/v1/responses"
	case strings.HasSuffix(path, "/responses"):
	case strings.HasSuffix(path, "/v1"):
		path += "/responses"
	default:
		path += "/v1/responses"
	}
	parsed.Path = path
	return parsed, nil
}

func parseResponse(wire responsesResponse) (Response, error) {
	result := Response{ID: wire.ID, ToolCalls: make([]ToolCall, 0)}
	var text strings.Builder
	for _, item := range wire.Output {
		switch item.Type {
		case "message":
			for _, content := range item.Content {
				if content.Type == "output_text" {
					text.WriteString(content.Text)
				}
			}
		case "function_call":
			call, err := toolCallFromWire(item.CallID, item.Name, item.Arguments)
			if err != nil {
				return Response{}, err
			}
			result.ToolCalls = append(result.ToolCalls, call)
		}
	}
	result.Text = text.String()
	return result, nil
}

func toolCallFromWire(id, name, arguments string) (ToolCall, error) {
	if id == "" || name == "" || !json.Valid([]byte(arguments)) {
		return ToolCall{}, fmt.Errorf("%w: invalid function call %q", ErrProviderResponse, name)
	}
	return ToolCall{ID: id, Name: name, Arguments: json.RawMessage(arguments)}, nil
}
