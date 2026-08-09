# ADR: Shared Basic Auth Primitives With Separate Confluence Session

Date: 2026-08-09

## Status

Accepted

## Context

Jira already held Basic Auth credentials in memory and atomically replaced the
active session only after validation. Confluence needs the same credential and
session-store behavior, but a Jira login must never authenticate Confluence or
the reverse.

## Decision

The credential value and atomic in-memory session store live in `internal/auth`.
Jira and Confluence each create their own `auth.NewSessionStore()` when their
module is statically enabled. Module-specific validation, error codes, REST
paths, and result shapes stay in the Jira and Confluence packages.

Confluence validates credentials with `GET /rest/api/user/current`; Jira keeps
its existing `serverInfo` then `myself` validation. Both modules may fall back
to their own username/password environment variables and both automatic startup
authentication paths call their module service's public `Authenticate` method.

## Alternatives Considered

1. Keep a copied Jira auth package under `internal/confluence`. Rejected:
   duplicate secret-handling code would drift for no gain.
2. Share one active Atlassian session store across Jira and Confluence.
   Rejected: credentials and permissions are module-specific, and cross-service
   authentication would be a security bug.
3. Create a generic Atlassian REST/auth service. Rejected: Jira and Confluence
   have different validation endpoints, base API paths, and tool error codes.

## Consequences

Positive:

- Secret string redaction and failed-replacement preservation are implemented
  once.
- Jira and Confluence sessions remain isolated by construction.

Tradeoffs:

- Each module still owns a small amount of similar authentication orchestration.
  That is intentional until a real third Basic Auth module needs the same flow.

## Follow-Up

- None planned.
