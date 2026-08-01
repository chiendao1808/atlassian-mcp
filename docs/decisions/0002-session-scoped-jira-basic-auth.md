# ADR: Session Scoped Jira Basic Auth

Date: 2026-08-01

## Status

Accepted

## Decision

Jira authentication is performed by `jira_authenticate` once per MCP process session. The server validates candidate credentials against `/rest/api/2/serverInfo` and `/rest/api/2/myself`, then atomically replaces the active in-memory credential.

## Consequences

Credentials are not read from process environment and are not persisted by application code. Failed re-authentication preserves the existing active credential.