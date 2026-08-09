# Execution Plan: Expand Confluence Core Content And Space Reads

Date: 2026-08-09

> **For agentic workers:** Execute this plan task-by-task, keep this file's
> Progress and Decisions current, and do not move it to `completed/` until the
> listed validation is recorded.

## Status

Completed - implementation, remediation, and independent re-review accepted;
live Confluence 6.1.2 host validation remains unavailable.

The plan is ready for promotion to `docs/plans/completed/`. A resuming worker
should not repeat the historical implementation sequence unless a new defect is
identified.

## Outcome

Expand the enabled Confluence module from three to exactly nine MCP tools by
retaining authentication, raw CQL search, and content-by-ID retrieval and adding
six endpoint-backed, GET-only core reads:

- `confluence_list_content` -> `GET /rest/api/content`
- `confluence_list_content_properties` ->
  `GET /rest/api/content/{id}/property`
- `confluence_get_content_property` ->
  `GET /rest/api/content/{id}/property/{key}`
- `confluence_list_spaces` -> `GET /rest/api/space`
- `confluence_get_space` -> `GET /rest/api/space/{spaceKey}`
- `confluence_list_space_content` ->
  `GET /rest/api/space/{spaceKey}/content`

Reorganize Confluence handlers into content and space groups, following the
existing Bitbucket tool layout without changing the Confluence client,
authentication lifecycle, result envelope, configuration, or existing tool
contracts.

Acceptance criteria:

- The exact roster is the nine tools named in this plan; all eight post-auth
  tools are annotated read-only and open-world.
- Every post-auth tool returns `CONFLUENCE_NOT_AUTHENTICATED` without sending a
  request when no Confluence session exists.
- Each new tool issues only its documented GET request, applies the exact input
  and default contract below, and structurally preserves the redacted upstream
  JSON map directly in `result.Envelope.Data`.
- Existing `confluence_authenticate`, `confluence_search_content`, and
  `confluence_get_content` names, inputs, behavior, output shape, and definition
  order remain compatible.
- No typed space-content tool, mutating Content Property tool, or any other
  excluded endpoint category is registered.
- Focused tests, repository tests, vet, build/version, documentation checks, and
  whitespace checks pass; any unavailable real-host proof is disclosed.

## Context

Authority:

- `AGENTS.md` and `docs/WORKFLOW.md` define the read-only planning boundary,
  durable-plan lifecycle, minimal-change requirement, and completion evidence.
- `docs/specs/confluence-6.1.2_6.1.4-rest-api-reference.md` is the endpoint
  contract authority. The runtime target remains Confluence Server 6.1.2, with
  REST 6.1.4 used only as the readable reference in the same 6.1.x line.
  Section 9 is the sole authority for the two Content Property reads; its
  POST, PUT, and DELETE rows do not authorize MCP mutation tools.
- `internal/confluence/tools/content.go`,
  `internal/confluence/tools/spaces.go`,
  `internal/confluence/tools/service.go`,
  `internal/confluence/tools/register.go`, and
  `internal/confluence/tools/tools_test.go` are current executable truth for
  content and space handlers, session gating and shared helpers, the nine-tool
  definitions and bindings, and contract proof.
- `internal/confluence/client/client.go` is the existing bounded, GET-only,
  Basic Auth client. Its `GetJSON` and `urlFor` methods already provide the
  needed method restriction, context-path preservation, `net/url` query
  encoding, response cap, and sanitized HTTP error mapping.
- `internal/bitbucket/tools/{register.go,service.go,branches.go,commits.go,
  pull_requests.go}` establishes the local grouping pattern: shared service and
  query/error helpers, resource-group handler files, and group-specific
  registration methods.
- `docs/plans/completed/build-confluence-read-search-module.md` is historical
  implementation evidence only. It must remain unchanged.

Pre-implementation baseline verified on 2026-08-09:

- `tools.Service` owned a dedicated Confluence client and module-local
  `auth.SessionStore`; `Authenticate` validated `/user/current`, and
  `requireCredential` blocked reads before authentication.
