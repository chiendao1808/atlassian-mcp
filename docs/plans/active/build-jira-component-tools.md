# Build Jira Component MCP Tools

## Status

Implementation is locally complete after workflow state `IMPLEMENTATION` authorized the revised plan. Release remains gated on the user-owned actual-environment verification defined below; the plan stays active until that gate is declared acceptable.

## Objective And Acceptance Criteria

Add exactly six Jira Component-management MCP tools, increasing the registered Jira tool roster from 24 to 30 while retaining Jira Server 6.4.14 compatibility under `/rest/api/2`:

| Tool | Jira request |
| --- | --- |
| `jira_create_component` | `POST /rest/api/2/component` |
| `jira_get_component` | `GET /rest/api/2/component/{id}` |
| `jira_update_component` | `PUT /rest/api/2/component/{id}` |
| `jira_delete_component` | `DELETE /rest/api/2/component/{id}` with optional `moveIssuesTo` query parameter |
| `jira_get_component_issue_count` | `GET /rest/api/2/component/{id}/relatedIssueCounts` |
| `jira_list_project_components` | `GET /rest/api/2/project/{projectIdOrKey}/components` |

Acceptance criteria:

- The implementation adds only these six Component-management tools. It does not add an Issue Component assignment or convenience wrapper; callers continue to assign Issue Components through `jira_update_issue_fields` with native Jira `fields`/`update` JSON.
- `jira_create_component` exposes one project selector named `projectKey`. Its Jira request body contains `project: <projectKey>` and never contains `projectId`.
- The implementation extends the established Jira service, registration, tests, and documentation structure; it does not introduce a parallel transport or abstraction layer.
- No production change is made to `internal/jira/client/client.go`; existing authenticated request, response-envelope, error, response-size, and redaction behavior is reused.
- All six tools reject missing or invalid authentication before making a network request.
- Local `httptest` tests prove the documented Jira 6.4.14 request and response shapes, including both valid update `200` forms: empty body and JSON Component body.
- Delete includes `moveIssuesTo` only when supplied, sends exactly one DELETE request, and is never retried.
- Duplicate-name, `UNASSIGNED`, and cross-project `moveIssuesTo` outcomes are not invented or prevalidated locally; Jira's success or error is passed through the shared envelope/error mapping.
- Tests prove the exact HTTP method, `/rest/api/2` path, query encoding, request body, success/error envelope, secret redaction, and MCP annotations for every new tool.
- Registration tests prove all tool names are unique and the Jira roster is exactly 30.
- User documentation describes exact inputs, documented response handling, destructive behavior, pass-through cases, and the user-owned release gate.
- `go test ./...`, `go build ./cmd/atlassian-mcp`, and `go vet ./internal/jira/...` pass.

## Supplied-Plan Reference And Revision Summary

This document revises the existing `docs/plans/active/build-jira-component-tools.md` plan after plan-review clarification. Valid endpoint, architecture, testing, documentation, compatibility, and validation decisions are retained. The revision:

- fixes create input to the single user-facing `projectKey` field mapped to Jira JSON `project`;
- explicitly excludes an Issue Component assignment wrapper and retains `jira_update_issue_fields` for that use case;
- replaces implementer-run actual-environment discovery before code with local documented-contract tests plus a user-owned actual-environment verification/release gate after implementation; and
- resolves server-specific duplicate-name, `UNASSIGNED`, and cross-project behavior as upstream pass-through rather than local policy.

## Verified Current State

