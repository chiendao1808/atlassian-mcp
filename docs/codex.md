# Codex MCP Configuration

Register one MCP server named `atlassian` that launches the final wrapper or binary for `atlassian-mcp`. Configure non-secret environment values only, such as `JIRA_BASE_URL`, `BITBUCKET_BASE_URL`, `BITBUCKET_PROJECT_KEY`, `BITBUCKET_USER_SLUG`, CA paths, and shared timeout/TLS settings.

Do not put Jira credentials in Codex config. Use `jira_authenticate` after each new MCP process starts.

Use Codex write approval defaults for Jira mutations:

```toml
default_tools_approval_mode = "writes"
```

Treat `jira_add_issue_comment`, `jira_update_issue_fields`, and `jira_transition_issue` as write tools. `jira_authenticate` is not an upstream mutation, but it carries a password through a tool call, so require a prompt for it when your Codex configuration supports per-tool sensitive approval.
