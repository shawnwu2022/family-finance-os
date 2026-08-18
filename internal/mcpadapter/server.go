package mcpadapter

import (
	"context"
	"encoding/json"
	"errors"
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
		}, toolHandler(service, opts.Principal, definition.Name))
	}
	return server, nil
}

func toolHandler(service *agentadapter.AuditedService, principal agentadapter.Principal, name agentadapter.ToolName) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if req == nil || req.Params == nil || req.Session == nil {
			return toolErrorResult(agentadapter.CodeInternal, "MCP request metadata unavailable"), nil
		}
		initialize := req.Session.InitializeParams()
		if initialize == nil || strings.TrimSpace(initialize.ProtocolVersion) == "" {
			return toolErrorResult(agentadapter.CodeInternal, "MCP session metadata unavailable"), nil
		}

		metadata := agentadapter.CallMetadata{
			Protocol:        "mcp",
			ProtocolVersion: initialize.ProtocolVersion,
		}
		if initialize.ClientInfo != nil {
			metadata.ClientName = initialize.ClientInfo.Name
			metadata.ClientVersion = initialize.ClientInfo.Version
		}

		result, err := service.Call(ctx, principal, metadata, name, req.Params.Arguments)
		if err != nil {
			code := agentadapter.CodeInternal
			message := "tool call failed"
			var adapterErr *agentadapter.Error
			if errors.As(err, &adapterErr) {
				code = adapterErr.Code
				if strings.TrimSpace(adapterErr.Message) != "" {
					message = adapterErr.Message
				}
			}
			return toolErrorResult(code, message), nil
		}
		return toolSuccessResult(result), nil
	}
}

func toolSuccessResult(result agentadapter.Result) *mcp.CallToolResult {
	payload, err := json.Marshal(result)
	if err != nil {
		return toolErrorResult(agentadapter.CodeInternal, "tool result unavailable")
	}
	var structured map[string]any
	if err := json.Unmarshal(payload, &structured); err != nil {
		return toolErrorResult(agentadapter.CodeInternal, "tool result unavailable")
	}
	return &mcp.CallToolResult{
		Content:           []mcp.Content{&mcp.TextContent{Text: string(payload)}},
		StructuredContent: structured,
	}
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