- `internal/jira/tools/service.go` owns Jira input types, auth-first service methods, URL/query construction, response shaping, and `jira_update_issue_fields`. `Service.requireCredential` returns `JIRA_NOT_AUTHENTICATED` before any request, and existing mutation methods use the shared `result.Envelope` contract.
- `internal/jira/tools/register.go` contains `Definitions` and `Service.Register`. It currently registers 24 Jira tools and applies the shared output schema and MCP annotations.
- `internal/jira/tools/tools_test.go` contains authenticated mock-server coverage, zero-network pre-auth checks, exact request assertions, error/redaction checks, annotation checks, unique-name checks, and the exact 24-tool roster assertion.
- `internal/jira/client/client.go` already builds `/rest/api/2` URLs, encodes structured query parameters, accepts empty successful bodies when an output value is supplied, applies response limits, maps Jira errors, and performs no retries. This production client is sufficient and remains out of scope.
- `docs/tools/jira.md` is the user-facing Jira tool catalog and already documents `jira_update_issue_fields` as the native Issue-field update mechanism.
- `docs/specs/jira-6.4.14-component-rest-api-reference-table-v2.md` documents the six Component endpoints, create/update fields, optional `moveIssuesTo`, empty-or-JSON update `200`, bare-array project listing, and server-dependent cases. The user decision in this plan resolves create project selection to `projectKey` -> `project` for the MCP surface.
- `docs/specs/SPECS.md` contains historical/current-looking references to “five Jira tools”; implementation must inspect the surrounding statements before changing them.

## Proposed Architecture And API Contract

Keep Component operations in the existing `tools.Service`, expose them through the existing registration list and `mcp.AddTool` pattern, and use the existing Jira client unchanged. Each service method authenticates first, validates only MCP-owned structural requirements, constructs one documented Jira request, and returns the shared sanitized result envelope.

### Tool inputs and wire mapping

| Tool | MCP input | Required local behavior | Jira wire mapping |
| --- | --- | --- | --- |
| `jira_create_component` | `projectKey string`, `name string`, optional `description`, `leadUserName`, `assigneeType` | Require nonblank `projectKey` and `name`; expose no `projectId` | Body uses `project`, `name`, and supplied optional fields |
| `jira_get_component` | `componentId string` | Require a nonblank safe path segment | `GET /component/{componentId}` |
| `jira_update_component` | `componentId string`, optional `name`, `description`, `leadUserName`, `assigneeType` | Require a safe ID and at least one supplied update field; pointer/optional representation must preserve omission versus an explicitly empty value | `PUT /component/{componentId}` with only supplied fields |
| `jira_delete_component` | `componentId string`, optional `moveIssuesTo string` | Require a safe source ID; if supplied, require a nonblank safe target ID; do not compare projects or component identity locally | `DELETE /component/{componentId}` and add `moveIssuesTo` through structured query encoding only when supplied |
| `jira_get_component_issue_count` | `componentId string` | Require a nonblank safe path segment | `GET /component/{componentId}/relatedIssueCounts` |
| `jira_list_project_components` | `projectIdOrKey string` | Require a nonblank safe path segment; this endpoint continues to accept the documented ID-or-key selector | `GET /project/{projectIdOrKey}/components` |

The documented assignee values are `PROJECT_DEFAULT`, `COMPONENT_LEAD`, `PROJECT_LEAD`, and `UNASSIGNED`. The schema may enumerate these documented values, but the service must not add environment-specific acceptance rules. In particular, `UNASSIGNED` is forwarded unchanged and Jira decides whether it is valid.

### Response behavior

- Create, get, issue-count, and list decode their documented JSON success bodies and return them through the standard sanitized success envelope.
- Update accepts either an empty `200` body or a JSON Component body. A JSON body is preserved as the response data; an empty body returns the established mutation acknowledgement `{"mutationApplied": true}` and never fabricates a Component object.
- Delete accepts the documented empty `204` and returns `{"mutationApplied": true}`.
- A bare empty list remains an empty list. No pagination, truncation, uniqueness, assignee, or project-relationship policy is invented in the service.
- Jira non-2xx responses, including duplicate names, unsupported `UNASSIGNED`, same/missing replacement, and cross-project replacement errors, use the existing sanitized error envelope. A Jira success for the same cases remains a success.

### MCP annotations

