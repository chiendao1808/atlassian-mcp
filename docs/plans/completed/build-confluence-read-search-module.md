# Execution Plan: Build Confluence Read/Search Module

Date: 2026-08-09

## Status

Completed - implementation, independent review, and user-approved live staging
validation complete.

## Outcome

Add an independently configured Confluence Server 6.1.2 module with exactly
three MCP tools:

- `confluence_authenticate`
- `confluence_search_content`
- `confluence_get_content`

The module uses the same in-memory Basic Auth credential/session primitives as
Jira, but each module owns a separate session. It is enabled only with a valid
`CONFLUENCE_BASE_URL`; invalid Confluence configuration must not disable Jira
or Bitbucket. When both `CONFLUENCE_USERNAME` and `CONFLUENCE_PASSWORD` are
present at startup and no Confluence session exists yet, the module validates
those credentials once in a background goroutine through the same shared
authentication logic as `confluence_authenticate`, then commits only if the
session is still empty. A failure is a sanitized non-fatal warning and leaves
the module registered but unauthenticated.

## Context

Authority and workflow:

- `AGENTS.md`
- `.codex/AGENTS.md`
- `docs/WORKFLOW.md`
- `.agents/orchestration/wf_feature_development/WORKFLOW.md`
- `.agents/orchestration/wf_feature_development/state-model.md`

API authority: `docs/specs/confluence-6.1.2_6.1.4-rest-api-reference.md`.
Runtime compatibility target is Confluence Server 6.1.2; REST 6.1.4 is the
readable reference in the same compatibility line.

Verified repository anchors:

- `internal/app/module.go` and `internal/app/module_registry.go` for module
  lifecycle and static-configuration isolation.
- `internal/jira/auth`, `internal/jira/client`, and `internal/jira/tools` for
  the current Basic Auth/session, bounded JSON, and MCP handler patterns.
- `internal/transport`, `internal/result`, and `internal/observability` for
  shared HTTP policy, result envelopes, and redaction.
- `internal/jira/module.go` and `cmd/atlassian-mcp/main.go` for the existing
  non-blocking automatic Jira authentication lifecycle.

## Scope

In scope:

- Extract Jira's Basic credential and atomic in-memory session store to
  `internal/auth`; update Jira imports without observable Jira behavior change.
- Add Confluence static configuration, GET-only JSON client, module wiring,
  three handlers, focused tests, tool documentation, and an ADR for shared
  primitives with separate sessions.
- Use `CONFLUENCE_BASE_URL` and optional `CONFLUENCE_CA_FILE` as static
  configuration. During an explicit `confluence_authenticate` call, fall back
  field-by-field to optional `CONFLUENCE_USERNAME` and `CONFLUENCE_PASSWORD`.
- Automatically validate environment credentials once at startup when both
  credential variables are present and no Confluence session already exists. It
  remains asynchronous and non-fatal, matching Jira startup behavior, but
  commits only to an empty session.
- Add user-authorized installer support in `scripts/install-from-remote.sh` and
  `scripts/install-from-remote.ps1` for enabling Confluence, setting base URL
  and optional CA file, and resolving optional username/password environment
  indirection into the fixed runtime `CONFLUENCE_*` variables.

Out of scope:

- Any Confluence mutation, attachment transfer, child/comment/space/label
  tool, generic request tool, content conversion, crawl/index/cache, dependency
  addition, synchronous startup authentication, or a second authentication
  implementation.
- Jira or Bitbucket contract changes, complete HTTP-client generalization, and
  changes to `docs/specs/SPECS.md` or the supplied REST reference.

## Tool Contracts

| Tool | REST request | Contract |
| --- | --- | --- |
| `confluence_authenticate` | `GET /rest/api/user/current` | Resolves explicit Basic credentials then environment fallback. A response must identify a known authenticated user before replacing the module-local session. |
| `confluence_search_content` | `GET /rest/api/content/search` | Requires raw `cql`; accepts `cqlcontext`, `expand`, `start`, and `limit`. Sends `limit=25` if omitted. |
| `confluence_get_content` | `GET /rest/api/content/{id}` | Requires a single safe `contentId`; accepts `status`, positive `version`, and caller-supplied `expand`. |

