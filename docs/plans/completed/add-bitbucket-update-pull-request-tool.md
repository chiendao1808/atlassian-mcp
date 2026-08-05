# Execution Plan: Bitbucket `bitbucket_update_pull_request` tool (27th Bitbucket tool)

Date: 2026-08-05

## Status

Planned and fully approved by the user (2026-08-05), including every judgment
call the planner flagged. Awaiting `implementer` dispatch.

## Outcome

The Bitbucket MCP module exposes 27 tools total (the existing 26 plus one new
`bitbucket_update_pull_request`). The new tool updates an existing PR's `title`,
`description`, and `reviewers` via `PUT
/rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/pull-requests/{pullRequestId}`,
using **auto-preserve** semantics: before the PUT it issues one GET to read the
current `version` plus current `title`/`description`/`reviewers`; any field the
caller omits (nil pointer) is preserved from that GET (reviewers normalized to
the minimal `{"user":{"name":...}}` write shape, not echoed verbatim — see D-5),
any field the caller supplies (including an explicitly empty `reviewers` array
= "clear all") is applied as given. `version` is never caller-managed — it is
always sourced fresh from the pre-PUT GET to satisfy optimistic locking, and a
missing `version` in that GET fails validation before any PUT is issued. On a
409 from the PUT the tool surfaces the conflict through the existing Bitbucket
HTTP-error path and never blind-retries. The tool authenticates/authorizes
exactly like sibling PR-mutation tools, carries correct MCP annotations, has
unit-test coverage mirroring existing patterns, and is documented in the
implementation guide and SPECS (all genuine "26 tool" count references updated
to 27). `go build ./...`, `go build ./cmd/atlassian-mcp`, `go test ./...`, and
`go vet ./...` must pass on Windows.

## Context

- Spec row (authoritative endpoint contract):
  `docs/specs/jira-6.4.14_bitbucket-5.10.2-rest-api-reference.md:380` — "Cập nhật
  pull request", `PUT .../pull-requests/{pullRequestId}`, body
  `{"version":1,"title":"...","description":"...","reviewers":[{"user":{"name":"charlie"}}]}`,
  success 200 returns the updated PR with a bumped `version`.
- Optimistic-locking notes: same file lines 395-404 — on 409, re-read the
  resource, evaluate, and retry deliberately; do not blind-overwrite the new
  version if business state has changed.
- Reusable client/service plumbing:
  - `internal/bitbucket/tools/service.go:62-72` — generic `putJSON(ctx, tool,
    slug, path, q, body, key)` already exists; it resolves via `endpoint`
    (service.go:97-112) and issues `http.MethodPut` through `client.DoJSON`. No
    new client method is required.
  - `internal/bitbucket/tools/service.go:179-189` — `clientError` maps a
    `*bbclient.HTTPError` to `result.FailHTTPDetail(...)` preserving upstream
    `code`, `message`, `StatusCode`, and `Detail`. This is the "surface the
    conflict, let the caller decide" path.
- PR handler precedent: `internal/bitbucket/tools/pull_requests.go`:
  - `reviewerInput` (55-60), `createPRInput` (62-70) — reviewer/body shapes to
    reuse.
  - `transition()` (199-217) — the direct precedent for "auto-fetch version via
    `getPR` when caller omits it, early-return if the GET fails, then mutate."
    `transition()` issues a query-string POST with **no** body and is therefore
    structurally different from the body-based PUT this tool needs — do NOT call
    `transition()`; write a parallel handler that fetches then PUTs a JSON body.
  - `getPR()` (219-224) — guards `id <= 0` and slug (via `endpoint`), then GETs
    the PR; reuse it directly for the pre-PUT fetch.
  - `prPath()` (226-229) — builds `pull-requests/{id}`; the update PUT targets
    exactly `prPath(in.PullRequestID)` with no trailing segment.
  - Map-extraction idiom from `transition()` line 207:
    `env.Data.(map[string]any)["pullRequest"].(map[string]any)` then
    `pr["version"].(float64)`.
- Registration: `internal/bitbucket/tools/register.go` — `Definitions()`
  (10-64) builds a 26-entry `names` slice (indices 0-25);
  `registerPullRequestTools` (121-161) wires `defs[13]..defs[25]`. Annotation
  precedent in the same slice: create / add-comment / set-review-status use
  `&additive` (destructiveHint=false); merge / decline / reopen use
  `&destructive` (destructiveHint=true).
- Roster test (**correction to briefed evidence — see Verified Technical
  Assumptions**): `internal/bitbucket/tools/tools_test.go:19-62`
  `TestDefinitionsExposeExactlyTheBitbucketToolSet` hardcodes the exact 26-name
  `want` slice and asserts `slices.Equal(got, want)` plus every def having
  `OpenWorldHint`, and fixed annotation indices `defs[0]`/`defs[3]`/`defs[12]`.
  This test **will fail** when a 27th tool is added unless its `want` slice gains
  the new name.
- Test patterns to mirror (same file):
  `TestNestedDiffPathAndPullRequestTransitionVersion` (93-120, request-log
  assertions for auto-fetch-then-mutate sequencing and exact method/path/query)
  and `TestCreatePullRequestIncludesProjectKeyInRefs` (122-155, decodes the write
  body into `map[string]any` and asserts nested shape). Helper
  `newTestService(server.URL, server.Client())` (15-17).
