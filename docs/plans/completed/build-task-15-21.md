# Execution Plan: Build Tasks 15-21

Date: 2026-08-01

## Status

Completed

## Outcome

Complete `docs/specs/SPECS.md` Tasks 15-21: Jira authentication and issue tools, Jira tool-level security/approval policy, and the Bash remote installer at the stable repository-root path.

## Context

- `AGENTS.md`
- `docs/WORKFLOW.md`
- `.codex/AGENTS.md`
- `.codex/orchestration/ORCHESTRATOR.md`
- `.codex/orchestration/wf_feature_development/WORKFLOW.md`
- `.codex/orchestration/wf_feature_development/state-model.yaml`
- `docs/specs/SPECS.md`
- `docs/specs/jira-6.4.14_bitbucket-5.10.2-rest-api-reference.md`
- Existing Jira code under `internal/jira/`

## Scope

In scope:

- Task 15 `jira_authenticate`.
- Task 16 `jira_get_issue`.
- Task 17 `jira_add_issue_comment`.
- Task 18 `jira_update_issue_fields`.
- Task 19 `jira_transition_issue`.
- Task 20 Jira tool-level security and approval policy.
- Task 21 Bash installer replacement at `scripts/install-from-remote.sh` with tests at `tests/install-from-remote_test.sh`.
- Documentation updates needed for the changed Jira tools and Bash installer.

Out of scope:

- Task 22 PowerShell installer.
- Tasks 23-27 release, smoke, packaging, and real-host verification.
- Tasks 6-11 Bitbucket business tools.

## Approach

Use `wf_feature_development` with design skipped because this is CLI/MCP backend and installer work. Reuse existing Jira tool/client structure and add only missing contract behavior and tests. Implement Bash installer from scratch with a small shell test harness and mocked external commands.

## Risks And Recovery

- Risk: the checkout is on `main` with unrelated `.codex/*` changes already present. Mitigation: do not touch `.codex/*`; keep Task 15-21 edits scoped to Jira, docs, scripts, tests, and this plan.
- Risk: installer logic can overgrow. Mitigation: keep provider-neutral Git source handling, atomic install/config writes, dry-run, and idempotency only.
- Risk: Task 22 PowerShell paths are adjacent to Task 21. Mitigation: document the PowerShell URL pattern but do not implement `scripts/install-from-remote.ps1`.
- Recovery: remove changes to `internal/jira/`, `docs/`, `scripts/install-from-remote.sh`, `tests/install-from-remote_test.sh`, and this plan if rejected.

## Progress

- [x] Requirement analysis started for Tasks 15-21.
- [x] Codebase exploration started for Tasks 15-21.
- [x] Baseline validation: `GOCACHE=F:\CodeSource\atlassian-mcp\.tmp\go-build go test ./...`.
- [x] Implement missing Jira Task 15-20 contract coverage.
- [x] Implement Task 21 Bash installer and tests.
- [x] Run focused and repository validation.
- [x] Run code review gate.
- [x] Move this plan to `docs/plans/completed/` after validation and review.

## Decisions

- 2026-08-01: Selected `wf_feature_development`; no UI/design state is required.
- 2026-08-01: Agent config log: `req_analyzer` uses `.codex/agents/req_analyzer.toml`, model `gpt-5.5`, reasoning `medium`, access `read-only`, approval `never`.
- 2026-08-01: Agent config log: `explorer` uses `.codex/agents/explorer.toml`, model `gpt-5.5`, reasoning `medium`, access `read-only`, approval `never`.
- 2026-08-01: Agent config log: `implementer` uses `.codex/agents/implementer.toml`, model `gpt-5.5`, reasoning `high`, access `workspace-write`, approval `on-request`.
- 2026-08-01: Agent config log: `code_reviewer` uses `.codex/agents/code_reviewer.toml`, model `gpt-5.6-terra`, reasoning `high`, access `read-only`, approval `never`.
- 2026-08-01: `.codex/orchestration/runtime` is treated as unavailable in this sandbox because `.codex` is read-only; this plan file is the durable workflow ledger.
- 2026-08-01: Keep final no-TTY installer regression as a documented validation limitation; it passes in this non-interactive runner but can hang when run from an interactive terminal unless detached from the controlling TTY.
- 2026-08-01: Git index executable mode `100755` could not be recorded in this sandbox because `.git/index.lock` creation is denied; filesystem mode is `755`.

