# Execution Plan: Build Bitbucket Tools

Date: 2026-08-01

## Status

Completed

## Outcome

Register and implement the 26 Bitbucket Server business tools from `docs/specs/SPECS.md` section 10 so they appear in Codex when the Bitbucket module static configuration is valid.

## Context

- `AGENTS.md`
- `docs/WORKFLOW.md`
- `docs/specs/SPECS.md` section 10 and Tasks 6-11
- Existing Bitbucket config/client foundation under `internal/bitbucket`
- Existing Jira tool registration pattern under `internal/jira/tools`

## Scope

In scope:

- Implement Bitbucket tool definitions, registration, input validation, and REST calls for the 26 specified tools.
- Wire the Bitbucket module to register tools.
- Add focused registry and endpoint tests.

Out of scope:

- Full docs for every Bitbucket tool.
- Real Bitbucket Server staging smoke tests.
- Advanced response truncation beyond existing client max-response enforcement.

## Approach

Use the existing Bitbucket client as the only REST surface. Preserve upstream JSON as `data` maps, validate only local safety boundaries, and avoid model packages unless a typed request body is needed.

## Progress

- [x] Created active plan.
- [x] Implement tools and registration.
- [x] Add focused tests.
- [x] Run validation.
- [x] Split Bitbucket tool handlers and registration by branch, commit, and pull request groups.

## Validation

- Focused Bitbucket proof: `$env:GOCACHE='F:\CodeSource\atlassian-mcp\.tmp\go-build'; go test ./internal/bitbucket/...` passed.
- Repository proof: `$env:GOCACHE='F:\CodeSource\atlassian-mcp\.tmp\go-build'; go test ./...` passed.
- Follow-up grouping proof: `$env:GOCACHE='F:\CodeSource\atlassian-mcp\.tmp\go-build'; go test ./internal/bitbucket/...` passed.
- Follow-up repository proof: `$env:GOCACHE='F:\CodeSource\atlassian-mcp\.tmp\go-build'; go test ./...` passed.
- Build proof: `$env:GOCACHE='F:\CodeSource\atlassian-mcp\.tmp\go-build'; $env:GOMODCACHE='F:\CodeSource\atlassian-mcp\.tmp\gomodcache'; go build -o F:\CodeSource\atlassian-mcp\.tmp\atlassian-mcp-bitbucket-tools.exe ./cmd/atlassian-mcp` passed, and `--version` output `atlassian-mcp 0.1.0`.
- Installer regression proof: `powershell.exe -NoProfile -ExecutionPolicy Bypass -File tests/install-from-remote.Tests.ps1` passed.
- PowerShell syntax proof: PowerShell parser/token check for `scripts/install-from-remote.ps1` and `tests/install-from-remote.Tests.ps1` passed.

## Result

Implemented and registered the 26 Bitbucket business tools from `docs/specs/SPECS.md` section 10. The implementation preserves upstream JSON responses, uses the existing Bitbucket client, validates local safety boundaries, and includes focused registry/endpoint tests. A follow-up split grouped handlers and registration into branch, commit/file/compare, and pull request surfaces, with comments on path/ref/version/content logic. Real Bitbucket Server staging smoke tests and full per-tool documentation were not run because they are outside this plan's scope.
