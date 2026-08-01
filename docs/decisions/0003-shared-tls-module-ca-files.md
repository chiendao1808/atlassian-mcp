# ADR: Shared TLS Verify With Module CA Files

Date: 2026-08-01

## Status

Accepted

## Decision

Use one shared TLS verification flag, `ATLASSIAN_TLS_VERIFY`, defaulting to `false`. Jira and Bitbucket may each provide their own CA file through `JIRA_CA_FILE` and `BITBUCKET_CA_FILE` when verification is enabled.

## Consequences

TLS policy is consistent across modules while still allowing different internal CA chains per Atlassian product.