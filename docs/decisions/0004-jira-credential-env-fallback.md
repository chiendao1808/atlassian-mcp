# ADR: Jira Credential Environment Fallback

Date: 2026-08-04

## Status

Accepted

## Context

`jira_authenticate` required `username`/`password` as tool-call input on every
authentication (ADR-0002). Operators who re-authenticate often were retyping
credentials into the coding-agent chat session each time, which the MCP client may
retain in tool-call history, logs, or transcripts (`docs/security.md`). Operators who
can set process environment variables for the MCP server wanted a way to avoid ever
typing the password into chat at all.

## Decision

`jira_authenticate` resolves `username`/`password` with explicit tool input taking
precedence, falling back to the `JIRA_USERNAME`/`JIRA_PASSWORD` environment variables
(read via `os.Getenv` at call time) whenever the corresponding input field is empty. If
neither source supplies both values, the tool returns `VALIDATION_ERROR` without making
any Jira request -- the tool never prompts anyone itself, it only reports that a
username and password are still needed from one of the two sources.

This supersedes ADR-0002's "Credentials are not read from process environment"
consequence. `JIRA_USERNAME` and `JIRA_PASSWORD` are removed from the SPECS.md Sec 4.2
forbidden-variable list; the alternate-name and token-based auth variables it still
forbids remain forbidden and out of scope.

Everything else in ADR-0002 is unchanged: validation against `/rest/api/2/serverInfo`
and `/rest/api/2/myself`, atomic in-memory replacement only after both succeed, no disk
persistence, and no logout tool.

## Alternatives Considered

1. Keep tool-input-only credentials and rely solely on documentation warning operators
   about client retention policy. Rejected: does not remove the risk, only describes it.
2. Add a configurable env-var-name indirection (mirroring `-BitbucketTokenEnv`) for
   Jira. Rejected for this change: Jira needs two values, not one secret, and the
   installer is intentionally left untouched -- operators set `JIRA_USERNAME`/
   `JIRA_PASSWORD` themselves, the same way `JIRA_BASE_URL` already works.

## Consequences

Positive:

- Operators who can set process environment variables never need to type a Jira
  password into a chat session.
- Explicit tool input still works unchanged for operators who prefer it, or who need to
  switch users within a session.

Tradeoffs:

- `JIRA_USERNAME`/`JIRA_PASSWORD`, once set for the process, apply to every
  `jira_authenticate` call in that process unless the caller passes explicit input to
  override them.
- Changing the env vars at the OS level requires restarting the MCP server process to
  take effect, same as `JIRA_BASE_URL` today.

## Follow-Up

- None planned. Installer support for setting `JIRA_USERNAME`/`JIRA_PASSWORD` at
  install time was considered and deliberately left out of scope for this change.
