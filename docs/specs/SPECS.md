## Canonical implementation repository

The implementation repository for this plan is:

```text
https://github.com/chiendao1808/atlassian-mcp.git
```

Repository web URL:

```text
https://github.com/chiendao1808/atlassian-mcp
```

Installer paths in that repository:

```text
scripts/install-from-remote.sh
scripts/install-from-remote.ps1
```

Canonical raw installer URL patterns:

```text
https://raw.githubusercontent.com/chiendao1808/atlassian-mcp/<ref>/scripts/install-from-remote.sh
https://raw.githubusercontent.com/chiendao1808/atlassian-mcp/<ref>/scripts/install-from-remote.ps1
```

Rules:

- Use `https://github.com/chiendao1808/atlassian-mcp.git` as the default source repository in installer examples.
- `<ref>` must resolve to a branch, release tag, or commit SHA containing both installer files.
- Production examples should use a release tag or full commit SHA rather than `main`.
- This URL identifies the repository containing the MCP source; it is not the Bitbucket Server endpoint.
- Bitbucket Server settings continue to use explicit names such as `BITBUCKET_BASE_URL`, `BITBUCKET_PROJECT_KEY`, and `BITBUCKET_BEARER_TOKEN`.

---

# Atlassian MCP Server Implementation Plan

> **For agentic workers:** Implement task-by-task with test-first development and an independent review gate after every task. This document is a planning artifact only; it intentionally contains no backward-compatibility work for names that were never released.

**Goal:** Build one Go MCP stdio binary named `atlassian-mcp` that exposes independent Bitbucket Server 5.10.2 and Jira Server 6.4.14 modules, including 26 Bitbucket repository/commit/pull-request tools and Jira issue authentication, read, comment, field update, and transition tools.

**Architecture:** A shared MCP application hosts a module registry and common transport, TLS, HTTP, error, logging, and result infrastructure. `internal/bitbucket` and `internal/jira` are isolated business modules that register tools only when their static configuration is valid. Either module can run alone, both can run together, and failure in one module must not block the other.

**Tech stack:** Go; official MCP Go SDK; MCP `stdio`; Jira REST API `/rest/api/2`; Bitbucket Server Core REST API `/rest/api/1.0`; Bash and PowerShell installers; Claude Code and Codex MCP configuration.

---

## 1. Planning status and authoritative naming

This is the pre-implementation plan. Only the final names below are valid. There is no alias, compatibility shim, migration task, deprecation warning, or support for earlier draft names.

### 1.1 Final product names

| Item | Final name |
|---|---|
| Go command | `cmd/atlassian-mcp` |
| Binary | `atlassian-mcp` / `atlassian-mcp.exe` |
| MCP server registration name | `atlassian` |
| Unix wrapper | `atlassian-mcp-run` |
| Windows wrapper | `atlassian-mcp-run.ps1` |
| Bash installer | `scripts/install-from-remote.sh` at repository root |
| PowerShell installer | `scripts/install-from-remote.ps1` at repository root |
| Bash installer test | `tests/install-from-remote_test.sh` |
| PowerShell installer test | `tests/install-from-remote.Tests.ps1` |

### 1.2 No compatibility surface

Because implementation has not started, the first release supports only the final names in this document. Do not add aliases, compatibility wrappers, migration flags, deprecated parameter spellings, or alternate binary/script names.

### 1.3 Source repository terminology

The repository containing the `atlassian-mcp` source can be hosted on GitHub, GitLab, Bitbucket, or another Git remote. Installer parameters must therefore use provider-neutral names.

| Purpose | Bash | PowerShell |
|---|---|---|
| Source Git remote URL | `--source-repo-url` | `-SourceRepoUrl` |
| Branch, tag, or commit | `--source-ref` | `-SourceRef` |
| Clone depth | `--source-clone-depth` | `-SourceCloneDepth` |
| Keep cloned source | `--keep-source` | `-KeepSource` |

`source repository` always means the Git remote that contains MCP source code. `Bitbucket base URL`, `Bitbucket project key`, and `repositorySlug` always mean resources managed by the Bitbucket module.

---

## 2. Scope

### 2.1 In scope

- One `atlassian-mcp` stdio process.
- Independent Jira and Bitbucket modules.
- The 26 Bitbucket repository, branch, file, commit, compare, diff, pull-request, review, and lifecycle tools enumerated in Section 10.
- Jira session authentication with Basic Auth credentials supplied through `jira_authenticate` once per MCP process session.
- Jira issue read, comment, generic field update, and transition tools using `issueIdOrKey`.
- Optional read-back after Jira mutation.
- Shared TLS verification policy with module-specific CA files.
- Installers for Linux/macOS and Windows that build from any Git remote and configure Claude Code, Codex, both, or neither.
- Jira-only, Bitbucket-only, and combined installation/runtime modes.
- Module-level startup isolation.
- Tool documentation, contract tests, smoke tests, release packaging, checksums, and security documentation.

### 2.2 Out of scope

- Jira Cloud API tokens or Atlassian Account IDs.
- Cookie authentication, OAuth 1.0a, or Trusted Applications for Jira.
- Persistent Jira credential storage.
- A `jira_logout` tool.
- Dynamic removal or addition of Jira tools after authentication.
- Automatic Jira field discovery or normalization.
- Automatic conversion of display names to `customfield_*` identifiers.
- Automatic inference of required transition-screen fields.
- Jira issue creation, deletion, attachment, watcher, worklog, link, or search tools in this phase.
- Legacy installer/binary aliases.
- Atomic multi-file Bitbucket commits through the Core REST API.

---

## 3. Runtime modes and module isolation

### 3.1 Supported modes

| Valid static configuration | Runtime mode |
|---|---|
| Only Bitbucket configuration valid | Bitbucket-only |
| Only Jira configuration valid | Jira-only |
| Both valid | Jira and Bitbucket |
| One valid and one invalid | Start valid module; warn and disable invalid module |
| Neither configured | Start MCP infrastructure with no business tools and a sanitized warning |

### 3.2 Module startup rules

1. The application parses shared settings first.
2. Each module validates only its own static configuration.
3. A module configuration error must not terminate another module.
4. Startup must not make a network request to Jira or Bitbucket solely to decide whether tools are registered.
5. Jira tools are registered when `JIRA_BASE_URL` and Jira TLS/CA configuration are statically valid.
6. Jira network reachability and credentials are checked only by `jira_authenticate`.
7. Bitbucket startup behavior remains consistent with the existing Bitbucket plan, except that a Bitbucket configuration failure is isolated to that module.
8. Warnings go to `stderr`; `stdout` remains valid MCP protocol traffic only.

### 3.3 Module registry contract

The shared application owns a registry with these conceptual operations:

- `ValidateStaticConfig`
- `RegisterTools`
- `Name`
- `Enabled`
- `DisabledReason`

The exact Go interface is chosen during implementation, but the behaviors above are mandatory. A module cannot register a partial tool set after static validation fails.

---

## 4. Final environment-variable contract

### 4.1 Shared variables

| Variable | Required | Default | Meaning |
|---|---:|---|---|
| `ATLASSIAN_TLS_VERIFY` | No | `false` | Shared TLS certificate and hostname verification flag for both modules |
| `ATLASSIAN_LOG_LEVEL` | No | `info` | Structured logging level written to `stderr` |
| `ATLASSIAN_CONNECT_TIMEOUT` | No | `5s` | TCP/TLS connection timeout |
| `ATLASSIAN_REQUEST_TIMEOUT` | No | `60s` | Overall HTTP request timeout |
| `ATLASSIAN_MAX_RESPONSE_BYTES` | No | implementation limit | Maximum body retained in memory before truncation/error handling |

`ATLASSIAN_TLS_VERIFY` is the only TLS verification switch. No module-specific verification flags exist.

### 4.2 Jira variables

| Variable | Required to enable Jira | Default | Meaning |
|---|---:|---|---|
| `JIRA_BASE_URL` | Yes | none | Jira Server base URL including context path such as `/jira` |
| `JIRA_CA_FILE` | No | system trust store | Jira-specific PEM CA file when verification is enabled |

The following Jira credential variables must not exist in the implementation or installer contract:

- `JIRA_USERNAME`
- `JIRA_USER`
- `JIRA_PASSWORD`
- `JIRA_TOKEN`

### 4.3 Bitbucket variables

| Variable | Required to enable Bitbucket | Meaning |
|---|---:|---|
| `BITBUCKET_BASE_URL` | Yes | Bitbucket Server base URL including context path such as `/bitbucket` |
| `BITBUCKET_PROJECT_KEY` | Yes | Project key fixed for the process |
| `BITBUCKET_BEARER_TOKEN` | Yes | Bearer PAT read from process environment or wrapper-provided environment |
| `BITBUCKET_USER_SLUG` | Required for current-user review tools | User slug whose PR participant state is changed |
| `BITBUCKET_CA_FILE` | No | Bitbucket-specific PEM CA file when verification is enabled |

`repositorySlug` remains mandatory in every Bitbucket business toolcall; it is never promoted to process configuration.

### 4.4 TLS behavior

| Configuration | Required behavior |
|---|---|
| `ATLASSIAN_TLS_VERIFY=false` and HTTPS | Encrypt traffic but skip certificate and hostname verification; emit one sanitized warning per enabled module |
| `ATLASSIAN_TLS_VERIFY=true`, module CA set | Load that module's CA in addition to or as defined against system trust policy |
| `ATLASSIAN_TLS_VERIFY=true`, no module CA | Use system trust store |
| CA path set while verification is false | Ignore the CA and emit a sanitized warning |
| Module CA unreadable while verification is true | Disable only that module |
| HTTP base URL | Permit for internal use but emit a clear transport-security warning |

No credential, Authorization header, Basic Auth encoding, token, password, issue content, diff content, or file content may appear in the warning.

---

## 5. Repository structure