| Tool | `readOnlyHint` | `destructiveHint` | `idempotentHint` | `openWorldHint` |
| --- | ---: | ---: | ---: | ---: |
| `jira_create_component` | `false` | `false` | `false` | `true` |
| `jira_get_component` | `true` | `false` | `true` | `true` |
| `jira_update_component` | `false` | `true` | `true` | `true` |
| `jira_delete_component` | `false` | `true` | `true` | `true` |
| `jira_get_component_issue_count` | `true` | `false` | `true` | `true` |
| `jira_list_project_components` | `true` | `false` | `true` | `true` |

Update and delete idempotence describe request effects, not identical repeated statuses or bodies. The implementation must not add retry behavior.

## Step-By-Step Implementation Plan

### 1. Add focused failing service-contract tests

**File:** `internal/jira/tools/tools_test.go`

- Add `httptest` cases for each exact method and `/rest/api/2` path.
- Assert create accepts `projectKey` and emits exact JSON containing `project` while omitting `projectKey` and `projectId` from the Jira body.
- Assert optional create/update fields are emitted only when supplied, and update can represent explicit empty `description` or `leadUserName` separately from omission.
- Assert update rejects a request with no update field before network activity, accepts empty `200`, preserves JSON `200`, and rejects malformed non-empty success JSON.
- Assert delete omission produces no query, supplied `moveIssuesTo` produces one correctly encoded query value, and every invocation produces exactly one DELETE request.
- Simulate Jira success and Jira errors for duplicate names, `UNASSIGNED`, and same/missing/cross-project replacements; assert the service forwards the upstream outcome through the shared envelope without a locally invented branch or follow-up request.
- Cover empty and multi-item project Component arrays, issue count zero/nonzero, required-input failures, all relevant non-2xx mappings, complete success/error envelopes, and sentinel-secret absence from returned content and captured errors.
- For every tool, call it without an authenticated session and assert the exact `JIRA_NOT_AUTHENTICATED` envelope and zero server calls.

Run the focused Jira tools package tests and confirm the new tests fail because the six service methods and registrations do not yet exist, not because the test harness is invalid.

### 2. Implement the six service operations

**File:** `internal/jira/tools/service.go`

- Add explicit input types matching the table above. Use pointer/optional fields where omission must differ from an empty string.
- Add six service methods that call `requireCredential` before all other behavior capable of network access.
- Reuse the existing safe path-segment helpers, structured query builder, `GetJSON`, `PostJSON`, `PutJSON`, `DeleteJSON`, `jiraClientError`, `observability.Redact`, and `result.OK` conventions.
- Build create JSON with `project: input.ProjectKey`; do not define an input `ProjectID`/`projectId` and do not emit `projectId`.
- Decode update into an optional Component result so the method handles both documented successful response forms without changing the client.
- Build delete query values only when `moveIssuesTo` was supplied and invoke `DeleteJSON` exactly once. Do not add any retry, project lookup, target lookup, or same-ID check.
- Leave duplicate-name, assignee configuration, and source/target project policy to Jira.

Run the focused service tests until all service-level contract cases pass.

### 3. Register exactly six tools

**File:** `internal/jira/tools/register.go`

- Append the six exact tool definitions and handlers using the existing `Definitions` and `mcp.AddTool` conventions.
- Give create only the user-facing `projectKey` selector; retain `projectIdOrKey` only for the documented list endpoint.
- Apply the exact annotations above and the shared output schema.
- Keep issue-component assignment out of the new definitions; documentation may point callers to existing `jira_update_issue_fields`.

Extend `internal/jira/tools/tools_test.go` to compare the complete input schemas and annotation fields, assert unique names, assert all six names are present, and change the exact roster count from 24 to 30.

### 4. Update user-facing documentation and count references

**Files:**

- Modify `docs/tools/jira.md`.
- Conditionally modify `docs/specs/SPECS.md` only for confirmed current-roster assertions.

