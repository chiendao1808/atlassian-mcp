# Execution Plan: Jira MCP tools — Sections 7 & 8 completion (19 new tools)

Date: 2026-08-04

## Status

Completed

## Outcome

The Jira MCP module exposes 24 tools total: the 5 existing plus 19 new tools covering every remaining REST row in spec sections 7 (Issue CRUD & JQL search) and 8 (comment, transition, attachment, worklog, watcher, vote, issue link). Each new tool authenticates through the process session, validates inputs and path segments before any network call, redacts response data, maps upstream failures through the shared HTTP-error path, carries correct MCP security annotations, has unit-test coverage mirroring existing patterns, and is documented in `docs/tools/jira.md`. `go build ./cmd/atlassian-mcp` and `go test ./...` pass on Windows.

## Context

- Spec: `docs/specs/jira-6.4.14_bitbucket-5.10.2-rest-api-reference.md` lines 166-206 (sections 7 and 8).
- Registration + annotations: `internal/jira/tools/register.go` (`Definitions()` 10-21, `Register` 23-43).
- Handlers + shared helpers: `internal/jira/tools/service.go` (`GetIssue` 111, `AddIssueComment` 129, `UpdateIssueFields` 158, `TransitionIssue` 187, `requireCredential` 226, `refreshAfterMutation` 240, `resolveTransitionName` 256, `cleanIssueID` 285, `optionalQuery` 299, `jiraClientError` 314).
- HTTP client: `internal/jira/client/client.go` (`GetJSON`/`PostJSON`/`PutJSON` 48-58, `doJSON` 60-104, `urlFor` 106-117, `mapHTTPError`/`mapStatusCode` 119-146).
- Multipart analog to mirror: `internal/bitbucket/client/request.go` `DoMultipart` (49-76) + generic `do` (78-137); multipart round-trip test `internal/bitbucket/client/client_test.go` (220-246).
- Envelope constructors: `internal/result/envelope.go` (`OK` 26, `Fail` 30, `FailHTTP` 34, `FailHTTPDetail` 38).
- Redaction: `internal/observability/redact.go` `Redact` (13-36) — recurses over `map[string]any` and `[]any`.
- Test roster enforcement: `internal/jira/tools/tools_test.go` `TestJiraToolDefinitionsHaveSecurityAnnotations` (363-383).
- Docs: `docs/tools/jira.md`.
- Module wiring: `internal/jira/module.go` / `internal/jira/config.go` — single-unit registration gated on `JIRA_BASE_URL`; no change required.

## Scope

In scope:

- Client primitives: DELETE wrapper, query-capable POST wrapper, and a multipart+custom-header primitive in `internal/jira/client/client.go`.
- 19 new tools (input structs, service handlers, definitions, registration wiring).
- Unit tests per tool/group plus the roster-test extension.
- `docs/tools/jira.md` row per new tool.

Out of scope:

- Section 6 metadata endpoints (createmeta/editmeta, field/priority/status lists) — callers supply valid `fields` themselves.
- Section 9 (project/component/version) and any Bitbucket work.
- Per-tool conditional registration, new env vars, module wiring changes.
- Retry/backoff on Jira writes (Jira client has none today; not introduced here).

## Approach

Land the work in one coherent sequence. The client extension is the only hard prerequisite (attachment tools depend on it); everything else is independent and grouped for review clarity. All handlers follow the established shape: `requireCredential` → validate/`cleanPathSegment` → build request → call client → `jiraClientError` on failure → `result.OK` with redacted data. `implementer` performs all writes; approval gates before code.

### Group 0 — Client extension (`internal/jira/client/client.go`) — PREREQUISITE

Refactor the response tail of `doJSON` into a shared low-level method, then add three entry points. This mirrors bitbucket's `do`/`DoJSON`/`DoMultipart` split without importing bitbucket code.

- Extract private `do(ctx, cred, method, apiPath string, query map[string][]string, contentType string, extraHeaders map[string]string, body io.Reader, out any) error`. It sets Basic Auth + `Accept: application/json`, sets `Content-Type` only when `contentType != ""`, applies each `extraHeaders` entry, then runs the existing 204/limit-read/size-check/`mapHTTPError`/unmarshal tail verbatim (client.go:78-104). No retry (parity with current Jira client).
- Rewrite `doJSON` to marshal the body, set `contentType="application/json"` when body is non-nil, and delegate to `do`. Existing `GetJSON`/`PostJSON`/`PutJSON` keep their signatures and behavior.
- Add `func (c *Client) DeleteJSON(ctx context.Context, cred auth.Credential, apiPath string, query map[string][]string, out any) error` → `do(..., http.MethodDelete, query, "", nil, nil, out)`. Query support is required for `deleteSubtasks` and watcher-removal `username`.
- Add `func (c *Client) PostJSONQuery(ctx context.Context, cred auth.Credential, apiPath string, query map[string][]string, body any, out any) error` — marshals body and delegates with method POST + query. Needed only for worklog `adjustEstimate`; keeps `PostJSON` untouched.
- Add `func (c *Client) DoMultipart(ctx context.Context, cred auth.Credential, apiPath string, fields map[string]string, fileField, fileName string, content io.Reader, extraHeaders map[string]string, out any) error`. Builds a `mime/multipart` writer (write `fields`, `CreateFormFile(fileField, fileName)`, copy content, `Close()`), then `do(POST, apiPath, nil, w.FormDataContentType(), extraHeaders, &buf, out)`. Attachment upload passes `fileField="file"` and `extraHeaders={"X-Atlassian-Token":"nocheck"}`.

