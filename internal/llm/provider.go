package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

var (
	ErrModelNotConfigured = errors.New("LLM model role is not configured")
	ErrProviderResponse   = errors.New("invalid LLM provider response")
)

type ModelRole string

const (
	ModelRoleFast     ModelRole = "fast"
	ModelRolePlanner  ModelRole = "planner"
	ModelRoleReviewer ModelRole = "reviewer"
)

type ModelSet struct {
	Fast     string
	Planner  string
	Reviewer string
}

func (m ModelSet) Resolve(role ModelRole) (string, error) {
	var model string
	switch role {
	case ModelRoleFast:
		model = m.Fast
	case ModelRolePlanner:
		model = m.Planner
	case ModelRoleReviewer:
		model = m.Reviewer
	default:
		return "", fmt.Errorf("%w: %q", ErrModelNotConfigured, role)
	}
	if model == "" {
		return "", fmt.Errorf("%w: %s", ErrModelNotConfigured, role)
	}
	return model, nil
}

type ToolDefinition struct {
	Name        string
	Description string
	Parameters  json.RawMessage
	Strict      bool
}

type ToolCall struct {
	ID        string
	Name      string
	Arguments json.RawMessage
}

type Request struct {
	Role         ModelRole
	Instructions string
	Input        string
	Tools        []ToolDefinition
}

type Response struct {
	ID        string
	Text      string
	ToolCalls []ToolCall
}

type StreamEventType uint8

const (
	StreamEventUnknown StreamEventType = iota
	StreamEventTextDelta
	StreamEventToolCall
	StreamEventCompleted
)

type StreamEvent struct {
	Type       StreamEventType
	TextDelta  string
	ToolCall   *ToolCall
	ResponseID string
}

type StreamHandler func(StreamEvent) error

type Provider interface {
	Respond(ctx context.Context, request Request) (Response, error)
	Stream(ctx context.Context, request Request, handler StreamHandler) error
}