Document all six exact tool names, inputs, endpoints, response shapes, `projectKey` -> Jira `project`, optional `moveIssuesTo`, empty-or-JSON update success, destructive delete semantics, lack of automatic retry, shared errors/redaction, and the pass-through treatment of duplicate names, `UNASSIGNED`, and cross-project replacement. State that Issue Component assignment remains available through `jira_update_issue_fields`; do not document a seventh wrapper.

Inspect each “five Jira tools” occurrence in `docs/specs/SPECS.md`. Change only statements that assert the current complete roster, using 30 or accurate non-drifting wording. Preserve historical or intentionally scoped statements and record that decision under **Decisions**.

Use fictional values only. Do not add credentials, authorization headers, cookies, private base URLs, or user-environment evidence to code, fixtures, documentation, or this plan.

### 5. Validate the implementation and diff

- Format only changed Go files.
- Run the focused Jira tool tests.
- Run `go test ./...`.
- Run `go build ./cmd/atlassian-mcp`.
- Run `go vet ./internal/jira/...`.
- Inspect the final diff and confirm it contains no production change to `internal/jira/client/client.go`, no configuration/dependency/migration change, no seventh tool, and no unrelated edits.
- Record commands, exit statuses, concise results, implementation decisions, and any pre-existing unrelated failure in this plan. A pre-existing failure prevents an unqualified completion claim but does not authorize unrelated repair.

### 6. Apply the user-owned actual-environment release gate

After implementation and local validation, the user—not the implementer or agent—tests against the intended actual Jira Server 6.4.14 environment. The user owns access, credentials, test-data selection, cleanup, evidence retention, and the release decision. The implementation workflow must not request or record credentials, connect to that environment, or copy private environment data into the repository.

Using disposable Components and Issues under the user's controls, the user verifies these exact behaviors:

1. `jira_create_component` with `projectKey` creates the Component in the intended project and the Jira request is accepted using body field `project`; there is no `projectId` tool input.
2. Create and update with a duplicate name return the actual Jira success or validation error through the MCP envelope without client-side uniqueness behavior.
3. Create or update with `assigneeType: "UNASSIGNED"` returns the actual environment's success or configuration-dependent Jira error without MCP substitution or rejection beyond the documented enum.
4. `jira_update_component` succeeds whether the actual server returns an empty `200` or a JSON `200`, and the requested fields are persisted.
5. Delete without `moveIssuesTo` removes the Component reference according to Jira behavior; delete with a valid replacement moves references according to Jira behavior.
6. Delete with same-component, missing-component, and cross-project `moveIssuesTo` values surfaces the actual Jira result/error unchanged by local policy. Each invocation sends one DELETE and is not automatically retried.
7. Get, related-issue-count, and project-list responses match the intended environment, including zero count and an empty `[]`; a representative larger list is returned without fabricated pagination.
8. Returned errors and operator-visible diagnostics contain no credentials or authorization values.

Release is approved only after the user declares this matrix acceptable. A failed row blocks release and supplies a sanitized behavioral result for a new planning decision; it does not authorize the implementer to infer a replacement policy or seek environment access.

## Data, Compatibility, Deployment, And Security

- No data model, migration, persistence, cache, dependency, background job, or configuration change is required.
- All endpoint paths remain pinned to Jira Server 6.4.14 `/rest/api/2`; do not use Jira Cloud-only fields or `/rest/api/latest`.
- Existing maximum-response handling applies to the unpaged list endpoint. Do not silently truncate results or raise the global limit as part of this feature.
- Authentication remains process-session based through `jira_authenticate`. Auth-first checks must prevent accidental unauthenticated network traffic.
- Reuse centralized error normalization and redaction. Test with recognizable sentinel secrets and assert absence from complete envelopes and captured diagnostics.
- Mutation requests are not retried. Delete is destructive and may alter Issue Component references.

## Risks, Trade-Offs, Assumptions, And Recovery