```text
atlassian-mcp/
├── cmd/
│   └── atlassian-mcp/
│       └── main.go
├── internal/
│   ├── app/
│   │   ├── app.go
│   │   ├── module.go
│   │   ├── module_registry.go
│   │   └── module_registry_test.go
│   ├── config/
│   │   ├── shared.go
│   │   └── shared_test.go
│   ├── transport/
│   │   ├── http_client.go
│   │   ├── tls.go
│   │   └── tls_test.go
│   ├── result/
│   │   ├── success.go
│   │   ├── error.go
│   │   └── error_test.go
│   ├── observability/
│   │   ├── logger.go
│   │   ├── redact.go
│   │   └── redact_test.go
│   ├── bitbucket/
│   │   ├── module.go
│   │   ├── config.go
│   │   ├── config_test.go
│   │   ├── client/
│   │   │   ├── client.go
│   │   │   ├── request.go
│   │   │   ├── pagination.go
│   │   │   ├── errors.go
│   │   │   ├── multipart.go
│   │   │   └── client_test.go
│   │   ├── model/
│   │   │   ├── repository.go
│   │   │   ├── ref.go
│   │   │   ├── commit.go
│   │   │   ├── diff.go
│   │   │   ├── pull_request.go
│   │   │   ├── comment.go
│   │   │   └── error.go
│   │   └── tools/
│   │       ├── register.go
│   │       ├── registry_test.go
│   │       ├── repository.go
│   │       ├── repository_test.go
│   │       ├── branches.go
│   │       ├── branches_test.go
│   │       ├── files.go
│   │       ├── files_test.go
│   │       ├── commits.go
│   │       ├── commits_test.go
│   │       ├── commit_file.go
│   │       ├── commit_file_test.go
│   │       ├── pull_requests_read.go
│   │       ├── pull_requests_read_test.go
│   │       ├── pull_requests_write.go
│   │       ├── pull_requests_write_test.go
│   │       ├── comments.go
│   │       ├── comments_test.go
│   │       ├── reviews.go
│   │       ├── reviews_test.go
│   │       ├── transitions.go
│   │       └── transitions_test.go
│   └── jira/
│       ├── module.go
│       ├── config.go
│       ├── config_test.go
│       ├── client/
│       │   ├── client.go
│       │   ├── request.go
│       │   ├── response.go
│       │   └── client_test.go
│       ├── auth/
│       │   ├── credential.go
│       │   ├── session_store.go
│       │   └── session_store_test.go
│       ├── model/
│       │   ├── issue.go
│       │   ├── comment.go
│       │   ├── transition.go
│       │   └── error.go
│       └── tools/
│           ├── register.go
│           ├── authenticate.go
│           ├── authenticate_test.go
│           ├── get_issue.go
│           ├── get_issue_test.go
│           ├── add_comment.go
│           ├── add_comment_test.go
│           ├── update_issue.go
│           ├── update_issue_test.go
│           ├── transition_issue.go
│           └── transition_issue_test.go
├── scripts/
│   ├── install-from-remote.sh
│   ├── install-from-remote.ps1
│   ├── verify-mcp.sh
│   └── verify-mcp.ps1
├── tests/
│   ├── contract/
│   │   ├── jira_auth_test.go
│   │   ├── jira_issue_test.go
│   │   ├── bitbucket_client_test.go
│   │   ├── bitbucket_repository_branch_test.go
│   │   ├── bitbucket_commit_diff_test.go
│   │   ├── bitbucket_commit_file_test.go
│   │   ├── bitbucket_pull_request_read_test.go
│   │   └── bitbucket_pull_request_mutation_test.go
│   ├── install-from-remote_test.sh
│   └── install-from-remote.Tests.ps1
├── docs/
│   ├── architecture.md
│   ├── bitbucket-tool-test-matrix.md
│   ├── configuration.md
│   ├── tools/
│   │   ├── jira_authenticate.md
│   │   ├── jira_get_issue.md
│   │   ├── jira_add_issue_comment.md
│   │   ├── jira_update_issue_fields.md
│   │   ├── jira_transition_issue.md
│   │   ├── bitbucket_repository_and_branches.md
│   │   ├── bitbucket_files_commits_and_diff.md
│   │   ├── bitbucket_commit_file.md
│   │   ├── bitbucket_pull_request_read.md
│   │   ├── bitbucket_pull_request_create.md
│   │   ├── bitbucket_pull_request_comment.md
│   │   ├── bitbucket_pull_request_review.md
│   │   └── bitbucket_pull_request_transitions.md
│   ├── installation-linux-macos.md
│   ├── installation-windows.md
│   ├── claude-code.md
│   └── codex.md
├── testdata/
│   ├── jira/
│   │   ├── server_info.json
│   │   ├── myself.json
│   │   ├── issue.json
│   │   ├── comment.json
│   │   ├── transitions.json
│   │   └── errors/
│   └── bitbucket/
├── go.mod
├── go.sum
├── README.md
└── LICENSE
```

File paths can be adjusted to existing repository conventions, but module boundaries and responsibilities must remain equivalent.

---

## 6. Shared MCP requirements

### 6.1 Transport

- Use MCP `stdio` only.
- Every valid protocol message is written to `stdout`.
- Logs, diagnostics, warnings, and human-readable startup messages are written to `stderr`.
- No banner is written to `stdout`.
- The process terminates when stdin closes or the parent client ends the session.

### 6.2 Tool schema rules

- Every input schema uses `additionalProperties: false` at the outer object level.
- Required properties are explicit.
- Nullable semantics are not used as a substitute for omitted optional fields.
- Generic Jira `fields` and `update` objects allow arbitrary JSON object members because Jira owns their field schema.
- Tool descriptions state authentication prerequisites and mutation effects.
- Read tools set `readOnlyHint=true`.
- Mutation tools set `readOnlyHint=false`, `destructiveHint` according to actual behavior, and appropriate idempotency annotations.
- Tools interacting with Jira or Bitbucket set `openWorldHint=true`.

### 6.3 Stable result envelope

All tools return a shared structured envelope with:

- `success`
- `service`: `jira` or `bitbucket`
- `tool`
- `data` on success
- `error` on failure
- optional `meta` containing sanitized HTTP status, request duration, pagination, truncation, and refresh information

Tool-specific payloads remain available under `data` without flattening or renaming Jira/Bitbucket fields unnecessarily.

### 6.4 Error categories

At minimum:

- `CONFIG_INVALID`
- `MODULE_DISABLED`
- `VALIDATION_ERROR`
- `UPSTREAM_UNREACHABLE`
- `UPSTREAM_TIMEOUT`
- `UPSTREAM_AUTHENTICATION_FAILED`
- `UPSTREAM_PERMISSION_DENIED`
- `UPSTREAM_NOT_FOUND`
- `UPSTREAM_CONFLICT`
- `UPSTREAM_RATE_LIMITED`
- `UPSTREAM_SERVER_ERROR`
- `RESPONSE_TOO_LARGE`
- `JIRA_NOT_AUTHENTICATED`
- `JIRA_AUTHENTICATION_FAILED`
- `JIRA_TRANSITION_NOT_FOUND`
- `JIRA_TRANSITION_AMBIGUOUS`
- `JIRA_REFRESH_FAILED`
- `BITBUCKET_REPOSITORY_EMPTY`
- `BITBUCKET_DIFF_TRUNCATED`
- `BITBUCKET_VERSION_CONFLICT`
- `BITBUCKET_MERGE_VETOED`
- `BITBUCKET_REVIEW_IDENTITY_REQUIRED`
- `BITBUCKET_COMMIT_FILE_CONFLICT`

Jira's `errorMessages` and `errors` maps are preserved in a sanitized upstream detail field.

---

## 7. Jira authentication design

### 7.1 Tool: `jira_authenticate`

Input schema:

| Property | Type | Required | Validation |
|---|---|---:|---|
| `username` | string | Yes | Non-empty after trim; preserve exact value for Basic Auth |
| `password` | string | Yes | Non-empty; never trim or normalize password |

The password property must be documented as sensitive. The MCP server cannot guarantee that a coding-agent client will not retain tool-call history; documentation must warn users to review client logging/history policy.

### 7.2 Authentication flow

1. Copy candidate username/password into an isolated candidate credential object.
2. Call `GET {JIRA_BASE_URL}/rest/api/2/serverInfo` using the candidate Basic Auth credential.
3. Validate that the response is Jira Server-compatible and retain sanitized version metadata.
4. Call `GET {JIRA_BASE_URL}/rest/api/2/myself` using the same candidate credential.
5. Confirm successful HTTP status and parse the authenticated user.
6. Atomically replace the active credential only after both calls succeed.
7. Return sanitized server and user metadata; never return password or Authorization data.
8. Immediately release references to a failed candidate credential.

### 7.3 Atomic replacement behavior

- The active credential store is process-memory only.
- The store supports concurrent reads by Jira tools and atomic replacement by authentication.
- Re-authentication failure leaves the previous active credential unchanged.
- Re-authentication success replaces both username and password as one unit.
- No credential is written to disk, environment, crash dump output, logs, metrics, traces, or MCP results by application code.
- Credential lifetime ends when the MCP process exits.
- There is no logout tool in this phase.

### 7.4 Authentication gating

When Jira static configuration is valid, all five Jira tools are registered from startup. Until authentication succeeds, every Jira business tool except `jira_authenticate` returns `JIRA_NOT_AUTHENTICATED` without sending a request to Jira.

---

## 8. Jira tool contracts

### 8.1 `jira_get_issue`

Purpose: Read one issue by numeric ID or issue key.

Endpoint:

```text
GET /rest/api/2/issue/{issueIdOrKey}
```

Input:

| Property | Type | Required | Behavior |
|---|---|---:|---|
| `issueIdOrKey` | string | Yes | URL path segment; numeric ID or key such as `PROJ-123` |
| `fields` | array of string | No | Join and send as Jira `fields` query; omit when absent |
| `expand` | array of string | No | Join and send as Jira `expand` query; omit when absent |

Rules:

- Do not impose a fixed field set.
- Do not automatically add fields or expansions.
- Deduplicate exact duplicate entries while preserving the first occurrence, or reject duplicates consistently; document the selected policy.
- URL-encode the issue ID/key path segment and query values.
- Preserve the Jira response structure under `data.issue`.
- Mark as read-only.

### 8.2 `jira_add_issue_comment`

Purpose: Add a comment to one issue.

Endpoint:

```text
POST /rest/api/2/issue/{issueIdOrKey}/comment
```

Input:

| Property | Type | Required | Behavior |
|---|---|---:|---|
| `issueIdOrKey` | string | Yes | Jira issue numeric ID or key |
| `body` | string | Yes | Non-empty comment body |
| `visibility` | object | No | Omit for normal comments |
| `visibility.type` | enum | Conditional | `role` or `group` |
| `visibility.value` | string | Conditional | Non-empty role/group name |

Rules:

- When `visibility` exists, both child properties are required.
- The server does not verify that the role or group exists before posting.
- Jira performs permission and existence validation.
- Return the created Jira comment from the HTTP `201` response.
- Mark as a mutation requiring coding-agent approval.
- Do not retry automatically after an ambiguous network failure following POST unless a future idempotency/deduplication design is approved.

### 8.3 `jira_update_issue_fields`

Purpose: Update Jira issue fields using Jira's native `fields` and `update` payload model.

Endpoint:

```text
PUT /rest/api/2/issue/{issueIdOrKey}
```

Input:

| Property | Type | Required | Behavior |
|---|---|---:|---|
| `issueIdOrKey` | string | Yes | Jira issue numeric ID or key |
| `fields` | JSON object | Conditional | Native Jira `fields` object |
| `update` | JSON object | Conditional | Native Jira `update` operations object |
| `returnIssue` | boolean | No | Default `false` |
| `returnFields` | array of string | No | Used only for refresh when `returnIssue=true` |
| `returnExpand` | array of string | No | Used only for refresh when `returnIssue=true` |

Validation:

- Require at least one non-empty object among `fields` and `update`.
- Do not define an allowlist of Jira fields.
- Do not translate field names.
- Do not change values, operation ordering, or custom-field object shapes.
- Reject `returnFields` or `returnExpand` when `returnIssue=false`, or explicitly ignore them with a warning; the implementation plan selects rejection to prevent silent mistakes.
- Do not expose unsupported `notifyUsers` in this phase.

Success behavior:

- With `returnIssue=false`, return mutation status without an extra GET.
- With `returnIssue=true`, perform a follow-up `jira_get_issue`-equivalent GET using `returnFields` and `returnExpand`.
- If mutation succeeds and refresh fails, return `success=true`, `mutationApplied=true`, no issue, and a separate `refreshError` with code `JIRA_REFRESH_FAILED`.
- Never resend the PUT because the refresh failed.

### 8.4 `jira_transition_issue`

Purpose: Move an issue through one available Jira workflow transition.

Endpoints:

```text
GET  /rest/api/2/issue/{issueIdOrKey}/transitions
POST /rest/api/2/issue/{issueIdOrKey}/transitions
```

Input:

| Property | Type | Required | Behavior |
|---|---|---:|---|
| `issueIdOrKey` | string | Yes | Jira issue numeric ID or key |
| `transitionId` | string | Exclusive | Direct transition ID |
| `transitionName` | string | Exclusive | Exact transition name resolution |
| `fields` | JSON object | No | Native Jira transition `fields` object |
| `update` | JSON object | No | Native Jira transition `update` object |
| `returnIssue` | boolean | No | Default `false` |
| `returnFields` | array of string | No | Refresh query only |
| `returnExpand` | array of string | No | Refresh query only |

Validation:

- Require exactly one of `transitionId` or `transitionName`.
- `transitionId` and `transitionName` must be non-empty strings.
- `fields` and `update` pass through unchanged.
- Reject refresh query properties when `returnIssue=false`.

Name resolution:

1. Call the transitions GET endpoint for the issue.
2. Compare available transition names with exact string equality.
3. One match: use its ID.
4. No matches: return `JIRA_TRANSITION_NOT_FOUND` and include sanitized available names if response size policy allows.
5. Multiple exact matches: return `JIRA_TRANSITION_AMBIGUOUS`; never select one automatically.

Transition execution:

- POST `transition.id` plus optional `fields` and `update`.
- Do not infer or auto-populate required transition-screen fields.
- Do not retry an ambiguous POST automatically.
- Implement `returnIssue` and `refreshError` behavior identically to the field update tool.
- Mark as mutation requiring approval.

---

## 9. Jira client and REST behavior

### 9.1 URL construction

- Normalize `JIRA_BASE_URL` once, preserving a context path such as `/jira`.
- Reject query strings and fragments in the configured base URL.
- Avoid double slashes between base path and `/rest/api/2`.
- Encode path segments without encoding the `/rest/api/2` structure.
- Use Go URL builders for query parameters; never concatenate raw query strings from tool input.

### 9.2 Headers

- GET: `Accept: application/json`.
- PUT/POST: `Accept: application/json` and `Content-Type: application/json`.
- Basic Auth set by the HTTP library from the active in-memory credential.
- Optional generated correlation ID only if documented; never accept an arbitrary header map from tool input.

### 9.3 Error parsing

Parse Jira's standard shape:

- `errorMessages`: request-level messages.
- `errors`: field-level message map.

Do not assume every error response is JSON. For HTML proxy errors or malformed responses, retain a bounded sanitized summary and HTTP status.

### 9.4 Retry policy

| Situation | Read request | Mutation request |
|---|---|---|
| Connect timeout/reset before response | Limited retry with exponential backoff and jitter | No automatic retry unless it is provably before request write; conservative default is no retry |
| HTTP 502/503/504 | Limited retry | No blind retry |
| HTTP 429 | Honor `Retry-After` when present | Do not blind retry mutation |
| HTTP 400/401/403/404 | No retry | No retry |
| HTTP 409 | Re-read state only when tool semantics define it | No automatic replay |
| Refresh after successful mutation fails | Retry only the GET within read policy | Never resend mutation |

### 9.5 Response limits

- Bound response body reads.
- Do not silently return invalid truncated JSON.
- For oversized responses, return `RESPONSE_TOO_LARGE` and recommend narrower `fields`/`expand` where applicable.
- Tool documentation must explain that `changelog` or rendered fields can be large.

---

## 10. Bitbucket module contracts and implementation coverage

The endpoint-level authority for the Bitbucket module is [`bitbucket-tool-implementation-guide.md`](bitbucket-tool-implementation-guide.md). Implementation agents must use its per-tool source anchor, method/path, query/body, success, response, permission, error, retry, truncation, annotation, and test requirements. A shorter task description in this plan does not override that guide.

### 10.1 Source hierarchy and stop condition