### Group 0b — Shared service helper generalization (`internal/jira/tools/service.go`)

- Add `cleanPathSegment(tool, field, value string) (string, *result.Envelope)` carrying the current `cleanIssueID` logic (trim, empty check, reject `/?#\`) but with a caller-supplied `field` in the error message. Reimplement `cleanIssueID` as `cleanPathSegment(tool, "issueIdOrKey", value)` so existing messages/tests are unchanged. New tools validating `commentId`, `attachmentId`, worklog `id`, etc. reuse `cleanPathSegment` so error text names the right field.
- Add a `query map[string][]string` helper type with fluent `.add(k, v string)`, `.bool(k string, v *bool)`, `.int(k string, v *int)` methods (verified against the existing analog `internal/bitbucket/tools/service.go:127-173` — Jira's `GetJSON`/`PostJSON`/`DeleteJSON`/etc. already take `map[string][]string` for query, so this is a direct, low-risk port, not a new design). Used by `jira_search_issues` (`startAt`/`maxResults`/`validateQuery`), `jira_list_issue_comments` (`startAt`/`maxResults`/`orderBy`), `jira_delete_issue` (`deleteSubtasks`), `jira_add_issue_worklog` (`adjustEstimate`/`newEstimate`/`reduceBy`), and `jira_remove_issue_watcher` (`username`) — replacing ad hoc per-tool query-map construction with one consistent helper.
- Every `†` tool (see the read-back note below) must replicate the existing guard at service.go:170/196 verbatim: reject with `VALIDATION_ERROR` when `ReturnFields`/`ReturnExpand` is non-empty but `ReturnIssue` is not `true` — this is already-established behavior on `jira_update_issue_fields`/`jira_transition_issue`, not a new rule invented for this plan.

### Per-tool implementation detail

Every handler calls `s.requireCredential(<tool>)` first (zero network calls pre-auth — test-enforced), then validates path segments with `cleanIssueID`/`cleanPathSegment`, then hits the client, then wraps failures via `jiraClientError(<tool>, "", err)` and success via `result.OK("jira", <tool>, ...)` with response data passed through `observability.Redact`. One `<Verb><Noun>Input` struct per tool with `json` + `jsonschema` tags. Annotation legend: RO = `ReadOnlyHint:true`; ADD = `DestructiveHint:&additive(false)`; DES = `DestructiveHint:&destructive(true)`; all carry `OpenWorldHint:&open`.

**Post-mutation issue read-back (decision D-R, confirmed by user 2026-08-04).** Every tool marked **†** below additionally accepts the same optional triad already used by `jira_update_issue_fields`/`jira_transition_issue` — `ReturnIssue bool \|omitempty`, `ReturnFields []string \|omitempty`, `ReturnExpand []string \|omitempty` — and, when `ReturnIssue` is true, appends a best-effort refreshed `issue` object to its success data via the same `refreshAfterMutation`-style read-back (a failed refresh reports `mutationApplied:true` without `issue` rather than failing the whole call, exactly like the existing two write tools). Tools NOT marked † are deliberately excluded from this triad because they lack a natural single existing-issue anchor to refresh:
- `jira_delete_issue` — the issue itself is gone after the call.
- `jira_delete_issue_attachment` — its REST path (`/attachment/{id}`) carries no issue key at all.
- `jira_create_issue` / `jira_bulk_create_issues` — these create new issue(s) rather than mutate an existing one; there is no pre-existing "the issue" to refresh (a post-create fetch by the new key is a possible follow-up, not built now).
- `jira_create_issue_link` — links two issues; no single unambiguous issue to refresh.
- All read-only (RO) tools are unaffected — they already return issue-scoped data directly.

**Group A — Issue CRUD (section 7)**

| Tool | Input struct fields | Client call | Request shape | Success data | Errors | Annot |
|---|---|---|---|---|---|---|
| `jira_create_issue` | `Fields map[string]any` (required), `Update map[string]any \|omitempty` | `PostJSON` | POST `/issue`, body `{fields, update?}` | `result.OK` with created object `{id,key,self}` (map) | `VALIDATION_ERROR` if `Fields` empty; else `jiraClientError` | ADD |
| `jira_bulk_create_issues` | `IssueUpdates []map[string]any` (required, each `{fields,update?}`) | `PostJSON` | POST `/issue/bulk`, body `{issueUpdates:[...]}` | `{issues:[...], errors:[...]}` redacted — partial success surfaced, not treated as failure | `VALIDATION_ERROR` if list empty | ADD |
| `jira_delete_issue` | `IssueIDOrKey string` (required), `DeleteSubtasks bool \|omitempty` | `DeleteJSON` | DELETE `/issue/{id}`, query `deleteSubtasks=true` only when set | `{mutationApplied:true}` (204, no body) | `cleanIssueID`; else `jiraClientError` | DES |
| `jira_assign_issue` † | `IssueIDOrKey string` (required); `Name string` (may be empty); `ReturnIssue bool \|omitempty`; `ReturnFields []string \|omitempty`; `ReturnExpand []string \|omitempty` | `PutJSON` then `refreshAfterMutation` | PUT `/issue/{id}/assignee`, body `{"name": <value>}` when `Name` non-empty, else `{"name": ""}` | `{mutationApplied:true}` (+ `issue` when `ReturnIssue`) | `cleanIssueID` | DES |

Assignee semantics (decision D-A, confirmed by user 2026-08-04): only two states are modeled — `Name` empty/omitted → body `{"name": ""}` (unassign, per the spec's own note that an empty string is used for unassign); `Name` non-empty → body `{"name": "<value>"}` (assign). The spec's separate "automatic assignee via null" case is intentionally not distinguished from unassign — both collapse to the empty-string request, per user's explicit choice. `Name` is a plain `string`, not a pointer, so no pointer/omitted-vs-null ambiguity in the MCP input schema.

**Group B — JQL search (section 7)**

| Tool | Input struct fields | Client call | Request shape | Success data | Errors | Annot |
|---|---|---|---|---|---|---|
| `jira_search_issues` | `JQL string` (required); `StartAt *int \|omitempty`; `MaxResults *int \|omitempty`; `Fields []string \|omitempty`; `Expand []string \|omitempty`; `ValidateQuery *bool \|omitempty` | `PostJSON` | POST `/search`, body built from set fields only (`jql` always; others when non-nil) | `{startAt,maxResults,total,issues:[...]}` redacted | `VALIDATION_ERROR` if `JQL` blank | RO |

Implemented via POST only (decision D-4); GET `/search` deliberately not exposed to avoid URL-length limits.

**Group C — Comments (section 8)** — additive to existing `jira_add_issue_comment`

| Tool | Input struct fields | Client call | Request shape | Success data | Errors | Annot |
|---|---|---|---|---|---|---|
| `jira_list_issue_comments` | `IssueIDOrKey string`; `StartAt *int`; `MaxResults *int`; `OrderBy string \|omitempty`; `Expand []string \|omitempty` | `GetJSON` | GET `/issue/{id}/comment`, query from set fields (`orderBy` passed only when non-empty) | `{startAt,maxResults,total,comments:[...]}` redacted | `cleanIssueID` | RO |
| `jira_update_issue_comment` † | `IssueIDOrKey string`; `CommentID string` (required); `Body string` (required); `Visibility *Visibility \|omitempty`; `Expand []string \|omitempty`; `ReturnIssue bool \|omitempty`; `ReturnFields []string \|omitempty`; `ReturnExpand []string \|omitempty` | `PutJSON` (+ `refreshAfterMutation` when `ReturnIssue`) | PUT `/issue/{id}/comment/{commentId}`, body `{body, visibility?}` | `{comment: <updated comment>}` redacted (200), + `issue` when `ReturnIssue` | `cleanIssueID`, `cleanPathSegment(_, "commentId", …)`, blank-body `VALIDATION_ERROR`, reuse existing visibility validation | ADD |
| `jira_delete_issue_comment` † | `IssueIDOrKey string`; `CommentID string` (required); `ReturnIssue bool \|omitempty`; `ReturnFields []string \|omitempty`; `ReturnExpand []string \|omitempty` | `DeleteJSON` (+ `refreshAfterMutation` when `ReturnIssue`) | DELETE `/issue/{id}/comment/{commentId}` | `{mutationApplied:true}` (204) (+ `issue` when `ReturnIssue`) | `cleanIssueID`, `cleanPathSegment` for commentId | DES |

Reuse the existing `Visibility` struct (service.go:46) and its validation block (service.go:142-149) for the update path.

**Group D — Transition listing (section 8)** — additive read to existing write-only `jira_transition_issue`

| Tool | Input struct fields | Client call | Request shape | Success data | Errors | Annot |
|---|---|---|---|---|---|---|
| `jira_list_issue_transitions` | `IssueIDOrKey string`; `TransitionID string \|omitempty`; `Expand []string \|omitempty` (defaults to `transitions.fields` when caller omits — build query with that value) | `GetJSON` | GET `/issue/{id}/transitions`, query `expand=transitions.fields` (+ optional `transitionId`) | `{transitions:[...]}` redacted | `cleanIssueID` | RO |

**Group E — Attachments (section 8)** — depends on Group 0

| Tool | Input struct fields | Client call | Request shape | Success data | Errors | Annot |
|---|---|---|---|---|---|---|
| `jira_add_issue_attachment` † | `IssueIDOrKey string`; `Filename string` (required); `ContentBase64 string` (required); `ReturnIssue bool \|omitempty`; `ReturnFields []string \|omitempty`; `ReturnExpand []string \|omitempty` | `DoMultipart` (+ `refreshAfterMutation` when `ReturnIssue`) | POST `/issue/{id}/attachments`, field `file`, header `X-Atlassian-Token: nocheck`; decode base64 → `bytes.NewReader` | array response wrapped `{attachments:[...]}`, `Redact`ed (200, not 201), + `issue` when `ReturnIssue` | `cleanIssueID`; base64 decode failure → `VALIDATION_ERROR`; blank filename → `VALIDATION_ERROR` | ADD |
| `jira_delete_issue_attachment` | `AttachmentID string` (required) | `DeleteJSON` | DELETE `/attachment/{attachmentId}` — root path differs, **no** issue key; pass `attachment/{id}` as `apiPath`; `urlFor` already builds `<base>/rest/api/2/attachment/{id}` | `{mutationApplied:true}` (204) | `cleanPathSegment(_, "attachmentId", …)` | DES |

Attachment content is accepted as base64 (`ContentBase64`) because MCP tool inputs are JSON — no raw file handle is available. Decode into an in-memory `io.Reader`. Unmarshal the 200 array into `[]any` (or `any`) so `Redact` recurses correctly.

**Group F — Worklog (section 8)**

| Tool | Input struct fields | Client call | Request shape | Success data | Errors | Annot |
|---|---|---|---|---|---|---|
| `jira_list_issue_worklogs` | `IssueIDOrKey string` | `GetJSON` | GET `/issue/{id}/worklog` | `{startAt,maxResults,total,worklogs:[...]}` redacted | `cleanIssueID` | RO |
| `jira_add_issue_worklog` † | `IssueIDOrKey string`; `TimeSpentSeconds int` (required, >0); `Comment string \|omitempty`; `Started string \|omitempty`; `AdjustEstimate string \|omitempty`; `NewEstimate string \|omitempty`; `ReduceBy string \|omitempty`; `ReturnIssue bool \|omitempty`; `ReturnFields []string \|omitempty`; `ReturnExpand []string \|omitempty` | `PostJSONQuery` (+ `refreshAfterMutation` when `ReturnIssue`) | POST `/issue/{id}/worklog`, body `{timeSpentSeconds, comment?, started?}`, query `adjustEstimate` (+`newEstimate`/`reduceBy` when applicable) | `{worklog: <created worklog>}` redacted (201), + `issue` when `ReturnIssue` | `cleanIssueID`; `TimeSpentSeconds<=0` → `VALIDATION_ERROR`; if `AdjustEstimate` set, validate it is one of `new\|leave\|manual\|auto` else `VALIDATION_ERROR` | ADD |

**Group G — Watchers & votes (section 8)** — three watcher tools + two vote tools (decisions D-7, D-2)

| Tool | Input struct fields | Client call | Request shape | Success data | Errors | Annot |
|---|---|---|---|---|---|---|
| `jira_get_issue_watchers` | `IssueIDOrKey string` | `GetJSON` | GET `/issue/{id}/watchers` | `{isWatching,watchCount,watchers:[...]}` redacted | `cleanIssueID` | RO |
| `jira_add_issue_watcher` † | `IssueIDOrKey string`; `Username string` (required); `ReturnIssue bool \|omitempty`; `ReturnFields []string \|omitempty`; `ReturnExpand []string \|omitempty` | `PostJSON` (+ `refreshAfterMutation` when `ReturnIssue`) | POST `/issue/{id}/watchers`, body is the **bare JSON string** `"bob"` — pass the Go `string` directly to `PostJSON`; `json.Marshal("bob")` yields `"bob"` (decision D-W) | `{mutationApplied:true}` (204) (+ `issue` when `ReturnIssue`) | `cleanIssueID`; blank username → `VALIDATION_ERROR` | ADD |
| `jira_remove_issue_watcher` † | `IssueIDOrKey string`; `Username string` (required); `ReturnIssue bool \|omitempty`; `ReturnFields []string \|omitempty`; `ReturnExpand []string \|omitempty` | `DeleteJSON` (+ `refreshAfterMutation` when `ReturnIssue`) | DELETE `/issue/{id}/watchers`, query `username=<value>` (URL-encoded by `urlFor`) | `{mutationApplied:true}` (204) (+ `issue` when `ReturnIssue`) | `cleanIssueID`; blank username → `VALIDATION_ERROR` | DES |
| `jira_vote_issue` † | `IssueIDOrKey string`; `ReturnIssue bool \|omitempty`; `ReturnFields []string \|omitempty`; `ReturnExpand []string \|omitempty` | `PostJSON` (nil body) (+ `refreshAfterMutation` when `ReturnIssue`) | POST `/issue/{id}/votes`, no body | `{mutationApplied:true}` (204) (+ `issue` when `ReturnIssue`) | `cleanIssueID` | ADD |
| `jira_unvote_issue` † | `IssueIDOrKey string`; `ReturnIssue bool \|omitempty`; `ReturnFields []string \|omitempty`; `ReturnExpand []string \|omitempty` | `DeleteJSON` (nil query) (+ `refreshAfterMutation` when `ReturnIssue`) | DELETE `/issue/{id}/votes` | `{mutationApplied:true}` (204) (+ `issue` when `ReturnIssue`) | `cleanIssueID` | DES |

**Group H — Issue link (section 8)**

| Tool | Input struct fields | Client call | Request shape | Success data | Errors | Annot |
|---|---|---|---|---|---|---|
| `jira_create_issue_link` | `Type map[string]any` (required, e.g. `{"name":"Blocks"}`); `InwardIssue map[string]any` (required); `OutwardIssue map[string]any` (required); `Comment map[string]any \|omitempty` | `PostJSON` | POST `/issueLink`, body `{type,inwardIssue,outwardIssue,comment?}` | `{mutationApplied:true}` — Jira returns 201 with usually no body | `VALIDATION_ERROR` if any of the three required maps empty | ADD |

`/issueLink` and `/attachment/{id}` are not `/issue/...` paths — pass `issueLink` / `attachment/{id}` as `apiPath`; `urlFor` prepends `/rest/api/2/` correctly.

### Registration wiring (`internal/jira/tools/register.go`)

- Append 19 `*mcp.Tool` entries to the `Definitions()` slice (indices 5-23) with the annotations above and a one-line description each ("Requires jira_authenticate first"; add "and client approval" for writes).
- Add one `mcp.AddTool(server, defs[n], func…)` block per tool in `Register`, each delegating to its `Service` method and returning `(nil, envelope, nil)`, matching the existing 5 blocks.

## Verified Technical Assumptions

Checked directly against the current source (not assumed) on 2026-08-04:

- `internal/jira/client/client.go:106-117` (`urlFor`) builds `<base>/rest/api/2/<apiPath>` generically with no hardcoded `issue/` segment — confirms `attachment/{id}` and `issueLink` paths (Group E/H) resolve correctly without special-casing.
- `internal/jira/client/client.go:60-104` (`doJSON`) only ever marshals a Go value to JSON and sets a fixed `Content-Type`/`Accept`/Basic-Auth header set — confirms Group 0's need for a lower-level `do` with custom content-type/headers/raw-`io.Reader` body is real, not speculative.
- Pointer-typed optional fields (`*int`, `*bool`) for MCP tool inputs are an established, working pattern already used throughout `internal/bitbucket/tools/*.go` (`commits.go`, `branches.go`, `pull_requests.go`) — de-risks the plan's use of `*int`/`*bool` for `jira_search_issues`/`jira_list_issue_comments` pagination and `ValidateQuery`.
- `internal/jira/tools/service.go:63-65,75-77,170,196` confirms the exact existing shape of `ReturnIssue bool`/`ReturnFields []string`/`ReturnExpand []string` (all plain, `omitempty`) plus the "fields/expand without ReturnIssue=true is a VALIDATION_ERROR" guard — the plan's `†`-tool triad now matches this exactly rather than inventing a new shape.
- No existing `query`-map builder helper exists in `internal/jira/tools/service.go` today (only `optionalQuery` for `[]string` joins) — confirmed gap, now added to Group 0b by porting `internal/bitbucket/tools/service.go:127-173`'s `query` type.

## Risks And Recovery

- **`orderBy` on comment listing (spec line 194: "nếu patch hỗ trợ") may not exist on Jira 6.4.14.** Mitigation: send `orderBy` only when the caller sets it; document in `docs/tools/jira.md` that unsupported values surface as a passed-through upstream error rather than being silently dropped. Do not default it.
- **`notifyUsers` on issue update (spec line 173: "tùy cấu hình/patch").** Out of scope here (belongs to the existing `jira_update_issue_fields`); not added, avoiding an unverified param.
- **Watcher POST/DELETE body is a bare JSON string, not an object.** POST sends the Go `string` (marshals to `"bob"`); DELETE uses the `username` **query** param, not a body — modeled exactly per spec line 204. A test asserts the request body is literally `"bob"` and the DELETE carries `?username=`.
- **Attachment upload returns 200 (not 201) and an array body.** Handler treats 2xx uniformly (client accepts 200-299) and unmarshals into an array; the `X-Atlassian-Token: nocheck` header is mandatory or Jira returns XSRF 403. Test asserts header presence, multipart field name `file`, and array unwrapping.
- **`/attachment/{id}` and `/issueLink` are outside the `/issue/` tree.** Path built as `attachment/{id}` / `issueLink` (not prefixed with `issue/`); `cleanPathSegment` guards the attachment ID. Test asserts the exact request path.
- **Assignee empty-string unassign is config-dependent.** Per spec, whether an empty-string `name` actually unassigns depends on project/server configuration; Jira rejects invalid states with its own error, surfaced via `jiraClientError`. User explicitly chose empty-string over `null` for the unassign case (2026-08-04) despite this config-dependence risk. Recovery: caller reads the upstream detail.
- **Bulk create partial success.** `{errors:[...]}` in a 2xx body is returned as data with `success:true` (user-confirmed 2026-08-04), not converted to a tool failure, so callers see which rows failed by reading the `errors` array themselves. Documented explicitly in `docs/tools/jira.md`.
- **Client refactor regression risk.** Extracting `do` from `doJSON` could alter existing GET/POST/PUT behavior. Mitigation: keep `GetJSON`/`PostJSON`/`PutJSON` signatures and semantics identical; the full existing `client_test.go` suite must pass unchanged before adding new client tests.

Recovery/rollback: the change is purely additive at the module boundary (new tools + new client methods + one internal refactor). Reverting the commit restores the 5-tool state; no data migration, config, or persisted state is involved.

## Progress

- [x] Group 0: refactor `doJSON`→`do`; add `DeleteJSON`, `PostJSONQuery`, `DoMultipart` in `client.go`; add client unit tests (DELETE with query, POST with query, multipart round-trip mirroring bitbucket test). Verified: `go build ./...`, `go test ./internal/jira/... ./internal/bitbucket/...`, `go vet ./internal/jira/...` all pass; zero caller changes needed elsewhere.
- [x] Group 0b: add `cleanPathSegment`; reimplement `cleanIssueID` on top of it; add `query` map helper (`.add`/`.bool`/`.int`) mirroring `internal/bitbucket/tools/service.go`. Helper added and unit-tested, not yet wired into any handler (by design — wiring happens as each tool group is implemented next).
- [x] Group A: `jira_create_issue`, `jira_bulk_create_issues`, `jira_delete_issue`, `jira_assign_issue` + tests. Verified: `go build ./...`, `go test ./internal/jira/...`, `go vet ./internal/jira/...` all pass; D-A/D-H (literal `{"name":""}` unassign body) and D-I (bulk-create partial-failure `errors` surfaced with `success:true`) each covered by a dedicated test.
- [x] Group B: `jira_search_issues` + tests. Verified: POST `/search` body built from only caller-set optional fields; blank-JQL validation covered.
- [x] Group C: `jira_list_issue_comments`, `jira_update_issue_comment`, `jira_delete_issue_comment` + tests. Verified: `go build ./...`, `go test ./internal/jira/...`, `go vet ./internal/jira/...` all pass; both † tools replicate AssignIssue's read-back triad/guard exactly (dedicated "ReturnFields without ReturnIssue" and successful-read-back tests for each); orderBy-omitted-unless-set covered directly. Deviation noted: `jira_update_issue_comment`'s `expand` is accepted on input but not forwarded on the wire (PutJSON has no query support; a follow-up `PutJSONQuery` would be needed), and its DestructiveHint is set to `true` to match `jira_update_issue_fields`'s actual current annotation rather than the additive-false assumption in the dispatch note.
- [x] Group D: `jira_list_issue_transitions` + tests. Verified: default `expand=transitions.fields` sent when Expand omitted, and caller-supplied Expand replaces (not merges with) the default -- both covered by dedicated tests.
- [x] Group E: `jira_add_issue_attachment`, `jira_delete_issue_attachment` + tests. Verified: `go build ./...`, `go test ./internal/jira/...`, `go vet ./internal/jira/...` all pass; dedicated tests assert the multipart `Content-Type` prefix, mandatory `X-Atlassian-Token: nocheck` header, `file` field name, and 200 JSON-array-to-`data.attachments` unwrapping; a separate test asserts the attachment-delete path is exactly `/rest/api/2/attachment/{id}` with no `issue/` segment. `jira_add_issue_attachment` replicates AssignIssue's read-back triad/guard exactly (dedicated "ReturnFields without ReturnIssue" and successful-read-back tests); `jira_delete_issue_attachment` has no triad by design (no issue key in its input).
- [x] Group F: `jira_list_issue_worklogs`, `jira_add_issue_worklog` + tests. Verified: `go build ./...`, `go test ./internal/jira/...`, `go vet ./internal/jira/...` all pass; a dedicated test asserts `adjustEstimate`/`newEstimate`/`reduceBy` land on the query string while `comment`/`started`/`timeSpentSeconds` land in the JSON body (and never cross over), plus a separate test asserts the estimate query params are fully omitted when `adjustEstimate` is unset; invalid-`adjustEstimate`-value and non-positive-`timeSpentSeconds` validation-error cases are covered. `jira_add_issue_worklog` replicates AssignIssue's read-back triad/guard exactly.
- [x] Group G: `jira_get_issue_watchers`, `jira_add_issue_watcher`, `jira_remove_issue_watcher`, `jira_vote_issue`, `jira_unvote_issue` + tests. Verified: bare-JSON-string watcher-add body, query-based watcher-remove, read-back triad parity on all 5 mutating tools.
- [x] Group H: `jira_create_issue_link` + tests. Verified: POSTs to `issueLink` path outside the `/issue/...` tree, correct body shape, no read-back triad (deliberate exclusion).
- [x] Registration: append 19 defs + 19 `AddTool` blocks. `len(defs) == 24` asserted by test.
- [x] Extend `TestJiraToolDefinitionsHaveSecurityAnnotations` roster and annotation assertions to all 24 tools. Passes.
- [x] `docs/tools/jira.md`: add 19 rows. 24 rows total, verified no duplicates/gaps.
- [x] Validation: `go build ./...`, `go build ./cmd/atlassian-mcp`, `go test ./...`, `go vet ./...` all green (115/115 tests pass in `internal/jira/...`).

## Decisions

Orchestrator scope decisions recorded verbatim so future readers know why scope looks as it does:

- D-1: Attachment upload/delete IS in scope. It requires extending `internal/jira/client/client.go` with a multipart POST primitive plus custom-header injection, mirroring `internal/bitbucket/client/request.go`'s `DoMultipart`. This is a prerequisite sub-step before the attachment tools.
- D-2: Vote/unvote and issueLink creation ARE in scope (both are literal rows in section 8).
- D-3: Bulk issue create is a SEPARATE tool from single create (different path, body shape `issueUpdates[]`, partial-success semantics).
- D-4: JQL search is ONE tool, `jira_search_issues`, implemented via POST `/rest/api/2/search` internally (avoids URL length limits) — do not add a second GET-based search tool.
- D-5: Assignee gets its OWN dedicated tool `jira_assign_issue` (distinct null/auto-assign/unassign semantics not modeled by the generic fields-update tool), additive to the existing `jira_update_issue_fields`.
- D-6: List transitions gets its OWN dedicated read tool `jira_list_issue_transitions` (expand=transitions.fields), additive to the existing write-only `jira_transition_issue`.
- D-7: Watchers are THREE separate tools (get/add/remove), matching the existing one-tool-per-HTTP-verb granularity convention already used by all 5 existing tools — do not build one action-multiplexed tool.
- D-8: No createmeta/editmeta helper tool. Section 6 metadata endpoints are explicitly out of scope; callers must supply valid `fields` themselves.

Task-local implementation decisions (all confirmed with user 2026-08-04 via explicit question+options):

- D-A: `jira_assign_issue` models assignee as a plain `Name string` with only 2 states — empty/omitted → unassign, non-empty → `{"name":"<value>"}` (assign). The spec's third "automatic assignee via null" case is deliberately not distinguished from unassign; user chose to collapse them rather than use a pointer or sentinel-string design. (See D-H for the exact wire value sent for the empty case — `{"name":""}`, not `{"name":null}`.)
- D-W: watcher add sends the bare JSON string body (Go `string` → `"bob"`); watcher remove uses the `username` query param — both per spec line 204, not object-wrapped.
- D-B: attachment content accepted as `ContentBase64` (JSON-safe) and decoded to an in-memory reader, since MCP inputs carry no file handle. `jira_add_issue_attachment` is single-file per call by user's explicit choice (multi-file upload is an out-of-scope follow-up, not built now).
- D-C: client response tail extracted into a private `do`; `GetJSON`/`PostJSON`/`PutJSON` keep identical signatures/behavior to bound regression risk.
- D-G: watchers remain 3 separate tools (`jira_get_issue_watchers`/`jira_add_issue_watcher`/`jira_remove_issue_watcher`), confirmed over a single action-multiplexed alternative, to match the existing one-tool-per-HTTP-verb convention.
- D-4 (reconfirmed): JQL search stays a single tool (`jira_search_issues`) using POST `/search` internally; no separate GET-based tool is added.
- D-H (2026-08-04): assignee empty/unassign body is `{"name": ""}` (empty string), not `{"name": null}` — chosen to match the spec's own note that an empty string, not null, is what triggers unassign (config-permitting). `null` is reserved by the spec for "automatic assignee," a state this plan does not expose separately (see D-A).
- D-I (2026-08-04): `jira_bulk_create_issues` reports `success:true` with the upstream `errors` array passed through in `data` even when some rows failed — HTTP 201 means the request itself was accepted; callers must inspect `errors` themselves rather than relying on the envelope's `success` flag to detect partial failure.
- D-R (2026-08-04): the post-mutation read-back triad (`ReturnIssue`/`ReturnFields`/`ReturnExpand`), previously only on `jira_update_issue_fields`/`jira_transition_issue`, is extended to every new tool that mutates something on an existing, still-extant issue: `jira_assign_issue`, `jira_update_issue_comment`, `jira_delete_issue_comment`, `jira_add_issue_attachment`, `jira_add_issue_worklog`, `jira_add_issue_watcher`, `jira_remove_issue_watcher`, `jira_vote_issue`, `jira_unvote_issue` (marked † in the tables above). Excluded by mechanical criteria, not fresh judgment: `jira_delete_issue` (issue no longer exists to refresh), `jira_delete_issue_attachment` (no issue key in its path), `jira_create_issue`/`jira_bulk_create_issues` (create new issues, no pre-existing anchor), `jira_create_issue_link` (two issues, no single anchor).
- D-CR001 (2026-08-04, post code-review): `jira_update_issue_comment`'s `Expand` input field is accepted but not forwarded to Jira on the wire (the shared `PutJSON` client helper has no query-parameter support). Code review flagged this as Low/non-blocking since it's documented at every layer (struct comment, jsonschema description, `docs/tools/jira.md`). User explicitly confirmed "ship as-is" over adding a `PutJSONQuery` client method or removing the field — accepted as a known, documented limitation, not fixed in this plan.

## Open questions

None outstanding — all flagged judgment calls (watchers granularity, assignee input shape and empty-value semantics, attachment multi-file, JQL search tool count, bulk-create partial-failure reporting, post-mutation read-back triad scope) were resolved with the user via explicit question+options on 2026-08-04 and are recorded above.

## Validation

- Focused proof (per group, mirroring `internal/jira/tools/tools_test.go` and `internal/jira/client/client_test.go` with `httptest.NewServer`): for each new tool assert HTTP method + path + query/body + envelope success/error/data shape; assert every business tool makes **zero** network calls before `jira_authenticate` (call-counter closure, as existing pre-auth tests do). Client tests: DELETE-with-query round-trip, POST-with-query round-trip, multipart round-trip asserting `multipart/form-data`, field name `file`, and `X-Atlassian-Token: nocheck` header (mirror bitbucket `client_test.go` 220-246). Watcher-body test asserts literal `"bob"` and assignee-unassign test asserts literal `{"name":""}`. Attachment test asserts 200 array unwrapping. Bulk-create test asserts `success:true` with a non-empty `errors` array still surfaced in `data` on partial failure. For every † tool, a read-back test asserts `ReturnIssue:true` triggers a follow-up `GetIssue`-equivalent call and that a failed refresh still returns `mutationApplied:true` without failing the tool (mirroring existing `refreshAfterMutation` tests). Extend `TestJiraToolDefinitionsHaveSecurityAnnotations` to assert all 24 names present and each new tool's RO/ADD/DES annotation.
- Integration/end-to-end proof: none automated (no live Jira 6.4.14 in CI); the `httptest` handlers stand in for upstream contract shape per the spec rows.
- Repository-required checks (Windows/PowerShell, from repo root `D:\Source Code\atlassian-mcp`):
  - `go test ./...`
  - `go build ./cmd/atlassian-mcp`
  - `go vet ./internal/jira/...`

## Result

Implemented and shipped all 19 planned tools; the Jira MCP module now registers 24 tools total (5 original + 19 new), confirmed by `len(defs) == 24` in `TestJiraToolDefinitionsHaveSecurityAnnotations`.

Delivered across five sequential implementation passes (Group 0/0b prerequisite, then A+B, C+D, E+F, G+H), each independently validated with `go build ./...`, `go test ./internal/jira/...`, `go vet ./internal/jira/...` before proceeding to the next. Final full-repo validation: `go build ./...`, `go build ./cmd/atlassian-mcp`, `go test ./...`, `go vet ./...` — all green, 115/115 tests passing in `internal/jira/...`.

Independent code review (`code_reviewer`) found zero blocking findings. It directly re-verified — against code and real wire-shape test assertions, not implementer self-reports — every decision this plan recorded: D-H (assignee empty-body is `{"name":""}` not `null`), D-I (bulk-create partial failure still reports `success:true`), D-W (watcher add is a bare JSON string, watcher remove uses a query param), the attachment/issueLink path-scoping outside `/issue/...`, the †-tool read-back triad and its validation guard on all 9 applicable tools, `requireCredential`-first ordering on all 19 new handlers, and consistent `observability.Redact` usage. It also confirmed the `doJSON`→`do` client refactor is behavior-preserving for the 5 pre-existing tools.

One low-severity, non-blocking finding (CR-001: `jira_update_issue_comment`'s `Expand` input is accepted but not forwarded to Jira, since `PutJSON` has no query-parameter support) was surfaced, fully documented in code/docs, and explicitly accepted as-is by the user rather than fixed (see decision D-CR001).

**Limitations / follow-ups not built (all deliberate, recorded in Scope/Decisions):**
- No live Jira 6.4.14 integration test — all upstream contract shapes are modeled from the spec via `httptest`, not verified against a real server.
- `jira_add_issue_attachment` supports one file per call, not Jira's repeated-`-F file=@...` multi-file capability (user's explicit choice).
- `jira_update_issue_comment`'s `Expand` parameter is a documented no-op (D-CR001).
- Section 6 metadata endpoints (createmeta/editmeta) remain out of scope — callers must supply valid `fields` themselves for `jira_create_issue`/`jira_bulk_create_issues`.
- `jira_create_issue`/`jira_bulk_create_issues` have no post-create read-back option (no pre-existing issue to anchor to).

Docs updated: `docs/tools/jira.md` now documents all 24 tools. No `docs/decisions/` entry was created — this is additive tool-surface work following existing auth/session/versioning decisions (0002/0004/0005), not a new lasting architectural decision.

## Key Files

Implementer will touch (all relative to repo root):

- `internal/jira/client/client.go` (Group 0 primitives + refactor)
- `internal/jira/client/client_test.go` (new client tests)
- `internal/jira/tools/service.go` (19 handlers + input structs + `cleanPathSegment`)
- `internal/jira/tools/register.go` (19 defs + 19 AddTool blocks)
- `internal/jira/tools/tools_test.go` (per-tool tests + roster-test extension)
- `docs/tools/jira.md` (19 rows)

Reference-only (no change): `internal/bitbucket/client/request.go` (multipart analog), `internal/jira/module.go` / `config.go` (registration already single-unit), `internal/result/envelope.go`, `internal/observability/redact.go`.