- **Actual-server variation:** Published behavior leaves update body form and several validation cases environment-dependent. The code deliberately accepts both documented update success forms and passes server policy through. The user-owned gate decides release suitability.
- **Destructive delete:** A wrong replacement can modify Issue references. Local tests prove exact query construction and one request; actual-environment checks use user-selected disposable data.
- **Optional-field loss:** Plain strings can collapse omitted and empty values. Use optional representations and exact JSON tests, especially for clearing lead/description.
- **Large unpaged list:** The endpoint may exceed the configured response cap. Preserve the existing safe failure instead of truncating or changing global limits.
- **Count/documentation drift:** Registration, tests, Jira docs, and current-roster statements can disagree. Exact 30-tool assertions and a focused documentation review control the risk.
- **Rollback:** The feature is additive and stores no state. Roll back the four required file changes plus any justified `docs/specs/SPECS.md` edit. No client, configuration, migration, or data rollback is expected; the user owns cleanup of actual-environment test data.
- **Assumption:** The repository's documented Jira 6.4.14 reference and existing client/service behavior remain the implementation authority. No unresolved product choice blocks implementation after user approval.
- **Open questions:** None.

## Progress

- [x] Original plan created from repository and Jira 6.4.14 Component references.
- [x] Plan revised to incorporate the user's project selector, six-tool scope, pass-through policy, and user-owned release gate decisions.
- [x] User approves this revised plan through workflow `wf-jira-components-20260811-001` state `IMPLEMENTATION`.
- [x] Six service methods and registrations are implemented without changing `internal/jira/client/client.go`.
- [x] Focused contract, auth-first, wire, envelope, error, redaction, annotation, and roster tests pass.
- [x] Jira tool documentation and confirmed current-count assertions are updated.
- [x] Full test, build, and vet validation passes.
- [x] Local diff and validation evidence are recorded.
- [ ] User completes the actual-environment release gate and declares the matrix acceptable.
- [ ] Plan moves to `docs/plans/completed/` only after user-owned release approval.

## Decisions

- Jira Server 6.4.14 and `/rest/api/2` are fixed compatibility targets.
- The change adds exactly six Component-management tools to the existing 24-tool Jira roster, producing exactly 30.
- Create exposes only `projectKey` and maps it to Jira JSON `project`; `projectId` is not exposed.
- Issue Component assignment remains the responsibility of existing `jira_update_issue_fields`; no convenience wrapper is added.
- Existing Jira request, authentication, response-envelope, error, response-limit, and redaction behavior is reused; `internal/jira/client/client.go` remains unchanged.
- Local `httptest` is the implementation evidence for documented wire and response contracts.
- Update supports empty-or-JSON `200`; delete uses optional `moveIssuesTo`, sends one request, and never retries.
- `jira_update_component` uses `destructiveHint=true` as a client safety/approval hint because it can rename or clear Component metadata; Jira permissions remain enforced by Jira.
- Duplicate-name, `UNASSIGNED`, and same/missing/cross-project replacement behavior is decided by Jira and passed through, not recreated locally.
- Actual-environment verification and release approval belong to the user and occur after implementation/local validation without sharing environment access or credentials with the implementation workflow.
- `internal/jira/client/client_test.go` is out of scope unless an otherwise-unprovable existing-client invariant is identified; no production client change is permitted.
- `docs/specs/SPECS.md` changes only where “five Jira tools” asserts the current complete roster.
- User approval of this plan is required before any implementation change.
- `codegraph sync .` reported `CodeGraph not initialized in D:\Source Code\atlassian-mcp` despite the `.codegraph/` marker; implementation used direct filesystem inspection.
- `docs/specs/SPECS.md` was updated only for current complete-roster assertions, using non-drifting "all registered/all Jira tool definitions" wording. Historical/scoped approval-list text was preserved.
- `go build ./cmd/atlassian-mcp` produced local `atlassian-mcp.exe`; that generated executable was removed after the successful build.
- Remediation CR-001: `jira_update_component` now uses `destructiveHint=true` as a client safety/approval hint.
- Remediation CR-002: `jira_create_component` trims `projectKey` and `name` only to reject blank values; nonblank caller-provided values are forwarded unchanged in the Jira request body.
- Remediation CR-003: Component MCP dispatch coverage now invokes all six registrations through the real SDK path for success, pre-auth zero-network errors, envelope tool identity, and redacted Jira errors for JSON-returning routes.
- Remediation CR-004: Progress now separates recorded local diff/validation evidence from the still-pending user-owned release gate and plan completion.