1. Official Bitbucket Server 5.10.2 REST resource.
2. The bundled reference, including its stable [Bitbucket MCP tool anchor index](../references/jira-6.4.14_bitbucket-5.10.2-rest-api-reference.md#20-bitbucket-mcp-tool-anchor-index).
3. MCP restrictions and safety decisions in the implementation guide.
4. Sanitized real-host contract evidence when the published reference is incomplete.

Do not infer undocumented fields, enum values, status bodies, permissions, retryability, pagination, or identity conversion. Stop and raise a specification question when the source chain cannot determine the behavior.

### 10.2 Common Bitbucket rules

- Core REST paths are pinned to `/rest/api/1.0`; base URL context paths are preserved.
- `BITBUCKET_PROJECT_KEY` is fixed process configuration and every business tool requires `repositorySlug`.
- Inputs use `additionalProperties:false`; callers cannot supply absolute upstream URLs, arbitrary headers, project keys, or arbitrary participant identities.
- Bearer auth uses `BITBUCKET_BEARER_TOKEN` and secrets/content are redacted from logs.
- Genuine paged APIs preserve and follow `nextPageStart`; no cursor arithmetic or implicit fetch-all.
- One-page and hard-capped changes/diff resources do not fabricate later pages.
- Upstream hard caps/truncation and MCP response limits are represented separately.
- Read retry follows the shared bounded policy. Branch creation, file commit, PR creation/comment/status/transition are sent at most once.
- Error handling preserves sanitized upstream `errors[]` and endpoint context; notably, PR transition `409` is not automatically “stale version”.

### 10.3 Exact 26-tool registry and API links

- [`bitbucket_get_repository`](bitbucket-tool-implementation-guide.md#tool-bitbucket_get_repository) — `GET /projects/{projectKey}/repos/{repositorySlug}`; 200 JSON repository.; REPO_READ on the repository.
- [`bitbucket_list_branches`](bitbucket-tool-implementation-guide.md#tool-bitbucket_list_branches) — `GET /projects/{projectKey}/repos/{repositorySlug}/branches`; 200 JSON page.; REPO_READ.
- [`bitbucket_get_default_branch`](bitbucket-tool-implementation-guide.md#tool-bitbucket_get_default_branch) — `GET /projects/{projectKey}/repos/{repositorySlug}/branches/default`; 200 JSON branch; 204 empty body when the repository has no default branch.; REPO_READ.
- [`bitbucket_create_branch`](bitbucket-tool-implementation-guide.md#tool-bitbucket_create_branch) — `POST /projects/{projectKey}/repos/{repositorySlug}/branches`; 200 JSON branch.; REPO_WRITE.
- [`bitbucket_get_file`](bitbucket-tool-implementation-guide.md#tool-bitbucket_get_file) — `GET /projects/{projectKey}/repos/{repositorySlug}/raw/{path:.*}`; 200 raw response body; upstream JSON error bodies for failures.; REPO_READ.
- [`bitbucket_list_commits`](bitbucket-tool-implementation-guide.md#tool-bitbucket_list_commits) — `GET /projects/{projectKey}/repos/{repositorySlug}/commits`; 200 JSON page.; REPO_READ.
- [`bitbucket_get_commit`](bitbucket-tool-implementation-guide.md#tool-bitbucket_get_commit) — `GET /projects/{projectKey}/repos/{repositorySlug}/commits/{commitId}`; 200 JSON commit.; REPO_READ.
- [`bitbucket_get_commit_changes`](bitbucket-tool-implementation-guide.md#tool-bitbucket_get_commit_changes) — `GET /projects/{projectKey}/repos/{repositorySlug}/commits/{commitId}/changes`; 200 JSON change page/envelope.; REPO_READ.
- [`bitbucket_get_commit_diff`](bitbucket-tool-implementation-guide.md#tool-bitbucket_get_commit_diff) — `GET /projects/{projectKey}/repos/{repositorySlug}/commits/{commitId}/diff/{path:.*}`; 200 JSON diff object.; REPO_READ.
- [`bitbucket_compare_commits`](bitbucket-tool-implementation-guide.md#tool-bitbucket_compare_commits) — `GET /projects/{projectKey}/repos/{repositorySlug}/compare/commits`; 200 JSON page.; REPO_READ for repositories required by the comparison.
- [`bitbucket_compare_changes`](bitbucket-tool-implementation-guide.md#tool-bitbucket_compare_changes) — `GET /projects/{projectKey}/repos/{repositorySlug}/compare/changes`; 200 JSON change page/envelope.; REPO_READ.
- [`bitbucket_compare_diff`](bitbucket-tool-implementation-guide.md#tool-bitbucket_compare_diff) — `GET /projects/{projectKey}/repos/{repositorySlug}/compare/diff/{path:.*}`; 200 JSON diff.; REPO_READ.
- [`bitbucket_commit_file`](bitbucket-tool-implementation-guide.md#tool-bitbucket_commit_file) — `PUT /projects/{projectKey}/repos/{repositorySlug}/browse/{path:.*}`; 200 JSON commit.; REPO_WRITE.
- [`bitbucket_list_pull_requests`](bitbucket-tool-implementation-guide.md#tool-bitbucket_list_pull_requests) — `GET /projects/{projectKey}/repos/{repositorySlug}/pull-requests`; 200 JSON page.; REPO_READ.
- [`bitbucket_get_pull_request`](bitbucket-tool-implementation-guide.md#tool-bitbucket_get_pull_request) — `GET /projects/{projectKey}/repos/{repositorySlug}/pull-requests/{pullRequestId}`; 200 JSON pull request.; REPO_READ.
- [`bitbucket_get_pull_request_activities`](bitbucket-tool-implementation-guide.md#tool-bitbucket_get_pull_request_activities) — `GET /projects/{projectKey}/repos/{repositorySlug}/pull-requests/{pullRequestId}/activities`; 200 JSON page.; REPO_READ.
- [`bitbucket_get_pull_request_commits`](bitbucket-tool-implementation-guide.md#tool-bitbucket_get_pull_request_commits) — `GET /projects/{projectKey}/repos/{repositorySlug}/pull-requests/{pullRequestId}/commits`; 200 JSON page.; REPO_READ.
- [`bitbucket_get_pull_request_changes`](bitbucket-tool-implementation-guide.md#tool-bitbucket_get_pull_request_changes) — `GET /projects/{projectKey}/repos/{repositorySlug}/pull-requests/{pullRequestId}/changes`; 200 JSON one-page change envelope.; REPO_READ.
- [`bitbucket_get_pull_request_diff`](bitbucket-tool-implementation-guide.md#tool-bitbucket_get_pull_request_diff) — `GET /projects/{projectKey}/repos/{repositorySlug}/pull-requests/{pullRequestId}/diff/{path:.*}`; 200 JSON diff.; REPO_READ.
- [`bitbucket_check_pull_request_mergeability`](bitbucket-tool-implementation-guide.md#tool-bitbucket_check_pull_request_mergeability) — `GET /projects/{projectKey}/repos/{repositorySlug}/pull-requests/{pullRequestId}/merge`; 200 JSON mergeability object; upstream may return conflict/state errors for invalid PR states.; REPO_READ.
- [`bitbucket_create_pull_request`](bitbucket-tool-implementation-guide.md#tool-bitbucket_create_pull_request) — `POST /projects/{projectKey}/repos/{repositorySlug}/pull-requests`; 201 JSON pull request.; REPO_READ on both source and target repositories according to the 5.10.2 endpoint.
- [`bitbucket_add_pull_request_comment`](bitbucket-tool-implementation-guide.md#tool-bitbucket_add_pull_request_comment) — `POST /projects/{projectKey}/repos/{repositorySlug}/pull-requests/{pullRequestId}/comments`; 201 JSON comment.; REPO_READ; endpoint may also add the caller as watcher/participant behavior described upstream.
- [`bitbucket_set_pull_request_review_status`](bitbucket-tool-implementation-guide.md#tool-bitbucket_set_pull_request_review_status) — `PUT /projects/{projectKey}/repos/{repositorySlug}/pull-requests/{pullRequestId}/participants/{userSlug}`; 201 JSON participant.; REPO_READ.
- [`bitbucket_merge_pull_request`](bitbucket-tool-implementation-guide.md#tool-bitbucket_merge_pull_request) — `POST /projects/{projectKey}/repos/{repositorySlug}/pull-requests/{pullRequestId}/merge`; 200 JSON merged pull request.; REPO_WRITE.
- [`bitbucket_decline_pull_request`](bitbucket-tool-implementation-guide.md#tool-bitbucket_decline_pull_request) — `POST /projects/{projectKey}/repos/{repositorySlug}/pull-requests/{pullRequestId}/decline`; 200 response; preserve a JSON PR if supplied by the host, otherwise a successful empty result plus resolved version.; REPO_READ per the 5.10.2 endpoint documentation.
- [`bitbucket_reopen_pull_request`](bitbucket-tool-implementation-guide.md#tool-bitbucket_reopen_pull_request) — `POST /projects/{projectKey}/repos/{repositorySlug}/pull-requests/{pullRequestId}/reopen`; 200 JSON reopened pull request.; REPO_READ.

### 10.4 High-risk contract decisions

- Branch `orderBy`: `ALPHABETICAL|MODIFICATION`; default branch `204` is empty repository success.
- Commit list includes `followRenames`, `ignoreMissing`, `merges`, `path`, `since`, `until`, `withCounts`.
- Commit changes/diff implement documented hard caps and full query sets; no unsupported continuation is implied.
- Compare `fromRepo` is derived only from `fromRepositorySlug` under `BITBUCKET_PROJECT_KEY`.
- File commit is multipart PUT, one file, one request, with `sourceCommitId` stale-write safety.
- PR participant filters are continuous indexed query names and capped at 10.
- PR activities require `fromType` when `fromId` is supplied.
- PR comments validate general/reply/file/line payloads separately.
- Review status cannot ship until the host confirms configured slug versus body `user.name` compatibility.
- Merge/decline/reopen preserve optimistic-lock version and distinguish conflict, veto, stale, and invalid state.

### 10.5 Documentation and test gate

The registry is complete only when every tool has:

- a resolving bundled-source anchor and exact official resource heading;
- request serialization and response-preservation contract tests;
- permission/error/retry/truncation tests;
- MCP schema and annotation snapshots;
- a sanitized real-host fixture wherever the implementation guide declares a staging gate.

## 11. Installer interface: provider-neutral source repository

### 11.1 Bash installer final interface

```text
scripts/install-from-remote.sh

--source-repo-url URL                required unless --binary is used
--source-ref REF                     default: main
--source-clone-depth N               optional; default implementation value
--keep-source                        optional
--binary FILE_PATH                   install prebuilt binary instead of cloning/building
--install-dir DIRECTORY              default: $HOME/.local/bin
--agents claude|codex|both|none      interactive when omitted
--scope local|project|user           agent configuration scope
--project-dir DIRECTORY              required for project scope when not current directory
--enable-jira                        optional
--jira-base-url URL                  required with --enable-jira
--jira-ca-file FILE_PATH             optional
--enable-bitbucket                   optional
--bitbucket-base-url URL             required with --enable-bitbucket
--bitbucket-project-key KEY          required with --enable-bitbucket
--bitbucket-user-slug SLUG           required when review tools are enabled
--bitbucket-token-env VARIABLE_NAME  default: BITBUCKET_BEARER_TOKEN
--bitbucket-ca-file FILE_PATH         optional
--atlassian-tls-verify true|false    default: false
--skip-tests                         optional
--dry-run                            optional
--replace                            optional
--non-interactive                    optional
```

### 11.2 PowerShell installer final interface

```text
scripts/install-from-remote.ps1

-SourceRepoUrl URL
-SourceRef REF
-SourceCloneDepth N
-KeepSource
-Binary FILE_PATH
-InstallDir DIRECTORY
-Agents Claude|Codex|Both|None
-Scope Local|Project|User
-ProjectDir DIRECTORY
-EnableJira
-JiraBaseUrl URL
-JiraCaFile FILE_PATH
-EnableBitbucket
-BitbucketBaseUrl URL
-BitbucketProjectKey KEY
-BitbucketUserSlug SLUG
-BitbucketTokenEnv VARIABLE_NAME
-BitbucketCaFile FILE_PATH
-AtlassianTlsVerify true|false
-SkipTests
-DryRun
-Replace
-NonInteractive
```

### 11.3 Installer validation

- At least one module must be selected for a normal install; `none` business modules may be allowed only with an explicit diagnostic flag if later required, not in MVP.
- `--enable-jira` requires `--jira-base-url`.
- `--enable-bitbucket` requires base URL, project key, and an available token environment variable.
- Jira username/password must never be prompted, accepted, stored, or exported by the installer.
- A source URL containing embedded HTTPS credentials is rejected.
- SSH and credential-helper based private clones are allowed.
- `--source-ref` may be branch, tag, or commit; installer must fetch/checkout safely and verify resulting worktree state.
- Clone/build temporary directories are removed unless `--keep-source` is set.
- No old parameter aliases exist.

### 11.4 Generated runtime configuration

The installer creates a non-secret module configuration file or wrapper environment containing:

- `ATLASSIAN_TLS_VERIFY`
- `JIRA_BASE_URL` when Jira enabled
- `JIRA_CA_FILE` when supplied
- `BITBUCKET_BASE_URL` when Bitbucket enabled
- `BITBUCKET_PROJECT_KEY` when Bitbucket enabled
- `BITBUCKET_USER_SLUG` when supplied
- `BITBUCKET_CA_FILE` when supplied

Bitbucket token handling:

- The installer records only the configured token environment-variable name.
- It does not place the token value in Claude/Codex configuration.
- The wrapper forwards the token from the launching process environment.
- Non-interactive installation fails if Bitbucket is enabled and the named token variable is unavailable, unless a documented `configure-only` mode is later approved.

Jira credential handling:

- No credential variables are generated.
- The user calls `jira_authenticate` each new MCP process session.

---

## 12. Coding-agent configuration

### 12.1 Claude Code

Register the wrapper or binary as a stdio server named `atlassian`.

- User/local scope can be created with `claude mcp add`.
- Project scope uses `.mcp.json` with variable references or a resolved wrapper path.
- Project configuration contains no Jira password and no Bitbucket token value.
- Installer verifies `claude mcp get atlassian` when CLI is available.

### 12.2 Codex

Create one `[mcp_servers.atlassian]` entry.

- Configure command, args, startup timeout, tool timeout, enabled, and required behavior.
- Use `default_tools_approval_mode = "writes"`.
- Add explicit prompt approval for all Bitbucket mutation tools from Section 10.6 plus these Jira tools:
  - `jira_add_issue_comment`
  - `jira_update_issue_fields`
  - `jira_transition_issue`
- `jira_authenticate` is sensitive but not an upstream mutation. It should still require prompt approval if Codex supports per-tool approval for sensitive input; document the exact supported setting confirmed during implementation.
- Project configuration contains no credential value.

### 12.3 Client credential warning

Documentation must state that `jira_authenticate` passes credentials through an MCP toolcall. The server protects credentials from its own persistence/logging, but client-side conversation history, telemetry, diagnostics, or tool-call recording are controlled by the coding-agent product and organizational policy.

---

## 13. Detailed implementation tasks

The 27 tasks below are self-contained. Each task ends with unit/contract tests, documentation updates for its behavior, and a focused commit. Do not combine independent tasks into one large change.

### Task 1 — Freeze specification, naming, and ADRs

**Files:**

- Create `docs/architecture.md`.
- Create ADR for one binary/two modules.
- Create ADR for session-scoped Jira Basic Auth.
- Create ADR for shared TLS verify and separate CA files.
- Update root `README.md` with `atlassian-mcp` naming.

**Steps:**

- Document all final names from Section 1.
- Document that no compatibility aliases are required.
- Record Jira REST version `/rest/api/2` and Bitbucket REST version `/rest/api/1.0`.
- Record independent module startup behavior.
- Record Jira credential lifecycle and atomic replacement.
- Add a repository-wide forbidden-name check for superseded names.

**Tests/verification:**

- Search repository for excluded names and fail CI if found outside historical reference documents.
- Review environment-variable table for exact spelling.
- Confirm tool list contains exactly the agreed Jira tool names.

**Acceptance:** A new engineer can identify every product, script, parameter, variable, module, and tool name without ambiguity.

### Task 2 — Rename command and distribution surface

**Files:**

- Create/move `cmd/atlassian-mcp/main.go`.
- Update build scripts, release configuration, README examples, and version package.
- Remove draft installer names rather than preserving wrappers.

**Steps:**

- Change produced binary to `atlassian-mcp` and Windows `.exe` form.
- Set MCP server metadata name to `atlassian-mcp` and registration label to `atlassian`.
- Update release artifact names and checksum manifest patterns.
- Ensure `--version` exits without starting stdio transport.

**Tests/verification:**

- Build on supported targets.
- Assert artifact names.
- Assert no alternate or compatibility executable is produced.

**Acceptance:** Only final product names are emitted by builds and docs.

### Task 3 — Shared config and module registry

**Files:**

- Create `internal/app/module.go` and `module_registry.go`.
- Create `internal/config/shared.go`.
- Add tests for module combinations and failure isolation.

**Steps:**

- Parse shared timeouts, log level, response limits, and `ATLASSIAN_TLS_VERIFY`.
- Validate boolean strictly and default to false.
- Define module static-validation and tool-registration lifecycle.
- Register valid modules independently.
- Emit sanitized disabled-module warnings to `stderr`.

**Test matrix:**

- Jira only.
- Bitbucket only.
- Both valid.
- Jira invalid, Bitbucket valid.
- Bitbucket invalid, Jira valid.
- Neither configured.
- Shared TLS flag invalid.

**Acceptance:** No module-level configuration error blocks a separately valid module.

### Task 4 — Shared TLS, HTTP, logging, and redaction

**Files:**

- Create shared transport and observability packages.
- Add tests using self-signed TLS servers and fake secrets.

**Steps:**

- Build per-module HTTP clients from shared verify flag plus module CA path.
- Support system trust store and custom PEM CA.
- Implement HTTP/internal HTTPS warning behavior.
- Implement header, URL, and body redaction.
- Enforce stdout/stderr separation.
- Add bounded response-body reading.

**Tests:**

- Self-signed HTTPS succeeds when verify false.
- Self-signed HTTPS fails when verify true without CA.
- Self-signed HTTPS succeeds with correct module CA.
- Invalid Jira CA disables Jira only.
- Invalid Bitbucket CA disables Bitbucket only.
- Secret sentinel values never appear in logs or errors.

**Acceptance:** Shared infrastructure is safe for both authentication models.

### Task 5 — Bitbucket static configuration and REST client foundation

**Files:**

- Create `internal/bitbucket/config.go` and `config_test.go`.
- Create `internal/bitbucket/client/client.go`, `request.go`, `pagination.go`, `errors.go`, and tests.
- Create bounded fixtures under `testdata/bitbucket/errors`.

**Interfaces:**

- Consumes shared TLS, timeout, response-limit, logging, and result packages from Tasks 3–4.
- Produces an immutable Bitbucket client used by Tasks 6–11.

**Steps:**

- [ ] Implement only after reading the applicable per-tool contracts and source anchors in `docs/bitbucket-tool-implementation-guide.md`.
- [ ] Add or update the bundled-reference link in every generated tool document; do not replace a documented contract with inference.

- [ ] Parse `BITBUCKET_BASE_URL`, `BITBUCKET_PROJECT_KEY`, `BITBUCKET_BEARER_TOKEN`, optional `BITBUCKET_USER_SLUG`, and `BITBUCKET_CA_FILE`.
- [ ] Treat a completely absent Bitbucket configuration as module not requested.
- [ ] Treat partial or invalid Bitbucket configuration as module disabled without blocking Jira.
- [ ] Normalize the base URL while preserving a context path such as `/bitbucket`; reject query strings and fragments.
- [ ] Build endpoint-aware URL helpers for project key, repository slug, commit/ref values, PR IDs, and file paths.
- [ ] Add Bearer Authorization internally; tool input cannot set headers.
- [ ] Implement JSON, raw-body, empty-204, and multipart request paths.
- [ ] Parse Bitbucket `errors[]` responses and non-JSON reverse-proxy errors.
- [ ] Implement bounded reads and context cancellation.
- [ ] Implement read-only retry with exponential backoff/jitter; never retry mutation methods blindly.
- [ ] Implement page types that retain `nextPageStart`.
- [ ] Emit sanitized request method, path template, status, duration, and request ID to `stderr`.

**Tests:**

- [ ] Valid, absent, partial, and invalid static configuration.
- [ ] Bitbucket-only and Jira-only module isolation.
- [ ] Context-path URL construction.
- [ ] Encoding for `~project`, repository slug, refs containing `/`, Unicode paths, spaces, and query values.
- [ ] Bearer header reaches mock upstream but token sentinel is absent from logs/errors/results.
- [ ] `400`, `401`, `403`, `404`, `409`, `415`, `429`, and `5xx` mapping.
- [ ] Malformed JSON, HTML proxy response, empty `204`, oversized body, timeout, and cancellation.
- [ ] GET retry count; POST/PUT/DELETE request count remains one.
- [ ] Pagination follows server-provided `nextPageStart=37` rather than calculating from request values.

**Acceptance:** Bitbucket endpoint handlers contain no duplicated transport/auth/retry/error parsing logic.

### Task 6 — Bitbucket repository and branch tools

**Files:**

- Create `internal/bitbucket/model/repository.go` and `ref.go`.
- Create `internal/bitbucket/tools/repository.go`, `branches.go`, and tests.
- Add repository/branch fixtures and tool docs.

**Interfaces:**

- Consumes the Bitbucket client from Task 5.
- Produces four registered tools: repository read, branch list/default read, and branch create.

**Steps:**

- [ ] Implement only after reading the applicable per-tool contracts and source anchors in `docs/bitbucket-tool-implementation-guide.md`.
- [ ] Add or update the bundled-reference link in every generated tool document; do not replace a documented contract with inference.

- [ ] Implement `bitbucket_get_repository`.
- [ ] Implement `bitbucket_list_branches` with optional filters and upstream cursor pagination.
- [ ] Implement `bitbucket_get_default_branch`, including empty repository `204`.
- [ ] Implement `bitbucket_create_branch` with `name`, `startPoint`, and optional `message`.
- [ ] Preserve branch identifiers and latest commit metadata.
- [ ] Mark only branch creation as a write.
- [ ] Ensure all schemas require `repositorySlug`.
- [ ] Prevent automatic replay of create-branch POST.

**Contract tests:**

- [ ] Exact method/path for all four tools.
- [ ] Repository success, not found, and permission denied.
- [ ] Branch filters, enum validation, paging, and `nextPageStart`.
- [ ] Default branch `200` and empty repository `204`.
- [ ] Create success, duplicate/conflict, invalid start point, and ambiguous network failure with one POST.
- [ ] MCP schema and annotation snapshots.
- [ ] Result contains requested `repositorySlug`.

**Acceptance:** A coding agent can inspect repository/default branch, find branches, and create one without bypassing the fixed project scope.

### Task 7 — Bitbucket file, commit, compare, and diff read tools

**Files:**

- Create `internal/bitbucket/model/commit.go` and `diff.go`.
- Create `internal/bitbucket/tools/files.go`, `commits.go`, and tests.
- Add raw-file, commit, change, diff, and truncation fixtures.

**Interfaces:**

- Consumes Task 5 client and shared result limits.
- Produces eight read-only tools for file and history inspection.

**Steps:**

- [ ] Implement only after reading the applicable per-tool contracts and source anchors in `docs/bitbucket-tool-implementation-guide.md`.
- [ ] Add or update the bundled-reference link in every generated tool document; do not replace a documented contract with inference.

- [ ] Implement `bitbucket_get_file` with text/base64 output and byte-size metadata.
- [ ] Implement `bitbucket_list_commits` and `bitbucket_get_commit`.
- [ ] Implement `bitbucket_get_commit_changes` and `bitbucket_get_commit_diff`.
- [ ] Implement `bitbucket_compare_commits`, `bitbucket_compare_changes`, and `bitbucket_compare_diff`.
- [ ] Validate commit-list merge mode and pagination inputs.
- [ ] Pass `since`, `until`, refs, optional cross-repository slug, paths, whitespace, and context settings exactly.
- [ ] Enforce response/file/diff caps.
- [ ] Surface both upstream truncation and MCP-layer truncation without returning invalid JSON.
- [ ] Never auto-fetch every page by default.
- [ ] Never log raw file bytes, source lines, or diff hunks.

**Contract tests:**

- [ ] Raw UTF-8 text, invalid UTF-8/binary as base64, empty file, and oversized file.
- [ ] Commit-list `since`/`until`, merge enum, path filter, and cursor behavior.
- [ ] Commit success/not-found.
- [ ] Commit changes paging.
- [ ] Whole-commit and path-specific diff.
- [ ] Unicode, spaces, and nested path encoding.
- [ ] Upstream `truncated=true`.
- [ ] MCP size cap with explicit truncation metadata.
- [ ] Cross-repository comparison uses only `fromRepositorySlug`; absolute URL input is impossible.
- [ ] All eight tools are read-only and require `repositorySlug`.

**Acceptance:** Read tools expose sufficient bounded history/diff context while making every omitted page or truncation explicit.

### Task 8 — Bitbucket safe single-file commit tool

**Files:**

- Create `internal/bitbucket/client/multipart.go`.
- Create `internal/bitbucket/tools/commit_file.go` and tests.
- Add multipart request fixtures and conflict responses.
- Create `docs/tools/bitbucket_commit_file.md`.

**Interfaces:**

- Consumes Task 5 client.
- Produces `bitbucket_commit_file`.

**Steps:**

- [ ] Implement only after reading the applicable per-tool contracts and source anchors in `docs/bitbucket-tool-implementation-guide.md`.
- [ ] Add or update the bundled-reference link in every generated tool document; do not replace a documented contract with inference.

- [ ] Validate path, branch, message, and exactly one content input.
- [ ] Decode base64 before applying maximum content size.
- [ ] Distinguish create and update semantics.
- [ ] Require `sourceCommitId` for update mode.
- [ ] Omit `sourceCommitId` for create mode.
- [ ] Require `sourceBranch` when a new branch is requested.
- [ ] Build the exact multipart PUT payload with form field `content`.
- [ ] Send the PUT once.
- [ ] Map stale source commit, existing file, unchanged content, and other `409` details to `BITBUCKET_COMMIT_FILE_CONFLICT`.
- [ ] Return commit metadata with explicit `singleFileCommit=true`.
- [ ] Mark as write/destructive and require client approval.
- [ ] Document that atomic multi-file commit is unsupported.

**Contract tests:**

- [ ] Create a new file.
- [ ] Update an existing file with valid `sourceCommitId`.
- [ ] Stale source commit.
- [ ] Existing file in create mode.
- [ ] Unchanged content.
- [ ] Missing branch/message/content.
- [ ] Both content fields supplied.
- [ ] Invalid base64 and decoded content over limit.
- [ ] New branch with and without `sourceBranch`.
- [ ] Path traversal/NUL rejection.
- [ ] Multipart body contains expected fields but content sentinel never appears in logs.
- [ ] Upstream reset after request write does not cause a second PUT.

**Acceptance:** The tool cannot silently overwrite a changed file when the approved safety contract is followed.

### Task 9 — Bitbucket pull-request read and create tools

**Files:**

- Create `internal/bitbucket/model/pull_request.go`.
- Create `internal/bitbucket/tools/pull_requests_read.go`, `pull_requests_write.go`, and tests.
- Add PR, activity, change, diff, mergeability, and create fixtures.
- Add read/create tool documentation.

**Interfaces:**

- Consumes Task 5 client and Task 7 diff-limit behavior.
- Produces seven PR read tools plus `bitbucket_create_pull_request`.

**Steps:**

- [ ] Implement only after reading the applicable per-tool contracts and source anchors in `docs/bitbucket-tool-implementation-guide.md`.
- [ ] Add or update the bundled-reference link in every generated tool document; do not replace a documented contract with inference.

- [ ] Implement PR list/get, activities, commits, changes, diff, and mergeability tools.
- [ ] Implement create PR.
- [ ] Normalize branch IDs to `refs/heads/...` exactly once.
- [ ] Build source/target repository objects from configured project key and explicit slugs.
- [ ] Preserve PR version/state/refs/author/reviewers/participants.
- [ ] Validate reviewer inputs without adding implicit reviewers.
- [ ] Bound activities/changes/diff responses and surface truncation.
- [ ] Send create POST once.

**Contract tests:**

- [ ] PR list filters and pagination.
- [ ] Get PR includes version.
- [ ] Activities and commits paging.
- [ ] Changes and diff range/path/query handling.
- [ ] Mergeability success, veto, and conflict response.
- [ ] Same-repository create.
- [ ] Cross-repository source slug when supported by the contract.
- [ ] Invalid branches/reviewers.
- [ ] Duplicate/open PR conflict.
- [ ] Permission errors.
- [ ] Ambiguous create response does not trigger a second POST.
- [ ] All eight tools require `repositorySlug`; seven are read-only and create is a write.

**Acceptance:** The agent can gather complete bounded review context and create a PR without hidden refs or reviewers.

### Task 10 — Bitbucket pull-request comments and review status

**Files:**

- Create `internal/bitbucket/model/comment.go`.
- Create `internal/bitbucket/tools/comments.go`, `reviews.go`, and tests.
- Add comment/review fixtures and docs.

**Interfaces:**

- Consumes Task 5 client and configured `BITBUCKET_USER_SLUG`.
- Produces `bitbucket_add_pull_request_comment` and `bitbucket_set_pull_request_review_status`.

**Steps:**

- [ ] Implement only after reading the applicable per-tool contracts and source anchors in `docs/bitbucket-tool-implementation-guide.md`.
- [ ] Add or update the bundled-reference link in every generated tool document; do not replace a documented contract with inference.

- [ ] Implement general comments.
- [ ] Implement replies using `parentId`.
- [ ] Implement inline comments only when required anchor fields are complete.
- [ ] Validate non-empty and bounded comment text.
- [ ] Send comment POST once.
- [ ] Implement participant update using only configured user slug.
- [ ] Map `APPROVED`, `NEEDS_WORK`, and `UNAPPROVED` to exact upstream payloads.
- [ ] Reject missing review identity with `BITBUCKET_REVIEW_IDENTITY_REQUIRED`.
- [ ] Never expose an input property for arbitrary reviewer impersonation.
- [ ] Preserve created comment/activity IDs in results.

**Contract tests:**

- [ ] General, reply, and inline comment payloads.
- [ ] Incomplete inline anchor.
- [ ] Empty/oversized comment.
- [ ] Comment POST ambiguity with one request.
- [ ] Exact participant payload for each status.
- [ ] Missing configured user slug.
- [ ] Current user is PR author/upstream rejection.
- [ ] Permission errors.
- [ ] Comment text and token sentinels absent from logs.
- [ ] Both tools require `repositorySlug` and write approval.

**Acceptance:** The service identity can comment and update only its own participant status, without an impersonation surface.

### Task 11 — Bitbucket safe PR transitions, registry, and approval gate

**Files:**

- Create `internal/bitbucket/tools/transitions.go` and tests.
- Complete `internal/bitbucket/tools/register.go` and `registry_test.go`.
- Add transition/version-conflict fixtures.
- Complete Bitbucket tool documentation and approval configuration.

**Interfaces:**

- Consumes PR read/mergeability methods from Task 9.
- Produces merge, decline, reopen tools and the complete 26-tool Bitbucket registry.

**Steps:**

- [ ] Implement only after reading the applicable per-tool contracts and source anchors in `docs/bitbucket-tool-implementation-guide.md`.
- [ ] Add or update the bundled-reference link in every generated tool document; do not replace a documented contract with inference.

- [ ] Implement shared expected-version resolution.
- [ ] Use caller `expectedVersion` unchanged when supplied.
- [ ] Otherwise GET PR once immediately before transition.
- [ ] Implement merge precheck defaulting to true.
- [ ] Stop before POST on merge veto/conflict.
- [ ] Implement merge, decline, and reopen with at most one POST.
- [ ] Validate compatible current PR state.
- [ ] On `409`, optionally GET current PR once, return current state, and do not replay.
- [ ] Register exactly the 26 names in Section 10.7.
- [ ] Apply read/write/destructive/idempotency/open-world annotations.
- [ ] Add Codex/Claude approval guidance for all Bitbucket mutations.
- [ ] Generate/update all Bitbucket tool docs and the coverage matrix.

**Contract tests:**

- [ ] Merge success, veto, and conflict.
- [ ] Stale caller `expectedVersion`.
- [ ] Auto-fetch version.
- [ ] Decline open PR.
- [ ] Reopen declined PR.
- [ ] Invalid transition from merged/closed state.
- [ ] Network reset after POST with request count one.
- [ ] `409` safe refresh with request count one for POST.
- [ ] Concurrent version-change fixture.
- [ ] Registry exact-name count is 26.
- [ ] Every registered schema requires `repositorySlug`.
- [ ] No unknown outer properties.
- [ ] Annotation snapshots.
- [ ] Documentation file or grouped documentation entry exists for every tool.
- [ ] Token, file, diff, and comment sentinels are absent from all logs/results.

**Acceptance:** Every Bitbucket tool has an explicit implementation owner, schema contract, endpoint-level test, permission/approval classification, and safety regression test.


### Task 12 — Jira static configuration and module registration

**Files:**

- Create `internal/jira/config.go`, `module.go`, and tests.

**Steps:**

- Treat absent `JIRA_BASE_URL` as module not requested.
- Parse and normalize URL including context path.
- Accept only HTTP/HTTPS schemes.
- Reject query and fragment components.
- Validate `JIRA_CA_FILE` only when Jira is requested and verify is true.
- Register all five Jira tools after successful static validation.
- Do not make network requests at startup.

**Acceptance:** Jira tool visibility depends only on valid static Jira configuration, not live network or credentials.

### Task 13 — Jira credential session store

**Files:**

- Create `internal/jira/auth/credential.go` and `session_store.go`.
- Add concurrency and replacement tests.

**Steps:**

- Define immutable active credential values.
- Implement concurrent snapshot reads.
- Implement atomic candidate replacement.
- Ensure failed replacement leaves old credential unchanged.
- Ensure unauthenticated snapshot returns a typed state error.
- Prevent string formatting methods from exposing password.

**Tests:**

- Initial state unauthenticated.
- Successful set and read.
- Concurrent readers during replacement.
- Failed candidate path preserves old credential.
- Redaction tests with sentinel password.

**Acceptance:** Jira tools can safely share one process-session credential without persistence.

### Task 14 — Jira REST client foundation

**Files:**

- Create client request/response/error files and fixtures.

**Steps:**

- Build `/rest/api/2` endpoint paths from normalized base URL.
- Add Basic Auth from credential snapshot per request.
- Implement GET/POST/PUT JSON helpers without accepting arbitrary headers.
- Parse Jira JSON errors and non-JSON proxy errors.
- Apply shared timeouts, retry policy, response limits, and redaction.

**Tests:**

- Context-path URL construction.
- Path and query encoding.
- Basic Auth reaches mock server but is absent from diagnostics.
- Jira error shape mapping.
- HTML proxy error mapping.
- Oversized response handling.

**Acceptance:** Tool handlers do not implement HTTP mechanics directly.

### Task 15 — `jira_authenticate`

**Files:**

- Create tool handler and unit/contract tests.
- Create tool documentation.

**Steps:**

- Validate username and password inputs.
- Use candidate credential for `serverInfo` and `myself`.
- Parse sanitized server version and authenticated user.
- Atomically activate candidate only after both calls succeed.
- Preserve old credential on any candidate failure.
- Return no password or Authorization-derived data.

**Contract tests:**

- Both endpoints succeed.
- `serverInfo` fails.
- `myself` fails after `serverInfo` succeeds.
- Re-authentication succeeds.
- Re-authentication fails and old credential remains usable.
- Client logs and MCP result contain no password sentinel.

**Acceptance:** Authentication exactly matches the approved session model.

### Task 16 — `jira_get_issue`

**Files:**

- Create handler, schema, tests, and docs.

**Steps:**

- Gate on active credential.
- Validate `issueIdOrKey`.
- Convert optional arrays to Jira query values.
- Call issue GET endpoint.
- Return original issue JSON under the shared envelope.
- Mark tool read-only.

**Contract tests:**

- Numeric ID and issue key.
- No optional query.
- Fields only.
- Expand only.
- Both query sets.
- 401, 403, 404, and oversized responses.
- Pre-auth call sends no network request.

**Acceptance:** Tool is a transparent, bounded Jira issue read interface.

### Task 17 — `jira_add_issue_comment`

**Files:**

- Create handler, schema, tests, and docs.

**Steps:**

- Validate non-empty body.
- Validate optional visibility object.
- Restrict visibility type to `role` or `group`.
- Omit visibility entirely for a normal comment.
- POST comment and return the created comment.
- Configure mutation annotation and agent approval guidance.

**Contract tests:**

- Normal comment.
- Role visibility.
- Group visibility.
- Missing visibility child property.
- Unsupported visibility type.
- Jira rejects nonexistent role/group.
- Ambiguous connection failure is not automatically replayed.

**Acceptance:** Common comments are simple; restricted comments follow Jira's native shape.

### Task 18 — `jira_update_issue_fields`

**Files:**

- Create handler, schema, refresh helper tests, and docs.

**Steps:**

- Accept arbitrary JSON objects for `fields` and `update`.
- Require at least one non-empty update object.
- Preserve JSON structure exactly through serialization.
- Send PUT to issue endpoint.
- Implement optional refresh with `returnFields` and `returnExpand`.
- Return partial-success result when refresh fails.
- Reject refresh options when `returnIssue=false`.

**Contract tests:**

- Fields only.
- Update operations only.
- Both together.
- Custom field object preserved.
- Empty mutation rejected locally.
- Jira field validation error preserved.
- Mutation 204 without refresh.
- Mutation succeeds and refresh succeeds.
- Mutation succeeds and refresh fails without replay.

**Acceptance:** No Jira field-specific logic is embedded in the MCP server.

### Task 19 — `jira_transition_issue`

**Files:**

- Create handler, transition resolver, tests, and docs.

**Steps:**

- Enforce exactly one transition selector.
- For ID, post directly.
- For name, list available transitions and resolve exact matches.
- Preserve optional `fields` and `update` objects.
- Implement shared refresh behavior.
- Configure mutation annotations and approvals.

**Contract tests:**

- Transition by ID.
- Transition by exact name.
- No name match.
- Multiple exact name matches.
- Both selectors rejected.
- Neither selector rejected.
- Required transition field rejected by Jira and preserved in error detail.
- Mutation success plus refresh failure.

**Acceptance:** The server never guesses a transition or missing workflow field.

### Task 20 — Jira tool-level security and approval policy

**Files:**

- Update MCP registration, Claude/Codex docs, security docs, and tests.

**Steps:**

- Confirm annotations for each Jira tool.
- Mark `jira_get_issue` read-only.
- Mark comment/update/transition as writes.
- Treat `jira_authenticate` as sensitive input and document client-history risk.
- Verify tool descriptions never invite credentials in any tool other than authentication.
- Add property-based or recursive redaction tests against tool input/result/log data.

**Acceptance:** Sensitive input and write operations have explicit client-visible guidance.

### Installer repository placement contract

The final installer files must be committed in `https://github.com/chiendao1808/atlassian-mcp.git` at these exact repository-root paths:

```text
<repository-root>/
├── scripts/
│   ├── install-from-remote.sh
│   └── install-from-remote.ps1
└── tests/
    ├── install-from-remote_test.sh
    └── install-from-remote.Tests.ps1
```

Rules:

- `/scripts` means the `scripts` directory directly below the Git repository root.
- The installers must be committed in normal branches and release tags so they can be fetched by raw-file URL.
- The installer filenames and paths are stable public bootstrap contracts.
- The `<ref>` segment may be a branch, release tag, or full commit SHA.
- Production documentation should prefer an immutable release tag or commit SHA; `main` is only a development convenience.
- The installer may be downloaded from GitHub while `--source-repo-url` / `-SourceRepoUrl` remains provider-neutral and can point to GitHub, GitLab, Bitbucket, or another Git remote.
- No Jira username/password or Bitbucket token may appear in a raw URL, query string, installer argument, or committed repository file.

Canonical GitHub raw URLs:

```text
https://raw.githubusercontent.com/chiendao1808/atlassian-mcp/<ref>/scripts/install-from-remote.sh
https://raw.githubusercontent.com/chiendao1808/atlassian-mcp/<ref>/scripts/install-from-remote.ps1
```

### Task 21 — Bash installer replacement

**Files:**

- Create `scripts/install-from-remote.sh` directly below the repository root.
- Create `tests/install-from-remote_test.sh`.
- Update the root README with the canonical GitHub raw URL.
- Remove old draft script names from release package.

**Steps:**

- Implement final provider-neutral source parameters.
- Clone/fetch/checkout source or use a prebuilt binary.
- Run Go tests unless skipped.
- Build `cmd/atlassian-mcp`.
- Install binary and wrapper atomically.
- Select Jira, Bitbucket, or both.
- Generate only approved non-secret configuration.
- Configure selected coding agents.
- Backup and rollback agent configuration on failure.
- Support dry-run and non-interactive validation.
- Commit the Bash installer with Git mode `100755`.
- Document `curl -fsSL <raw-url> | bash -s -- ...` without putting credentials in the URL.

**Tests:**

- GitHub-style HTTPS remote.
- GitLab-style HTTPS remote.
- Bitbucket Server-style HTTPS remote.
- SSH remote passed to mocked Git.
- Embedded credential URL rejection.
- Jira-only, Bitbucket-only, both.
- Missing required module arguments.
- Final names only.
- Token value absent from agent configuration.
- Jira credential variables absent everywhere.
- Re-running does not duplicate config.
- Installer exists only at `scripts/install-from-remote.sh`.
- Documentation fixtures resolve the GitHub raw URL to the repository-root `/scripts` path.

**Acceptance:** One Bash installer at the stable repository-root path works without provider-specific source naming and can be bootstrapped from a GitHub raw URL.

### Task 22 — PowerShell installer replacement

**Files:**

- Create `scripts/install-from-remote.ps1` directly below the repository root.
- Create `tests/install-from-remote.Tests.ps1`.
- Update the root README with the canonical GitHub raw URL.

**Steps:**

- Mirror Bash parameters and validation semantics exactly.
- Support Windows PowerShell and PowerShell 7 as documented targets.
- Install `atlassian-mcp.exe` and PowerShell wrapper.
- Restrict config-file ACLs where files are created.
- Configure Claude Code and Codex with escaped Windows paths.
- Implement backup, rollback, dry-run, and idempotent re-run.
- Document download to a temporary local file with `Invoke-WebRequest`, followed by explicit `pwsh -File` or `powershell.exe -File` execution.
- Do not recommend executing downloaded text directly with `Invoke-Expression`.

**Tests:**

- Pester parameter validation.
- Source remote provider neutrality.
- Module selection combinations.
- TOML/JSON escaping.
- Credential non-persistence.
- ACL command invocation.
- Final names only.
- Installer exists only at `scripts/install-from-remote.ps1`.
- Documentation fixtures resolve the GitHub raw URL to the repository-root `/scripts` path.

**Acceptance:** Windows install behavior matches Bash semantics, uses the stable repository-root path, and can be downloaded from a GitHub raw URL.

### Task 23 — Claude Code integration

**Files:**

- Update installer functions and Claude documentation.

**Steps:**

- Register server name `atlassian` with stdio wrapper.
- Support local, project, and user scopes.
- Generate `.mcp.json` only for project scope.
- Ensure no secret values are written.
- Document module configuration and Jira per-session authentication.
- Verify with installed Claude Code version used by the organization.

**Acceptance:** `/mcp` shows one Atlassian server and expected tools for configured modules.

### Task 24 — Codex integration

**Files:**

- Update TOML mutation logic and Codex documentation.

**Steps:**

- Create exactly one `[mcp_servers.atlassian]` block.
- Use final wrapper path.
- Configure startup/tool timeouts.
- Set write approval defaults and explicit tool overrides.
- Preserve unrelated user/project config.
- Back up and restore on failure.
- Verify with the installed Codex version used by the organization.

**Acceptance:** Re-running the installer updates one managed Atlassian block without duplicates or secrets.

### Task 25 — End-to-end MCP and module-isolation tests

**Files:**

- Add protocol harness and integration fixtures.

**Scenarios:**

- Jira-only startup and tool list.
- Bitbucket-only startup and tool list.
- Combined startup and tool list.
- Invalid Jira config with functional Bitbucket tools.
- Invalid Bitbucket config with functional Jira tools.
- Jira tools before authentication.
- Successful authentication followed by all Jira tools.
- Failed re-authentication preserving old session.
- Bitbucket registry exposes exactly 26 tools and every schema requires `repositorySlug`.
- Bitbucket repository/branch read-write flow.
- Bitbucket file/commit/compare/diff read flow with pagination and truncation.
- Bitbucket single-file commit create/update/conflict flow.
- Bitbucket PR read/create/comment/review flow.
- Bitbucket merge/decline/reopen flow with version conflict and one-POST invariants.
- TLS verify false with self-signed Jira and Bitbucket endpoints.
- TLS verify true with separate Jira and Bitbucket CAs.
- No logs on stdout.
- Parent stdin close terminates process.

**Acceptance:** A real MCP client can discover and invoke tools without protocol contamination.

### Task 26 — Documentation and operator runbooks

**Files:**

- Complete root README, configuration, installation, tool, security, and troubleshooting docs.

**Required documentation:**

- Final naming matrix.
- Environment-variable reference.
- Jira authentication sequence for every new MCP session.
- Credential history/logging caveat for clients.
- Module enablement matrix.
- TLS risk warning with default false.
- Custom CA configuration.
- Provider-neutral source remote examples.
- Jira field/transition passthrough examples.
- Partial-success refresh semantics.
- Common errors and remediation.
- Permission requirements.

**Acceptance:** An operator can install and use Jira-only, Bitbucket-only, or combined mode without reading source code.

### Task 27 — Release and compatibility gates

**Files:**

- Release workflow, checksums, SBOM, changelog, and verification scripts.

**Steps:**

- Cross-build supported OS/architectures.
- Produce `atlassian-mcp_*` artifacts only.
- Generate SHA-256 checksums and SBOM.
- Run Go unit/race/contract tests.
- Run ShellCheck for Bash.
- Run PSScriptAnalyzer and Pester for PowerShell.
- Run Claude Code and Codex smoke tests on pinned organizational versions.
- Run contract tests against Jira Server 6.4.14 and Bitbucket Server 5.10.2 internal staging hosts.
- Verify forbidden-name scan.

**Acceptance:** Release evidence demonstrates behavior on the exact target server and client versions.

---

## 14. Test strategy

### 14.1 Test pyramid

| Layer | Focus |
|---|---|
| Unit | Shared/module config, URL construction, Jira session store, Bitbucket pagination/version helpers, validators, redaction, truncation, refresh result composition |
| HTTP contract | Exact Jira and all 26 Bitbucket tool method/path/query/header/body/status parsing against request-recording mock servers |
| MCP protocol | Tool list, schema, annotations, stdio framing, structured results |
| Installer | Parameter parsing, source checkout, generated configs, rollback, idempotency |
| Internal staging smoke | Real Jira 6.4.14 and Bitbucket 5.10.2 compatibility |

### 14.2 Mandatory security invariants

- Password sentinel never appears outside the transient authentication input object and outbound Basic Auth operation.
- Bitbucket token sentinel never appears in generated agent configuration or logs.
- Authorization headers never appear in errors.
- No diagnostic bytes appear on stdout.
- Failed Jira authentication never destroys a valid active credential.
- Pre-auth business toolcalls send zero HTTP requests.
- Mutation refresh failure sends no second mutation.
- Invalid module config cannot block another valid module.
- Every Bitbucket business tool schema requires `repositorySlug`.
- Bitbucket pagination always follows `nextPageStart`.
- File content, diff hunks, PR comment text, and Bearer token sentinels never appear in logs.
- Ambiguous Bitbucket create/comment/commit/transition requests are sent at most once.
- PR `409` handling never performs a second transition POST.

### 14.3 Compatibility cases requiring real-host verification

- Jira comment visibility with `group` on the internal Jira 6.4.14 instance.
- Supported `expand` values and response sizes.
- Jira transition-name uniqueness behavior in active workflows.
- Jira error bodies behind the internal reverse proxy.
- TLS behavior with internal certificates and context paths.
- Claude Code and Codex handling of sensitive `jira_authenticate` input and per-tool approval settings.

---

## 15. Permission matrix

### Jira

| Tool | Minimum functional permission concept |
|---|---|
| `jira_authenticate` | Authenticated Jira user; server and self endpoints accessible |
| `jira_get_issue` | Browse Project and issue visibility |
| `jira_add_issue_comment` | Browse Project plus Add Comments; role/group visibility permission and membership rules apply |
| `jira_update_issue_fields` | Edit Issues and field/edit-screen constraints |
| `jira_transition_issue` | Transition Issues plus workflow conditions and transition-screen field requirements |

### Bitbucket

| Tool group | Tools | Minimum functional permission concept |
|---|---|---|
| Repository/branch read | `bitbucket_get_repository`, `bitbucket_list_branches`, `bitbucket_get_default_branch` | `REPO_READ` |
| Branch create | `bitbucket_create_branch` | `REPO_WRITE` |
| File/commit/compare/diff read | Eight tools in Section 10.3 | `REPO_READ` |
| Single-file commit | `bitbucket_commit_file` | `REPO_WRITE` |
| PR read/mergeability | Seven tools in Section 10.5 | `REPO_READ` |
| PR create | `bitbucket_create_pull_request` | Source and target repository read; write permission/policy required by the target instance |
| PR comment | `bitbucket_add_pull_request_comment` | `REPO_READ` plus comment permission |
| Review status | `bitbucket_set_pull_request_review_status` | `REPO_READ`; current service identity cannot review its own PR |
| Merge/decline/reopen | Three transition tools | Repository/PR permissions and branch restrictions required by the target instance |

Use service accounts and least privilege. Jira credentials are user-supplied per session and therefore reflect that user's permissions. Bitbucket Bearer PAT scope cannot exceed the creating user's permissions.

---

## 16. Error and partial-success examples

### 16.1 Unauthenticated Jira call

```json
{
  "success": false,
  "service": "jira",
  "tool": "jira_get_issue",
  "error": {
    "code": "JIRA_NOT_AUTHENTICATED",
    "message": "Call jira_authenticate before using Jira issue tools."
  }
}
```

### 16.2 Successful mutation with failed refresh

```json
{
  "success": true,
  "service": "jira",
  "tool": "jira_update_issue_fields",
  "data": {
    "mutationApplied": true,
    "issue": null,
    "refreshError": {
      "code": "JIRA_REFRESH_FAILED",
      "message": "Issue updated, but refreshing the issue failed."
    }
  }
}
```

### 16.3 Failed re-authentication

The response reports candidate authentication failure. It must not claim that the existing Jira session was cleared. A subsequent Jira read using the prior credential must continue to succeed.

---

## 17. Installer usage examples

### 17.1 Bootstrap the Bash installer from GitHub

Use a release tag or immutable commit SHA for production:

```bash
INSTALLER_REF='v1.0.0'
INSTALLER_URL="https://raw.githubusercontent.com/chiendao1808/atlassian-mcp/${INSTALLER_REF}/scripts/install-from-remote.sh"

curl -fsSL "$INSTALLER_URL" |
  bash -s -- \
    --source-repo-url https://github.com/chiendao1808/atlassian-mcp.git \
    --source-ref "$INSTALLER_REF" \
    --agents both \
    --enable-jira \
    --jira-base-url https://jira.internal.example.com/jira \
    --atlassian-tls-verify false
```

The raw URL identifies where the installer is hosted. `--source-repo-url` identifies the remote repository to clone and build; the two repositories or providers may differ.

### 17.2 Bootstrap the PowerShell installer from GitHub

```powershell
$InstallerRef = 'v1.0.0'
$InstallerUrl = "https://raw.githubusercontent.com/chiendao1808/atlassian-mcp/$InstallerRef/scripts/install-from-remote.ps1"
$InstallerFile = Join-Path $env:TEMP 'install-from-remote.ps1'

Invoke-WebRequest -Uri $InstallerUrl -OutFile $InstallerFile

pwsh `
  -NoProfile `
  -File $InstallerFile `
  -SourceRepoUrl 'https://github.com/chiendao1808/atlassian-mcp.git' `
  -SourceRef $InstallerRef `
  -Agents Both `
  -EnableJira `
  -JiraBaseUrl 'https://jira.internal.example.com/jira' `
  -AtlassianTlsVerify false
```

Windows PowerShell 5.1 may use `powershell.exe -NoProfile -ExecutionPolicy Bypass -File $InstallerFile ...` according to organizational policy.

### 17.3 Bash: combined mode from a generic remote

```bash
export BITBUCKET_BEARER_TOKEN='***'

./scripts/install-from-remote.sh \
  --source-repo-url https://github.com/chiendao1808/atlassian-mcp.git \
  --source-ref main \
  --agents both \
  --scope user \
  --enable-jira \
  --jira-base-url https://jira.internal.example.com/jira \
  --enable-bitbucket \
  --bitbucket-base-url https://bitbucket.internal.example.com/bitbucket \
  --bitbucket-project-key PRJ \
  --bitbucket-user-slug svc-atlassian-mcp \
  --bitbucket-token-env BITBUCKET_BEARER_TOKEN \
  --atlassian-tls-verify false \
  --non-interactive
```

### 17.4 Bash: Jira-only

```bash
./scripts/install-from-remote.sh \
  --source-repo-url git@gitlab.internal:tools/atlassian-mcp.git \
  --source-ref v1.0.0 \
  --agents claude \
  --enable-jira \
  --jira-base-url https://jira.internal.example.com/jira \
  --atlassian-tls-verify false
```

### 17.5 PowerShell: combined mode

```powershell
.\scripts\install-from-remote.ps1 `
  -SourceRepoUrl 'ssh://git@git.internal/tools/atlassian-mcp.git' `
  -SourceRef 'main' `
  -Agents Both `
  -Scope User `
  -EnableJira `
  -JiraBaseUrl 'https://jira.internal.example.com/jira' `
  -EnableBitbucket `
  -BitbucketBaseUrl 'https://bitbucket.internal.example.com/bitbucket' `
  -BitbucketProjectKey 'PRJ' `
  -BitbucketUserSlug 'svc-atlassian-mcp' `
  -BitbucketTokenEnv 'BITBUCKET_BEARER_TOKEN' `
  -AtlassianTlsVerify false `
  -NonInteractive
```

No installer example includes Jira username/password.

---

## 18. Definition of Done

- [ ] Binary and MCP product name are `atlassian-mcp`; server registration name is `atlassian`.
- [ ] Only `scripts/install-from-remote.sh` and `scripts/install-from-remote.ps1` are shipped, at those exact repository-root paths.
- [ ] Root README contains raw GitHub bootstrap URLs with an explicit `<ref>` segment.
- [ ] Only provider-neutral source repository parameters are documented and accepted.
- [ ] No superseded draft alias exists.
- [ ] Jira and Bitbucket can run independently or together.
- [ ] A module configuration failure cannot block another valid module.
- [ ] `ATLASSIAN_TLS_VERIFY=false` is the only default TLS verification policy.
- [ ] Jira and Bitbucket use separate optional CA files.
- [ ] Jira credentials are accepted only by `jira_authenticate` and held only in process memory.
- [ ] Authentication calls `serverInfo` then `myself` before atomic activation.
- [ ] Failed re-authentication preserves the active credential.
- [ ] All five Jira tools are registered after valid static configuration.
- [ ] Pre-auth Jira business tools return `JIRA_NOT_AUTHENTICATED` without network calls.
- [ ] `jira_get_issue` supports passthrough `fields` and `expand`.
- [ ] Comment visibility supports optional `role` and `group`.
- [ ] Field updates accept native generic `fields` and `update` objects.
- [ ] Transitions support exactly one of ID/name and native `fields`/`update`.
- [ ] Mutation read-back supports `returnIssue`, `returnFields`, and `returnExpand`.
- [ ] Refresh failure is represented as partial success and never replays mutation.
- [ ] Bitbucket registry exposes exactly the 26 tools listed in Section 10.7.
- [ ] Every Bitbucket tool has endpoint-level contract tests and requires `repositorySlug`.
- [ ] Repository/branch, file/commit/diff, single-file commit, PR read/create/comment/review, and PR transition test suites pass.
- [ ] Bitbucket pagination, response limits, diff truncation, no-secret logging, no-blind-retry, and PR optimistic-locking invariants pass.
- [ ] Bash tests, ShellCheck, PowerShell Pester, and PSScriptAnalyzer pass.
- [ ] MCP protocol tests prove stdout cleanliness.
- [ ] Internal Jira 6.4.14 and Bitbucket 5.10.2 staging smoke tests pass.
- [ ] Claude Code and Codex installation and approval behavior are verified on organizational versions.
- [ ] Release artifacts include checksums and SBOM.

---

## 19. Risks and mitigations

| Risk | Impact | Mitigation |
|---|---|---|
| Jira password appears in coding-agent history | High | Explicit warning, prompt approval, organizational logging policy review, no server-side persistence |
| TLS verification defaults off | High | Startup warning, fixed configured hosts, easy opt-in verification, separate CA tests |
| Jira 6.4.14 behavior differs from 6.4.13 REST reference examples | Medium | Contract tests against exact internal 6.4.14 host before release |
| Generic Jira update payload can make broad changes | High | Write approval, exact passthrough, no silent normalization, clear tool description |
| Transition names are duplicated | Medium | Exact match plus ambiguity error; prefer ID when known |
| POST/PUT result is ambiguous after network failure | High | Conservative no-blind-retry mutation policy |
| One module's config breaks the whole process | High | Per-module validation and registration isolation |
| Installer source URL terminology is confused with Bitbucket target | Medium | Provider-neutral `source-*` names and explicit `bitbucket-*` target names |
| Private Git remote credentials leak | High | Reject embedded HTTPS credentials; rely on SSH/credential helper |
| Old planning names reappear during implementation | Medium | Forbidden-name CI scan |

---

## 20. Source basis

Implementation agents must use the bundled REST reference as the primary compatibility source for the target internal versions:

- Jira Server 6.4.14 concepts and Jira REST API `/rest/api/2`.
- Jira issue read: `GET /rest/api/2/issue/{issueIdOrKey}`.
- Jira comment creation: `POST /rest/api/2/issue/{issueIdOrKey}/comment`.
- Jira transition listing and execution: `GET`/`POST /rest/api/2/issue/{issueIdOrKey}/transitions`.
- Jira Basic Auth behavior and `serverInfo`/`myself` smoke checks.
- Bitbucket Server 5.10.2 Core REST API `/rest/api/1.0`.
- Repository and branch endpoints.
- Raw file, commit, commit changes/diff, compare commits/changes/diff, and single-file browse PUT.
- Pull-request list/get/activities/commits/changes/diff/mergeability endpoints.
- Pull-request create/comment/review participant and merge/decline/reopen endpoints.
- Bitbucket cursor pagination through `nextPageStart` and PR optimistic locking through resource `version`.

Where the reference says behavior may depend on patch/configuration, the implementation must add a real-host contract test rather than assume support.

---

## 21. Recommended execution order

1. Tasks 1–4: freeze names and build shared platform infrastructure.
2. Tasks 5–11: implement and verify the complete 26-tool Bitbucket module.
3. Tasks 12–15: establish Jira module, client, session store, and authentication.
4. Tasks 16–19: implement Jira business tools in read-to-write order.
5. Task 20: finalize Jira security and approval behavior.
6. Tasks 21–24: implement installers and configure coding agents.
7. Task 25: run complete module/MCP integration tests.
8. Task 26: finish operator and tool documentation.
9. Task 27: release and exact-version compatibility gates.

No implementation task should introduce a superseded name for temporary compatibility.
