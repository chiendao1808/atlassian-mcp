# Execution Plan: Build PowerShell Installer

Date: 2026-08-01

## Status

Completed

## Outcome

Complete the PowerShell remote installer counterpart at `scripts/install-from-remote.ps1` with tests at `tests/install-from-remote.Tests.ps1`, matching the installer semantics defined in `docs/specs/SPECS.md`.

## Context

- `AGENTS.md`
- `docs/WORKFLOW.md`
- `.codex/AGENTS.md`
- `.codex/agents/implementer.toml`
- `.codex/agents/code_reviewer.toml`
- `docs/specs/SPECS.md` sections 11.2, 11.3, 11.4, 17.2, and Task 22
- Existing Bash installer at `scripts/install-from-remote.sh`
- Existing Bash installer tests at `tests/install-from-remote_test.sh`

## Scope

In scope:

- Create `scripts/install-from-remote.ps1`.
- Create `tests/install-from-remote.Tests.ps1`.
- Update README PowerShell bootstrap documentation if needed.
- Mirror Bash installer validation semantics: provider-neutral source, module selection, non-secret config, token env indirection, dry-run, backup/rollback, idempotency, and stable final names.

Out of scope:

- PowerShell package signing.
- Real Claude Code, Codex, Jira, or Bitbucket smoke tests.
- Release packaging and checksums.
- Reworking the completed Bash installer except for documentation consistency.

## Approach

Reuse the Bash installer behavior as the executable reference and implement the smallest PowerShell script that supports Windows PowerShell 5.1. Keep tests self-contained with mocked `git` and `go` commands and direct PowerShell assertions.

## Risks And Recovery

- Risk: current checkout already has Task 15-21 uncommitted changes. Mitigation: do not revert or rewrite them; touch only PowerShell installer, tests, README/docs, and this plan.
- Risk: Windows ACL and parser behavior can vary. Mitigation: test command invocation and generated file content locally; disclose missing real-client smoke tests.
- Recovery: remove `scripts/install-from-remote.ps1`, `tests/install-from-remote.Tests.ps1`, related README edits, and this plan if rejected.

## Progress

- [x] Confirmed spec and runtime availability.
- [x] Created active plan.
- [x] Implement PowerShell installer and tests with RED/GREEN evidence.
- [x] Run focused PowerShell validation.
- [x] Run repository validation.
- [x] Run code review gate narrowed to `scripts/install-from-remote.ps1`.
- [x] Move this plan to `docs/plans/completed/`.

## Decisions

- 2026-08-01: Treat this as the PowerShell counterpart to Task 21; in current `SPECS.md` it is Task 22.
- 2026-08-01: Use Windows PowerShell 5.1 for local validation because `pwsh` is not available.
- 2026-08-01: Use direct PowerShell test execution; Pester 3.4 is present but newer Pester semantics are not assumed.
- 2026-08-01: Agent config log: `implementer` uses `.codex/agents/implementer.toml`, model `gpt-5.5`, reasoning `high`, access `workspace-write`, approval `on-request`.
- 2026-08-01: Agent config log: `code_reviewer` uses `.codex/agents/code_reviewer.toml`, model `gpt-5.6-terra`, reasoning `high`, access `read-only`, approval `never`.
- 2026-08-01: Review requirement was narrowed to the new PowerShell installer file only.

## Validation

