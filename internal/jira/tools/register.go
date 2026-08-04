package tools

import (
	"context"

	"github.com/chiendao1808/atlassian-mcp/internal/result"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func Definitions() []*mcp.Tool {
	open := true
	additive := false
	destructive := true
	return []*mcp.Tool{
		{Name: "jira_authenticate", Description: "Authenticate this MCP process session to Jira with a username and sensitive password. Review client logging/history policy before use.", Annotations: &mcp.ToolAnnotations{OpenWorldHint: &open}},
		{Name: "jira_get_issue", Description: "Read one Jira issue. Requires jira_authenticate first.", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &open}},
		{Name: "jira_add_issue_comment", Description: "Add a Jira issue comment. Requires jira_authenticate first and may require client approval.", Annotations: &mcp.ToolAnnotations{DestructiveHint: &additive, OpenWorldHint: &open}},
		{Name: "jira_update_issue_fields", Description: "Update Jira issue fields using native Jira fields/update JSON. Requires jira_authenticate first and client approval.", Annotations: &mcp.ToolAnnotations{DestructiveHint: &destructive, OpenWorldHint: &open}},
		{Name: "jira_transition_issue", Description: "Transition a Jira issue by ID or exact name. Requires jira_authenticate first and client approval.", Annotations: &mcp.ToolAnnotations{DestructiveHint: &destructive, OpenWorldHint: &open}},
	}
}

func (s *Service) Register(server *mcp.Server) {
	defs := Definitions()
	for _, def := range defs {
		def.OutputSchema = result.MustOutputSchema()
	}
	mcp.AddTool(server, defs[0], func(ctx context.Context, req *mcp.CallToolRequest, input AuthenticateInput) (*mcp.CallToolResult, result.Envelope, error) {
		return nil, s.Authenticate(ctx, input), nil
	})
	mcp.AddTool(server, defs[1], func(ctx context.Context, req *mcp.CallToolRequest, input GetIssueInput) (*mcp.CallToolResult, result.Envelope, error) {
		return nil, s.GetIssue(ctx, input), nil
	})
	mcp.AddTool(server, defs[2], func(ctx context.Context, req *mcp.CallToolRequest, input AddCommentInput) (*mcp.CallToolResult, result.Envelope, error) {
		return nil, s.AddIssueComment(ctx, input), nil
	})
	mcp.AddTool(server, defs[3], func(ctx context.Context, req *mcp.CallToolRequest, input UpdateIssueInput) (*mcp.CallToolResult, result.Envelope, error) {
		return nil, s.UpdateIssueFields(ctx, input), nil
	})
	mcp.AddTool(server, defs[4], func(ctx context.Context, req *mcp.CallToolRequest, input TransitionIssueInput) (*mcp.CallToolResult, result.Envelope, error) {
		return nil, s.TransitionIssue(ctx, input), nil
	})
}