- `SearchContentInput`/`SearchContent` and `GetContentInput`/`GetContent` lived
  in `service.go`. Search sent an explicit default `limit=25`; get validated
  `contentId` as one path segment. Both returned `observability.Redact(out)`
  directly as envelope data.
- `Definitions` and `Register` exposed exactly three definitions from one file
  without resource-group registration helpers.
- `tools_test.go` proved authentication behavior, pre-auth network suppression,
  existing content paths/query encoding, upstream-shape preservation, and the
  former exact three-tool roster.
- `README.md`, `docs/tools/confluence.md`, `docs/architecture.md`, and
  `docs/security.md` described the former three-tool boundary or read-only
  session posture and therefore required alignment with the expanded roster.
  The security posture required no behavioral change.

## Scope

In scope:

- Preserve `confluence_authenticate`, `confluence_search_content`, and
  `confluence_get_content` unchanged at the MCP boundary.
- Add the six named GET-only tools and their exact schemas, validation,
  request construction, registrations, tests, and user-facing documentation.
- Move content input types and handlers from `service.go` to `content.go`; add
  both Content Property input types and handlers there; add space input types
  and handlers in `spaces.go`; retain authentication and shared helpers in
  `service.go`.
- Split registration into authentication, content, and space registration
  methods while keeping one ordered `Definitions` roster and the shared output
  schema.
- Reuse the current client, session store, redaction, result envelope, path
  segment validation, query encoding, and error mapping.

Out of scope:

- `GET /rest/api/space/{spaceKey}/content/{type}` and a
  `confluence_list_space_content_by_type` tool. The untyped space-content
  endpoint already returns the core grouped content view; the typed route is a
  near-duplicate convenience, not required to meet the resolved scope.
- History or macro bodies; content children, comments, or descendants;
  attachments; labels; space properties; restrictions; generic request tools;
  generic `/rest/api/search`; crawling, caching, or indexing.
- Every mutating Content Property route from REST reference section 9:
  `POST /rest/api/content/{id}/property`,
  `POST /rest/api/content/{id}/property/{key}`,
  `PUT /rest/api/content/{id}/property/{key}`, and
  `DELETE /rest/api/content/{id}/property/{key}`. No create, set, update, or
  delete property tool is registered.
- POST, PUT, PATCH, DELETE, multipart upload, or any other non-GET request.
- New configuration, environment variables, authentication behavior, client
  generalization, dependencies, migrations, installer changes, or Jira and
  Bitbucket behavior changes.
- Editing the REST reference or the completed historical Confluence plan.

## Tool Contract

All optional strings are omitted from the query when blank. `start`, when
present, must be non-negative. `limit`, when present, must be positive. Core
content/space collections send `limit=25` when omitted; the Content Property
collection sends its documented `limit=10`. Server defaults are not duplicated
when omission already expresses them (`type=page` for list content,
`depth=all` for list space content, and the native property `expand` default).
No tool injects an `expand` value.