Search and get require an authenticated Confluence session and return
`CONFLUENCE_NOT_AUTHENTICATED` with no network request otherwise. No default
body representation is injected: callers request `body.storage` or `body.view`
through `expand`. Preserve upstream JSON inside the shared result envelope,
including `_links` and `_expandable`.

## Approach

1. Move only credential/session primitives from `internal/jira/auth` to
   `internal/auth`; retain a fresh `auth.NewSessionStore()` per enabled module.
   Keep module-specific validation endpoints and error codes outside the shared
   package.
2. Add `internal/confluence` with config, module, client, and tools. Use the
   shared transport and `http.Request.SetBasicAuth`; do not generalize Jira's
   `/rest/api/2/` client because Confluence requires `/rest/api/`.
3. Wire the module in `cmd/atlassian-mcp/main.go`. Add `Module.AutoAuthenticate`
   to reuse shared Confluence authentication validation only when both
   credential variables are set and the Confluence session is empty; launch it
   asynchronously so it never blocks MCP initialization, and commit only if the
   session remains empty after validation.
4. Mirror focused Jira test patterns for configuration, Basic headers, URL
   context path, pre-auth guards, session replacement, query encoding, error
   mapping, exact tool roster, and module isolation.
5. Document configuration, explicit authentication, raw CQL, expansions,
   security boundaries, and real-host staging limitations. Record the lasting
   shared-auth/separate-session decision.

## Files

Create:

- `internal/auth/{credential.go,session_store.go,session_store_test.go}`
- `internal/confluence/config.go`, `module.go`, and their tests
- `internal/confluence/client/{client.go,client_test.go}`
- `internal/confluence/tools/{register.go,service.go,tools_test.go}`
- `docs/tools/confluence.md`
- `docs/decisions/0006-shared-basic-auth-confluence-session.md`

Modify:

- `internal/jira/module.go`, `internal/jira/client/client.go`,
  `internal/jira/tools/service.go`, and affected Jira tests, for shared-auth
  imports only.
- `cmd/atlassian-mcp/main.go` and
  `internal/app/module_isolation_integration_test.go`.
- `README.md`, `docs/architecture.md`, `docs/security.md`, and the decisions
  index when it exists.
- `scripts/install-from-remote.sh`, `scripts/install-from-remote.ps1`, and
  their focused installer tests after the user-approved scope extension.

Delete after the move:

- `internal/jira/auth/{credential.go,session_store.go,session_store_test.go}`

## Risks And Recovery

- Shared-auth extraction can regress Jira: move behavior unchanged first and
  run Jira tests before Confluence work. Revert the extraction and imports as
  one coherent change if needed.
- Do not share session stores: explicit isolation tests prove authenticating
  Confluence never authenticates Jira. Restarting the process clears both
  in-memory sessions.
- Automatic authentication can race an immediate read/search call. That call
  may return `CONFLUENCE_NOT_AUTHENTICATED`, matching Jira's non-blocking
  lifecycle; callers retry or call `confluence_authenticate` explicitly.
- A bad startup credential, anonymous response, TLS failure, or unreachable
  server leaves the module registered but unauthenticated. Emit one sanitized
  warning and retain explicit authentication as the recovery path.
- Confluence may return 404 for absent or invisible content: map it neutrally
  as `NOT_FOUND_OR_NOT_VISIBLE`.
- A context path may be present in the configured base URL: preserve it in
  `/rest/api/` request construction and test it with `/confluence`.
- No live 6.1.2 host is currently available: do not claim real-host
  compatibility until the staging checks below run.

## Progress

- [x] Requirements normalized as read/search-only V1.
- [x] User confirmed the read/search boundary.
- [x] REST reference, existing modules, and active/completed plans reviewed.
- [x] Design skipped: backend MCP module with no UI surface.
- [x] Implementation plan prepared.
- [x] User required Jira-style non-blocking Confluence auto-authentication.
- [x] User approves the exact implementation scope.
- [x] Shared auth extraction and Jira regression proof complete.
- [x] Confluence module and three tools complete.
- [x] Documentation, ADR, and self-verification complete.
- [x] Independent review complete; CR-001, CR-002, and CR-003 remediated and
  accepted on re-review.
