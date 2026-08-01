# Claude Code MCP Configuration

Register one MCP server named `atlassian` that launches the final wrapper or binary for `atlassian-mcp`. Project, user, or local scope may provide non-secret module configuration.

Do not put Jira credentials in Claude Code config. Use `jira_authenticate` after each new MCP process starts.

Route `jira_add_issue_comment`, `jira_update_issue_fields`, and `jira_transition_issue` through write-tool approval. `jira_authenticate` is not an upstream mutation, but it carries a password through a tool call, so prompt before calling it when Claude Code policy controls allow that.
