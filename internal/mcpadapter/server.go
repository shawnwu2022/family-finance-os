package mcpadapter

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shawnwu2022/family-finance-os/internal/agentadapter"
)

type ServerOptions struct {
	Name      string
	Version   string
	Principal agentadapter.Principal
}

func NewServer(service *agentadapter.AuditedService, opts ServerOptions) (*mcp.Server, error) {
	if service == nil {
		return nil, fmt.Errorf("mcpadapter: audited service is required")
	}
	name := strings.TrimSpace(opts.Name)
	if name == "" {
		return nil, fmt.Errorf("mcpadapter: server name is required")
	}
	version := strings.TrimSpace(opts.Version)
	if version == "" {
		return nil, fmt.Errorf("mcpadapter: server version is required")
	}
	if strings.TrimSpace(opts.Principal.Kind) == "" || opts.Principal.HouseholdID <= 0 {
		return nil, fmt.Errorf("mcpadapter: scoped principal is required")
	}

	definitions := service.Definitions()
	seen := make(map[agentadapter.ToolName]struct{}, len(definitions))
	for _, definition := range definitions {
		if definition.Name == "" {
			return nil, fmt.Errorf("mcpadapter: tool name is required")
		}
		if _, exists := seen[definition.Name]; exists {
			return nil, fmt.Errorf("mcpadapter: duplicate tool %q", definition.Name)
		}
		seen[definition.Name] = struct{}{}
		if !definition.ReadOnly {
			return nil, fmt.Errorf("mcpadapter: V2.0 tool %q must be read-only", definition.Name)
		}
		if err := validateObjectSchema(definition.InputSchema); err != nil {
			return nil, fmt.Errorf("mcpadapter: tool %q schema: %w", definition.Name, err)
		}
	}

	server := mcp.NewServer(&mcp.Implementation{Name: name, Version: version}, &mcp.ServerOptions{
		Capabilities: &mcp.ServerCapabilities{},
	})
	for _, definition := range definitions {
		definition := definition
		server.AddTool(&mcp.Tool{
			Name:        string(definition.Name),
			Description: definition.Description,
			InputSchema: cloneRawMessage(definition.InputSchema),
			Annotations: &mcp.ToolAnnotations{
				ReadOnlyHint:    true,
				DestructiveHint: boolPointer(false),
				IdempotentHint:  true,
				OpenWorldHint:   boolPointer(false),
			},
		}, func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return toolErrorResult(agentadapter.CodeInternal, "tool call unavailable"), nil
		})
	}
	return server, nil
}

func validateObjectSchema(schema json.RawMessage) error {
	if len(schema) == 0 {
		return fmt.Errorf("schema is required")
	}
	var value map[string]any
	if err := json.Unmarshal(schema, &value); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if value["type"] != "object" {
		return fmt.Errorf("root type must be object")
	}
	return nil
}

func cloneRawMessage(value json.RawMessage) json.RawMessage {
	return append(json.RawMessage(nil), value...)
}

func boolPointer(value bool) *bool {
	return &value
}

func toolErrorResult(code agentadapter.ErrorCode, message string) *mcp.CallToolResult {
	payload, _ := json.Marshal(struct {
		ErrorCode agentadapter.ErrorCode `json:"error_code"`
		Message   string                 `json:"message"`
	}{ErrorCode: code, Message: message})
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(payload)}},
		IsError: true,
	}
}
