package mcpadapter

import (
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shawnwu2022/family-finance-os/internal/agentadapter"
)

func TestNewServerRejectsMissingAuditedService(t *testing.T) {
	var _ *mcp.Server
	_, err := NewServer(nil, ServerOptions{
		Name:      "family-finance-os",
		Version:   "v2-test",
		Principal: agentadapter.Principal{Kind: "mcp", HouseholdID: 42},
	})
	if err == nil {
		t.Fatal("NewServer accepted nil audited service")
	}
}