## Validation

- Focused proof: `go test ./internal/jira/...`.
- Installer proof: `bash tests/install-from-remote_test.sh`.
- Repository proof: `GOCACHE=F:\CodeSource\atlassian-mcp\.tmp\go-build go test ./...`.
- Build proof: build `./cmd/atlassian-mcp` and run `--version`.
- 2026-08-01 Jira Task 15-20 RED: `GOCACHE=F:/CodeSource/atlassian-mcp/.tmp/go-build go test ./internal/jira/...` failed in `internal/jira/tools` for auth result sanitization, `data.issue` wrapping, blank visibility value, and transition refresh-option validation; `go test ./internal/observability` failed for typed tool input redaction.
- 2026-08-01 Jira Task 15-20 GREEN: `GOCACHE=F:/CodeSource/atlassian-mcp/.tmp/go-build go test ./internal/jira/...` passed; `GOCACHE=F:/CodeSource/atlassian-mcp/.tmp/go-build go test ./internal/observability` passed.
- 2026-08-01 Jira fix RED/GREEN: transition-name whitespace regression failed before fix and `GOCACHE=F:/CodeSource/atlassian-mcp/.tmp/go-build go test ./internal/jira/...` passed after exact-name preservation.
- 2026-08-01 Task 21 RED/GREEN: Bash installer tests first failed because `scripts/install-from-remote.sh` was missing, then passed with `PASS install-from-remote_test.sh`.
- 2026-08-01 Task 21 fix RED/GREEN: installer tests failed for wrapper name, missing non-interactive Bitbucket token env, omitted `--agents` prompt, piped installer source, service URL embedded credentials, and TOML/JSON command escaping before each corresponding fix; final installer suite passed.
- 2026-08-01 final repository validation passed: `GOCACHE=F:\CodeSource\atlassian-mcp\.tmp\go-build GOMODCACHE=F:\CodeSource\atlassian-mcp\.tmp\gomodcache go test ./...`.
- 2026-08-01 final installer validation passed: `"C:\Program Files\Git\bin\bash.exe" tests/install-from-remote_test.sh`.
- 2026-08-01 final Bash syntax validation passed: `"C:\Program Files\Git\bin\bash.exe" -lc 'bash -n scripts/install-from-remote.sh tests/install-from-remote_test.sh'`.
- 2026-08-01 final build validation passed: `go build -o F:\CodeSource\atlassian-mcp\.tmp\atlassian-mcp-task15-21.exe ./cmd/atlassian-mcp` and `--version` printed `atlassian-mcp 0.1.0`.
- 2026-08-01 `git diff --check -- . ':!.codex' ':!.superpowers' ':!.tmp'` passed.

## Result

Completed Tasks 15-21.

Implemented and validated Jira authentication/tool hardening, recursive typed redaction, Jira tool/security documentation, and the Bash installer at `scripts/install-from-remote.sh` with tests at `tests/install-from-remote_test.sh`.

Review gate:

- Jira scoped review found one Medium transition exact-match issue; fix re-review returned no actionable findings.
- Installer scoped reviews found Medium issues for non-interactive token validation, interactive `--agents`, wrapper naming, and piped installer stdin behavior; fixes were re-reviewed.
- Final whole-scope review found High service URL credential persistence and Medium agent-config escaping issues; fix re-review returned no blocking findings.

Limitations:

- ShellCheck is unavailable in this environment.
- Git index mode `100755` for `scripts/install-from-remote.sh` could not be verified or recorded because Git index writes fail with `.git/index.lock: Permission denied`; filesystem mode is executable.
- Real Jira 6.4.14 host compatibility and real Claude/Codex client smoke checks remain out of scope for Tasks 15-21.
