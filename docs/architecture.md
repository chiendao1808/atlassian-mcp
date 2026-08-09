# Architecture

`atlassian-mcp` is one Go stdio MCP server. The command lives at `cmd/atlassian-mcp`, the binary name is `atlassian-mcp`, and MCP clients register it with the server label `atlassian`.

The app loads shared process configuration first, then validates Jira, Confluence, and Bitbucket modules independently. A static configuration error in one module disables only that module and writes a sanitized warning to `stderr`; `stdout` is reserved for MCP protocol traffic.

Shared runtime settings:

| Variable | Default | Meaning |
|---|---|---|
| `ATLASSIAN_TLS_VERIFY` | `false` | Shared TLS certificate and hostname verification flag |
| `ATLASSIAN_LOG_LEVEL` | `info` | Log level for `stderr` diagnostics |
| `ATLASSIAN_CONNECT_TIMEOUT` | `5s` | TCP/TLS connection timeout |
| `ATLASSIAN_REQUEST_TIMEOUT` | `60s` | HTTP request timeout |
| `ATLASSIAN_MAX_RESPONSE_BYTES` | `10485760` | Maximum retained response body bytes |

Jira uses `JIRA_BASE_URL` and optional `JIRA_CA_FILE`. Confluence uses `CONFLUENCE_BASE_URL` and optional `CONFLUENCE_CA_FILE`. Bitbucket uses `BITBUCKET_BASE_URL`, `BITBUCKET_PROJECT_KEY`, `BITBUCKET_BEARER_TOKEN`, optional `BITBUCKET_USER_SLUG`, and optional `BITBUCKET_CA_FILE`.

Jira compatibility targets Jira Server REST `/rest/api/2`. Confluence V1 targets Confluence Server REST `/rest/api` for authentication plus grouped content and space GET handlers: raw CQL search, content-by-ID, content collection, content properties, space collection, space-by-key, and grouped space content. Bitbucket compatibility targets Bitbucket Server Core REST `/rest/api/1.0`. The Bitbucket module owns static configuration and a shared REST client foundation with endpoint-aware URL builders, Bearer auth, bounded responses, pagination cursors, sanitized request logs, read-only retry, and Bitbucket error mapping; business tool handlers build on that foundation in later tasks.

Jira credentials are supplied only through `jira_authenticate`, held in process memory, and atomically replaced only after `serverInfo` and `myself` both succeed. Failed re-authentication leaves the previous credential active.

Confluence credentials are supplied only through `confluence_authenticate`, held in a separate process-memory session from Jira, and atomically replaced only after `/user/current` returns a known authenticated user. Failed re-authentication leaves the previous Confluence credential active. When both `CONFLUENCE_USERNAME` and `CONFLUENCE_PASSWORD` exist at startup, the same authentication logic runs once in a background goroutine; missing either variable performs no request.