- [x] User approved extending the implementation to Confluence installer flags
  and README coverage.
- [x] Bash and PowerShell installer support for Confluence configuration and
  credential env-name indirection implemented.
- [x] Installer review findings CR-INST-001/002/003 remediated: selected
  Confluence config is authoritative across reinstall/ambient environments,
  README no longer prints secrets, and PowerShell validates configured secret
  env-name values before dry-run/build.
- [x] User tested the real Confluence environment and approved the implemented
  plan.

## Decisions

- 2026-08-09: V1 has only authentication, raw CQL search, and content-by-ID
  retrieval. CQL covers search by space, title, type, and label without extra
  tools.
- 2026-08-09: Reuse shared Basic credential/session-store code, but never
  share an active credential between Jira and Confluence.
- 2026-08-09: Confluence has a dedicated GET-only client; no generic Atlassian
  HTTP abstraction is justified.
- 2026-08-09: When both Confluence credential variables are present, validate
  them once in a background goroutine at startup only if no Confluence session
  exists yet. Missing either variable sends no request and writes no warning.
- 2026-08-09: Failed startup authentication is non-fatal: the module remains
  registered and unauthenticated, while a sanitized warning is written to
  `stderr`.
- 2026-08-09: `expand` is always caller-selected; no implicit storage or view
  body representation is chosen.
- 2026-08-09: Background Confluence auto-authentication must not overwrite a
  successful explicit `confluence_authenticate` session. It may start only from
  an empty session and may commit only if the module-local session is still
  empty after validation.

## Validation

Focused proof:

- `go test ./internal/auth/... ./internal/jira/... ./internal/confluence/... ./internal/app/...`
- Exact Basic header and context-path URL construction.
- Auth success, anonymous rejection, failed re-authentication preservation,
  input precedence/environment fallback, zero-network pre-auth guards, and
  startup success/skip/failure/race behavior.
- CQL/query encoding, caller-provided expand, result-shape preservation,
  bounded response, sanitized errors, exact tool roster, and module isolation.
- Regression proof for CR-001/CR-002/CR-003: delayed startup auto-auth versus
  fast explicit re-auth, explicit-before-auto skip with zero env request,
  username-only/password-only startup skip, and MCP tool descriptions that
  distinguish authenticated-session requirements from the explicit
  setup/recovery tool.
- Installer proof: Bash wrapper keeps only Confluence password environment
  variable names, Codex receives resolved fixed `CONFLUENCE_*` values, Claude
  project config remains secret-free, Confluence username requires
  `--enable-confluence`/`-EnableConfluence`, and missing configured
  Confluence password variables fail validation.
- Installer remediation proof: Bash wrapper clears stale fixed
  `CONFLUENCE_*` runtime values when Confluence is disabled or credentials are
  omitted, while preserving `--confluence-password-env=CONFLUENCE_PASSWORD` by
  capturing the source value before clearing fixed runtime keys. PowerShell
  clears stale managed Confluence User env keys on disable/omit and rejects
  missing configured credential env-name values before dry-run/build regardless
  of `-NonInteractive`. README credential checks report only set/missing state.

Repository checks:

- `go test ./...`
- `go vet ./...`
- `go build ./cmd/atlassian-mcp`
- Run the built binary with `--version`.
- `git diff --check` and complete Git diff review.

Staging gates before claiming real Confluence 6.1.2 compatibility:

1. Verify a real context-path base URL and TLS/CA configuration.
2. Verify valid, invalid, and anonymous current-user authentication behavior.
3. Search `type = page` with an explicit small limit.
4. Fetch a returned ID with `body.storage` and `body.view` expansions.
5. Record target 403 versus 404 visibility behavior and confirm diagnostics do
   not expose credentials.
6. Start with both credentials to verify background authentication, then with
   one missing credential to prove there is no request or warning, and with an
   invalid credential to prove explicit re-authentication recovers the session.

## Result