| Tool | JSON inputs | Validation and defaults | Successful data |
| --- | --- | --- | --- |
| `confluence_authenticate` | Existing `username`, `password` | Existing tool-input then environment fallback; validate `GET /user/current` before replacing the session | Existing redacted `{"user": ...}` map |
| `confluence_search_content` | Existing required `cql`; optional `cqlcontext`, `expand`, `start`, `limit` | Preserve raw CQL and explicit omitted `limit=25` behavior | Redacted upstream search object, including `results`, paging fields, `_links`, and `_expandable` when supplied |
| `confluence_get_content` | Existing required `contentId`; optional `status`, positive `version`, `expand` | Preserve one-safe-path-segment validation; omitted status remains upstream `current`; omitted expand remains Confluence's native default | Redacted upstream content object without wrapper reshaping |
| `confluence_list_content` | Optional `type`, `spaceKey`, `title`, `status`, `postingDay`, `expand`, `start`, `limit` | If supplied, `type` is `page` or `blogpost`; `postingDay` parses exactly as `yyyy-mm-dd`; omitted type relies on upstream `page`; omitted limit sends `25` | Redacted upstream collection object, preserving `results`, `start`, `limit`, `size`, links, expansions, and unknown fields |
| `confluence_list_content_properties` | Required `contentId`; optional `expand`, `start`, `limit` | `contentId` is trimmed, non-empty, and exactly one URL path segment; start is non-negative and limit positive when supplied; omitted start is absent; omitted limit sends the section 9 default `10`; blank expand is absent so Confluence applies its native default | Redacted upstream property collection object, preserving `results`, `start`, `limit`, `size`, links, expansions, property `key`/`value`/`version`, and unknown fields |
| `confluence_get_content_property` | Required `contentId`, `key`; optional `expand` | Both path inputs are trimmed, non-empty, and exactly one URL path segment; blank expand is absent so Confluence applies its native default | Redacted upstream property object, including native `key`, arbitrary JSON `value`, `version`, links, expansions, and unknown fields without reshaping |
| `confluence_list_spaces` | Optional `spaceKey`, `type`, `status`, `label`, `expand`, `start`, `limit` | If supplied, type is `global` or `personal`; status is `current` or `archived`; omitted limit sends `25`; no implicit type/status/expand | Redacted upstream space collection object without reshaping |
| `confluence_get_space` | Required `spaceKey`; optional `expand` | `spaceKey` is trimmed, non-empty, and exactly one URL path segment | Redacted upstream space object without reshaping |
| `confluence_list_space_content` | Required `spaceKey`; optional `depth`, `expand`, `start`, `limit` | Safe `spaceKey`; if supplied, depth is `all` or `root`; omitted depth relies on upstream `all`; omitted limit sends `25` | Redacted upstream grouped content object (for example `page` and `blogpost`) without flattening or typed-route emulation |

`status` on `confluence_list_content` remains a passthrough string because the
authoritative row does not define a closed enum. Invalid values are left for
Confluence's own endpoint validation rather than inventing a narrower MCP
contract.

For both Content Property tools, authentication is checked before local input
validation, matching every existing read. Missing sessions return
`CONFLUENCE_NOT_AUTHENTICATED`; invalid path segments or collection paging
return `VALIDATION_ERROR`; all such failures make zero HTTP requests. Upstream
HTTP errors continue through `confluenceClientError`, including the existing
neutral missing-or-not-visible mapping for 404, while transport failures remain
`UPSTREAM_UNREACHABLE`. Success data remains the redacted upstream map directly
in the shared result envelope.

## Approach

Use the smallest coherent extension of the existing package:

1. Add one shared pagination validator in `service.go` and use it from existing
   CQL search plus the four new paged handlers. It accepts the endpoint's
   documented default (`25` or `10`), validates the common trust boundary once,
   and returns the effective limit rather than duplicating the rule.
2. Keep authentication, `Service`, `requireCredential`, `cleanPathSegment`,
   query encoding, and error mapping in `service.go`. Move existing content
   schemas/handlers unchanged in behavior to `content.go`, then add list
   content and the two Content Property reads there.
3. Add `spaces.go` for list/get/content-in-space schemas and handlers. Build
   paths only from fixed literals plus `cleanPathSegment`; never accept a raw
   URL or arbitrary REST path.
4. Preserve the first three definition positions, append list content, then the
   property collection/item definitions, then the three space definitions.
   Register them through group methods so source organization matches
   Bitbucket while existing clients see no renamed or reordered pre-existing
   tool.
5. Extend the existing focused test file rather than creating parallel fixture
   or helper layers. Update only current product, architecture, and security
   documentation whose roster statement becomes stale.

## Affected Files And Symbols

Create:

- `internal/confluence/tools/content.go`
  - `SearchContentInput`, `GetContentInput`, and their existing handlers moved
    from `service.go` without MCP contract changes.
  - `ListContentInput` and `(*Service).ListContent`.
  - `ListContentPropertiesInput`, `GetContentPropertyInput`,
    `(*Service).ListContentProperties`, and
    `(*Service).GetContentProperty`; these remain in the content resource group
    rather than introducing a property package or service.
- `internal/confluence/tools/spaces.go`
  - `ListSpacesInput`, `GetSpaceInput`, `ListSpaceContentInput`.
  - `(*Service).ListSpaces`, `(*Service).GetSpace`, and
    `(*Service).ListSpaceContent`.

Modify:

