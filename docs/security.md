# MCP Client Security Notes

`jira_authenticate` receives a sensitive password as tool input. The server does not persist it, but an MCP client may retain tool-call history, logs, or transcripts. Operators must review client retention policy before using Jira authentication.

Only `jira_get_issue` is read-only. `jira_add_issue_comment`, `jira_update_issue_fields`, and `jira_transition_issue` mutate Jira and should be routed through client approval where available.

The server redacts password, token, authorization, secret, Basic, Bearer, and credential-bearing URL values from its own formatted diagnostics and Jira authentication results. This does not remove raw sensitive values already recorded by an MCP client before the request reaches the server.

Diagnostics and warnings go to `stderr`; MCP protocol traffic goes to `stdout`.
