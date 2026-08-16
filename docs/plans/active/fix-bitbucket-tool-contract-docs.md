# Execution Plan: Fix Bitbucket Tool Contract And Docs

Date: 2026-08-01 (revised 2026-08-16)

## Status

Active — Phase 1 Gap 1 and Gap 3 are implementation-ready now. Phase 1 Gap 2 is
blocked on open questions OQ-1/OQ-2 (user decision required). Phase 2 and
Phase 3 are scoped and ready pending the OQ-4 scope confirmation.

## Outcome

Bring the implemented Bitbucket MCP tools back into alignment with
`docs/specs/bitbucket-tool-implementation-guide.md` (the endpoint-level
authority) for three verified high-risk contract gaps, then close the remaining
verified secondary schema/query gaps and the grouped tool documentation, so
that all 27 registered Bitbucket tools serialize exactly the requests their
guide sections contract.

Observable results:

1. `bitbucket_compare_commits`, `bitbucket_compare_changes`, and
   `bitbucket_compare_diff` serialize MCP input `fromRepositorySlug` as query
   parameter `fromRepo={BITBUCKET_PROJECT_KEY}/{slug}` and never send a
   `fromRepositorySlug` query parameter or accept project/URL injection.
2. `bitbucket_commit_file` enforces the documented safety policy:
   `sourceCommitId` required for update, omitted for create; `sourceBranch`
   handling per the resolution of OQ-2. Exactly one PUT is ever sent.
3. `bitbucket_set_pull_request_review_status` sends the documented participant
   body `{"user":{"name": identity}, "approved": ..., "status": ...}`.
4. Secondary documented inputs (Phase 2) are exposed and serialized per guide
   sections 3.6, 3.8, 3.9, 3.14, 3.16, 3.17, 3.18, 3.19, and 3.22.
5. `docs/tools/bitbucket.md` documents all 27 tools; `README.md` links it.
6. `go build ./...`, `go vet ./...`, and `go test ./...` pass; the focused
   request-recording cases in the `verification_plan` below pass.

## Supplied-Plan Reference And Revision Summary

This revises the 2026-08-01 plan of the same name in place (planner policy:
preserve and revise, do not replace). Preserved decisions:

- Keep this as a separate active plan from `build-task-1-14.md` (Task 5
  foundation vs Tasks 6-11 tool contracts).
- Treat `docs/specs/bitbucket-tool-implementation-guide.md` as the
  endpoint-level authority; never resolve mismatches by weakening docs.
- Grouped documentation is acceptable only if every tool has an explicit entry
  and source link (tool count updated 26 → 27).
- The secondary schema/query coverage items from the old Approach section 3
  remain in scope (now Phase 2, with verified per-field detail).

What changed in the 2026-08-16 revision:

- All "26 tools" references updated to 27 (registry grew by
  `bitbucket_update_pull_request`; `register.go`, the roster test,
  `SPECS.md` §10.3, and the guide already say 27).
- The three high-risk gaps were re-verified against current source with exact
  line numbers and promoted into an implementation-ready Phase 1 with exact
  function signatures and test assertions.
- Dropped the scope item "update stale docs that describe Bitbucket tools as
  future work": verified moot on 2026-08-16 (no such language in `README.md`,
  `docs/tools/catalog.md`, `docs/claude-code.md`, or `docs/codex.md`;
  `catalog.md` already lists all 27 Bitbucket tools).
- Corrected the bundled API reference path: it lives at
  `docs/specs/jira-6.4.14_bitbucket-5.10.2-rest-api-reference.md`, not
  `docs/references/...` as the old Context list stated.
- 409 conflict mapping for `bitbucket_commit_file` verified as already
  SPECS-compliant at the code level (all 409 categories map to
  `BITBUCKET_COMMIT_FILE_CONFLICT` with sanitized detail preserved); only
  exception-name sub-classification is deferred to the real-host staging gate.
  No code change planned for the mapping itself.
- Added: Verified Current State, Open Questions (OQ-1..OQ-5),
  `verification_plan` YAML block, Compatibility Considerations, and Definition
  Of Done, per `.agents/agent_instructions/planner.md`.

## Context

- `AGENTS.md`
- `docs/WORKFLOW.md`
- `docs/specs/SPECS.md` section 10 (27-tool registry) and Tasks 6-11
  (Task 8 lines 1126-1173 is authoritative for `bitbucket_commit_file`
  create/update semantics)
- `docs/specs/bitbucket-tool-implementation-guide.md` (endpoint authority;
  §3.10-3.13, §3.23 for Phase 1; §3.6, §3.8, §3.9, §3.14, §3.16-3.19, §3.22
  for Phase 2; §4 mandatory real-host gates)
- `docs/specs/bitbucket-tool-test-matrix.md`
- `docs/specs/jira-6.4.14_bitbucket-5.10.2-rest-api-reference.md` (bundled
  endpoint table; line 392 confirms the review-status body shape
  `{"user":{"name":"alice"},"approved":true,"status":"APPROVED"}`)
- `docs/specs/bitbucket-api-reference-review.md` (stop conditions, including
  the identity stop condition)
- `internal/bitbucket/tools/` (handlers, registry, tests)
- `internal/bitbucket/client/` (HTTP client; NOT modified by this plan)
- `internal/bitbucket/config.go` (`BITBUCKET_USER_SLUG`)
- `README.md`, `docs/tools/catalog.md`, `docs/tools/jira.md` (doc conventions)
- `docs/plans/completed/build-bitbucket-tools.md`
- `docs/plans/completed/add-bitbucket-update-pull-request-tool.md` (27th tool;
  plan-convention precedent)

## Verified Current State (2026-08-16)

All line numbers verified against the working tree on 2026-08-16.

### Gap 1 — Compare tools send `fromRepositorySlug` instead of `fromRepo`

- Guide contract: §3.10 line 206 — "MCP input `fromRepositorySlug` serializes
  to `fromRepo={BITBUCKET_PROJECT_KEY}/{slug}`; never accept arbitrary project,
  numeric ID, URL, or raw `fromRepo`." §3.11 line 225 and §3.12 line 244 apply
  the same derivation to compare changes/diff.
- Current code `internal/bitbucket/tools/commits.go:187-189`:

  ```go
  func compareq(in compareInput) query {
      return q("from", in.From, "to", in.To, "fromRepositorySlug", in.FromRepositorySlug)
  }
  ```

  The MCP input name is emitted verbatim as the query parameter. Bitbucket
  5.10.2 does not define a `fromRepositorySlug` query parameter on the compare
  endpoints, so cross-repository comparison is silently broken.
- Affected handlers (all call `compareq`): `CompareCommits` (commits.go:131-136),
  `CompareChanges` (138-143), `CompareDiff` (145-150). Each also duplicates the
  same `from`/`to` required-validation block.
- Available primitive: `(*bbclient.Client).ProjectKey()` (client/client.go:49-51)
  returns the configured project key. No client change needed.
- No existing test covers compare query serialization
  (`internal/bitbucket/tools/tools_test.go` has no compare case).

### Gap 2 — `bitbucket_commit_file` safety policy not enforced

- Guide contract: §3.13 line 269 — "For update, require `sourceCommitId` under
  the project safety policy. For a new branch require `sourceBranch`. Map 409
  details separately for existing file, unchanged content, and stale source
  commit. Send one PUT only; no blind retry."
