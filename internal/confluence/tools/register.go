package tools

import (
	"context"

	"github.com/chiendao1808/atlassian-mcp/internal/result"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Definitions returns the exact Confluence V1 tool roster.
func Definitions() []*mcp.Tool {
	open := true
	return []*mcp.Tool{
		{Name: "confluence_authenticate", Description: "Explicit setup/recovery for the authenticated Confluence session. Uses CONFLUENCE_USERNAME/CONFLUENCE_PASSWORD when set; otherwise username and password must be passed as tool input -- review client logging/history policy before doing so.", Annotations: &mcp.ToolAnnotations{OpenWorldHint: &open}},
		{Name: "confluence_search_content", Description: "Search Confluence content with raw CQL. Requires an authenticated Confluence session; call confluence_authenticate for explicit setup or recovery.", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &open}},
		{Name: "confluence_get_content", Description: "Read one Confluence content item by ID. Requires an authenticated Confluence session; call confluence_authenticate for explicit setup or recovery.", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &open}},
	}
}

// Register binds Confluence V1 handlers to MCP.
func (s *Service) Register(server *mcp.Server) {
	defs := Definitions()
	for _, def := range defs {
		def.OutputSchema = result.MustOutputSchema()
	}
	mcp.AddTool(server, defs[0], func(ctx context.Context, req *mcp.CallToolRequest, input AuthenticateInput) (*mcp.CallToolResult, result.Envelope, error) {
		return nil, s.Authenticate(ctx, input), nil
	})
	mcp.AddTool(server, defs[1], func(ctx context.Context, req *mcp.CallToolRequest, input SearchContentInput) (*mcp.CallToolResult, result.Envelope, error) {
		return nil, s.SearchContent(ctx, input), nil
	})
	mcp.AddTool(server, defs[2], func(ctx context.Context, req *mcp.CallToolRequest, input GetContentInput) (*mcp.CallToolResult, result.Envelope, error) {
		return nil, s.GetContent(ctx, input), nil
	})
}