## Validation Evidence

- `go test ./internal/jira/tools` before implementation: exit 1, expected RED build failure with missing `CreateComponent`, `GetComponent`, `UpdateComponent`, `DeleteComponent`, `GetComponentIssueCount`, and related input types.
- `gofmt -w internal/jira/tools/service.go internal/jira/tools/register.go internal/jira/tools/tools_test.go`: exit 0.
- `go test ./internal/jira/tools`: exit 0, `ok github.com/chiendao1808/atlassian-mcp/internal/jira/tools 1.482s`; rerun after docs remained green from cache.
- `go test ./...`: exit 0; all packages passed or had no test files.
- `go build ./cmd/atlassian-mcp`: exit 0; generated `atlassian-mcp.exe` removed afterward.
- `go vet ./internal/jira/...`: exit 0, no findings.
- `git diff --check`: exit 0, no whitespace errors; Git emitted LF-to-CRLF working-copy warnings only.
- `Test-Path .\atlassian-mcp.exe`: `False` after cleanup.
- Remediation RED `go test ./internal/jira/tools`: exit 1, expected failures for trimmed create body values and non-destructive `jira_update_component` annotation.
- Remediation `gofmt -w internal/jira/tools/service.go internal/jira/tools/register.go internal/jira/tools/tools_test.go`: exit 0.
- Remediation `go test ./internal/jira/tools`: exit 0, `ok github.com/chiendao1808/atlassian-mcp/internal/jira/tools 1.864s`.
- Remediation `go test ./...`: exit 0; all packages passed or had no test files.
- Remediation `go build ./cmd/atlassian-mcp`: exit 0; generated `atlassian-mcp.exe` removed afterward.
- Remediation `go vet ./internal/jira/...`: exit 0, no findings.
- Remediation `git diff --check`: exit 0, no whitespace errors; Git emitted LF-to-CRLF working-copy warnings only.
- Remediation `Test-Path .\atlassian-mcp.exe`: `False` after cleanup.

## Validation Evidence Required For Completion

Record command, exit status, and concise result for:

1. The repository's focused Jira tools package/test selector.
2. `go test ./...`
3. `go build ./cmd/atlassian-mcp`
4. `go vet ./internal/jira/...`

The final local evidence matrix must confirm:

- all six exact methods and `/rest/api/2` paths;
- create's `projectKey` input maps only to Jira body `project`;
- update JSON omissions and explicit empty values are distinct;
- both empty and JSON update `200` forms succeed;
- delete omits or encodes `moveIssuesTo` exactly and emits one request with no retry;
- duplicate-name, `UNASSIGNED`, and cross-project results are upstream pass-through cases;
- empty and multi-item Component lists preserve returned elements without fabricated pagination;
- authentication failures produce zero network requests;
- success and error envelopes match existing Jira tools exactly;
- sentinel credentials and upstream secret material are redacted;
- all six schemas and annotations match registration expectations;
- registered Jira tool names are unique and total exactly 30;
- Issue Component assignment remains documented under `jira_update_issue_fields`, with no new wrapper;
- documentation matches the implemented schema and behavior; and
- the diff contains no production `internal/jira/client/client.go`, configuration, dependency, migration, or unrelated change.

## Definition Of Done

Implementation is done only when the user has approved this plan, exactly six Component tools are registered and documented, the Jira roster is exactly 30, all local contract and repository validation passes, the production Jira client remains unchanged, the diff contains no unrelated scope, and the user has declared the actual-environment release matrix acceptable. Until then this plan remains active.
