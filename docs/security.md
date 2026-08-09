# MCP Client Security Notes

`jira_authenticate` accepts a sensitive password as tool input, or reads it from the `JIRA_USERNAME`/`JIRA_PASSWORD` environment variables when the tool call omits `username`/`password` (see ADR-0004). `confluence_authenticate` follows the same pattern with `CONFLUENCE_USERNAME`/`CONFLUENCE_PASSWORD` (see ADR-0006). Setting those environment variables before starting the MCP server is the safer default: it avoids ever putting the password into the chat/tool-call transcript, and lets the authenticate tools be called with no arguments. When both variables for a module are already set, the server also authenticates that module automatically in the background right after startup (see ADR-0005 and ADR-0006), so no explicit authenticate call is needed at all in that case. The server does not persist either source, but an MCP client may retain tool-call history, logs, or transcripts for input actually passed as an argument. Operators must review client retention policy before passing credentials as tool input.

Confluence V1 data tools are read-only content and space GET requests after authentication. Jira includes both read and mutation tools; Jira mutation tools should be routed through client approval where available.

The server redacts password, token, authorization, secret, Basic, Bearer, and credential-bearing URL values from its own formatted diagnostics and Jira/Confluence authentication results. This does not remove raw sensitive values already recorded by an MCP client before the request reaches the server.

Diagnostics and warnings go to `stderr`; MCP protocol traffic goes to `stdout`.