- Docs:
  - `docs/specs/bitbucket-tool-implementation-guide.md` — per-tool sections
    through `### 3.26 bitbucket_reopen_pull_request` (505-521); a new
    `### 3.27 bitbucket_update_pull_request` must be authored in the same format
    before `## 4. Mandatory real-host compatibility gates` (523). Count
    assertions live at line 5 ("all 26 Bitbucket MCP tools") and line 537
    ("Exactly 26 tool sections exist").
  - `docs/specs/SPECS.md` — the enumeration in `### 10.3 Exact 26-tool registry
    and API links` (706-733; PR rows 721-733) needs a new row; count references
    at lines 43 and 94 (task-authorized) plus additional "26" tool-count
    mentions elsewhere (see Open Questions).
- No `docs/decisions/*.md` covers Bitbucket PR write conventions. This plan does
  **not** add a new ADR — it is additive tool-surface work following the existing
  auto-fetch-then-mutate convention already shipped by `transition()`. Decision
  D-1 below is sufficient provenance for future tool authors.

## Scope

In scope:

- One new input struct `updatePRInput` and one new handler
  `Service.UpdatePullRequest` in `internal/bitbucket/tools/pull_requests.go`.
- One new `names` entry (`defs[26]`) and one new `mcp.AddTool` block in
  `internal/bitbucket/tools/register.go`.
- Updating `TestDefinitionsExposeExactlyTheBitbucketToolSet` (add the 27th name)
  and adding focused behavior tests in
  `internal/bitbucket/tools/tools_test.go`.
- New `### 3.27` section in `docs/specs/bitbucket-tool-implementation-guide.md`
  and a new enumeration row + count updates in `docs/specs/SPECS.md`.

Out of scope:

- Any new client method or HTTP primitive (`putJSON` is reused as-is).
- PR delete (`DELETE .../pull-requests/{id}` with `{"version":n}` body) — a
  separate destructive tool, not requested here.
- PR comment edit/delete, watch/unwatch — separate spec rows, not requested.
- Reviewer-object normalization / user-existence validation (reviewers are
  passed through as given / preserved verbatim; upstream validates).
- Any change to `Register()`, module wiring, config, or env vars.
- A `docs/decisions/` ADR (see Context).

## Approach

Land one coherent additive change. The handler follows the established
auto-fetch-then-mutate shape used by `transition()` but issues a JSON-body PUT
instead of a query-string POST. `implementer` performs all writes; user approval
gates before any code.

### 1. Input struct (`internal/bitbucket/tools/pull_requests.go`)

Add alongside the other PR input structs. Pointer types make "omitted"
distinguishable from "supplied empty"; `RepositorySlug`/`PullRequestID` stay
plain required fields matching every other PR tool. The struct uses plain `json`
tags only — this package documents fields via Go doc comments, not `jsonschema`
struct tags (verified: `grep jsonschema internal/bitbucket/tools` returns no
matches).

```go
// updatePRInput updates an existing pull request's editable metadata using
// auto-preserve semantics. Any pointer field left nil is preserved from the
// current PR (fetched via one GET immediately before the PUT); a non-nil field
// overrides. Reviewers is a pointer-to-slice so a nil pointer ("leave reviewers
// untouched") is distinguishable from a non-nil empty slice ("clear all
// reviewers"). The optimistic-locking version is never supplied by the caller —
// it is always read fresh from the pre-PUT GET.
type updatePRInput struct {
	RepositorySlug string           `json:"repositorySlug"`
	PullRequestID  int              `json:"pullRequestId"`
	Title          *string          `json:"title,omitempty"`
	Description    *string          `json:"description,omitempty"`
	Reviewers      *[]reviewerInput `json:"reviewers,omitempty"`
}
```

### 2. Handler (`internal/bitbucket/tools/pull_requests.go`)

```go
func (s *Service) UpdatePullRequest(ctx context.Context, in updatePRInput) result.Envelope {
	tool := "bitbucket_update_pull_request"
	// Auto-fetch the current PR: getPR guards pullRequestId<=0 and (via
	// endpoint) repositorySlug, and gives us the fresh version plus the current
	// title/description/reviewers to preserve. Propagate a failed GET verbatim,
	// exactly like transition() does (pull_requests.go:203-206).
	env := s.getPR(ctx, tool, in.RepositorySlug, in.PullRequestID, "pullRequest")
	if !env.Success {
		return env
	}
	pr := env.Data.(map[string]any)["pullRequest"].(map[string]any)

	body := map[string]any{}
	// version is always sourced fresh from the GET (never caller-supplied).
	if raw, ok := pr["version"].(float64); ok {
		body["version"] = int(raw)
	}
	// title: caller override, else preserve current.
	if in.Title != nil {
		body["title"] = *in.Title
	} else if v, ok := pr["title"]; ok {
		body["title"] = v
	}
	// description: caller override, else preserve current when present.
	if in.Description != nil {
		body["description"] = *in.Description
	} else if v, ok := pr["description"]; ok {
		body["description"] = v
	}
	// reviewers: caller override (non-nil, incl. empty slice = clear all), else
	// preserve the current reviewer set, normalized to the write shape
	// ({"user":{"name":...}}) rather than echoed as the full GET participant
	// object (which also carries role/approved/lastReviewedCommit and is not
	// guaranteed accepted on write).
	if in.Reviewers != nil {
		body["reviewers"] = *in.Reviewers
	} else {
		body["reviewers"] = normalizeReviewers(pr["reviewers"])
	}

	return s.putJSON(ctx, tool, in.RepositorySlug, prPath(in.PullRequestID), nil, body, "pullRequest")
}

// normalizeReviewers reduces GET's full participant objects
// ({"user":{"name":...},"role":"REVIEWER","approved":...,...}) to the minimal
// write shape Bitbucket's PUT expects ({"user":{"name":...}}), dropping
// read-only fields the write endpoint does not accept.
func normalizeReviewers(raw any) []map[string]any {
	list, _ := raw.([]any)
	out := make([]map[string]any, 0, len(list))
	for _, item := range list {
		participant, ok := item.(map[string]any)
		if !ok {
			continue
		}
		user, ok := participant["user"].(map[string]any)
		if !ok {
			continue
		}
		name, ok := user["name"].(string)
		if !ok || name == "" {
			continue
		}
		out = append(out, map[string]any{"user": map[string]any{"name": name}})
	}
	return out
}
```