- `internal/confluence/tools/service.go`
  - Retain `Service`, authentication/session behavior, path/query helpers, and
    `confluenceClientError`.
  - Add `validatedPage(tool string, start, limit *int, defaultLimit int) (int,
    *result.Envelope)` returning the explicit limit or the caller's documented
    default (`25` for current collections, `10` for content properties).
  - Remove only the content types and methods moved verbatim to `content.go`.
- `internal/confluence/tools/register.go`
  - Expand `Definitions` to the exact nine-tool order:
    `confluence_authenticate`, `confluence_search_content`,
    `confluence_get_content`, `confluence_list_content`,
    `confluence_list_content_properties`,
    `confluence_get_content_property`, `confluence_list_spaces`,
    `confluence_get_space`, `confluence_list_space_content`.
  - Keep `result.MustOutputSchema()` on every definition.
  - Set `ReadOnlyHint=true` and `OpenWorldHint=&open` on both property
    definitions, matching every data read; do not set a destructive annotation.
  - Add `registerAuthenticationTool`, `registerContentTools`, and
    `registerSpaceTools`; `Register` delegates in that order.
- `internal/confluence/tools/tools_test.go`
  - Preserve current tests and extend them with the matrix below.
- `README.md`
  - Replace the three-tool roster sentence with the exact nine-tool core read
    roster and retain the link to detailed Confluence tool docs.
- `docs/tools/confluence.md`
  - Add all inputs, defaults, endpoint paths, authentication requirement,
    output preservation, and explicit exclusions.
- `docs/architecture.md`
  - Replace the content-search/content-ID-only statement with the exact core
    content/space GET boundary and grouped handler layout.
- `docs/security.md`
  - Keep the separate in-memory Confluence session and read-only data-tool
    posture current for the nine-tool roster; no security behavior changes.
- This plan
  - Update Progress, Decisions, Validation, and Result during execution; move
    it to `docs/plans/completed/` only after completion evidence is recorded.

Explicitly unchanged:

- `internal/confluence/client/*`, `internal/confluence/module.go`, configuration,
  shared authentication, installers, the REST reference, and
  `docs/plans/completed/build-confluence-read-search-module.md`.

## Implementation Order (Historical, Completed)

The sequence below records completed implementation work. Promotion of this
plan is the only pending lifecycle action and remains with the main agent after
review.

- [x] Add focused failing tests for the exact nine-tool roster, annotations,
  descriptions, and absence of typed space-content or mutating Content Property
  definitions; run
  `go test ./internal/confluence/tools` and record the expected failures.
- [x] Add table-driven failing tests that call each of the six new handlers before
  authentication and prove `CONFLUENCE_NOT_AUTHENTICATED` with zero HTTP
  requests.
- [x] Add failing request-contract tests for list content, including GET path,
  all query names, URL encoding, omitted start/type, explicit default limit,
  response-shape preservation, and no-request failures for invalid paging,
  type, and posting day.
- [x] Add failing request-contract tests for the Content Property collection and
  item reads: exact GET paths, safe `contentId`/`key`, optional `expand`,
  collection paging and explicit default `limit=10`, direct output preservation,
  and zero requests for every local validation failure.
- [x] Add failing request-contract tests for list/get spaces and list space
  content, covering GET paths, safe path segments, filters, enum validation,
  defaults, paging, grouped-output preservation, and zero requests on local
  validation failure.
- [x] Refactor existing content types and handlers into `content.go`; add
  `validatedPage` and switch `SearchContent` to it. Run existing focused tests
  before adding new behavior to prove the move is contract-neutral.
- [x] Implement `ListContent` with only the validated documented query set and
  direct redacted upstream output; run its focused tests.
- [x] Implement `ListContentProperties` and `GetContentProperty` in `content.go`
  with fixed section 9 GET paths, shared safe-segment/paging validation, and
  direct redacted upstream output; run their focused tests.
- [x] Implement the three space handlers with fixed endpoint paths, shared path
  and paging validation, documented enum checks, and direct redacted upstream
  output; run their focused tests.
- [x] Expand definitions and registration by resource group to the exact
  nine-tool order, preserving the original first three positions; run roster
  and handler registration tests.
