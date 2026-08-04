# ADR: Automatic Jira Authentication On Startup

Date: 2026-08-04

## Status

Accepted

## Context

ADR-0004 let `jira_authenticate` resolve `username`/`password` from `JIRA_USERNAME`/
`JIRA_PASSWORD` when the tool call omits them, but an operator who sets those variables
still had to make one explicit `jira_authenticate` call (with empty/no arguments) at the
start of every MCP session before any other Jira tool would work.

## Decision

When the Jira module is enabled and `JIRA_USERNAME`/`JIRA_PASSWORD` are both already
present in the process environment, `atlassian-mcp` calls the same `jira_authenticate`
logic once automatically right after startup, in a background goroutine that does not
block the stdio transport from serving `initialize`. If either variable is absent, no
extra network call happens and nothing is logged -- the operator sees no difference from
today's behavior. If both are present but authentication fails (bad credentials,
unreachable Jira), a warning is logged to `stderr` and the module stays registered with
tools returning `JIRA_NOT_AUTHENTICATED`, exactly as if `jira_authenticate` had simply
never been called; no reachability check runs against Bitbucket, and this never affects
whether tools get registered (Section 3.2's static-config-only registration rule is
unchanged).

This reuses `Service.Authenticate` unchanged (`internal/jira/tools/service.go`) via a new
`Module.AutoAuthenticate` method (`internal/jira/module.go`) invoked from
`cmd/atlassian-mcp/main.go`; no new validation or network path was added.

## Alternatives Considered

1. Require every session to call `jira_authenticate` explicitly, even with the env
   fallback from ADR-0004. Rejected: still an extra round trip per session for the
   common case where credentials are fully supplied via environment.
2. Run the startup authentication synchronously before `server.Run`. Rejected: an
   unreachable or slow Jira instance would delay the MCP `initialize` handshake with the
   client; a background goroutine keeps startup responsive regardless of Jira's health.

## Consequences

Positive:

- Operators who set `JIRA_USERNAME`/`JIRA_PASSWORD` get a ready-to-use Jira session with
  zero explicit authentication calls.

Tradeoffs:

- The first few seconds after startup have a race between the background
  authentication attempt and any Jira tool call the client makes immediately; a call
  that lands before authentication finishes still gets `JIRA_NOT_AUTHENTICATED` and must
  be retried, same as it would today if a client called a Jira tool before calling
  `jira_authenticate`.
- A failed automatic attempt (e.g. wrong password) is only visible on `stderr`, which
  the MCP client may or may not surface to the operator.

## Follow-Up

- None planned.