Notes:
- If the GET response omits `version` (should never happen for a real PR), the
  handler returns `fail(tool, "current PR version could not be read")` before
  issuing any PUT, mirroring `transition()` lines 213-215 (resolved OQ-3).
- `putJSON` already wraps success as `result.OK("bitbucket", tool,
  {"pullRequest": <updated PR>, "repositorySlug": slug})`, matching sibling PR
  tools' envelope keying — no custom result assembly needed.
- `normalizeReviewers` returns an empty (non-nil) slice, not nil, when the
  current PR has no reviewers or the GET shape is unexpected — so the PUT body
  always carries an explicit `reviewers` key, never a missing one.

### 3. Error handling

- **Initial GET fails** (auth/404/network): `getPR` returns a non-success
  envelope; the handler returns it unchanged (`if !env.Success { return env }`),
  identical to `transition()` at pull_requests.go:204-206. No PUT is issued.
- **PUT returns 409** (stale version / concurrent edit): `putJSON` →
  `client.DoJSON` returns a `*bbclient.HTTPError`; `clientError`
  (service.go:179-189) maps it to `result.FailHTTPDetail("bitbucket", tool,
  code, message, 409, detail)`, preserving upstream status and detail. The
  handler returns that failure — **no auto-retry**, honoring "never
  blind-overwrite". The caller re-reads and decides. We deliberately do **not**
  add a special conflict code (unlike `bitbucket_commit_file`, which maps its 409
  to `BITBUCKET_COMMIT_FILE_CONFLICT` at service.go:183-185); the generic
  `FailHTTPDetail` already conveys status 409 + detail. See OQ-2.

### 4. MCP annotation — decision: ADD (additive, destructiveHint=false)

`bitbucket_update_pull_request` edits mutable metadata (title/description/
reviewers) and destroys no history or state. It matches how the additive
mutators in the same registry are annotated:
`bitbucket_create_pull_request` (register.go:45),
`bitbucket_add_pull_request_comment` (46), and
`bitbucket_set_pull_request_review_status` (47) all use `&additive`
(destructiveHint=false). The `&destructive` annotation is reserved for
irreversible state transitions — merge (48), decline (49), reopen (50) — and file
overwrite (`commit_file`, 35). The guide corroborates this: the comment tool
(§3.22, line 443) and review-status tool (§3.23, line 462) declare
`destructiveHint=false`, while merge/decline/reopen (§3.24-3.26) declare
`destructiveHint=true`. An update is closer to create/comment than to
merge/decline. **Therefore: ReadOnlyHint=false, DestructiveHint=&additive
(false), OpenWorldHint=true** — i.e. use `&additive` in the `names` entry.

### 5. Registration wiring (`internal/bitbucket/tools/register.go`)

- Append one entry to the `names` slice, immediately after the
  `bitbucket_reopen_pull_request` line (currently line 50):

```go
{"bitbucket_update_pull_request", "Update one Bitbucket pull request's title, description, and reviewers; omitted fields are preserved and the version is resolved automatically.", false, &additive},
```

- Append one block to `registerPullRequestTools`, after the `defs[25]` (reopen)
  block (currently ends line 160):

```go
mcp.AddTool(server, defs[26], func(ctx context.Context, req *mcp.CallToolRequest, input updatePRInput) (*mcp.CallToolResult, result.Envelope, error) {
	return nil, s.UpdatePullRequest(ctx, input), nil
})
```

No change to `Register()`, `registerBranchTools`, or `registerCommitTools`. The
`defs[26].OutputSchema = result.MustOutputSchema()` assignment is already handled
by the `for _, def := range defs` loop in `Register()` (register.go:67-70).

### 6. Test plan (`internal/bitbucket/tools/tools_test.go`)

Reuse `newTestService` + `httptest.NewServer` with a request log (mirroring
`TestNestedDiffPathAndPullRequestTransitionVersion`). Add:

1. **`TestUpdatePullRequestAutoPreservesOmittedFields`** — server GET
   `/pull-requests/9` returns
   `{"id":9,"version":4,"title":"old","description":"olddesc","reviewers":[{"user":{"name":"charlie","slug":"charlie","displayName":"Charlie C"},"role":"REVIEWER","approved":true,"lastReviewedCommit":"abc123"}]}`.
   Call `UpdatePullRequest` with only `Title` set (a `*string`). Decode the PUT
   body and assert: `version == 4` (from the GET), `title == "new"` (override),
   `description == "olddesc"` (preserved), `reviewers == [{"user":{"name":"charlie"}}]`
   — i.e. **normalized** to the minimal write shape, with `slug`/`displayName`/
   `role`/`approved`/`lastReviewedCommit` stripped, not echoed verbatim. Assert
   the request sequence is exactly `GET .../pull-requests/9` then
   `PUT .../pull-requests/9` (method PUT, exact path, no query string).
2. **`TestNormalizeReviewersDropsReadOnlyFields`** — unit test for
   `normalizeReviewers` directly: given a list of full participant objects
   (including one with a missing/empty `user.name`, which must be skipped) and
   given `nil`/non-array input (must return an empty, non-nil slice), assert
   the minimal `{"user":{"name":...}}`-only output.
3. **`TestUpdatePullRequestClearsReviewersWithEmptySlice`** — pass
   `Reviewers: &[]reviewerInput{}`. Assert the PUT body contains a `reviewers`
   key whose value is an empty array (present, not omitted) — proving
   empty-slice = "clear all" is distinguishable from nil = "preserve".
4. **`TestUpdatePullRequestPropagatesGetFailure`** — GET returns HTTP 404. Assert
   the result is a failure and the request log contains only the GET (no PUT).
5. **`TestUpdatePullRequestPropagatesConflictWithoutRetry`** — GET succeeds, PUT
   returns HTTP 409. Assert the result is a failure surfacing status 409, and the
   request log contains exactly one PUT (no retry).
6. **Update `TestDefinitionsExposeExactlyTheBitbucketToolSet`** — append
   `"bitbucket_update_pull_request"` to the `want` slice (after
   `"bitbucket_reopen_pull_request"`), so `slices.Equal(got, want)` passes with
   27 names. The existing `defs[0]`/`defs[3]`/`defs[12]` annotation-index checks
   remain valid (the new tool is appended at index 26). Optionally assert
   `defs[26].Annotations.DestructiveHint != nil &&
   !*defs[26].Annotations.DestructiveHint` to lock the additive choice.

Helper note: a local `strPtr(v string) *string { return &v }` may be added
(analogous to the existing `intPtr`, tools_test.go:157) for the `*string` inputs.

### 7. Docs

**`docs/specs/bitbucket-tool-implementation-guide.md`** — insert a new section
between `### 3.26` (ends ~line 521) and `## 4.` (line 523), mirroring 3.24-3.26:

```
<a id="tool-bitbucket_update_pull_request"></a>
### 3.27 `bitbucket_update_pull_request`

- **MCP purpose:** Update an open pull request's title, description, and reviewers using optimistic locking with auto-preserved omitted fields.
- **Source:** [bundled exact-resource anchor](../references/jira-6.4.14_bitbucket-5.10.2-rest-api-reference.md#bb-api-update-pull-request); [official Bitbucket Server 5.10.2 REST](https://docs.atlassian.com/bitbucket-server/rest/5.10.2/bitbucket-rest.html), resource heading `PUT /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/pull-requests/{pullRequestId}`.
- **Method/path:** `PUT /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/pull-requests/{pullRequestId}`.
- **Query/path inputs:** None. Required resolved `version` is sourced from one immediate GET; callers never pass a version.
- **Request headers:** `Authorization: Bearer …`; `Content-Type: application/json`; `Accept: application/json`.
- **Request body:** JSON `{version, title, description, reviewers}`. `version` is always the value read from the pre-PUT GET; if that GET yields no `version`, the tool fails validation before issuing any PUT. Any of `title`/`description`/`reviewers` the caller omits is preserved from the GET; a supplied value overrides. A supplied empty `reviewers` array clears all reviewers; an omitted `reviewers` preserves the current reviewer set — but the preserved reviewers are **normalized** to `{"user":{"name": identity}}` (read-only fields such as `slug`, `displayName`, `role`, `approved`, `lastReviewedCommit` are stripped), never echoed as the full GET participant object.
- **Expected success:** 200 JSON updated pull request with a bumped `version`.
- **Response preservation:** Preserve the updated PR object, its new `version`, title, description, reviewers, and state metadata returned upstream.
- **Permission concept:** PR author or equivalent write permission on the repository.
- **Error mapping / stop conditions:** A 409 means a stale version or concurrent edit; surface it via the shared HTTP-error path with upstream detail and never replay the PUT. A failed pre-PUT GET is returned unchanged and no PUT is issued.
- **Retry behavior:** No blind retry. Exactly one GET followed by at most one PUT; on 409 the caller re-reads and decides.
- **Response-size / truncation:** Apply the shared bounded-reader policy. Do not silently truncate a JSON object; return the shared response-too-large error when a complete bounded representation cannot be produced.
- **MCP annotations:** `readOnlyHint=false; destructiveHint=false; idempotentHint=false`; mutation tools require client approval under the project policy.
- **Required tests:** Auto-preserve omitted title/description/reviewers with normalization of preserved reviewers to `{"user":{"name":...}}`; explicit empty reviewers clears all; version taken from GET (missing version fails validation, no PUT issued); failed GET propagates with no PUT; 409 surfaced without retry; exactly one PUT; additive annotation.
- **Test layers:** unit validation/serialization; request-recording HTTP contract; MCP schema/annotation snapshot; real Bitbucket 5.10.2 host where this section declares a staging gate.
```