Implemented on 2026-08-09. Delivered `internal/auth`, migrated Jira to the
shared auth package, added Confluence static config/client/module/tools, wired
Confluence into `cmd/atlassian-mcp`, and documented the V1 tool contract plus
ADR-0006.

Automated proof recorded so far:

- Red: `GOCACHE=F:\CodeSource\atlassian-mcp\.tmp\go-build go test ./internal/auth ./internal/confluence/...` failed because the new packages/functions did not exist yet.
- Green focused: `GOCACHE=F:\CodeSource\atlassian-mcp\.tmp\go-build go test ./internal/auth ./internal/jira/... ./internal/confluence/... ./internal/app/...` passed.
- Repository-wide: `GOCACHE=F:\CodeSource\atlassian-mcp\.tmp\go-build go test ./...` passed.
- Static analysis: `GOCACHE=F:\CodeSource\atlassian-mcp\.tmp\go-build go vet ./...` passed.
- Build/version: after deleting the old artifact, `GOCACHE=F:\CodeSource\atlassian-mcp\.tmp\go-build go build -o .tmp\atlassian-mcp.exe ./cmd/atlassian-mcp` produced a new binary and `.tmp\atlassian-mcp.exe --version` printed `atlassian-mcp 0.1.0`. Go also emitted a non-fatal stat-cache warning for `C:\Users\leope\go\pkg\mod\cache\...: Access is denied` because the default module cache is outside the writable sandbox.
- Whitespace: `git diff --check` passed.
- Remediation red: focused Confluence tests failed for CR-001 stale auto-auth
  overwrite and CR-002 missing setup/recovery wording before the fix.
- Remediation focused green: `GOCACHE=F:\CodeSource\atlassian-mcp\.tmp\go-build go test -run 'TestAutoAuthenticate|TestReplaceIfUnchanged|TestConfluenceToolDefinitions' ./internal/auth ./internal/confluence ./internal/confluence/tools` passed.
- Remediation repository-wide: `GOCACHE=F:\CodeSource\atlassian-mcp\.tmp\go-build go test ./...` passed.
- Remediation static analysis: `GOCACHE=F:\CodeSource\atlassian-mcp\.tmp\go-build go vet ./...` passed.
- Remediation build/version: after deleting the old artifact,
  `GOCACHE=F:\CodeSource\atlassian-mcp\.tmp\go-build go build -o .tmp\atlassian-mcp.exe ./cmd/atlassian-mcp` produced a new binary and `.tmp\atlassian-mcp.exe --version` printed `atlassian-mcp 0.1.0`. Go again emitted the non-fatal stat-cache warning for `C:\Users\leope\go\pkg\mod\cache\...: Access is denied`.
- Remediation whitespace: `git diff --check` passed.
- Second remediation red: `GOCACHE=F:\CodeSource\atlassian-mcp\.tmp\go-build go test -run 'TestAutoAuthenticate' ./internal/confluence` failed because AutoAuthenticate still sent one env credential request when an explicit session already existed.
- Second remediation focused green: `GOCACHE=F:\CodeSource\atlassian-mcp\.tmp\go-build go test -run 'TestAutoAuthenticate' ./internal/confluence` passed.
- Second remediation race detector: `GOCACHE=F:\CodeSource\atlassian-mcp\.tmp\go-build go test -race ./internal/auth ./internal/confluence ./internal/confluence/tools` passed.
- Second remediation repository-wide: `GOCACHE=F:\CodeSource\atlassian-mcp\.tmp\go-build go test ./...` passed.
- Second remediation static analysis: `GOCACHE=F:\CodeSource\atlassian-mcp\.tmp\go-build go vet ./...` passed.
- Second remediation build/version: after deleting the old artifact,
  `GOCACHE=F:\CodeSource\atlassian-mcp\.tmp\go-build go build -o .tmp\atlassian-mcp.exe ./cmd/atlassian-mcp` produced a binary and `.tmp\atlassian-mcp.exe --version` printed `atlassian-mcp 0.1.0`. Go again emitted the non-fatal stat-cache warning for `C:\Users\leope\go\pkg\mod\cache\...: Access is denied`.