- SPECS Task 8 (lines 1147-1150): "Distinguish create and update semantics.
  Require `sourceCommitId` for update mode. Omit `sourceCommitId` for create
  mode. Require `sourceBranch` when a new branch is requested." Task 8 contract
  tests (lines 1160-1171) include "Existing file in create mode" and "New
  branch with and without `sourceBranch`" — the spec assumes explicit
  create/update modes.
- Current code `internal/bitbucket/tools/commits.go:152-185` (`CommitFile`) and
  input struct `commitFileInput` (67-76): `sourceCommitId` (line 74) and
  `sourceBranch` (line 75) are optional pass-through fields. There is no mode
  concept and no enforcement: an update without `sourceCommitId` is sent
  as-is, which can silently overwrite a concurrently changed file.
- 409 mapping status: `clientError` (service.go:179-189, special case at
  183-185) already maps every 409 on this tool to
  `BITBUCKET_COMMIT_FILE_CONFLICT` via `result.FailHTTPDetail`, preserving the
  sanitized upstream `Detail` (client/errors.go:29-43). This satisfies SPECS
  Task 8 line 1153 ("Map stale source commit, existing file, unchanged content,
  and other `409` details to `BITBUCKET_COMMIT_FILE_CONFLICT`"). The exact
  5.10.2 exception names per category are NOT documented in the guide or
  bundled reference, so sub-classification is a real-host staging gate
  (guide §4 line 552), not a code change in this plan.
- Multipart plumbing verified sufficient: `client.DoMultipart`
  (client/request.go:49-76) writes `fields` then the `content` file part and
  issues exactly one PUT (`do()` uses `attempts=1` for mutations,
  request.go:78-82). No client change needed.

### Gap 3 — Review-status body missing `user` member

- Guide contract: §3.23 line 455 — "JSON contains `user:{"name": identity}`,
  `approved` and `status`." Bundled reference line 392 confirms the same body
  shape. Guide §4 line 551 (staging gate): "Participant status: URL `userSlug`
  versus request `user.name`; do not release the tool until one configured
  identity works for both or the configuration contract is explicitly revised."
- Current code `internal/bitbucket/tools/pull_requests.go:184-194`, body built
  at line 192:

  ```go
  body := map[string]any{"status": status, "approved": status == "APPROVED"}
  ```

  The `user` member is absent. The URL path segment already uses
  `s.userSlug` (line 193, via `prPath(in.PullRequestID, "participants", s.userSlug)`),
  and the missing-identity guard exists (lines 185-187,
  `BITBUCKET_REVIEW_IDENTITY_REQUIRED`).
- Identity source: `Service.userSlug` (service.go:16-24), populated from
  `BITBUCKET_USER_SLUG` (config.go:16,24). No other identity is configured.
- No existing test covers the review-status request body.

### Registry / count state

- `internal/bitbucket/tools/register.go:14-52` defines exactly 27 tools
  (indices 0-26); `bitbucket_commit_file` at index 12 (destructive),
  `bitbucket_set_pull_request_review_status` at index 22 (additive),
  `bitbucket_update_pull_request` at index 26 (additive).
- Roster test `TestDefinitionsExposeExactlyTheBitbucketToolSet`
  (tools_test.go:19-66) already asserts the 27-name slice and annotation
  indices 0/3/12/26. This plan adds no tools, so the roster must stay 27.
- `docs/tools/catalog.md:54-84` already lists all 27 Bitbucket tools.

### Secondary (Phase 2) gaps, each verified against current structs

| # | Guide requirement (line) | Current code gap |
|---|---|---|
| P2-1 | §3.6 l.130: `ignoreMissing` on commit list | `commitListInput` (commits.go:21-31) lacks the field; `ListCommits` (101-108) never sends it |
| P2-2 | §3.8 l.168: commit changes `since`, `withComments`, `limit` (no `start`) | `commitPagedInput` (38-43) has only CommitID/Start/Limit; `GetCommitChanges` (117-122) sends `start` which the guide says not to expose |
| P2-3 | §3.9 l.187: commit diff `autoSrcPath`, `since`, `withComments` (plus existing `srcPath`, `contextLines`, `whitespace`) | `diffInput` (45-52) lacks Since/WithComments/AutoSrcPath; `GetCommitDiff` (124-129) never sends them |
| P2-4 | §3.14 l.282, l.288: PR list participant filters as continuous `username.N`/`role.N`/`approved.N` (N from 1, no gaps, max 10, reject >10) plus `withAttributes`, `withProperties`; enums `state`/`direction`/`order` | `prListInput` (pull_requests.go:11-20) exposes a single undocumented `participant` string; no `withAttributes`/`withProperties`; `ListPullRequests` (116-120) validates no enums |
| P2-5 | §3.16 l.320: activities `fromId`, `fromType` (`COMMENT\|ACTIVITY`; `fromType` required when `fromId` present) | `prPagedInput` (27-32) has only Start/Limit; `GetPullRequestActivities` (126-128) sends neither |
| P2-6 | §3.17 l.339: PR commits `withCounts` | same shared `prPagedInput`; `GetPullRequestCommits` (130-132) lacks it |
| P2-7 | §3.18 l.358: PR changes `changeScope` (`ALL\|UNREVIEWED\RANGE`, both IDs required for RANGE); do not expose meaningful `start` | `prChangesInput` (34-42) lacks ChangeScope and exposes Start |
| P2-8 | §3.19 l.377: PR diff `diffType`, `withComments` (plus existing params); hash pairs required for RANGE/COMMIT | `prDiffInput` (44-53) lacks DiffType/WithComments; `GetPullRequestDiff` (138-140) validates nothing |
| P2-9 | §3.22 l.436, l.440: comment anchor `diffType`, `fromHash`, `toHash`; file vs line anchor validation; no partially populated anchor | `anchorInput` (72-78) lacks DiffType/FromHash/ToHash; `AddPullRequestComment` (167-182) requires `path+line+lineType+fileType` for every anchor, making file (non-line) comments impossible |

### Docs state (Phase 3)

- `docs/tools/` contains `catalog.md`, `jira.md`, `confluence.md` — no
  `bitbucket.md`. `README.md` line 27 says "Detailed module notes remain in
  `docs/tools/jira.md` and `docs/tools/confluence.md`", implying the missing
  Bitbucket module doc. `docs/tools/jira.md` (table: Tool | Access | Notes,
  plus envelope and staging notes) is the convention to mirror.
- No user-facing doc describes Bitbucket tools as future work (verified
  2026-08-16); that old scope item is dropped.

## Scope

In scope:

- Phase 1 (three verified high-risk contract gaps):
  - Gap 1: `fromRepo` serialization for the three compare tools.
  - Gap 2: `bitbucket_commit_file` create/update safety enforcement per the
    resolution of OQ-1/OQ-2.
  - Gap 3: review-status participant body `user` member.
  - Focused request-recording tests for each changed request shape and the
    one-mutation invariant.
- Phase 2: the nine verified secondary schema/query gaps P2-1..P2-9 with
  focused tests (after OQ-4 scope confirmation).
- Phase 3: `docs/tools/bitbucket.md` covering all 27 tools (purpose, inputs,
  endpoint, permissions/approval, safety notes, response shape, source link to
  the guide section) and the `README.md` link update.
- Recording unresolved real-host compatibility gates (identity value, 409
  exception names) as explicit limitations in this plan's Result section.

Out of scope:

- Real Bitbucket Server 5.10.2 staging execution (no host/credentials in this
  session); staging gates remain recorded in guide §4.
- New tools (registry stays exactly 27), tool renames, annotation changes.
- Atomic multi-file commits.
- Any change to `internal/bitbucket/client/` — verified that `ProjectKey()`,
  `DoJSON`, `DoMultipart`, `DoRaw`, and the endpoint builders cover every
  planned request shape.
- 409 exception-name sub-classification for `bitbucket_commit_file`
  (staging-gated; mapping code already compliant).
- Repairing the guide's broken bundled-anchor links
  (`../references/...#bb-api-*` does not resolve; the file is in
  `docs/specs/` and has no `bb-api-*` anchors) — see OQ-5.
- Installer work.

## Approach / Proposed Architecture

All fixes land in the existing handler layer (`internal/bitbucket/tools/`),
reusing established idioms:

- Fallible request-building helpers return `(value, *result.Envelope)` —
  the `endpoint()` idiom (service.go:97-112).
- Validation failures return `fail(tool, msg)` (`VALIDATION_ERROR`) before any
  network call.
- HTTP failures flow through `clientError(tool, err)` unchanged.
- Query building stays on the nil-safe `query`/`q()` fluent helpers
  (service.go:127-173).
- Tests stay in `internal/bitbucket/tools/tools_test.go` using the existing
  `httptest.NewServer` request-log pattern and `newTestService`
  (project key `"PRJ"`, user slug `"svc-user"`, tools_test.go:15-17).

Phase 1 lands first (Gap 1 and Gap 3 immediately; Gap 2 after OQ-1/OQ-2 are
resolved), then Phase 2, then Phase 3. Each phase ends with its
verification-plan cases passing before the next begins.

## Implementation Plan

### Step 1 — Gap 1: compare `fromRepo` serialization (`internal/bitbucket/tools/commits.go`)

Replace the free function `compareq` (lines 187-189) with a fallible method on
`*Service`, consolidating the duplicated `from`/`to` validation currently
copy-pasted in the three handlers, and add slug-injection rejection:

```go
// compareq validates compare inputs and builds the shared compare query.
// MCP input fromRepositorySlug serializes to fromRepo={projectKey}/{slug};
// the raw input name is never emitted and no caller-supplied project, URL,
// or slash-qualified value is accepted (guide §3.10).
func (s *Service) compareq(tool string, in compareInput) (query, *result.Envelope) {
	if strings.TrimSpace(in.From) == "" || strings.TrimSpace(in.To) == "" {
		env := fail(tool, "from and to are required")
		return nil, &env
	}
	if strings.Contains(in.FromRepositorySlug, "/") {
		env := fail(tool, "fromRepositorySlug must be a bare repository slug in the configured project")
		return nil, &env
	}
	qq := q("from", in.From, "to", in.To)
	if strings.TrimSpace(in.FromRepositorySlug) != "" {
		qq = qq.add("fromRepo", s.client.ProjectKey()+"/"+in.FromRepositorySlug)
	}
	return qq, nil
}
```

Update the three handlers to the same shape (shown for commits; changes/diff
identical with their own tool names and trailing params):

```go
func (s *Service) CompareCommits(ctx context.Context, in compareInput) result.Envelope {
	qq, env := s.compareq("bitbucket_compare_commits", in)
	if env != nil {
		return *env
	}
	return s.getJSON(ctx, "bitbucket_compare_commits", in.RepositorySlug, "compare/commits", qq.page(in.Start, in.Limit), "commits")
}
```

`CompareDiff` keeps its extra `.add("srcPath", ...).add("whitespace", ...).int("contextLines", ...)`
chain after `compareq` (commits.go:149). The `compareInput` struct is unchanged
(`fromRepositorySlug` remains the MCP input name). Note: `url.Values.Encode()`
percent-encodes the `/` in the value, so the wire form is
`fromRepo=PRJ%2Fother-repo`; Bitbucket decodes query values normally.

### Step 2 — Gap 3: review-status participant body (`internal/bitbucket/tools/pull_requests.go`)

In `SetPullRequestReviewStatus`, replace line 192:

```go
body := map[string]any{
	"user":     map[string]any{"name": s.userSlug},
	"status":   status,
	"approved": status == "APPROVED",
}
```

Everything else in the handler (identity guard, status enum, PUT path via
`prPath(..., "participants", s.userSlug)`, `putJSON`) is unchanged. The
identity value uses `s.userSlug` — the only configured identity — pending the
staging confirmation in OQ-3; guide §4 line 551 gates release, not
implementation. No config change in this step.

### Step 3 — Gap 2: commit_file safety policy (`internal/bitbucket/tools/commits.go`)

BLOCKED on OQ-1 (mode detection) and OQ-2 (sourceBranch enforcement). The
recommended design (option (a) for both, as resolved by the user) is specified
here so implementation can start immediately upon resolution.

3.1. Add a required `mode` input to `commitFileInput` (no `omitempty`, so the
generated MCP schema marks it required):

```go
type commitFileInput struct {
	RepositorySlug string `json:"repositorySlug"`
	Path           string `json:"path"`
	Branch         string `json:"branch"`
	Mode           string `json:"mode"` // exactly "create" or "update"
	Content        string `json:"content,omitempty"`
	ContentBase64  string `json:"contentBase64,omitempty"`
	Message        string `json:"message"`
	SourceCommitID string `json:"sourceCommitId,omitempty"`
	SourceBranch   string `json:"sourceBranch,omitempty"`
}
```

3.2. Extend `CommitFile` validation (after the existing path/branch/message and
exactly-one-content checks, before content decoding):

```go
mode := strings.ToLower(strings.TrimSpace(in.Mode))
if mode != "create" && mode != "update" {
	return fail("bitbucket_commit_file", "mode must be create or update")
}
if mode == "update" && strings.TrimSpace(in.SourceCommitID) == "" {
	return fail("bitbucket_commit_file", "sourceCommitId is required in update mode")
}
if mode == "create" && strings.TrimSpace(in.SourceCommitID) != "" {
	return fail("bitbucket_commit_file", "sourceCommitId must be omitted in create mode")
}
```

3.3. `sourceBranch` handling per OQ-2 resolution:

- Option (i) (recommended baseline, no invented behavior): keep `sourceBranch`
  as an optional pass-through field in both modes (existing lines 172-174);
  document in the tool description and `docs/tools/bitbucket.md` that when
  `branch` does not yet exist the caller MUST supply `sourceBranch`, and that
  the upstream error is surfaced otherwise. No additional request.
- Option (ii) (if the user requires tool-level enforcement): add a
  caller-declared branch-creation signal and hard-require `sourceBranch` when
  it is set; this revises the guide §3.13 input list first (guide revision is
  part of this step under that option).

3.4. The multipart `fields` map and single-PUT behavior (lines 168-184) are
otherwise unchanged: `branch`, `message`, `sourceCommitId` (update mode only,
now enforced), optional `sourceBranch`, `content` file part; response keeps
`singleFileCommit: true`.

3.5. No change to `clientError` / 409 mapping (verified compliant; see
Verified Current State). If OQ-2 resolves to option (ii), update the
`bitbucket_commit_file` description in `register.go:35` to mention the mode
requirement (description text only; name and annotations unchanged).

### Step 4 — Phase 2: secondary schema/query gaps (after OQ-4 confirmation)

All in `internal/bitbucket/tools/`; every added field uses the existing
`query` fluent helpers; every enum is validated with `fail(...)` before any
network call.

- P2-1 `commits.go`: add `IgnoreMissing *bool \`json:"ignoreMissing,omitempty"\``
  to `commitListInput`; `ListCommits` chain gains `.bool("ignoreMissing", in.IgnoreMissing)`.
- P2-2 `commits.go`: add `Since string \`json:"since,omitempty"\`` and
  `WithComments *bool \`json:"withComments,omitempty"\`` to `commitPagedInput`;
  REMOVE `Start *int` (guide §3.8 does not expose usable paging start);
  `GetCommitChanges` query becomes `q("since", in.Since).bool("withComments", in.WithComments).int("limit", in.Limit)`.
- P2-3 `commits.go`: add `Since string`, `WithComments *bool`,
  `AutoSrcPath *bool` (all `omitempty`) to `diffInput`; `GetCommitDiff` query
  gains `"since"`, `.bool("withComments", ...)`, `.bool("autoSrcPath", ...)`.
- P2-4 `pull_requests.go`: replace `Participant string` in `prListInput` with

  ```go
  Participants   []prParticipantFilter `json:"participants,omitempty"`
  WithAttributes *bool                 `json:"withAttributes,omitempty"`
  WithProperties *bool                 `json:"withProperties,omitempty"`
  ```

  plus a new struct

  ```go
  type prParticipantFilter struct {
      Username string `json:"username"`
      Role     string `json:"role,omitempty"` // AUTHOR|REVIEWER|PARTICIPANT
      Approved *bool  `json:"approved,omitempty"`
  }
  ```

  `ListPullRequests` validates `state` (OPEN|DECLINED|MERGED|ALL),
  `direction` (INCOMING|OUTGOING), `order` (OLDEST|NEWEST) when non-empty;
  rejects `len(Participants) > 10` with `fail(...)`; serializes filters
  consecutively from N=1 with no gaps: `username.N` always, `role.N` when
  non-empty (enum-validated), `approved.N` when non-nil; adds
  `.bool("withAttributes", ...)` / `.bool("withProperties", ...)`. The
  undocumented `participant` parameter is removed (clean cutover).
- P2-5/P2-6 `pull_requests.go`: split the shared `prPagedInput` into
  `prActivitiesInput{RepositorySlug, PullRequestID, FromID *int \`json:"fromId,omitempty"\`, FromType string \`json:"fromType,omitempty"\`, Start, Limit}` and
  `prCommitsInput{RepositorySlug, PullRequestID, WithCounts *bool \`json:"withCounts,omitempty"\`, Start, Limit}`; delete `prPagedInput` (used only by
  these two handlers). Update the two `mcp.AddTool` input types in
  `register.go` accordingly: `defs[15]` (activities, register.go:129-131) takes
  `prActivitiesInput`; `defs[16]` (commits, register.go:132-134) takes
  `prCommitsInput`. Activities validation: `FromType` must be COMMENT or
  ACTIVITY when non-empty, and is required (`fail(...)`) when `FromID != nil`.
- P2-7 `pull_requests.go`: add `ChangeScope string \`json:"changeScope,omitempty"\``
  to `prChangesInput`; REMOVE `Start *int`; validation: `changeScope` in
  ALL|UNREVIEWED|RANGE when non-empty; RANGE requires both `sinceId` and
  `untilId` (`fail(...)` otherwise). Query: `q("changeScope", ...)` plus
  existing params.
- P2-8 `pull_requests.go`: add `DiffType string \`json:"diffType,omitempty"\`` and
  `WithComments *bool \`json:"withComments,omitempty"\`` to `prDiffInput`;
  validation: `diffType` in EFFECTIVE|RANGE|COMMIT when non-empty; RANGE
  requires `sinceId` and `untilId`; COMMIT requires `untilId`. Query gains
  `"diffType"` and `.bool("withComments", ...)`. (Exact per-mode hash pairing
  is a contract detail the implementer confirms against the bundled reference;
  staging verifies host behavior per guide §3.19.)
- P2-9 `pull_requests.go`: add `DiffType string`, `FromHash string`,
  `ToHash string` (all `omitempty`) to `anchorInput`. Rework
  `AddPullRequestComment` anchor validation:
  - anchor present with empty `path` → `fail(...)`.
  - Line anchor (`Line != nil`): requires `lineType` in ADDED|REMOVED|CONTEXT
    and `fileType` in FROM|TO; missing either → `fail(...)`.
  - File anchor (`Line == nil`): `lineType`/`fileType` must also be absent
    (partially populated anchor rejected per guide §3.22 line 440).
  - `fromHash`/`toHash` non-empty with empty `diffType` → `fail(...)`
    (non-backcompat anchor combinations require `diffType`).

Each P2 item gets at least one request-recording test asserting exact query
serialization and one validation-failure test (see verification_plan).

### Step 5 — Phase 3: grouped documentation

- Create `docs/tools/bitbucket.md` mirroring `docs/tools/jira.md`: one table
  row per tool (all 27, Tool | Access | Notes) covering purpose, inputs
  (including the new `mode`, participant filters, and anchor fields), endpoint
  method/path, permission concept, approval/safety notes (mutations require
  client approval; commit_file safety policy; review-status identity gate),
  response shape notes, and a source link to the tool's guide section
  (`docs/specs/bitbucket-tool-implementation-guide.md#tool-<name>`).
- Update `README.md` line 27 to add `docs/tools/bitbucket.md` to the module
  notes link list.
- `docs/tools/catalog.md` needs no change (already lists all 27; one-line
  function descriptions remain accurate).

### Step 6 — Validation

Run the verification_plan below (implementer: `go build ./...`,
`go vet ./...`; tester: the planned cases). Record results and unresolved
staging gates in Result, then move this plan to `docs/plans/completed/`.

## Compatibility Considerations

- **Breaking schema change (Gap 2):** making `mode` required rejects existing
  `bitbucket_commit_file` calls that omit it. This is the intended safety
  tightening (SPECS Task 8 acceptance: "The tool cannot silently overwrite a
  changed file when the approved safety contract is followed") and is gated on
  OQ-1 user approval.
- **Breaking schema changes (Phase 2):** removing `participant` (PR list),
  `start` (commit changes, PR changes) and splitting `prPagedInput` change
  published MCP input schemas. All removals are guide-required (undocumented
  or explicitly not-exposed parameters). No tool names, endpoints, or
  annotations change.
- **Gap 1 and Gap 3** change only wire output, not input schemas (Gap 3 adds
  no input; Gap 1 keeps `fromRepositorySlug` as the input name).
- No config, migration, deployment, or client-package changes. The MCP
  registry count (27) and all annotations are unchanged, so roster/annotation
  snapshot tests must pass without modification.
- Windows note: all verification commands (`go build ./...`,
  `go test ./internal/bitbucket/...`) must pass on the project's Windows
  development host, matching prior plans.

## Testing And Validation Strategy

- Framework: existing Go `testing` + `net/http/httptest` request-recording
  servers in `internal/bitbucket/tools/tools_test.go` (no new dependency).
  Helper `newTestService` constructs the client with project key `"PRJ"` and
  user slug `"svc-user"` — assertions below use those values.
- Compile/build validation (`implementer`): `go build ./...`,
  `go build ./cmd/atlassian-mcp`, `go vet ./...`.
- Behavioral verification (`tester`): the cases below. Multipart assertions
  parse the recorded request with `r.ParseMultipartForm` /
  `r.FormValue("branch"|"message"|"sourceCommitId"|"sourceBranch")` /
  `r.FormFile("content")`. JSON body assertions decode into `map[string]any`
  as in `TestCreatePullRequestIncludesProjectKeyInRefs`.
- Real-host staging (NOT in this plan's execution scope): guide §4 gates —
  identity value for `user.name` (line 551) and 409 exception bodies
  (line 552) — remain recorded limitations.

```yaml
verification_plan:
  objective: Prove the three Phase-1 contract gaps are fixed at the wire level (exact query/body/multipart shapes, one-mutation invariant), that Phase-2 documented inputs serialize per guide, and that the 27-tool registry/annotations are unchanged.
  source_acceptance_criteria:
    - "Compare tools serialize fromRepositorySlug as fromRepo={projectKey}/{slug} and never emit fromRepositorySlug (guide §3.10 l.206, §3.11 l.225, §3.12 l.244)"
    - "commit_file requires sourceCommitId for update, omits it for create, enforces mode, sends exactly one PUT (guide §3.13 l.269; SPECS Task 8 l.1147-1153)"
    - "review status body contains user:{name: identity}, approved, status (guide §3.23 l.455)"
    - "Registry still exposes exactly the 27 Bitbucket tools with unchanged annotations"
    - "Phase-2 inputs serialize exactly as documented (guide §3.6, §3.8, §3.9, §3.14, §3.16, §3.17, §3.18, §3.19, §3.22)"
  environment:
    prerequisites:
      - "Go 1.25 toolchain on the Windows development host"
      - "Repository dependencies vendored/downloadable (go mod)"
    constraints:
      - "No real Bitbucket host; all HTTP assertions use httptest request-recording servers"
      - "No changes to internal/bitbucket/client/"
      - "Tests live in internal/bitbucket/tools/tools_test.go"
  cases:
    - id: VP-001
      objective: Cross-repo compare serializes fromRepo with configured project key
      level: contract
      mandatory: true
      setup:
        - "httptest server logging method+path+RawQuery, responding 200 {}"
        - "svc := newTestService(server.URL, server.Client())"
      inputs: "compareInput{RepositorySlug:\"repo\", From:\"feature\", To:\"master\", FromRepositorySlug:\"other\"} passed to CompareCommits, CompareChanges, and CompareDiff (Path:\"f.txt\" for diff)"
      steps:
        - "Invoke all three handlers"
        - "Assert each recorded request query contains fromRepo=PRJ%2Fother"
        - "Assert no recorded request query contains fromRepositorySlug"
      expected_result: "All three requests are GETs to .../compare/commits, .../compare/changes, .../compare/diff/f.txt with from=feature, to=master, fromRepo=PRJ%2Fother; all envelopes Success=true"
      evidence_required:
        - "Recorded request lines"
    - id: VP-002
      objective: Same-repo compare omits fromRepo entirely
      level: contract
      mandatory: true
      setup:
        - "Same recording server as VP-001"
      inputs: "compareInput{RepositorySlug:\"repo\", From:\"a\", To:\"b\"} (empty FromRepositorySlug) to CompareCommits"
      steps:
        - "Invoke handler; inspect recorded query"
      expected_result: "Query contains from and to only; neither fromRepo nor fromRepositorySlug present"
      evidence_required:
        - "Recorded request line"
    - id: VP-003
      objective: Project/URL injection via fromRepositorySlug is rejected before any request
      level: unit
      mandatory: true
      setup:
        - "Recording server that fails the test if any request arrives"
      inputs: "compareInput{RepositorySlug:\"repo\", From:\"a\", To:\"b\", FromRepositorySlug:\"OTHER/repo\"} to each compare handler"
      steps:
        - "Invoke all three handlers"
      expected_result: "Each envelope Success=false with Error.Code=VALIDATION_ERROR and a message naming fromRepositorySlug; zero HTTP requests recorded"
      evidence_required:
        - "Envelope error codes"
        - "Request count == 0"
    - id: VP-004
      objective: commit_file update mode requires sourceCommitId
      level: unit
      mandatory: true
      setup:
        - "Recording server that fails the test if any request arrives"
      inputs: "commitFileInput{RepositorySlug:\"repo\", Path:\"a.txt\", Branch:\"main\", Mode:\"update\", Content:\"x\", Message:\"m\"} (no SourceCommitID)"
      steps:
        - "Invoke CommitFile"
      expected_result: "Success=false, Error.Code=VALIDATION_ERROR, message states sourceCommitId is required in update mode; zero HTTP requests"
      evidence_required:
        - "Envelope error"
        - "Request count == 0"
    - id: VP-005
      objective: commit_file create mode rejects sourceCommitId and omits it from the multipart payload
      level: contract
      mandatory: true
      setup:
        - "httptest server parsing the multipart form and logging form values"
      inputs: "Two calls: (1) Mode:\"create\" with SourceCommitID set → expect validation failure, no request; (2) Mode:\"create\" without SourceCommitID → expect success"
      steps:
        - "Invoke CommitFile for both inputs"
        - "For call 2 assert multipart fields"
      expected_result: "Call 1 fails VALIDATION_ERROR with no request. Call 2 sends exactly one PUT whose multipart form contains branch=main, message=m, content part with exact bytes, and NO sourceCommitId field"
      evidence_required:
        - "Recorded form values and file-part bytes"
        - "PUT count == 1"
    - id: VP-006
      objective: commit_file update mode sends sourceCommitId in the multipart payload, one PUT
      level: contract
      mandatory: true
      setup:
        - "Multipart-parsing recording server responding 200 {\"id\":\"abc\"}"
      inputs: "commitFileInput{Mode:\"update\", SourceCommitID:\"abc0\", SourceBranch:\"base\", plus required fields}"
      steps:
        - "Invoke CommitFile; inspect multipart form and request count"
      expected_result: "Exactly one PUT; form contains branch, message, sourceCommitId=abc0, sourceBranch=base, content part; envelope Success=true with data.singleFileCommit=true"
      evidence_required:
        - "Recorded form values"
        - "PUT count == 1"
    - id: VP-007
      objective: commit_file rejects missing or invalid mode
      level: unit
      mandatory: true
      setup:
        - "Recording server that fails the test if any request arrives"
      inputs: "Three calls with Mode:\"\", Mode:\"UPDATE \" (accepted after normalization), Mode:\"delete\" (rejected)"
      steps:
        - "Invoke CommitFile for each"
      expected_result: "Empty and \"delete\" fail VALIDATION_ERROR (message: mode must be create or update) with zero requests; normalized \"update\" proceeds to sourceCommitId validation"
      evidence_required:
        - "Envelope results per call"
    - id: VP-008
      objective: commit_file 409 maps to BITBUCKET_COMMIT_FILE_CONFLICT with detail preserved and no retry
      level: contract
      mandatory: true
      setup:
        - "Server returning HTTP 409 with body {\"errors\":[{\"message\":\"stale source commit\",\"exceptionName\":\"...\"}]} for the PUT"
      inputs: "Valid update-mode CommitFile call"
      steps:
        - "Invoke CommitFile; inspect envelope and PUT count"
      expected_result: "Success=false, Error.Code=BITBUCKET_COMMIT_FILE_CONFLICT, Error.HTTPCode=409, Error.Detail preserves the sanitized errors array; exactly one PUT recorded"
      evidence_required:
        - "Envelope error fields"
        - "PUT count == 1"
    - id: VP-009
      objective: sourceBranch enforcement for new-branch commits per OQ-2 resolution
      level: contract
      mandatory: false # becomes mandatory with exact expected result once OQ-2 is resolved; under recommended option (i) the case asserts sourceBranch pass-through and documentation, not pre-validation
      setup:
        - "Multipart-parsing recording server"
      inputs: "Per resolved OQ-2 option"
      steps:
        - "Per resolved OQ-2 option"
      expected_result: "Option (i): sourceBranch passed through when supplied; absent enforcement documented. Option (ii): branch-creation signal without sourceBranch fails VALIDATION_ERROR before any PUT"
      evidence_required:
        - "Recorded form values or envelope error"
    - id: VP-010
      objective: review status PUT body contains user.name, status, and correct approved mapping for all three statuses
      level: contract
      mandatory: true
      setup:
        - "httptest server decoding the JSON body and logging the path, responding 201 {}"
      inputs: "SetPullRequestReviewStatus with Status \"APPROVED\", \"NEEDS_WORK\", \"UNAPPROVED\" (case-insensitive input e.g. \"approved\" accepted)"
      steps:
        - "Invoke handler three times; decode each body"
      expected_result: "Each request is PUT /rest/api/1.0/projects/PRJ/repos/repo/pull-requests/5/participants/svc-user; body == {\"user\":{\"name\":\"svc-user\"},\"status\":<STATUS>,\"approved\":<STATUS==APPROVED>}; approved true only for APPROVED"
      evidence_required:
        - "Decoded bodies and recorded paths"
    - id: VP-011
      objective: review status without configured identity fails before any request (regression)
      level: unit
      mandatory: true
      setup:
        - "Service constructed with empty userSlug; recording server that fails on any request"
      inputs: "SetPullRequestReviewStatus with Status \"APPROVED\""
      steps:
        - "Invoke handler"
      expected_result: "Success=false, Error.Code=BITBUCKET_REVIEW_IDENTITY_REQUIRED; zero HTTP requests"
      evidence_required:
        - "Envelope error code"
        - "Request count == 0"
    - id: VP-012
      objective: Registry still exposes exactly the 27 Bitbucket tools with unchanged annotations (regression)
      level: regression
      mandatory: true
      setup: []
      inputs: "go test -run TestDefinitionsExposeExactlyTheBitbucketToolSet ./internal/bitbucket/tools/"
      steps:
        - "Run the existing roster test unmodified"
      expected_result: "Test passes with the existing 27-name slice and annotation index assertions (defs[0], defs[3], defs[12], defs[26])"
      evidence_required:
        - "go test output"
    - id: VP-013
      objective: Phase 2 — commit read tools serialize newly documented params (P2-1..P2-3)
      level: contract
      mandatory: true
      setup:
        - "Recording server as in VP-001"
      inputs: "ListCommits with IgnoreMissing=true; GetCommitChanges with Since+WithComments (and no start accepted); GetCommitDiff with Since+WithComments+AutoSrcPath"
      steps:
        - "Invoke each handler; inspect queries"
      expected_result: "Queries contain ignoreMissing=true; since=<v>&withComments=true with no start param on changes; since/withComments/autoSrcPath present on commit diff; validation/serialization failures for malformed inputs produce VALIDATION_ERROR with zero requests"
      evidence_required:
        - "Recorded request lines"
    - id: VP-014
      objective: Phase 2 — PR list participant filters serialize as continuous username.N/role.N/approved.N, max 10, plus withAttributes/withProperties; enums validated (P2-4)
      level: contract
      mandatory: true
      setup:
        - "Recording server as in VP-001"
      inputs: "ListPullRequests with two participant filters (one full, one username-only), WithAttributes/WithProperties true; separately 11 filters; separately invalid state/direction/order/role values"
      steps:
        - "Invoke handler for each input; inspect query or envelope"
      expected_result: "Valid call query contains username.1, role.1, approved.1, username.2 (no role.2/approved.2 gaps), withAttributes=true, withProperties=true, and no participant= param; 11 filters fail VALIDATION_ERROR with zero requests; each invalid enum fails VALIDATION_ERROR with zero requests"
      evidence_required:
        - "Recorded request lines and envelope errors"
    - id: VP-015
      objective: Phase 2 — activities fromId/fromType and PR commits withCounts (P2-5, P2-6)
      level: contract
      mandatory: true
      setup:
        - "Recording server as in VP-001"
      inputs: "GetPullRequestActivities with FromID+FromType=COMMENT; FromID without FromType; FromType=BOGUS; GetPullRequestCommits with WithCounts=true"
      steps:
        - "Invoke handlers; inspect query/envelopes"
      expected_result: "Valid activities call sends fromId=<n>&fromType=COMMENT; fromId-without-fromType and bogus fromType fail VALIDATION_ERROR with zero requests; commits call sends withCounts=true"
      evidence_required:
        - "Recorded request lines and envelope errors"
    - id: VP-016
      objective: Phase 2 — PR changes changeScope validation and no start exposure (P2-7)
      level: contract
      mandatory: true
      setup:
        - "Recording server as in VP-001"
      inputs: "GetPullRequestChanges with changeScope=ALL; RANGE with both IDs; RANGE missing untilId; changeScope=BOGUS"
      steps:
        - "Invoke handler per input"
      expected_result: "ALL and complete RANGE serialize changeScope plus sinceId/untilId with no start param; incomplete RANGE and bogus scope fail VALIDATION_ERROR with zero requests"
      evidence_required:
        - "Recorded request lines and envelope errors"
    - id: VP-017
      objective: Phase 2 — PR diff diffType/withComments and hash-pair requirements (P2-8)
      level: contract
      mandatory: true
      setup:
        - "Recording server as in VP-001"
      inputs: "GetPullRequestDiff with diffType=EFFECTIVE; RANGE with sinceId+untilId; RANGE missing sinceId; COMMIT with untilId; diffType=BOGUS; WithComments=true"
      steps:
        - "Invoke handler per input"
      expected_result: "Valid calls serialize diffType, withComments and the required hash params; incomplete RANGE and bogus diffType fail VALIDATION_ERROR with zero requests"
      evidence_required:
        - "Recorded request lines and envelope errors"
    - id: VP-018
      objective: Phase 2 — comment anchor supports file and line modes with hash/diffType fields and rejects partial anchors (P2-9)
      level: contract
      mandatory: true
      setup:
        - "Recording server decoding JSON bodies"
      inputs: "AddPullRequestComment with: file anchor (path only); line anchor (path+line+lineType=ADDED+fileType=TO); anchor with fromHash/toHash+diffType=RANGE; anchor missing path; line anchor missing fileType; file anchor carrying lineType; fromHash without diffType"
      steps:
        - "Invoke handler per input; inspect bodies/envelopes"
      expected_result: "First three send one POST each with the exact anchor object (diffType/fromHash/toHash present only when supplied); the other four fail VALIDATION_ERROR with zero requests"
      evidence_required:
        - "Decoded bodies, envelope errors, POST counts"
    - id: VP-019
      objective: Phase 3 — grouped documentation covers all 27 tools and README links it
      level: functional
      mandatory: true
      setup: []
      inputs: "docs/tools/bitbucket.md and README.md after Phase 3"
      steps:
        - "Verify bitbucket.md contains one entry per name in the register.go 27-name list, each with endpoint, inputs, permission/approval note, and a resolving link to the guide section"
        - "Verify README.md links docs/tools/bitbucket.md"
      expected_result: "27/27 tools documented; every internal link resolves; README link present"
      evidence_required:
        - "Documented tool list diffed against register.go names"
  execution_order:
    - "VP-012 (registry regression baseline)"
    - "VP-001..VP-003 (Gap 1)"
    - "VP-010, VP-011 (Gap 3)"
    - "VP-004..VP-008 (Gap 2; after OQ-1/OQ-2 resolution), then VP-009 if OQ-2 mandates"
    - "VP-013..VP-018 (Phase 2; after OQ-4 confirmation)"
    - "VP-019 (Phase 3)"
    - "Repository checks: go build ./... ; go vet ./... ; go test ./..."
  non_goals:
    - "Real Bitbucket Server 5.10.2 staging execution (identity value, 409 exception names, response caps)"
    - "New tools, tool renames, or annotation changes"
    - "Changes to internal/bitbucket/client/"
    - "409 exception-name sub-classification for commit_file"
    - "Repair of guide bundled-anchor links (see OQ-5)"
```

## Risks And Recovery

- Risk: Some guide requirements depend on real Bitbucket Server 5.10.2
  behavior (participant `user.name` identity value; 409 exception bodies;
  response caps). Mitigation: stop before inventing behavior; keep the guide
  §4 staging gates recorded as release blockers in Result; request
  credentials/access separately.
- Risk: Tightening `bitbucket_commit_file` (required `mode`, required
  `sourceCommitId` on update) rejects existing local usage patterns.
  Mitigation: this is the documented safety contract; the change is gated on
  explicit OQ-1/OQ-2 user approval, and the error messages tell callers
  exactly which field to add.
- Risk: Removing `participant`/`start` inputs (Phase 2) breaks callers that
  relied on them. Mitigation: both are undocumented/ignored by 5.10.2 per the
  guide; removal is a clean cutover with guide-backed justification recorded
  here.
- Risk: `fromRepo` value encoding (`PRJ%2Fother`) could theoretically be
  rejected by an unusual reverse proxy. Mitigation: percent-encoding of query
  values is standard and Bitbucket decodes it; staging gate would catch a
  proxy issue; alternative (raw `/`) would require client `urlFor` changes,
  which are out of scope.
- Recovery: each phase touches disjoint files
  (`commits.go` for Gap 1/Gap 2 + P2-1..P2-3; `pull_requests.go` for Gap 3 +
  P2-4..P2-9; `docs/tools/bitbucket.md` + `README.md` for Phase 3). Revert the
  files of a rejected phase and restore the last passing state via
  `go test ./internal/bitbucket/...`. No data migration or git-state recovery
  is involved.

## Open Questions (user decision required)

- **OQ-1 (BLOCKS Gap 2 implementation): How does `bitbucket_commit_file`
  distinguish update from create?**
  - (a) RECOMMENDED: add a required `mode` input (`create` | `update`), as
    specified in Step 3. Backed by SPECS Task 8 language ("create mode",
    "update mode", "Existing file in create mode"). Deterministic, testable,
    zero extra requests, preserves the one-PUT invariant. Breaking schema
    change.
  - (b) Pre-check GET to test file existence before the PUT. Conflicts with
    the guide's "at most one mutation request; a follow-up GET is allowed only
    where explicitly stated" (§3.13 states no GET), is racy (file can appear
    between GET and PUT), and no documented 5.10.2 endpoint checks file
    existence without also conflating branch/revision errors.
  - (c) Always require `sourceCommitId`. Violates SPECS "Omit
    `sourceCommitId` for create mode" and breaks file creation.
  - Blocks: Step 3.1/3.2, VP-004..VP-007.
- **OQ-2 (BLOCKS Gap 2 implementation): How is "for a new branch require
  `sourceBranch`" enforced?** The tool cannot detect branch newness from any
  documented 5.10.2 read endpoint without inventing behavior.
  - (i) RECOMMENDED baseline: keep `sourceBranch` optional pass-through;
    document that callers MUST supply it when `branch` does not exist; the
    upstream error is surfaced when they don't. No extra request, no invented
    behavior; the guide's requirement is enforced at contract/documentation
    level plus upstream error mapping.
  - (ii) Add a caller-declared branch-creation signal (new input field) and
    hard-require `sourceBranch` when it is set. Requires a guide §3.13 input
    revision first (the field is not currently documented).
  - (iii) Pre-check GET branch existence. Not recommended: no exact
    single-branch GET is documented in the bundled 5.10.2 reference
    (`filterText` matching semantics are unspecified), so this would invent
    behavior.
  - Blocks: Step 3.3, VP-009 shape.
- **OQ-3 (BLOCKS release of Gap 3, not implementation): Is
  `BITBUCKET_USER_SLUG` the correct value for the body `user.name`, or is a
  separate `BITBUCKET_USER_NAME` config needed?** Guide §3.23 line 455 and
  §4 line 551 explicitly defer this to the target 5.10.2 host; in Bitbucket
  Server the username and the URL slug can differ. Plan default: implement
  with `s.userSlug` now (satisfies the documented body shape), keep the
  staging gate; add `BITBUCKET_USER_NAME` (config.go + `Service` field) only
  if staging proves the slug invalid. Blocks: whether `internal/bitbucket/config.go`
  and `NewService` gain a second identity value.
- **OQ-4 (scope confirmation): Execute Phase 2 (P2-1..P2-9) and Phase 3
  (docs) in this cycle?** The brief permits scoping them as a second phase;
  the original plan had them in scope. Plan default: execute after Phase 1.
- **OQ-5 (optional, out of scope): Repair the guide's broken bundled-anchor
  links?** Every guide section links `../references/jira-6.4.14_bitbucket-5.10.2-rest-api-reference.md#bb-api-*`,
  but the file lives at `docs/specs/jira-6.4.14_bitbucket-5.10.2-rest-api-reference.md`
  and contains no `bb-api-*` anchors (guide §5 line 558 requires local source
  links to resolve). Not included in this plan unless the user adds it.

Non-blocking assumptions (stated, not verified facts):

- `url.Values` percent-encoding of `/` inside `fromRepo` is accepted by the
  target Bitbucket host (standard query decoding; staging would confirm).
- `diffType` hash pairing in VP-017 (RANGE: sinceId+untilId; COMMIT: untilId)
  is inferred from the guide's "appropriate hash pair" wording and the
  available parameters; the implementer confirms against the bundled reference
  and staging verifies host behavior.
- `BITBUCKET_PROJECT_KEY` is always non-empty when the Service exists
  (config.go:29-31 rejects empty project key), so `compareq` needs no empty
  project guard.

## Progress

- [x] Re-verify the three high-risk gaps and secondary items against current source (2026-08-16).
- [x] Confirm registry count is 27 and update plan references (2026-08-16).
- [x] Verify 409 commit_file mapping already compliant; defer sub-classification to staging (2026-08-16).
- [x] Verify "future work" doc staleness item is moot; drop it (2026-08-16).
- [x] Resolve OQ-1 (commit_file mode detection) — user chose (a) required `mode` input (2026-08-16).
- [x] Resolve OQ-2 (sourceBranch enforcement) — user chose (i) optional pass-through with documentation (2026-08-16).
- [x] Resolve OQ-3 (user.name identity value) — user chose ship `s.userSlug` + staging gate (2026-08-16).
- [x] Confirm OQ-4 (Phase 2/3 scope this cycle) — user chose full scope Phase 1+2+3 (2026-08-16).
- [x] Implement + test Gap 1 (compare fromRepo) (2026-08-16).
- [x] Implement + test Gap 3 (review status body) (2026-08-16).
- [x] Implement + test Gap 2 (commit_file safety) after OQ-1/OQ-2 (2026-08-16).
- [x] Implement + test Phase 2 items P2-1..P2-9 (2026-08-16).
- [x] Write docs/tools/bitbucket.md (27 tools) and README link (2026-08-16).
- [x] Run focused and repository validation; record staging gates in Result (2026-08-16).

## Decisions

- 2026-08-01: Keep this as a separate active plan from `build-task-1-14.md`
  because that plan is Task 5 foundation-focused, while this work concerns
  Tasks 6-11 tool contracts and documentation.
- 2026-08-01: Treat `docs/specs/bitbucket-tool-implementation-guide.md` as the
  endpoint-level authority; do not resolve mismatches by weakening docs
  without an explicit source-backed reason.
- 2026-08-01: Grouped documentation is acceptable only if every tool has an
  explicit entry and source link.
- 2026-08-16: Registry count is 27 everywhere in this plan (verified against
  register.go:14-52 and the roster test).
- 2026-08-16: No `internal/bitbucket/client/` changes — verified existing
  primitives (`ProjectKey`, `DoJSON`, `DoMultipart`, endpoint builders) cover
  every planned request shape.
- 2026-08-16: commit_file 409 mapping stays `BITBUCKET_COMMIT_FILE_CONFLICT`
  with sanitized detail pass-through (already SPECS Task 8 compliant);
  exception-name sub-classification is staging-gated, not implemented now.
- 2026-08-16: Tests stay in `internal/bitbucket/tools/tools_test.go`
  (existing convention); the test matrix's `tests/contract/` layout does not
  exist yet and is not introduced by this plan.
- 2026-08-16: Gap 3 ships with `s.userSlug` as `user.name`; identity
  correctness is a release gate per guide §4 line 551 (OQ-3).
- 2026-08-16: OQ-1 resolved — `bitbucket_commit_file` gains a required `mode`
  input (`create` | `update`); `sourceCommitId` required for update, rejected
  for create. User chose option (a).
- 2026-08-16: OQ-2 resolved — `sourceBranch` remains optional pass-through;
  callers MUST supply it when `branch` does not exist; upstream error is
  surfaced when they don't. User chose option (i).
- 2026-08-16: OQ-3 resolved — Gap 3 ships with `s.userSlug` as `user.name`;
  identity correctness is a release/staging gate. User chose default.
- 2026-08-16: OQ-4 resolved — full scope: Phase 1 (3 gaps) + Phase 2
  (P2-1..P2-9 secondary schema gaps) + Phase 3 (docs/tools/bitbucket.md)
  all in this cycle. User chose full scope.

## Definition Of Done

- All mandatory `verification_plan` cases pass (VP-009 per OQ-2 resolution),
  executed by `tester` with recorded evidence.
- `go build ./...`, `go build ./cmd/atlassian-mcp`, `go vet ./...`, and
  `go test ./...` pass on the Windows development host (implementer
  compile/build validation plus tester suites).
- Roster test passes unmodified at exactly 27 tools; no annotation changes.
- `docs/tools/bitbucket.md` documents all 27 tools and `README.md` links it.
- OQ-1, OQ-2, OQ-3 resolved by the user and recorded in Decisions; staging
  gates that remain open (identity value, 409 bodies) are recorded as
  explicit limitations in Result.
- Plan moved to `docs/plans/completed/` with Result filled in.

## Validation

- Focused proof:
  - `go test ./internal/bitbucket/...`
  - The request-recording cases VP-001..VP-018 (compare serialization,
    commit_file mode/safety/one-PUT, review-status body, Phase 2 query/body
    shapes, registry regression).
- Integration or end-to-end proof:
  - Build binary and list MCP tools when local client support is available.
  - Real Bitbucket Server 5.10.2 staging checks (identity value, 409 bodies,
    caps) only when host/token/repository access is provided — recorded as
    gates, not blockers for this plan's local completion.
- Repository-required checks:
  - `go build ./...`, `go vet ./...`, `go test ./...`
  - Markdown link/path sanity check for `docs/tools/bitbucket.md` and the
    README link.

## Result

Implemented on 2026-08-16. All three Phase-1 contract gaps and all nine
Phase-2 secondary schema gaps are fixed at the wire level, with
request-recording tests. Phase-3 grouped documentation covers all 27 tools.

Resolved open questions (user decisions, 2026-08-16):

- OQ-1 → option (a): `bitbucket_commit_file` gains a required `mode` input
  (`create` | `update`); `sourceCommitId` required for update, rejected for
  create.
- OQ-2 → option (i): `sourceBranch` remains optional pass-through; callers
  MUST supply it when `branch` does not exist; upstream error surfaced
  otherwise.
- OQ-3 → default: Gap 3 ships with `s.userSlug` as `user.name`; identity
  correctness is a release/staging gate.
- OQ-4 → full scope: Phase 1 + Phase 2 + Phase 3 all executed this cycle.

Contract fixes delivered:

- Gap 1: `compareq` is now a fallible `(*Service)` method that serializes
  `fromRepo={projectKey}/{slug}`, rejects slash-qualified slugs, and
  consolidates `from`/`to` validation. Applied to `bitbucket_compare_commits`,
  `bitbucket_compare_changes`, and `bitbucket_compare_diff`.
- Gap 2: `bitbucket_commit_file` enforces `mode` (create/update), requires
  `sourceCommitId` for update, rejects it for create, keeps `sourceBranch`
  optional pass-through, and preserves the one-PUT invariant.
- Gap 3: `bitbucket_set_pull_request_review_status` body now includes
  `user:{"name": s.userSlug}` alongside `status` and `approved`.
- Phase 2 (P2-1..P2-9): commit read tools gain `ignoreMissing`/`since`/
  `withComments`/`autoSrcPath`; PR list gains participant filters
  (`username.N`/`role.N`/`approved.N`, max 10) plus `withAttributes`/
  `withProperties` and enum validation; activities/commits inputs split with
  `fromId`/`fromType` and `withCounts`; PR changes gain `changeScope`; PR diff
  gains `diffType`/`withComments` with hash-pair validation; comment anchor
  gains `diffType`/`fromHash`/`toHash` with file/line-mode validation.
- Phase 3: `docs/tools/bitbucket.md` documents all 27 tools; `README.md`
  links it.

Validation evidence (2026-08-16, Windows development host):

- `go build ./...` — passed.
- `go vet ./...` — passed.
- `go test ./internal/bitbucket/tools/ -count=1` — passed (26/26 tests,
  including the unmodified 27-tool registry test and all VP-001..VP-018
  request-recording cases).
- `go test ./... -count=1` — passed (13 packages ok, 3 no tests).
- `docs/tools/bitbucket.md` tool list diffed against `register.go` names —
  27/27 match, no extras.

Unresolved real-host staging gates (recorded limitations, not blockers for
local completion):

- The `user.name` identity value for `bitbucket_set_pull_request_review_status`
  (guide §4 line 551) — currently ships `s.userSlug`; must be confirmed against
  the target Bitbucket Server 5.10.2 host.
- 409 exception-name sub-classification for `bitbucket_commit_file`
  (guide §4 line 552) — mapping code is already SPECS-compliant
  (`BITBUCKET_COMMIT_FILE_CONFLICT` with sanitized detail); only the
  exception-name breakdown is staging-gated.

No changes to `internal/bitbucket/client/`, no new tools, no tool renames, and
no annotation changes. Registry remains exactly 27 tools.

This plan is ready to move to `docs/plans/completed/` once the user confirms
the local result is acceptable.
