package tools

import (
	"context"

	"github.com/chiendao1808/atlassian-mcp/internal/result"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Definitions returns the exact Confluence V1 tool roster.
func Definitions() []*mcp.Tool {
	open := true
	names := []struct {
		name        string
		description string
		readOnly    bool
	}{
		{
			name: "confluence_authenticate",
			description: "Explicit setup/recovery for the authenticated Confluence session. " +
				"Uses CONFLUENCE_USERNAME/CONFLUENCE_PASSWORD when set; otherwise username and password must be " +
				"passed as tool input -- review client logging/history policy before doing so.",
		},
		{
			name: "confluence_search_content",
			description: "Search Confluence content with raw CQL. " +
				"Requires an authenticated Confluence session; call confluence_authenticate for explicit setup or recovery.",
			readOnly: true,
		},
		{
			name: "confluence_get_content",
			description: "Read one Confluence content item by ID. " +
				"Requires an authenticated Confluence session; call confluence_authenticate for explicit setup or recovery.",
			readOnly: true,
		},
		{
			name: "confluence_list_content",
			description: "List Confluence content with documented content filters. " +
				"Requires an authenticated Confluence session; call confluence_authenticate for explicit setup or recovery.",
			readOnly: true,
		},
		{
			name: "confluence_list_content_properties",
			description: "List native Confluence content properties for one content item. " +
				"Requires an authenticated Confluence session; call confluence_authenticate for explicit setup or recovery.",
			readOnly: true,
		},
		{
			name: "confluence_get_content_property",
			description: "Read one native Confluence content property by key. " +
				"Requires an authenticated Confluence session; call confluence_authenticate for explicit setup or recovery.",
			readOnly: true,
		},
		{
			name: "confluence_list_spaces",
			description: "List visible Confluence spaces with documented filters. " +
				"Requires an authenticated Confluence session; call confluence_authenticate for explicit setup or recovery.",
			readOnly: true,
		},
		{
			name: "confluence_get_space",
			description: "Read one Confluence space by key. " +
				"Requires an authenticated Confluence session; call confluence_authenticate for explicit setup or recovery.",
			readOnly: true,
		},
		{
			name: "confluence_list_space_content",
			description: "List grouped Confluence content for one space. " +
				"Requires an authenticated Confluence session; call confluence_authenticate for explicit setup or recovery.",
			readOnly: true,
		},
	}
	defs := make([]*mcp.Tool, 0, len(names))
	for _, item := range names {
		def := &mcp.Tool{
			Name:        item.name,
			Description: item.description,
			Annotations: &mcp.ToolAnnotations{
				OpenWorldHint: &open,
			},
		}
		if item.readOnly {
			def.Annotations.ReadOnlyHint = true
		}
		defs = append(defs, def)
	}
	return defs
}

// Register binds Confluence V1 handlers to MCP.
func (s *Service) Register(server *mcp.Server) {
	defs := Definitions()
	for _, def := range defs {
		def.OutputSchema = result.MustOutputSchema()
	}
	s.registerAuthenticationTool(server, defs)
	s.registerContentTools(server, defs)
	s.registerSpaceTools(server, defs)
}

func (s *Service) registerAuthenticationTool(server *mcp.Server, defs []*mcp.Tool) {
	mcp.AddTool(
		server,
		defs[0],
		func(
			ctx context.Context,
			req *mcp.CallToolRequest,
			input AuthenticateInput,
		) (*mcp.CallToolResult, result.Envelope, error) {
			return nil, s.Authenticate(ctx, input), nil
		},
	)
}

func (s *Service) registerContentTools(server *mcp.Server, defs []*mcp.Tool) {
	mcp.AddTool(
		server,
		defs[1],
		func(
			ctx context.Context,
			req *mcp.CallToolRequest,
			input SearchContentInput,
		) (*mcp.CallToolResult, result.Envelope, error) {
			return nil, s.SearchContent(ctx, input), nil
		},
	)
	mcp.AddTool(
		server,
		defs[2],
		func(
			ctx context.Context,
			req *mcp.CallToolRequest,
			input GetContentInput,
		) (*mcp.CallToolResult, result.Envelope, error) {
			return nil, s.GetContent(ctx, input), nil
		},
	)
	mcp.AddTool(
		server,
		defs[3],
		func(
			ctx context.Context,
			req *mcp.CallToolRequest,
			input ListContentInput,
		) (*mcp.CallToolResult, result.Envelope, error) {
			return nil, s.ListContent(ctx, input), nil
		},
	)
	mcp.AddTool(
		server,
		defs[4],
		func(
			ctx context.Context,
			req *mcp.CallToolRequest,
			input ListContentPropertiesInput,
		) (*mcp.CallToolResult, result.Envelope, error) {
			return nil, s.ListContentProperties(ctx, input), nil
		},
	)
	mcp.AddTool(
		server,
		defs[5],
		func(
			ctx context.Context,
			req *mcp.CallToolRequest,
			input GetContentPropertyInput,
		) (*mcp.CallToolResult, result.Envelope, error) {
			return nil, s.GetContentProperty(ctx, input), nil
		},
	)
}

func (s *Service) registerSpaceTools(server *mcp.Server, defs []*mcp.Tool) {
	mcp.AddTool(
		server,
		defs[6],
		func(
			ctx context.Context,
			req *mcp.CallToolRequest,
			input ListSpacesInput,
		) (*mcp.CallToolResult, result.Envelope, error) {
			return nil, s.ListSpaces(ctx, input), nil
		},
	)
	mcp.AddTool(
		server,
		defs[7],
		func(
			ctx context.Context,
			req *mcp.CallToolRequest,
			input GetSpaceInput,
		) (*mcp.CallToolResult, result.Envelope, error) {
			return nil, s.GetSpace(ctx, input), nil
		},
	)
	mcp.AddTool(
		server,
		defs[8],
		func(
			ctx context.Context,
			req *mcp.CallToolRequest,
			input ListSpaceContentInput,
		) (*mcp.CallToolResult, result.Envelope, error) {
			return nil, s.ListSpaceContent(ctx, input), nil
		},
	)
}
