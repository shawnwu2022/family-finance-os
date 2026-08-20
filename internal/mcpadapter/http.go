package mcpadapter

import (
	"fmt"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func NewHTTPHandler(server *mcp.Server) (http.Handler, error) {
	if server == nil {
		return nil, fmt.Errorf("mcpadapter: MCP server is required")
	}
	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return server
	}, &mcp.StreamableHTTPOptions{
		JSONResponse: true,
	}), nil
}
