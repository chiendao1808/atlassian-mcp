# Kiro MCP Configuration

Register one MCP server named `atlassian` that launches the final wrapper or binary for `atlassian-mcp`. The installer merges the entry into `~/.kiro/settings/mcp.json` (user scope) or `<project>/.kiro/settings/mcp.json` (project and local scope), preserving unrelated root keys and other `mcpServers` entries.

Do not put Jira credentials in Kiro config. The registered JSON never contains resolved secrets: on Linux the wrapper resolves credential env indirection at runtime, and on Windows runtime config comes from the persisted User environment variables. Use `jira_authenticate` after each new MCP process starts.

The installer deliberately omits `autoApprove` so Kiro keeps its default per-tool approval. Treat `jira_add_issue_comment`, `jira_update_issue_fields`, and `jira_transition_issue` as write tools that require approval. `jira_authenticate` is not an upstream mutation, but it carries a password through a tool call, so prompt before calling it when Kiro policy controls allow that.

Restart Kiro after a Windows install so it picks up newly persisted User environment variables.
