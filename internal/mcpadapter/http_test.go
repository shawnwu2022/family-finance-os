package mcpadapter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shawnwu2022/family-finance-os/internal/agentadapter"
)

func TestNewHTTPHandlerRejectsNilServer(t *testing.T) {
	if _, err := NewHTTPHandler(nil); err == nil {
		t.Fatal("NewHTTPHandler accepted nil server")
	}
}

func TestHTTPHandlerSupportsStreamableMCP(t *testing.T) {
	audited := newTestAuditedService(t)
	server, err := NewServer(audited, ServerOptions{
		Name:      "family-finance-os",
		Version:   "v2-test",
		Principal: agentadapter.Principal{Kind: "mcp", HouseholdID: 42},
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	handler, err := NewHTTPHandler(server)
	if err != nil {
		t.Fatalf("NewHTTPHandler: %v", err)
	}

	httpServer := httptest.NewServer(handler)
	defer httpServer.Close()

	client := mcp.NewClient(&mcp.Implementation{Name: "http-test-client", Version: "1.0.0"}, &mcp.ClientOptions{
		Capabilities: &mcp.ClientCapabilities{},
	})
	ctx := context.Background()
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint: httpServer.URL,
		HTTPClient: &http.Client{
			Transport: http.DefaultTransport,
		},
	}, nil)
	if err != nil {
		t.Fatalf("client Connect over Streamable HTTP: %v", err)
	}
	defer session.Close()

	if session.InitializeResult() == nil || session.InitializeResult().ProtocolVersion == "" {
		t.Fatalf("missing initialize result: %#v", session.InitializeResult())
	}
	listed, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools over HTTP: %v", err)
	}
	if len(listed.Tools) != 11 {
		t.Fatalf("tool count=%d want 11", len(listed.Tools))
	}

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      string(agentadapter.ToolGetHouseholdOverview),
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("CallTool over HTTP: %v", err)
	}
	if result.IsError || result.StructuredContent == nil {
		t.Fatalf("HTTP tool call result=%#v", result)
	}
}
