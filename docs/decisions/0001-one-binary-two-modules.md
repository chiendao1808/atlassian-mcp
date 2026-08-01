# ADR: One Binary With Jira And Bitbucket Modules

Date: 2026-08-01

## Status

Accepted

## Decision

Ship one Go MCP stdio command at `cmd/atlassian-mcp`, producing `atlassian-mcp` / `atlassian-mcp.exe`. MCP clients register it as `atlassian`.

Jira and Bitbucket are independent modules. Each module validates only its own static configuration and registers tools only when valid.

## Consequences

A broken Jira configuration does not block Bitbucket startup, and a broken Bitbucket configuration does not block Jira startup. There are no draft aliases or compatibility wrappers in the first release.