- [x] Run `gofmt` only on changed Go files, then run focused Confluence package
  tests.
- [x] Update `README.md`, `docs/tools/confluence.md`, `docs/architecture.md`,
  and `docs/security.md`; search current docs for stale claims that Confluence
  has exactly three tools or only search/content-by-ID reads.
- [x] Run repository-wide validation and, when a configured real 6.1.2 host is
  available, the staging checks below. Record commands, outcomes, and any
  unavailable proof in this plan.
- [x] Review the complete diff against Scope and Tool Contract, update Result,
  and record the available definition-of-done evidence.
- [x] Main agent only: after review acceptance, move this file to
  `docs/plans/completed/`.

## API, Compatibility, Configuration, And Deployment

- API compatibility: use only the 6.1.2-compatible paths and parameters listed
  in the authoritative reference. Do not infer Cloud-era account, cursor, or
  REST v2 fields.
- Permission compatibility: both property reads use the caller's existing View
  content permission. Do not add a privilege probe; Confluence remains the
  authority and missing/not-visible responses retain neutral handling.
- MCP compatibility: the change is additive. Existing names, input JSON field
  names, response envelopes, authentication errors, and first three definition
  positions remain unchanged.
- Output compatibility: JSON is decoded into `map[string]any`, passed through
  `observability.Redact`, and returned directly as envelope data. Do not flatten
  collections, rename keys, discard links/expandables, or synthesize a common
  page type.
- Pagination compatibility: send an explicit `limit=25` on core content/space
  collections and `limit=10` on the Content Property collection; omit `start`
  when not supplied. Return Confluence paging metadata unchanged; do not
  auto-follow pages.
- Configuration and dependencies: none. Reuse the enabled module, shared HTTP
  client configuration, response cap, CA handling, and standard library date
  parsing/query encoding.
- Migration and deployment: there is no data or configuration migration. A
  normal rebuild/restart exposes the additive definitions; existing in-memory
  Confluence sessions are process-local and are naturally re-established by
  the current startup or explicit authentication path.

## Risks And Recovery

- Reorganization could accidentally change existing schemas or tool order.
  Preserve exported type/method names in the same package, keep the first three
  definitions in their current order, and run existing tests immediately after
  the move.
- Numeric definition indexes can drift from registrations. Keep definitions
  and group registrations adjacent, prove exact names/count through tests, and
  manually review each index-to-handler pair.
- Over-validation could reject values accepted by Confluence. Validate only
  enums/formats explicitly closed by the reference; keep content status and
  expand as passthrough strings.
- Unsafe path composition could permit endpoint escape. Reuse
  `cleanPathSegment` for every `spaceKey`, property `contentId`, and property
  `key`; queries continue through `net/url`.
- Property values are arbitrary JSON and property objects may gain fields.
  Preserve the upstream map; do not add value DTOs, version normalization, or
  property-key enumeration that would narrow the 6.1.x contract.
- List responses can be large. Retain `ATLASSIAN_MAX_RESPONSE_BYTES`, explicit
  default paging, and no auto-pagination.
- A host may collapse missing and invisible objects to 404. Preserve the
  existing neutral `NOT_FOUND_OR_NOT_VISIBLE` mapping and do not expose
  permission details not provided upstream.
- Recovery is a code/docs rollback only: revert the six definitions, grouped
  handlers, tests, and matching documentation as one coherent change. No
  remote data or credentials are changed by these GET-only tools.

## Progress

- [x] User resolved scope to core Confluence content and space reads.
- [x] Workflow, current code/tests/docs, Bitbucket grouping pattern, REST
  authority, and historical completed plan inspected.
- [x] Exact endpoint/input/default/output contract recorded.
- [x] Typed space-content endpoint deliberately excluded as redundant for core
  scope.
- [x] Scope revised before approval to include only the two section 9 Content
  Property GET endpoints; all property mutations remain excluded.
- [x] Failing focused tests added and observed.
- [x] Handler reorganization and six new tools implemented.
- [x] Current documentation updated.
- [x] Focused, repository-wide, static, build, and diff validation passed.
- [x] Real-host validation completed or its absence explicitly recorded.
- [x] Result recorded; plan retained in active per current implementation handoff.
- [x] Remediation CR-001: Added MCP-boundary registration coverage proving the
  public registered Confluence tools expose the nine intended names, input
  schemas, required fields, output schema, and endpoint-distinguishing
  name-to-handler bindings.