- Second remediation whitespace: `git diff --check` passed.
- Installer red proof: Bash dry-run with `--enable-confluence` failed with
  `unknown option: --enable-confluence`; PowerShell dry-run with
  `-EnableConfluence` still failed the module-selection gate that only accepted
  Jira/Bitbucket.
- Installer focused green: `C:\Program Files\Git\bin\bash.exe
  tests/install-from-remote_test.sh` passed, including Confluence wrapper,
  Codex env, Claude secret-free, and validation cases.
- PowerShell installer green checks: Confluence-only dry-run passed;
  PowerShell parser check for `scripts/install-from-remote.ps1` passed;
  negative dry-runs for `-ConfluenceUsername` without `-EnableConfluence` and
  for missing `-ConfluencePasswordEnv` returned the expected errors.
- PowerShell installer harness limitation: `powershell.exe -NoProfile
  -ExecutionPolicy Bypass -File tests\install-from-remote.Tests.ps1` could not
  run in this sandbox because Windows denied HKCU User environment registry
  writes during test environment restore (`Requested registry access is not
  allowed`).
- Installer extension repository proof: `GOCACHE=F:\CodeSource\atlassian-mcp\.tmp\go-build
  go test ./internal/confluence/... ./internal/jira/... ./internal/app/...`,
  `GOCACHE=F:\CodeSource\atlassian-mcp\.tmp\go-build go test ./...`,
  `GOCACHE=F:\CodeSource\atlassian-mcp\.tmp\go-build go vet ./...`,
  `GOCACHE=F:\CodeSource\atlassian-mcp\.tmp\go-build go build -o
  .tmp\atlassian-mcp.exe ./cmd/atlassian-mcp`, `.tmp\atlassian-mcp.exe
  --version`, and `git diff --check` passed. The build emitted the same
  non-fatal Go stat-cache warning for `C:\Users\leope\go\pkg\mod\cache\...:
  Access is denied`.
- Installer remediation red proof: `C:\Program Files\Git\bin\bash.exe
  tests/install-from-remote_test.sh` failed because the generated wrapper
  leaked stale fixed `CONFLUENCE_BASE_URL`, `CONFLUENCE_CA_FILE`,
  `CONFLUENCE_USERNAME`, and `CONFLUENCE_PASSWORD` from the ambient
  environment when Confluence was disabled. PowerShell dry-run without
  `-NonInteractive` accepted a missing `-ConfluencePasswordEnv` source and
  printed `dry-run: validated installer arguments`.
- Installer remediation green proof: `C:\Program Files\Git\bin\bash.exe
  tests/install-from-remote_test.sh` passed after adding wrapper clearing and
  password-alias preservation. PowerShell parser check passed. PowerShell
  Confluence dry-run with `$env:CONFLUENCE_PASSWORD` set passed, while the
  same dry-run with `-ConfluencePasswordEnv UNSET_CONFLUENCE_PASSWORD_DRYRUN`
  failed before dry-run success with `UNSET_CONFLUENCE_PASSWORD_DRYRUN is
  required when -ConfluenceUsername is set`.
- README remediation proof: `rg` found no remaining README commands that print
  `BITBUCKET_BEARER_TOKEN`, `JIRA_PASSWORD`, or `CONFLUENCE_PASSWORD` values
  directly.
- Installer remediation repository proof:
  `GOCACHE=F:\CodeSource\atlassian-mcp\.tmp\go-build go test ./...`,
  `GOCACHE=F:\CodeSource\atlassian-mcp\.tmp\go-build go vet ./...`,
  `GOCACHE=F:\CodeSource\atlassian-mcp\.tmp\go-build go build -o
  .tmp\atlassian-mcp.exe ./cmd/atlassian-mcp`, `.tmp\atlassian-mcp.exe
  --version`, and `git diff --check` passed. The build again emitted the
  non-fatal Go stat-cache warning for `C:\Users\leope\go\pkg\mod\cache\...:
  Access is denied`.

Completion evidence:

- Independent review accepted after remediation: no critical, high, medium, or
  low actionable findings remain. The shared-session conditional commit and
  both automatic-auth ordering tests passed under the race detector.
- 2026-08-09: User confirmed real-environment testing and approved the
  implemented plan. No remaining execution gate is open.