- RED proof: `powershell.exe -NoProfile -ExecutionPolicy Bypass -File tests/install-from-remote.Tests.ps1` failed because `scripts/install-from-remote.ps1` did not exist.
- Focused proof: `powershell.exe -NoProfile -ExecutionPolicy Bypass -File tests/install-from-remote.Tests.ps1` passed.
- Syntax proof: PowerShell parser/token check for `scripts/install-from-remote.ps1` and `tests/install-from-remote.Tests.ps1` passed.
- Final focused proof on 2026-08-01: `powershell.exe -NoProfile -ExecutionPolicy Bypass -File tests/install-from-remote.Tests.ps1` passed.
- Final syntax proof on 2026-08-01: PowerShell parser/token check for `scripts/install-from-remote.ps1` and `tests/install-from-remote.Tests.ps1` passed.
- Final repository proof on 2026-08-01: `$env:GOCACHE='F:\CodeSource\atlassian-mcp\.tmp\go-build'; go test ./...` passed.
- Regression proof on 2026-08-01: `powershell.exe -NoProfile -ExecutionPolicy Bypass -File tests/install-from-remote.Tests.ps1` passed after fixing command stdout leakage into the built binary path.
- Regression syntax proof on 2026-08-01: PowerShell parser/token check for `scripts/install-from-remote.ps1` and `tests/install-from-remote.Tests.ps1` passed.
- Regression proof on 2026-08-01: `powershell.exe -NoProfile -ExecutionPolicy Bypass -File tests/install-from-remote.Tests.ps1` passed after preserving native stderr progress without turning it into a terminating installer error.
- Regression syntax proof on 2026-08-01: PowerShell parser/token check for `scripts/install-from-remote.ps1` and `tests/install-from-remote.Tests.ps1` passed.
- Regression proof on 2026-08-01: `powershell.exe -NoProfile -ExecutionPolicy Bypass -File tests/install-from-remote.Tests.ps1` passed after preserving the root cause when selected agent configuration fails.
- Regression syntax proof on 2026-08-01: PowerShell parser/token check for `scripts/install-from-remote.ps1` and `tests/install-from-remote.Tests.ps1` passed.
- Regression proof on 2026-08-01: `powershell.exe -NoProfile -ExecutionPolicy Bypass -File tests/install-from-remote.Tests.ps1` passed after changing Codex project config to launch the PowerShell wrapper through `powershell.exe` instead of executing `.ps1` directly.
- Regression syntax proof on 2026-08-01: PowerShell parser/token check for `scripts/install-from-remote.ps1` and `tests/install-from-remote.Tests.ps1` passed.
- Regression proof on 2026-08-01: `powershell.exe -NoProfile -ExecutionPolicy Bypass -File tests/install-from-remote.Tests.ps1` passed after replacing existing files via `.NET File.Replace` instead of depending on `Move-Item -Force` overwrite behavior.
- Regression syntax proof on 2026-08-01: PowerShell parser/token check for `scripts/install-from-remote.ps1` and `tests/install-from-remote.Tests.ps1` passed.
- Regression proof on 2026-08-01: `powershell.exe -NoProfile -ExecutionPolicy Bypass -File tests/install-from-remote.Tests.ps1` passed after granting current-user Modify ACL before replacing files created by earlier restrictive ACL logic.
- Regression syntax proof on 2026-08-01: PowerShell parser/token check for `scripts/install-from-remote.ps1` and `tests/install-from-remote.Tests.ps1` passed.
- Regression proof on 2026-08-01: `powershell.exe -NoProfile -ExecutionPolicy Bypass -File tests/install-from-remote.Tests.ps1` passed after making the generated wrapper read the Bitbucket token from process env first and persisted User env second.
- Regression syntax proof on 2026-08-01: PowerShell parser/token check for `scripts/install-from-remote.ps1` and `tests/install-from-remote.Tests.ps1` passed.
- Regression proof on 2026-08-01: `powershell.exe -NoProfile -ExecutionPolicy Bypass -File tests/install-from-remote.Tests.ps1` passed after moving cloned-source cleanup into a successful install post-action and preserving `-KeepSource`.
- Regression syntax proof on 2026-08-01: PowerShell parser/token check for `scripts/install-from-remote.ps1` and `tests/install-from-remote.Tests.ps1` passed.

## Result

PowerShell installer and focused tests are implemented and validated. Later regression fixes prevent external command output from being captured as the built binary path, prevent native stderr progress from becoming a terminating installer error, preserve root-cause details for selected-agent configuration failures, prevent Codex from trying to execute a `.ps1` file as a Win32 binary, make existing file replacement reliable on Windows, repair earlier restrictive installer ACLs before replacement, allow the wrapper to read a persisted User env Bitbucket token, and clean cloned remote source as a successful install post-action unless `-KeepSource` is set. No real Claude Code, Codex, Jira, or Bitbucket smoke test was run because those checks are outside this plan's scope.
