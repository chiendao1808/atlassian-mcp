# Codex MCP Configuration

Register one MCP server named `atlassian` that launches the final wrapper or binary for `atlassian-mcp`. Configure non-secret environment values only, such as `JIRA_BASE_URL`, `BITBUCKET_BASE_URL`, `BITBUCKET_PROJECT_KEY`, `BITBUCKET_USER_SLUG`, CA paths, and shared timeout/TLS settings.

Do not put Jira credentials in Codex config. Use `jira_authenticate` after each new MCP process starts.