- [x] Remediation CR-002: Converted the pre-implementation baseline to strictly
  historical language, separated current executable truth including
  `content.go` and `spaces.go`, and reconciled Implementation Order with the
  completed Progress and Result record. Main-agent review and plan promotion
  remain pending.

## Decisions

- 2026-08-09: Keep the exact existing authentication/search/get contracts and
  add six tools; this is an additive core-read expansion, not a Confluence V2
  redesign.
- 2026-08-09: Exclude `/space/{spaceKey}/content/{type}`. The untyped endpoint
  meets the required core capability and preserves Confluence's grouped
  response; add a typed tool only after a concrete caller requires independent
  per-type paging.
- 2026-08-09: Send `limit=25` explicitly for all paged tools, matching the
  existing search behavior and documented server default, while leaving
  `type=page` and `depth=all` implicit.
- 2026-08-09: Validate only documented closed values: content type, posting-day
  format, space type/status, depth, paging, and path segments. Other query
  strings remain native passthroughs.
- 2026-08-09: Use two resource files plus the existing shared service and test
  file. Additional per-tool packages, interfaces, response DTOs, pagination
  frameworks, and client abstractions are unnecessary.
- 2026-08-09: No new ADR is required. The change applies the existing GET-only
  client and separate-session architecture without introducing a new lasting
  architectural policy.
- 2026-08-09: Add exactly
  `confluence_list_content_properties` and
  `confluence_get_content_property` for the two REST reference section 9 GET
  rows. Keep both handlers in `content.go`; no separate property abstraction is
  justified.
- 2026-08-09: Use the documented property collection default `limit=10` and
  rely on Confluence's native expansion default when `expand` is omitted. Do
  not copy section 9 POST/PUT/DELETE capabilities into the MCP surface.
- 2026-08-09: Keep the Confluence REST client unchanged. The six new tools can
  use its existing bounded, Basic-authenticated `GetJSON` method and fixed
  handler paths without adding a generic endpoint or non-GET surface.

## Testing And Validation

Focused test matrix:

| Behavior | Proof |
| --- | --- |
| Exact roster | `Definitions()` contains exactly the nine ordered names; auth remains open-world but not read-only; all eight reads are read-only/open-world and mention the authenticated session |
| No hidden surface | Roster omits typed space content, every Content Property mutation, and every other excluded category; no new client method can issue a non-GET request |
| Authentication gate | Table-driven calls to all six new handlers before auth return `CONFLUENCE_NOT_AUTHENTICATED` and leave server call count zero |
| List content request | Assert `GET /rest/api/content`; all supplied filters are encoded; omitted `start`/`type` are absent; omitted limit is `25` |
| List content validation | Negative start, zero/negative limit, unsupported type, and invalid posting day return `VALIDATION_ERROR` with zero requests |
| List content output | Preserve collection paging, `_links`, `_expandable`, and an unknown sentinel field after redaction |
| List content properties request | Assert `GET /rest/api/content/12345/property`; encode optional expand/start/limit; omit blank expand and absent start; omitted limit is explicitly `10` |
| List content properties validation/output | Blank or delimiter-bearing `contentId`, negative start, and zero/negative limit return `VALIDATION_ERROR` with zero requests; preserve `results`, arbitrary property values, versions, paging, links, expansions, and an unknown sentinel field |
| Get content property | Assert `GET /rest/api/content/12345/property/build` plus optional expand; blank or delimiter-bearing `contentId`/`key` returns `VALIDATION_ERROR` with zero requests; preserve arbitrary value/version/link/unknown fields |
| Content property error mapping | Representative upstream 404 remains the neutral existing missing-or-not-visible error; other HTTP/transport failures use the current sanitized envelope without exposing upstream credentials or raw internals |
| List spaces request | Assert `GET /rest/api/space`; encode `spaceKey`, type, status, label, expand, paging; omitted limit is `25` |
| List spaces validation | Unsupported type/status and invalid paging return `VALIDATION_ERROR` with zero requests |
| Get space | Assert `GET /rest/api/space/ENG` plus optional expand; blank or delimiter-bearing `spaceKey` returns `VALIDATION_ERROR` with zero requests |
| List space content | Assert `GET /rest/api/space/ENG/content`; depth/expand/paging encode correctly; omitted depth is absent and omitted limit is `25` |
| Space-content validation/output | Unsupported depth or unsafe key sends no request; grouped `page`/`blogpost`, links, and sentinel fields remain unchanged |
| Existing contracts | Existing authenticate, search, get-content, session-race, module, client, and context-path tests continue to pass |

