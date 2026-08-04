# MCP Client Security Notes

`jira_authenticate` accepts a sensitive password as tool input, or reads it from the `JIRA_USERNAME`/`JIRA_PASSWORD` environment variables when the tool call omits `username`/`password` (see ADR-0004). Setting those environment variables before starting the MCP server is the safer default: it avoids ever putting the password into the chat/tool-call transcript, and lets `jira_authenticate` be called with no arguments. When both variables are already set, the server also authenticates automatically in the background right after startup (see ADR-0005), so no `jira_authenticate` call is needed at all in that case. The server does not persist either source, but an MCP client may retain tool-call history, logs, or transcripts for input actually passed as an argument. Operators must review client retention policy before passing credentials as tool input.

Only `jira_get_issue` is read-only. `jira_add_issue_comment`, `jira_update_issue_fields`, and `jira_transition_issue` mutate Jira and should be routed through client approval where available.

The server redacts password, token, authorization, secret, Basic, Bearer, and credential-bearing URL values from its own formatted diagnostics and Jira authentication results. This does not remove raw sensitive values already recorded by an MCP client before the request reaches the server.

Diagnostics and warnings go to `stderr`; MCP protocol traffic goes to `stdout`.
