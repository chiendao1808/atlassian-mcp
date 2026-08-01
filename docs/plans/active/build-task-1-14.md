# Execution Plan: Build Tasks 1-14

Date: 2026-08-01

## Status

Active - reopened for supplemented Task 5 Bitbucket client foundation.

## Outcome

Build the initial `atlassian-mcp` Go stdio server for `docs/specs/SPECS.md` tasks 1-14: final naming, shared config/module isolation, shared transport/redaction, Jira static config, process-session Jira Basic Auth, and the five Jira MCP tools.

## Context

- `AGENTS.md`
- `docs/WORKFLOW.md`
- `.codex/AGENTS.md`
- `.codex/orchestration/wf_feature_development/WORKFLOW.md`
- `docs/specs/SPECS.md`

## Scope

In scope:

- Tasks 1-14 from `docs/specs/SPECS.md`.
- Task 5 Bitbucket static configuration and REST client foundation from the supplemented `docs/specs/SPECS.md`.

Out of scope:

- Tasks 6-11 Bitbucket business tool handlers.
- Tasks 15-21 installers, client integration smoke tests, release packaging, and real Jira/Bitbucket staging verification.

## Approach

Implement the smallest Go module that compiles, runs over MCP stdio via the official Go SDK, and exposes Jira tools when static Jira config is valid. Keep shared infrastructure small and covered by focused unit/contract tests.

## Risks And Recovery

- Risk: the repo has no committed baseline. Mitigation: do not mutate git history; report untracked state.
- Risk: Task 5 foundation could drift into Tasks 6-11 business tools. Mitigation: keep only shared config/client behavior here and leave MCP business tools to follow-up tasks.
- Recovery: remove newly added `cmd/`, `internal/`, `go.mod`, generated `go.sum`, and task docs if this implementation is rejected.

## Progress

- [x] Requirement analysis: feature development, tasks 1-14, no UI design phase.
- [x] Codebase exploration: docs-only checkout; no existing implementation.
- [x] Plan review: user requested implementation from planned spec for task 1-14.
- [x] Write failing tests.
- [x] Implement minimal Go server and Jira tools.
- [x] Complete supplemented Task 5 Bitbucket static config/client foundation.
- [x] Run Task 5 focused Go validation.
- [ ] Run Task 5 code review gate.
- [x] Add docs/ADRs for task 1 and tool/security docs for task 14.
- [x] Run focused Go validation.
- [x] Review diff and record result.

## Decisions

- 2026-08-01: Use `wf_feature_development`; design state skipped because the feature is a CLI/MCP backend with no UI.
- 2026-08-01: Use official `github.com/modelcontextprotocol/go-sdk/mcp`; official docs identify `mcp.NewServer`, `mcp.AddTool`, and `mcp.StdioTransport` as the stdio server path.
- 2026-08-01: Do not fabricate Bitbucket tools beyond the active task. The supplemented spec now makes Task 5 authoritative for Bitbucket static config and REST client foundation only; Tasks 6-11 own business tools.
- 2026-08-01: Orchestration runtime state in `.codex/orchestration/runtime` is unavailable in this sandbox because `.codex` is read-only; this plan file is the durable workflow ledger for Task 5.

## Validation

- Focused proof: `go test ./...`
- Integration or end-to-end proof: mock HTTP server contract tests for Jira client/tools.
- Repository-required checks: forbidden-name scan where practical.

## Result

Implemented the initial Go `atlassian-mcp` command, shared config/module registry, Jira static config, session credential store, Jira REST client, five Jira tool handlers, shared result/redaction/transport infrastructure, docs, ADRs, and focused tests.

Initial validation passed on 2026-08-01:

- `go test ./...`
- `go build -o $env:TEMP/atlassian-mcp-validation.exe ./cmd/atlassian-mcp` followed by `--version`, output `atlassian-mcp 0.1.0`.

Task 5 validation passed on 2026-08-01:

- RED: `go test ./internal/bitbucket/...` failed before client implementation with undefined Bitbucket client symbols.
- RED: `go test ./internal/bitbucket/client` failed before pagination helper implementation with `page.NextQuery undefined`.
- GREEN: `GOCACHE=F:\CodeSource\atlassian-mcp\.tmp\go-build go test ./...`
- GREEN: `go build -o F:\CodeSource\atlassian-mcp\.tmp\atlassian-mcp-validation.exe ./cmd/atlassian-mcp` followed by `--version`, output `atlassian-mcp 0.1.0`.

Task 5 review gate:

- `req_analyzer` used `.codex/agents/req_analyzer.toml`: model `gpt-5.5`, reasoning `medium`, read-only, approval `never`.
- `explorer` used `.codex/agents/explorer.toml`: model `gpt-5.5`, reasoning `medium`, read-only, approval `never`.
- Two `code_reviewer` attempts used `.codex/agents/code_reviewer.toml`: model `gpt-5.6-sol`, reasoning `high`, read-only, approval `never`; both timed out and were closed without findings.

Limitations:

- Tasks 6-11 Bitbucket business tools remain out of scope for this Task 5 completion.
- Tasks 15-21 remain out of scope for this execution.