Focused proof:

- `go test ./internal/confluence/tools ./internal/confluence/...`
- If the nested package pattern causes duplicate execution, prefer the single
  equivalent `go test ./internal/confluence/...` and record that exact command.

Integration or real-host proof when Confluence 6.1.2 credentials are available:

1. Authenticate through the existing safe environment-backed flow.
2. Call `confluence_list_content` with `limit=1` and one benign filter.
3. Use a returned content ID to call
   `confluence_list_content_properties` with `limit=1`; if a property exists,
   call `confluence_get_content_property` with its returned key. An empty
   collection is valid evidence for the collection contract and does not
   authorize creating test data.
4. Call `confluence_list_spaces` with `limit=1`; pass a returned key to
   `confluence_get_space`.
5. Call `confluence_list_space_content` for that key with `depth=root` and
   `limit=1`.
6. Confirm GET-only behavior, expected native response shapes, context-path/TLS
   operation, neutral missing/not-visible handling, and absence of credential
   material in diagnostics. Do not create or mutate remote content.

Repository-required checks:

- `go test ./...`
- `go vet ./...`
- `go build -o .tmp/atlassian-mcp.exe ./cmd/atlassian-mcp`
- `.tmp/atlassian-mcp.exe --version`
- `git diff --check`
- `rg -n "exactly three|exactly seven|only .*search|content-by-ID reads only|confluence_(list_content|list_content_properties|get_content_property|list_spaces|get_space|list_space_content)" README.md docs internal/confluence`
- Final diff review proving no changes outside this plan's affected files and no
  mutation/generic/typed endpoint surface.

Executed validation, 2026-08-09:

- `go test ./internal/confluence/tools` after adding tests failed as expected
  at compile time because `ListContent`, `ListContentProperties`,
  `GetContentProperty`, `ListSpaces`, `GetSpace`, `ListSpaceContent`, and their
  input types did not exist yet.
- `go test ./internal/confluence/tools` passed after implementation.
- `go test ./internal/confluence/tools ./internal/confluence/...` passed.
- `go test ./...` passed.
- `go test -race ./internal/confluence/...` passed.
- `go vet ./...` passed.
- `go build -o .tmp\atlassian-mcp.exe ./cmd/atlassian-mcp` passed.
- `.\.tmp\atlassian-mcp.exe --version` returned `atlassian-mcp 0.1.0`.
- `git diff --check` exited 0. Git emitted only local line-ending warnings for
  modified text files.
- `rg -n "exactly three|exactly seven|only .*search|content-by-ID reads only|read/search only|confluence_(list_content|list_content_properties|get_content_property|list_spaces|get_space|list_space_content)" README.md docs internal/confluence`
  completed. Remaining matches are the intended new roster, tests, spec rows,
  this active plan's pre-implementation context, the historical completed
  Confluence plan intentionally left unchanged, and an unrelated Jira docs line.
- Real-host Confluence 6.1.2 validation was not run because no live host
  credentials were provided in this implementation handoff.

Remediation validation, 2026-08-09:

- `go test ./internal/confluence/tools -run TestConfluenceRegisterExposesNineBoundMCPTools -count=1 -v`
  first failed for an SDK public-list ordering assumption; the test was corrected
  to assert the registered name set because `Definitions()` already covers
  definition order.
- Temporary mutation check: swapping the `confluence_search_content` and
  `confluence_get_content` positional bindings made
  `go test ./internal/confluence/tools -run TestConfluenceRegisterExposesNineBoundMCPTools -count=1 -v`
  fail with `required=[contentId], want [cql]`; the original binding was then
  restored.
