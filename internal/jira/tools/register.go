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
		{Name: "jira_authenticate", Description: "Authenticate this MCP process session to Jira. Uses JIRA_USERNAME/JIRA_PASSWORD when set; otherwise username and password must be passed as tool input -- review client logging/history policy before doing so.", Annotations: &mcp.ToolAnnotations{OpenWorldHint: &open}},
		{Name: "jira_get_issue", Description: "Read one Jira issue. Requires jira_authenticate first.", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &open}},
		{Name: "jira_add_issue_comment", Description: "Add a Jira issue comment. Requires jira_authenticate first and may require client approval.", Annotations: &mcp.ToolAnnotations{DestructiveHint: &additive, OpenWorldHint: &open}},
		{Name: "jira_update_issue_fields", Description: "Update Jira issue fields using native Jira fields/update JSON. Requires jira_authenticate first and client approval.", Annotations: &mcp.ToolAnnotations{DestructiveHint: &destructive, OpenWorldHint: &open}},
		{Name: "jira_transition_issue", Description: "Transition a Jira issue by ID or exact name. Requires jira_authenticate first and client approval.", Annotations: &mcp.ToolAnnotations{DestructiveHint: &destructive, OpenWorldHint: &open}},
		{Name: "jira_create_issue", Description: "Create a new Jira issue from native fields/update JSON. Requires jira_authenticate first and client approval.", Annotations: &mcp.ToolAnnotations{DestructiveHint: &additive, OpenWorldHint: &open}},
		{Name: "jira_bulk_create_issues", Description: "Create multiple Jira issues in one call; per-row failures are reported in the response and do not fail the tool. Requires jira_authenticate first and client approval.", Annotations: &mcp.ToolAnnotations{DestructiveHint: &additive, OpenWorldHint: &open}},
		{Name: "jira_delete_issue", Description: "Permanently delete a Jira issue, optionally including its subtasks. Requires jira_authenticate first and client approval; this is an irreversible write.", Annotations: &mcp.ToolAnnotations{DestructiveHint: &destructive, OpenWorldHint: &open}},
		{Name: "jira_assign_issue", Description: "Assign or unassign a Jira issue; an empty or omitted name unassigns it. Requires jira_authenticate first and client approval; this is an irreversible write.", Annotations: &mcp.ToolAnnotations{DestructiveHint: &destructive, OpenWorldHint: &open}},
		{Name: "jira_search_issues", Description: "Search Jira issues by JQL. Requires jira_authenticate first.", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &open}},
		{Name: "jira_list_issue_comments", Description: "List a Jira issue's comments with optional pagination, ordering, and expand. Requires jira_authenticate first.", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &open}},
		{Name: "jira_update_issue_comment", Description: "Update one Jira comment's body and optional visibility. Requires jira_authenticate first and client approval.", Annotations: &mcp.ToolAnnotations{DestructiveHint: &destructive, OpenWorldHint: &open}},
		{Name: "jira_delete_issue_comment", Description: "Permanently delete one Jira comment. Requires jira_authenticate first and client approval; this is an irreversible write.", Annotations: &mcp.ToolAnnotations{DestructiveHint: &destructive, OpenWorldHint: &open}},
		{Name: "jira_list_issue_transitions", Description: "List a Jira issue's available workflow transitions, defaulting expand to transitions.fields. Requires jira_authenticate first.", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &open}},
		{Name: "jira_add_issue_attachment", Description: "Upload a base64-encoded file attachment to a Jira issue. Requires jira_authenticate first and client approval.", Annotations: &mcp.ToolAnnotations{DestructiveHint: &additive, OpenWorldHint: &open}},
		{Name: "jira_delete_issue_attachment", Description: "Permanently delete one Jira attachment by attachment ID (no issue key in this path). Requires jira_authenticate first and client approval; this is an irreversible write.", Annotations: &mcp.ToolAnnotations{DestructiveHint: &destructive, OpenWorldHint: &open}},
		{Name: "jira_list_issue_worklogs", Description: "List a Jira issue's worklog entries. Requires jira_authenticate first.", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &open}},
		{Name: "jira_add_issue_worklog", Description: "Add a worklog entry (time spent) to a Jira issue, optionally adjusting the remaining estimate. Requires jira_authenticate first and client approval.", Annotations: &mcp.ToolAnnotations{DestructiveHint: &additive, OpenWorldHint: &open}},
		{Name: "jira_get_issue_watchers", Description: "List a Jira issue's watchers. Requires jira_authenticate first.", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &open}},
		{Name: "jira_add_issue_watcher", Description: "Add a user as a watcher on a Jira issue. Requires jira_authenticate first and client approval.", Annotations: &mcp.ToolAnnotations{DestructiveHint: &additive, OpenWorldHint: &open}},
		{Name: "jira_remove_issue_watcher", Description: "Remove a user as a watcher from a Jira issue. Requires jira_authenticate first and client approval; this is an irreversible write.", Annotations: &mcp.ToolAnnotations{DestructiveHint: &destructive, OpenWorldHint: &open}},
		{Name: "jira_vote_issue", Description: "Add the authenticated user's vote to a Jira issue. Requires jira_authenticate first and client approval.", Annotations: &mcp.ToolAnnotations{DestructiveHint: &additive, OpenWorldHint: &open}},
		{Name: "jira_unvote_issue", Description: "Remove the authenticated user's vote from a Jira issue. Requires jira_authenticate first and client approval; this is an irreversible write.", Annotations: &mcp.ToolAnnotations{DestructiveHint: &destructive, OpenWorldHint: &open}},
		{Name: "jira_create_issue_link", Description: "Create a native Jira issue link between two issues. Requires jira_authenticate first and client approval.", Annotations: &mcp.ToolAnnotations{DestructiveHint: &additive, OpenWorldHint: &open}},
		{Name: "jira_create_component", Description: "Create a Jira project Component using projectKey mapped to Jira body field project. Requires jira_authenticate first and client approval.", Annotations: &mcp.ToolAnnotations{DestructiveHint: &additive, OpenWorldHint: &open}},
		{Name: "jira_get_component", Description: "Read one Jira Component by componentId. Requires jira_authenticate first.", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, DestructiveHint: &additive, IdempotentHint: true, OpenWorldHint: &open}},
		{Name: "jira_update_component", Description: "Partially update one Jira Component. Requires jira_authenticate first and client approval; this may rename or clear Component metadata.", Annotations: &mcp.ToolAnnotations{DestructiveHint: &destructive, IdempotentHint: true, OpenWorldHint: &open}},
		{Name: "jira_delete_component", Description: "Delete one Jira Component, optionally moving affected issues to another Component. Requires jira_authenticate first and client approval; this is destructive.", Annotations: &mcp.ToolAnnotations{DestructiveHint: &destructive, IdempotentHint: true, OpenWorldHint: &open}},
		{Name: "jira_get_component_issue_count", Description: "Read Jira's related issue count for one Component. Requires jira_authenticate first.", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, DestructiveHint: &additive, IdempotentHint: true, OpenWorldHint: &open}},
		{Name: "jira_list_project_components", Description: "List Jira Components for one project ID or key. Requires jira_authenticate first.", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, DestructiveHint: &additive, IdempotentHint: true, OpenWorldHint: &open}},
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
	mcp.AddTool(server, defs[5], func(ctx context.Context, req *mcp.CallToolRequest, input CreateIssueInput) (*mcp.CallToolResult, result.Envelope, error) {
		return nil, s.CreateIssue(ctx, input), nil
	})
	mcp.AddTool(server, defs[6], func(ctx context.Context, req *mcp.CallToolRequest, input BulkCreateIssuesInput) (*mcp.CallToolResult, result.Envelope, error) {
		return nil, s.BulkCreateIssues(ctx, input), nil
	})
	mcp.AddTool(server, defs[7], func(ctx context.Context, req *mcp.CallToolRequest, input DeleteIssueInput) (*mcp.CallToolResult, result.Envelope, error) {
		return nil, s.DeleteIssue(ctx, input), nil
	})
	mcp.AddTool(server, defs[8], func(ctx context.Context, req *mcp.CallToolRequest, input AssignIssueInput) (*mcp.CallToolResult, result.Envelope, error) {
		return nil, s.AssignIssue(ctx, input), nil
	})
	mcp.AddTool(server, defs[9], func(ctx context.Context, req *mcp.CallToolRequest, input SearchIssuesInput) (*mcp.CallToolResult, result.Envelope, error) {
		return nil, s.SearchIssues(ctx, input), nil
	})
	mcp.AddTool(server, defs[10], func(ctx context.Context, req *mcp.CallToolRequest, input ListIssueCommentsInput) (*mcp.CallToolResult, result.Envelope, error) {
		return nil, s.ListIssueComments(ctx, input), nil
	})
	mcp.AddTool(server, defs[11], func(ctx context.Context, req *mcp.CallToolRequest, input UpdateIssueCommentInput) (*mcp.CallToolResult, result.Envelope, error) {
		return nil, s.UpdateIssueComment(ctx, input), nil
	})
	mcp.AddTool(server, defs[12], func(ctx context.Context, req *mcp.CallToolRequest, input DeleteIssueCommentInput) (*mcp.CallToolResult, result.Envelope, error) {
		return nil, s.DeleteIssueComment(ctx, input), nil
	})
	mcp.AddTool(server, defs[13], func(ctx context.Context, req *mcp.CallToolRequest, input ListIssueTransitionsInput) (*mcp.CallToolResult, result.Envelope, error) {
		return nil, s.ListIssueTransitions(ctx, input), nil
	})
	mcp.AddTool(server, defs[14], func(ctx context.Context, req *mcp.CallToolRequest, input AddIssueAttachmentInput) (*mcp.CallToolResult, result.Envelope, error) {
		return nil, s.AddIssueAttachment(ctx, input), nil
	})
	mcp.AddTool(server, defs[15], func(ctx context.Context, req *mcp.CallToolRequest, input DeleteIssueAttachmentInput) (*mcp.CallToolResult, result.Envelope, error) {
		return nil, s.DeleteIssueAttachment(ctx, input), nil
	})
	mcp.AddTool(server, defs[16], func(ctx context.Context, req *mcp.CallToolRequest, input ListIssueWorklogsInput) (*mcp.CallToolResult, result.Envelope, error) {
		return nil, s.ListIssueWorklogs(ctx, input), nil
	})
	mcp.AddTool(server, defs[17], func(ctx context.Context, req *mcp.CallToolRequest, input AddIssueWorklogInput) (*mcp.CallToolResult, result.Envelope, error) {
		return nil, s.AddIssueWorklog(ctx, input), nil
	})
	mcp.AddTool(server, defs[18], func(ctx context.Context, req *mcp.CallToolRequest, input GetIssueWatchersInput) (*mcp.CallToolResult, result.Envelope, error) {
		return nil, s.GetIssueWatchers(ctx, input), nil
	})
	mcp.AddTool(server, defs[19], func(ctx context.Context, req *mcp.CallToolRequest, input AddIssueWatcherInput) (*mcp.CallToolResult, result.Envelope, error) {
		return nil, s.AddIssueWatcher(ctx, input), nil
	})
	mcp.AddTool(server, defs[20], func(ctx context.Context, req *mcp.CallToolRequest, input RemoveIssueWatcherInput) (*mcp.CallToolResult, result.Envelope, error) {
		return nil, s.RemoveIssueWatcher(ctx, input), nil
	})
	mcp.AddTool(server, defs[21], func(ctx context.Context, req *mcp.CallToolRequest, input VoteIssueInput) (*mcp.CallToolResult, result.Envelope, error) {
		return nil, s.VoteIssue(ctx, input), nil
	})
	mcp.AddTool(server, defs[22], func(ctx context.Context, req *mcp.CallToolRequest, input UnvoteIssueInput) (*mcp.CallToolResult, result.Envelope, error) {
		return nil, s.UnvoteIssue(ctx, input), nil
	})
	mcp.AddTool(server, defs[23], func(ctx context.Context, req *mcp.CallToolRequest, input CreateIssueLinkInput) (*mcp.CallToolResult, result.Envelope, error) {
		return nil, s.CreateIssueLink(ctx, input), nil
	})
	mcp.AddTool(server, defs[24], func(ctx context.Context, req *mcp.CallToolRequest, input CreateComponentInput) (*mcp.CallToolResult, result.Envelope, error) {
		return nil, s.CreateComponent(ctx, input), nil
	})
	mcp.AddTool(server, defs[25], func(ctx context.Context, req *mcp.CallToolRequest, input GetComponentInput) (*mcp.CallToolResult, result.Envelope, error) {
		return nil, s.GetComponent(ctx, input), nil
	})
	mcp.AddTool(server, defs[26], func(ctx context.Context, req *mcp.CallToolRequest, input UpdateComponentInput) (*mcp.CallToolResult, result.Envelope, error) {
		return nil, s.UpdateComponent(ctx, input), nil
	})
	mcp.AddTool(server, defs[27], func(ctx context.Context, req *mcp.CallToolRequest, input DeleteComponentInput) (*mcp.CallToolResult, result.Envelope, error) {
		return nil, s.DeleteComponent(ctx, input), nil
	})
	mcp.AddTool(server, defs[28], func(ctx context.Context, req *mcp.CallToolRequest, input GetComponentIssueCountInput) (*mcp.CallToolResult, result.Envelope, error) {
		return nil, s.GetComponentIssueCount(ctx, input), nil
	})
	mcp.AddTool(server, defs[29], func(ctx context.Context, req *mcp.CallToolRequest, input ListProjectComponentsInput) (*mcp.CallToolResult, result.Envelope, error) {
		return nil, s.ListProjectComponents(ctx, input), nil
	})
}