Also update the two count assertions in the same guide: line 5 ("all 26
Bitbucket MCP tools" → "all 27 Bitbucket MCP tools") and line 537 ("Exactly 26
tool sections exist." → "Exactly 27 tool sections exist.").

**`docs/specs/SPECS.md`** —

- Append one enumeration row to `### 10.3` after the reopen row (line 733):

```
- [`bitbucket_update_pull_request`](bitbucket-tool-implementation-guide.md#tool-bitbucket_update_pull_request) — `PUT /projects/{projectKey}/repos/{repositorySlug}/pull-requests/{pullRequestId}`; 200 JSON updated pull request with bumped version.; PR author or equivalent repository write permission.
```

- Update the `### 10.3 Exact 26-tool registry and API links` heading (line 706)
  to `### 10.3 Exact 27-tool registry and API links`.
- Update the task-authorized count references at line 43 ("26 Bitbucket …
  tools" → 27) and line 94 ("The 26 Bitbucket … tools" → 27).
- Additional tool-count "26" references exist elsewhere in SPECS.md — see Open
  Questions (OQ-1) for the scope decision on those.

## Verified Technical Assumptions

Checked directly against the current source on 2026-08-05:

- **CORRECTION to briefed evidence.** The task brief stated "no roster/
  annotation-count test exists for Bitbucket … so no such test needs updating."
  That is inaccurate: `internal/bitbucket/tools/tools_test.go:19-62`
  `TestDefinitionsExposeExactlyTheBitbucketToolSet` hardcodes the exact 26-name
  `want` slice and asserts `slices.Equal(got, want)`. Adding a 27th tool without
  extending `want` **will fail `go test`**. Updating this test is therefore a
  required, non-optional part of the change (Test plan item 5).
- `internal/bitbucket/tools/service.go:62-72` `putJSON` exists and issues
  `http.MethodPut` with a JSON body and optional query — reusable directly; no
  new client method needed.
- `internal/bitbucket/tools/pull_requests.go:219-224` `getPR` guards `id <= 0`
  and (via `endpoint`→`fail`) the slug, returning a keyed `pullRequest`
  envelope — reusable directly for the pre-PUT fetch.
- `internal/bitbucket/tools/pull_requests.go:199-217` `transition()` confirms the
  exact auto-fetch-then-mutate idiom (version read via `getPR`, early-return on
  failed GET, `env.Data.(map[string]any)["pullRequest"].(map[string]any)`,
  `pr["version"].(float64)`) — this handler reuses that idiom but PUTs a body
  instead of POSTing a query string.
- `internal/bitbucket/tools/service.go:179-189` `clientError` maps
  `*bbclient.HTTPError` to `FailHTTPDetail` preserving status/detail — the 409
  path needs no new code.
- `internal/bitbucket/tools/register.go:10-64` `Definitions()` builds 26 entries
  (indices 0-25); `registerPullRequestTools` wires `defs[13]..defs[25]`. Adding
  `names[26]`/`defs[26]` plus one `AddTool` block is the complete registration
  delta; `Register()` needs no change.
- `grep jsonschema internal/bitbucket/tools` returns no matches — this package
  documents input fields with Go doc comments, not `jsonschema` struct tags, so
  the new struct uses plain `json` tags plus a doc comment (matches
  `createPRInput`/`reviewerInput`).
- Annotation precedent verified in `register.go` and the guide: create/comment/
  review-status are additive (destructiveHint=false); merge/decline/reopen and
  commit_file are destructive (destructiveHint=true). Update belongs with the
  additive group (§4 above).

## Risks And Recovery

- **Reviewer preservation wire-shape (resolved, user-confirmed 2026-08-05).**
  Bitbucket's GET returns full participant objects
  (`{"user":{"name":...,"slug":...,...},"role":"REVIEWER","approved":...,"lastReviewedCommit":...}`),
  while the update endpoint's documented request shape is the minimal
  `{"user":{"name":...}}`. Rather than risk the host rejecting (or silently
  misinterpreting) the extra read-only fields on write, `normalizeReviewers`
  (§2) strips every preserved reviewer down to `{"user":{"name":...}}` before
  the PUT. This removes the prior verbatim-echo assumption entirely — no
  staging-host confirmation is needed for this behavior, since the tool now
  only ever sends the shape the spec documents as valid input. Residual risk:
  if the real 5.10.2 host expects an additional required sub-field beyond
  `name` on write (not documented in the spec row), the PUT would fail
  upstream; the caller can still pass `reviewers` explicitly to work around it.
- **409 on PUT.** By design the tool surfaces the conflict and does not retry;
  the caller re-reads and retries deliberately. A test asserts exactly one PUT
  and a surfaced 409. Recovery is the caller's re-read loop, per spec lines
  395-404.
- **`version` absence in GET.** Optional defensive guard (§2 note / OQ-3) returns
  a validation failure rather than issuing a version-less PUT; low likelihood for
  a real PR.
- **Doc count drift.** Several "26" tool-count assertions exist across SPECS.md
  and the guide beyond the two lines the task authorized; leaving some updated
  and others stale creates internal contradiction. Handled as OQ-1 so the user
  sets the scope rather than the planner deciding silently.

Recovery/rollback: the change is purely additive (one struct, one handler, one
`names` entry + one `AddTool` block, test additions, doc additions). Reverting
the commit restores the 26-tool state; no data migration, config, persisted
state, or client-contract change is involved.

## Progress

- [x] Add `updatePRInput` struct + `Service.UpdatePullRequest` handler in
      `internal/bitbucket/tools/pull_requests.go` (auto-preserve merge, version
      from GET, JSON-body PUT via `putJSON`).
- [x] Register: append `names[26]` (additive annotation) + `defs[26]` `AddTool`
      block in `internal/bitbucket/tools/register.go`.
- [x] Update `TestDefinitionsExposeExactlyTheBitbucketToolSet` `want` slice to 27
      names (REQUIRED — corrects briefed evidence).
- [x] Add behavior tests: auto-preserve omitted fields; empty-reviewers clears
      all; version-from-GET; failed-GET propagation (no PUT); 409-without-retry;
      exactly-one-PUT with exact method/path.
- [x] Add `### 3.27 bitbucket_update_pull_request` to
      `docs/specs/bitbucket-tool-implementation-guide.md`; update its count
      assertions at lines 5 and 537.
- [x] Add the SPECS.md `### 10.3` enumeration row; update the 10.3 heading and
      all genuine "26"-tool-count references (43, 94, 706 heading, 1275, 1290,
      1306, 1698, 1763, 1987, 2039) to 27 per resolved OQ-1, leaving the two
      task-number references (1711, 2045) unchanged.
- [x] Validation: `go build ./...`, `go build ./cmd/atlassian-mcp`,
      `go test ./...`, `go vet ./...` all green on Windows.

## Decisions

- **D-1 (auto-preserve merge, user-confirmed 2026-08-05).** The handler
  auto-fetches the current PR (GET) before the PUT and merges caller input over
  it: fields the caller omits (nil pointer) are preserved from the GET; fields
  the caller supplies override; an explicitly empty `reviewers` array clears
  all reviewers. Preserved reviewers are **normalized** to `{"user":{"name":...}}`
  (via `normalizeReviewers`, §2), not echoed as the full GET participant object
  — see D-5. `version` is always sourced fresh from the pre-PUT GET, never from
  the caller; if the GET yields no `version`, the handler fails fast rather than
  issuing a version-less PUT. Input types: `Title *string`, `Description
  *string`, `Reviewers *[]reviewerInput` (pointer-to-slice so nil="untouched" is
  distinguishable from empty-slice="clear all"); `RepositorySlug` and
  `PullRequestID` remain required plain fields. Binding; not to be re-litigated.
- **D-2 (annotation = additive).** `destructiveHint=false` (`&additive`),
  matching `bitbucket_create_pull_request`/`_add_..._comment`/
  `_set_..._review_status`; an update edits mutable metadata and destroys no
  history, unlike merge/decline/reopen. (Rationale in §4.)
- **D-3 (no new ADR).** Additive tool-surface work following the existing
  auto-fetch-then-mutate convention; no `docs/decisions/` entry is created. D-1
  is the provenance record for future tool authors.
- **D-4 (generic 409 path, user-confirmed 2026-08-05).** Surface the PUT's 409
  through the shared `clientError`/`FailHTTPDetail` path with upstream detail;
  do not add a tool-specific conflict code.
- **D-5 (normalize preserved reviewers, user-confirmed 2026-08-05).** When
  `reviewers` is omitted, the handler does not echo GET's full participant
  objects verbatim; it reduces each to the documented write shape
  `{"user":{"name":...}}` via `normalizeReviewers`, dropping `slug`,
  `displayName`, `role`, `approved`, `lastReviewedCommit`, and any other
  read-only fields. Chosen over verbatim echo to avoid depending on an
  unverified assumption that Bitbucket 5.10.2 ignores extra fields on write.

## Open questions

None outstanding. OQ-1/OQ-2/OQ-3 (raised by `planner`) plus the reviewer
wire-shape question (surfaced as a Risk by `planner`, elevated to an explicit
question by the orchestrator) were all put to the user directly on 2026-08-05
and answered — every one matched the recommended option:

- **OQ-1 (user-confirmed → broader scope).** Update **all genuine tool-count
  assertions** to 27: SPECS.md lines 43, 94, the `### 10.3` heading (706), 1275,
  1290, 1306, 1698, 1763, 1987, 2039; guide lines 5 and 537. Leave the two
  task-number references alone (SPECS.md 1711 `### Task 26 — Documentation…`,
  2045 `Task 26: finish…`) — those are task numbers, not tool counts, and
  renumbering tasks is out of scope. Rationale: leaving some "26" tool-count
  references stale while others read 27 would make the spec internally
  contradictory, which is worse than the small extra edit surface.
- **OQ-2 (user-confirmed → D-4, generic 409 path).** No tool-specific conflict
  code; reuse the shared `FailHTTPDetail` path. Rationale: matches how most
  other PR tools surface upstream errors; a named code is easy to add later if
  a caller actually needs to branch on it, without breaking anything now.
- **OQ-3 (user-confirmed → include the defensive guard).** Add the
  `transition()`-style guard that fails fast with a clear validation error if
  the pre-PUT GET yields no `version`, instead of letting a version-less PUT
  fail upstream with a less clear error. Rationale: cheap, consistent with the
  existing `transition()` precedent, and gives a better error message.
- **OQ-4 / reviewer wire-shape (user-confirmed → D-5, normalize on write).**
  Preserved reviewers are reduced to `{"user":{"name":...}}` via
  `normalizeReviewers` rather than echoed as the full GET participant object.
  This removes the previously-flagged staging-host confirmation risk entirely,
  since the tool now only ever writes the shape the spec documents as valid.

The auto-preserve design (D-1) was user-confirmed on 2026-08-05 and was never
an open question.

## Validation

- Focused proof (per §6, `httptest.NewServer` + request log, mirroring
  `TestNestedDiffPathAndPullRequestTransitionVersion` and
  `TestCreatePullRequestIncludesProjectKeyInRefs`): assert GET-then-PUT
  sequencing, exact PUT method/path
  (`/rest/api/1.0/projects/PRJ/repos/{slug}/pull-requests/{id}`, no query),
  version sourced from the GET (missing version fails validation, no PUT
  issued), auto-preserve of omitted fields with reviewer normalization to
  `{"user":{"name":...}}`, empty-reviewers-clears-all, failed-GET propagation
  with no PUT, and 409 surfaced without retry. Extend
  `TestDefinitionsExposeExactlyTheBitbucketToolSet` to 27 names and assert the
  new tool's additive annotation.
- Integration/end-to-end proof: none automated (no live Bitbucket 5.10.2 in CI);
  the `httptest` handler stands in for the upstream contract per spec line 380.
  The prior reviewer-preservation wire-shape risk is resolved by design (D-5:
  normalize on write), so no staging-host confirmation is required for that
  specific behavior before shipping.
- Repository-required checks (Windows/PowerShell, from repo root
  `D:\Source Code\atlassian-mcp`):
  - `go build ./...`
  - `go build ./cmd/atlassian-mcp`
  - `go test ./...`
  - `go vet ./...`

## Result

Implemented exactly as planned. `internal/bitbucket/tools/pull_requests.go` gained
`updatePRInput`, `Service.UpdatePullRequest`, and `normalizeReviewers`, matching
the plan's verbatim code blocks with one intentional simplification: the
"missing `version` fails validation" guard (OQ-3) was folded directly into the
single `raw, ok := pr["version"].(float64); if !ok { return fail(...) }`
extraction rather than the plan's two-step
`if raw, ok := ...; ok { ... }` / separate nil-check-after pattern — behaviorally
identical (still fails fast before any PUT when `version` is absent) but one
branch shorter. `internal/bitbucket/tools/register.go` gained the `names[26]`
additive-annotated entry and the `defs[26]` `AddTool` block, unchanged from the
plan. No other deviations from the approved plan.

The Bitbucket MCP module now registers 27 tools total (26 existing + 1 new),
confirmed by `TestDefinitionsExposeExactlyTheBitbucketToolSet`'s 27-name `want`
slice plus a dedicated assertion that `defs[26]` carries the additive
(`destructiveHint=false`) annotation. Five new focused tests were added to
`internal/bitbucket/tools/tools_test.go`, all mirroring the existing
`httptest.NewServer` + request-log pattern:

- `TestUpdatePullRequestAutoPreservesOmittedFields` — GET returns full
  participant objects (with `slug`/`displayName`/`role`/`approved`/
  `lastReviewedCommit`); asserts the PUT carries `version` from the GET,
  `title` overridden, `description` preserved, and reviewers reduced to the
  minimal `{"user":{"name":"charlie"}}` shape (read-only fields stripped),
  with exactly one GET then one PUT in that order.
- `TestNormalizeReviewersDropsReadOnlyFields` — direct unit test of
  `normalizeReviewers`: strips read-only fields, skips an entry with an
  empty/missing `user.name`, and returns a non-nil empty slice for `nil` or a
  non-array input.
- `TestUpdatePullRequestClearsReviewersWithEmptySlice` — passing
  `Reviewers: &[]reviewerInput{}` produces a present-but-empty `reviewers` key
  in the PUT body, proving empty-slice ("clear all") is distinguishable from
  nil ("preserve").
- `TestUpdatePullRequestPropagatesGetFailure` — a 404 GET short-circuits with
  a failed envelope and exactly one request recorded (no PUT issued).
- `TestUpdatePullRequestPropagatesConflictWithoutRetry` — a 409 PUT surfaces
  as a failed envelope with `Error.HTTPCode == 409` and exactly one PUT
  recorded (no blind retry).

The added `strPtr` test helper mirrors the existing `intPtr` (tools_test.go).

Documentation was updated per §7: `docs/specs/bitbucket-tool-implementation-guide.md`
gained the new `### 3.27 bitbucket_update_pull_request` section (verbatim from
the plan) plus both count-assertion updates (line 5's "all 27 Bitbucket MCP
tools", the completion-scan's "Exactly 27 tool sections exist"). `docs/specs/SPECS.md`
gained the new enumeration row under `### 10.3` (now titled "Exact 27-tool
registry and API links") and every genuine tool-count "26" reference identified
by OQ-1 was updated to 27 (goal statement, task-authorization bullet, the 10.3
heading, and the remaining count references in the tasks/checklist/coverage
sections). The two task-number references ("Task 26 — Documentation…" and
"Task 26: finish…") were left unchanged, confirmed by a final grep showing only
those two `26` occurrences remain in `SPECS.md`.

One item outside the plan's explicit scope was noticed but deliberately left
untouched: `SPECS.md` contains two checklist lines referencing "Section 10.7"
("Register exactly the 27 names in Section 10.7." / "Bitbucket registry exposes
exactly the 27 tools listed in Section 10.7.") but no `### 10.7` section
actually exists in the document (the registry enumeration lives in `### 10.3`).
This is a pre-existing dangling cross-reference unrelated to this change — the
plan's line-number list did not include these lines for the count-word edit
they needed (26→27, which was applied), and fixing the stale section-number
cross-reference itself is a separate, out-of-scope documentation-consistency
issue not authorized by this plan.

Final validation, run from `D:\Source Code\atlassian-mcp` on Windows/PowerShell:

- `go build ./...` — passed, no output.
- `go build ./cmd/atlassian-mcp` — passed, no output.
- `go test ./...` — all packages pass, including
  `github.com/chiendao1808/atlassian-mcp/internal/bitbucket/tools`.
- `go vet ./...` — passed, no output.

Docs updated: `docs/specs/bitbucket-tool-implementation-guide.md` and
`docs/specs/SPECS.md` (see above). No `docs/decisions/` entry was created, per
D-3 — this is additive tool-surface work following the existing auto-fetch-
then-mutate convention already shipped by `transition()`, not a new lasting
architectural decision.

### Remediation: CR-001 (2026-08-05)

Independent code review of this change flagged one Low-severity issue,
**CR-001**: `normalizeReviewers` (`internal/bitbucket/tools/pull_requests.go`)
ran only on the "caller omitted reviewers, preserve unchanged" branch of
`UpdatePullRequest`, yet it silently `continue`d (dropped) any participant
whose `user.name` wasn't a usable non-empty string (missing, wrong type, or
empty). That meant an update that never intended to touch reviewers could
silently remove a reviewer from the PR as a side effect, if that reviewer's
data didn't normalize cleanly — a data-loss risk on a path meant to be a
no-op for reviewers. The user confirmed "fix now" on 2026-08-05.

Fix applied: `normalizeReviewers` no longer drops an entry when
`user.name` fails to normalize. Instead it falls back to preserving the
original `participant["user"]` sub-object verbatim (whatever shape it has),
so the reviewer is never silently removed from an "untouched" update. An
entry is skipped entirely only when it isn't a participant object at all —
not a `map[string]any`, or with no `"user"` key present. The doc comment
above `normalizeReviewers` was rewritten to state this fallback rule
precisely, and `docs/specs/bitbucket-tool-implementation-guide.md` §3.27's
request-body description was extended to describe the same fallback
(previously silent on this edge case).

Test changes: `TestNormalizeReviewersDropsReadOnlyFields` was renamed to
`TestNormalizeReviewersStripsReadOnlyFieldsAndPreservesUnnormalizable` and
extended with cases for empty `user.name`, missing `user.name`, and
wrong-type `user.name` — each now asserted to appear in the output with its
original `user` sub-object preserved intact (not dropped, not reduced) —
plus a case confirming a participant with no `"user"` key at all is still
skipped, and the existing nil/non-array-input assertions are retained
unchanged. `TestUpdatePullRequestAutoPreservesOmittedFields` was reviewed
and required no change: its fixture reviewer has a valid non-empty
`user.name` ("charlie"), which still takes the clean-normalization branch
and produces the same minimal `{"user":{"name":"charlie"}}` output as
before.

Validation re-run after the fix (`go build ./...`, `go build
./cmd/atlassian-mcp`, `go test ./...`, `go vet ./...`) — all passed; see the
implementer's report for this remediation for exact output.

## Key Files

Implementer will touch (all relative to repo root):

- `internal/bitbucket/tools/pull_requests.go` (`updatePRInput` struct +
  `UpdatePullRequest` handler)
- `internal/bitbucket/tools/register.go` (`names[26]` entry + `defs[26]`
  `AddTool` block)
- `internal/bitbucket/tools/tools_test.go` (roster-test `want` slice extension +
  new behavior tests; optional `strPtr` helper)
- `docs/specs/bitbucket-tool-implementation-guide.md` (new §3.27; count
  assertions at lines 5 and 537)
- `docs/specs/SPECS.md` (new §10.3 enumeration row; count updates per OQ-1)

Reference-only (no change): `internal/bitbucket/tools/service.go` (`putJSON`,
`clientError`, `endpoint`, `query`/`q`), `internal/result/envelope.go`
(`OK`/`FailHTTPDetail`),
`docs/specs/jira-6.4.14_bitbucket-5.10.2-rest-api-reference.md` (spec rows 380,
395-404).