- `go test ./internal/confluence/tools -run TestConfluenceRegisterExposesNineBoundMCPTools -count=1 -v`
  passed after restore.
- `go test ./internal/confluence/tools ./internal/confluence/...` passed.
- `go test ./...` passed.
- `go vet ./...` passed.
- `git diff --check` exited 0. Git emitted only local line-ending warnings for
  modified text files.
- CR-002 plan-only check: `git diff --check` exited 0. Git emitted only local
  line-ending warnings for modified text files.
- Latest CR-002 plan-state consistency inspection passed: the historical
  baseline contains no present-tense `now` or `current` state claims, every
  completed Implementation Order item is checked, the sole unchecked item is
  main-agent promotion, current executable truth names both `content.go` and
  `spaces.go`, and the live-host limitation remains explicit.
- `git diff --check` exited 0, but it did not inspect this untracked plan. The
  targeted Markdown inspection separately found no trailing whitespace in the
  plan.

## Security, Operations, And Observability

- Credentials remain process-memory-only in the existing module-local session;
  this change adds no credential input, storage, logging, or sharing path.
- Each read still requires authenticated session state before input validation
  and network access. Failed pre-auth calls remain observable only as the stable
  sanitized result error.
- Basic Auth is generated by `http.Request.SetBasicAuth`; queries use `net/url`;
  upstream error detail and success JSON pass through existing redaction.
- `ATLASSIAN_MAX_RESPONSE_BYTES`, shared timeout/TLS policy, and sanitized
  error mapping remain the operational bounds. No retry, auto-pagination,
  cache, metric, or new log is justified for this additive read surface.
- Tool annotations remain the client-facing safety signal: all eight data tools
  are read-only; authentication remains explicit sensitive setup/recovery and
  is not mislabeled read-only.

## Assumptions And Open Questions

Verified non-blocking assumptions: callers needing page-only or blogpost-only
space traversal can use `confluence_list_content` with `spaceKey` plus `type`;
Content Property callers need the native stored JSON value and version rather
than an MCP-specific typed value. No blocking product or API question remains
for the resolved scope.

## Definition Of Done

- Exactly nine tools are registered with the contract and annotations above.
- The six new endpoints work through the existing authenticated, bounded,
  GET-only client; existing three-tool behavior remains compatible.
- Confluence tool sources are grouped into shared service, content, and spaces
  responsibilities without extra abstraction or dependency.
- Focused and repository validation passes, or any unavailable external proof
  is named without overstating compatibility.
- README, tool docs, and architecture truth match the implemented roster;
  security truth remains accurate.
- Only the two section 9 GET routes are exposed for Content Properties; all
  POST, PUT, and DELETE property routes remain absent from definitions,
  registrations, handlers, and documentation.
- The completed historical plan and every out-of-scope endpoint remain
  untouched.
- This plan records final evidence and limitations, then moves to
  `docs/plans/completed/`.

## Result

Implemented the approved nine-tool Confluence V1 roster. The first three tools
remain ordered as `confluence_authenticate`, `confluence_search_content`, and
`confluence_get_content`; the six added tools are
`confluence_list_content`, `confluence_list_content_properties`,
`confluence_get_content_property`, `confluence_list_spaces`,
`confluence_get_space`, and `confluence_list_space_content`.

Confluence handlers are grouped into `content.go` and `spaces.go`, while
authentication, session gating, path validation, query helpers, paging
validation, and error mapping remain shared in `service.go`. The existing
Confluence client, module configuration, authentication lifecycle, bounded
response handling, redaction, and GET-only request method were not changed.

Focused tests cover exact roster/order/annotations, excluded tool names,
pre-auth zero-network behavior, documented paths and query defaults, local
validation failures, neutral 404 mapping, and direct upstream JSON preservation.
README, Confluence tools docs, architecture, and security documentation were
updated to match the expanded read-only content/space surface and exclusions.

Limitations: no live Confluence 6.1.2 host validation was performed in this
session. Automated validation and independent review passed; the plan is
promoted to `docs/plans/completed/` as the final lifecycle action.
