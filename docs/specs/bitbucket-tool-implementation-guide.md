# Bitbucket Tool Implementation Guide — Server 5.10.2

## 1. Authority and use

This guide is the endpoint-level implementation contract for all 26 Bitbucket MCP tools. It supplements the task plan; when a task summary is less specific, this guide controls. It is pinned to Bitbucket Server 5.10.2.

Source order:

1. The exact 5.10.2 resource in the official Atlassian REST reference.
2. The stable local anchors in `../references/jira-6.4.14_bitbucket-5.10.2-rest-api-reference.md`.
3. Explicit MCP restrictions and safety decisions in this guide.
4. A real-host contract test for details that the published reference does not fully define.

An implementation agent must stop and record a specification question rather than inventing a request field, enum, response shape, permission, retry, or error mapping not covered by those sources.

## 2. Cross-cutting HTTP contract

- Prefix every resource path below with the configured base URL/context path and `/rest/api/1.0` exactly once.
- Send `Authorization: Bearer <BITBUCKET_BEARER_TOKEN>` and `Accept: application/json` for JSON resources. Raw-file reads accept arbitrary successful media types. Multipart file commit sets the generated multipart boundary.
- Never accept caller-supplied upstream URLs, project keys, auth headers, or arbitrary headers.
- For genuine paged APIs preserve `size`, `limit`, `isLastPage`, `start`, `values`, and `nextPageStart`; the next call must use `nextPageStart` rather than arithmetic.
- Preserve the upstream `errors[]` envelope after secret sanitization. Map 400 validation, 401 auth/permission, 404 absence, and 409 conflict/state/version with endpoint context.
- Bounded retry is allowed only for read requests under the shared policy. No mutation in this guide is blindly replayed.
- Upstream hard caps and MCP response-size caps are different signals and must be represented separately.
- All input schemas require `repositorySlug`, use `additionalProperties:false`, and validate path/ref/ID fields before network I/O.

## 3. Per-tool contracts

<a id="tool-bitbucket_get_repository"></a>
### 3.1 `bitbucket_get_repository`

- **MCP purpose:** Retrieve repository metadata for the configured project and caller-selected repository slug.
- **Source:** [bundled exact-resource anchor](../references/jira-6.4.14_bitbucket-5.10.2-rest-api-reference.md#bb-api-get-repository); [official Bitbucket Server 5.10.2 REST](https://docs.atlassian.com/bitbucket-server/rest/5.10.2/bitbucket-rest.html), resource heading `GET /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}`.
- **Method/path:** `GET /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}`.
- **Query/path inputs:** None. Do not add undocumented expansion or projection parameters.
- **Request headers:** `Authorization: Bearer …`; `Accept: application/json`.
- **Request body:** None.
- **Expected success:** 200 JSON repository.
- **Response preservation:** Preserve the complete repository object, including `slug`, `id`, `name`, `scmId`, `state`, `statusMessage`, `forkable`, `project`, `public`, and `links`.
- **Permission concept:** REPO_READ on the repository.
- **Error mapping / stop conditions:** Map repository absence to `BITBUCKET_REPOSITORY_NOT_FOUND`; permission/auth failures to the shared upstream authorization error.
- **Retry behavior:** Bounded read retry only for eligible transient failures; never retry 4xx.
- **Response-size / truncation:** Apply the shared bounded-reader policy. Do not silently truncate a JSON object; return the shared response-too-large error when a complete bounded representation cannot be produced.
- **MCP annotations:** `readOnlyHint=true; destructiveHint=false; idempotentHint=true`; mutation tools require client approval under the project policy.
- **Required tests:** Exact path construction; context path; 200 shape preservation; 401; 404; no request body; schema requires `repositorySlug`.
- **Test layers:** unit validation/serialization; request-recording HTTP contract; MCP schema/annotation snapshot; real Bitbucket 5.10.2 host where this section declares a staging gate.

<a id="tool-bitbucket_list_branches"></a>
### 3.2 `bitbucket_list_branches`

- **MCP purpose:** List repository branches with server-supported filtering, ordering, metadata, and cursor paging.
- **Source:** [bundled exact-resource anchor](../references/jira-6.4.14_bitbucket-5.10.2-rest-api-reference.md#bb-api-list-branches); [official Bitbucket Server 5.10.2 REST](https://docs.atlassian.com/bitbucket-server/rest/5.10.2/bitbucket-rest.html), resource heading `GET /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/branches`.
- **Method/path:** `GET /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/branches`.
- **Query/path inputs:** Optional `base`, `details`, `filterText`, `orderBy`, `start`, `limit`. Validate `orderBy` as `ALPHABETICAL` or `MODIFICATION`; preserve booleans as query booleans.
- **Request headers:** `Authorization: Bearer …`; `Accept: application/json`.
- **Request body:** None.
- **Expected success:** 200 JSON page.
- **Response preservation:** Preserve page metadata and branch objects. Return upstream `nextPageStart`; never calculate `start + limit`.
- **Permission concept:** REPO_READ.
- **Error mapping / stop conditions:** Reject unsupported order values before network I/O. Do not auto-fetch all pages.
- **Retry behavior:** Bounded read retry only for eligible transient failures; never retry 4xx.
- **Response-size / truncation:** Apply the shared bounded-reader policy. Do not silently truncate a JSON object; return the shared response-too-large error when a complete bounded representation cannot be produced.
- **MCP annotations:** `readOnlyHint=true; destructiveHint=false; idempotentHint=true`; mutation tools require client approval under the project policy.
- **Required tests:** Every query independently and combined; both order values; bad enum; `nextPageStart` non-contiguous cursor; 401/404; page schema.
- **Test layers:** unit validation/serialization; request-recording HTTP contract; MCP schema/annotation snapshot; real Bitbucket 5.10.2 host where this section declares a staging gate.

<a id="tool-bitbucket_get_default_branch"></a>
### 3.3 `bitbucket_get_default_branch`

- **MCP purpose:** Retrieve the configured default branch without treating an empty repository as malformed JSON.
- **Source:** [bundled exact-resource anchor](../references/jira-6.4.14_bitbucket-5.10.2-rest-api-reference.md#bb-api-get-default-branch); [official Bitbucket Server 5.10.2 REST](https://docs.atlassian.com/bitbucket-server/rest/5.10.2/bitbucket-rest.html), resource heading `GET /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/branches/default`.
- **Method/path:** `GET /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/branches/default`.
- **Query/path inputs:** None.
- **Request headers:** `Authorization: Bearer …`; `Accept: application/json`.
- **Request body:** None.
- **Expected success:** 200 JSON branch; 204 empty body when the repository has no default branch.
- **Response preservation:** For 200 preserve the branch object. For 204 return a successful typed empty-repository result such as `{repositoryEmpty:true, defaultBranch:null}` and no JSON parse attempt.
- **Permission concept:** REPO_READ.
- **Error mapping / stop conditions:** Do not map 204 to generic upstream failure.
- **Retry behavior:** Bounded read retry only for eligible transient failures; never retry 4xx.
- **Response-size / truncation:** Apply the shared bounded-reader policy. Do not silently truncate a JSON object; return the shared response-too-large error when a complete bounded representation cannot be produced.
- **MCP annotations:** `readOnlyHint=true; destructiveHint=false; idempotentHint=true`; mutation tools require client approval under the project policy.
- **Required tests:** 200 branch; 204 empty body; 401; 404; assert zero decoder invocation on 204.
- **Test layers:** unit validation/serialization; request-recording HTTP contract; MCP schema/annotation snapshot; real Bitbucket 5.10.2 host where this section declares a staging gate.

<a id="tool-bitbucket_create_branch"></a>
### 3.4 `bitbucket_create_branch`

- **MCP purpose:** Create one branch from a supplied commit ID or ref.
- **Source:** [bundled exact-resource anchor](../references/jira-6.4.14_bitbucket-5.10.2-rest-api-reference.md#bb-api-create-branch); [official Bitbucket Server 5.10.2 REST](https://docs.atlassian.com/bitbucket-server/rest/5.10.2/bitbucket-rest.html), resource heading `POST /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/branches`.
- **Method/path:** `POST /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/branches`.
- **Query/path inputs:** None.
- **Request headers:** `Authorization: Bearer …`; `Content-Type: application/json`; `Accept: application/json`.
- **Request body:** JSON `{ "name": string, "startPoint": string, "message"?: string }`. Preserve field names exactly.
- **Expected success:** 200 JSON branch.
- **Response preservation:** Preserve created branch fields including `id`, `displayId`, `type`, `latestCommit`, `latestChangeset`, and `isDefault`.
- **Permission concept:** REPO_WRITE.
- **Error mapping / stop conditions:** Send exactly one POST. Map validation/conflict details without collapsing invalid start point, duplicate branch, and permission errors into one message.
- **Retry behavior:** No blind retry. At most one mutation request; a follow-up GET is allowed only where explicitly stated.
- **Response-size / truncation:** Apply the shared bounded-reader policy. Do not silently truncate a JSON object; return the shared response-too-large error when a complete bounded representation cannot be produced.
- **MCP annotations:** `readOnlyHint=false; destructiveHint=false; idempotentHint=false`; mutation tools require client approval under the project policy.
- **Required tests:** Exact JSON; optional message omitted vs present; 200; invalid start point; duplicate; 401/404; ambiguous network response has one POST; write annotations.
- **Test layers:** unit validation/serialization; request-recording HTTP contract; MCP schema/annotation snapshot; real Bitbucket 5.10.2 host where this section declares a staging gate.

<a id="tool-bitbucket_get_file"></a>
### 3.5 `bitbucket_get_file`

- **MCP purpose:** Retrieve exact raw bytes for one path at an optional revision.
- **Source:** [bundled exact-resource anchor](../references/jira-6.4.14_bitbucket-5.10.2-rest-api-reference.md#bb-api-get-file); [official Bitbucket Server 5.10.2 REST](https://docs.atlassian.com/bitbucket-server/rest/5.10.2/bitbucket-rest.html), resource heading `GET /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/raw/{path:.*}`.
- **Method/path:** `GET /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/raw/{path:.*}`.
- **Query/path inputs:** Optional `at`. The MCP tool intentionally does not expose `markup`, `hardwrap`, or `htmlEscape`, because it returns source bytes rather than rendered markup.
- **Request headers:** `Authorization: Bearer …`; raw response-capable `Accept` header.
- **Request body:** None.
- **Expected success:** 200 raw response body; upstream JSON error bodies for failures.
- **Response preservation:** Return bytes as UTF-8 text only when valid and requested; otherwise base64. Include `encoding`, byte length, upstream content type when present, and revision/path metadata.
- **Permission concept:** REPO_READ.
- **Error mapping / stop conditions:** Bound the body before decoding; never log content; distinguish an upstream non-JSON success body from JSON errors.
- **Retry behavior:** Bounded read retry only for eligible transient failures; never retry 4xx.
- **Response-size / truncation:** Apply the shared bounded-reader policy. Do not silently truncate a JSON object; return the shared response-too-large error when a complete bounded representation cannot be produced.
- **MCP annotations:** `readOnlyHint=true; destructiveHint=false; idempotentHint=true`; mutation tools require client approval under the project policy.
- **Required tests:** Text, empty, binary, invalid UTF-8, base64, oversized response, encoded nested path, `at`, 401/404, content absent from logs.
- **Test layers:** unit validation/serialization; request-recording HTTP contract; MCP schema/annotation snapshot; real Bitbucket 5.10.2 host where this section declares a staging gate.

<a id="tool-bitbucket_list_commits"></a>
### 3.6 `bitbucket_list_commits`

- **MCP purpose:** List commits constrained by refs, path, merge policy, rename behavior, and cursor paging.
- **Source:** [bundled exact-resource anchor](../references/jira-6.4.14_bitbucket-5.10.2-rest-api-reference.md#bb-api-list-commits); [official Bitbucket Server 5.10.2 REST](https://docs.atlassian.com/bitbucket-server/rest/5.10.2/bitbucket-rest.html), resource heading `GET /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/commits`.
- **Method/path:** `GET /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/commits`.
- **Query/path inputs:** Optional `followRenames`, `ignoreMissing`, `merges`, `path`, `since`, `until`, `withCounts`, `start`, `limit`. Pass `since` as exclusive lower bound and `until` as inclusive upper bound. Validate `merges` against the values documented by the 5.10.2 server/reference used by the project.
- **Request headers:** `Authorization: Bearer …`; `Accept: application/json`.
- **Request body:** None.
- **Expected success:** 200 JSON page.
- **Response preservation:** Preserve commit objects, paging fields, and optional count properties. Use only upstream `nextPageStart`.
- **Permission concept:** REPO_READ.
- **Error mapping / stop conditions:** Do not silently omit `ignoreMissing`; do not infer branch semantics for SHA values.
- **Retry behavior:** Bounded read retry only for eligible transient failures; never retry 4xx.
- **Response-size / truncation:** Apply the shared bounded-reader policy. Do not silently truncate a JSON object; return the shared response-too-large error when a complete bounded representation cannot be produced.
- **MCP annotations:** `readOnlyHint=true; destructiveHint=false; idempotentHint=true`; mutation tools require client approval under the project policy.
- **Required tests:** All eight endpoint-specific params; invalid merge enum; missing refs with both `ignoreMissing` values; counts; path/rename combination; cursor; 400/401/404.
- **Test layers:** unit validation/serialization; request-recording HTTP contract; MCP schema/annotation snapshot; real Bitbucket 5.10.2 host where this section declares a staging gate.

<a id="tool-bitbucket_get_commit"></a>
### 3.7 `bitbucket_get_commit`

- **MCP purpose:** Retrieve one commit by commit ID/SHA.
- **Source:** [bundled exact-resource anchor](../references/jira-6.4.14_bitbucket-5.10.2-rest-api-reference.md#bb-api-get-commit); [official Bitbucket Server 5.10.2 REST](https://docs.atlassian.com/bitbucket-server/rest/5.10.2/bitbucket-rest.html), resource heading `GET /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/commits/{commitId}`.
- **Method/path:** `GET /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/commits/{commitId}`.
- **Query/path inputs:** None.
- **Request headers:** `Authorization: Bearer …`; `Accept: application/json`.
- **Request body:** None.
- **Expected success:** 200 JSON commit.
- **Response preservation:** Preserve full commit metadata, IDs, parent commits, author/committer and timestamps.
- **Permission concept:** REPO_READ.
- **Error mapping / stop conditions:** Treat `commitId` as a commit identifier. Do not automatically rewrite it to `refs/heads/...` or assume it is a branch.
- **Retry behavior:** Bounded read retry only for eligible transient failures; never retry 4xx.
- **Response-size / truncation:** Apply the shared bounded-reader policy. Do not silently truncate a JSON object; return the shared response-too-large error when a complete bounded representation cannot be produced.
- **MCP annotations:** `readOnlyHint=true; destructiveHint=false; idempotentHint=true`; mutation tools require client approval under the project policy.
- **Required tests:** Full SHA and abbreviated ID passthrough; URL encoding; 200; invalid ID; 401/404; read-only schema.
- **Test layers:** unit validation/serialization; request-recording HTTP contract; MCP schema/annotation snapshot; real Bitbucket 5.10.2 host where this section declares a staging gate.

<a id="tool-bitbucket_get_commit_changes"></a>
### 3.8 `bitbucket_get_commit_changes`

- **MCP purpose:** Retrieve changed paths for a commit, optionally against a supplied earlier commit.
- **Source:** [bundled exact-resource anchor](../references/jira-6.4.14_bitbucket-5.10.2-rest-api-reference.md#bb-api-get-commit-changes); [official Bitbucket Server 5.10.2 REST](https://docs.atlassian.com/bitbucket-server/rest/5.10.2/bitbucket-rest.html), resource heading `GET /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/commits/{commitId}/changes`.
- **Method/path:** `GET /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/commits/{commitId}/changes`.
- **Query/path inputs:** Optional `since`, `withComments`, `limit`. Do not expose or promise usable subsequent-page traversal when the server hard cap is reached.
- **Request headers:** `Authorization: Bearer …`; `Accept: application/json`.
- **Request body:** None.
- **Expected success:** 200 JSON change page/envelope.
- **Response preservation:** Preserve change entries, page/envelope metadata, properties, and any indicators that the server limited results.
- **Permission concept:** REPO_READ.
- **Error mapping / stop conditions:** Bitbucket applies a server hard cap and documents that subsequent content cannot be requested. Distinguish `upstreamHardCap` from any MCP response cap.
- **Retry behavior:** Bounded read retry only for eligible transient failures; never retry 4xx.
- **Response-size / truncation:** Apply the shared bounded-reader policy. Preserve upstream cap/truncation independently from MCP-layer truncation.
- **MCP annotations:** `readOnlyHint=true; destructiveHint=false; idempotentHint=true`; mutation tools require client approval under the project policy.
- **Required tests:** With/without `since`; `withComments` true/false; hard-cap fixture; no fabricated `nextPageStart`; invalid commit; 401/404.
- **Test layers:** unit validation/serialization; request-recording HTTP contract; MCP schema/annotation snapshot; real Bitbucket 5.10.2 host where this section declares a staging gate.

<a id="tool-bitbucket_get_commit_diff"></a>
### 3.9 `bitbucket_get_commit_diff`

- **MCP purpose:** Retrieve a structured whole-commit or path-specific diff.
- **Source:** [bundled exact-resource anchor](../references/jira-6.4.14_bitbucket-5.10.2-rest-api-reference.md#bb-api-get-commit-diff); [official Bitbucket Server 5.10.2 REST](https://docs.atlassian.com/bitbucket-server/rest/5.10.2/bitbucket-rest.html), resource heading `GET /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/commits/{commitId}/diff/{path:.*}`.
- **Method/path:** `GET /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/commits/{commitId}/diff/{path:.*}`.
- **Query/path inputs:** Optional `autoSrcPath`, `contextLines`, `since`, `srcPath`, `whitespace`, `withComments`. Omit the trailing path segment for whole-commit diff; do not send a literal empty segment.
- **Request headers:** `Authorization: Bearer …`; `Accept: application/json`.
- **Request body:** None.
- **Expected success:** 200 JSON diff object.
- **Response preservation:** Preserve diff/file/hunk/segment/line objects and every upstream `truncated` flag.
- **Permission concept:** REPO_READ.
- **Error mapping / stop conditions:** The server hard-caps streamed lines and cannot serve subsequent pages. `autoSrcPath` and explicit `srcPath` behavior require contract tests for copied/moved files.
- **Retry behavior:** Bounded read retry only for eligible transient failures; never retry 4xx.
- **Response-size / truncation:** Apply the shared bounded-reader policy. Preserve upstream cap/truncation independently from MCP-layer truncation.
- **MCP annotations:** `readOnlyHint=true; destructiveHint=false; idempotentHint=true`; mutation tools require client approval under the project policy.
- **Required tests:** Whole/path diff; all query params; moved file; whitespace; comments; nested truncation flags; MCP cap separately; 400/401/404.
- **Test layers:** unit validation/serialization; request-recording HTTP contract; MCP schema/annotation snapshot; real Bitbucket 5.10.2 host where this section declares a staging gate.

<a id="tool-bitbucket_compare_commits"></a>
### 3.10 `bitbucket_compare_commits`

- **MCP purpose:** List commits reachable from `from` and not reachable from `to`, optionally using another repository in the configured project as the source.
- **Source:** [bundled exact-resource anchor](../references/jira-6.4.14_bitbucket-5.10.2-rest-api-reference.md#bb-api-compare-commits); [official Bitbucket Server 5.10.2 REST](https://docs.atlassian.com/bitbucket-server/rest/5.10.2/bitbucket-rest.html), resource heading `GET /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/compare/commits`.
- **Method/path:** `GET /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/compare/commits`.
- **Query/path inputs:** Required `from`, `to`; optional `fromRepo`, `start`, `limit`. MCP input `fromRepositorySlug` serializes to `fromRepo={BITBUCKET_PROJECT_KEY}/{slug}`; never accept arbitrary project, numeric ID, URL, or raw `fromRepo`.
- **Request headers:** `Authorization: Bearer …`; `Accept: application/json`.
- **Request body:** None.
- **Expected success:** 200 JSON page.
- **Response preservation:** Preserve commits and page metadata, using `nextPageStart`.
- **Permission concept:** REPO_READ for repositories required by the comparison.
- **Error mapping / stop conditions:** Cross-repository comparison is provider-scoped to the configured project.
- **Retry behavior:** Bounded read retry only for eligible transient failures; never retry 4xx.
- **Response-size / truncation:** Apply the shared bounded-reader policy. Do not silently truncate a JSON object; return the shared response-too-large error when a complete bounded representation cannot be produced.
- **MCP annotations:** `readOnlyHint=true; destructiveHint=false; idempotentHint=true`; mutation tools require client approval under the project policy.
- **Required tests:** Same repo; cross-repo serialization; reject URL/project injection; cursor; missing source/target refs; 401/404.
- **Test layers:** unit validation/serialization; request-recording HTTP contract; MCP schema/annotation snapshot; real Bitbucket 5.10.2 host where this section declares a staging gate.

<a id="tool-bitbucket_compare_changes"></a>
### 3.11 `bitbucket_compare_changes`

- **MCP purpose:** List changed paths between `from` and `to`, optionally across repositories in the configured project.
- **Source:** [bundled exact-resource anchor](../references/jira-6.4.14_bitbucket-5.10.2-rest-api-reference.md#bb-api-compare-changes); [official Bitbucket Server 5.10.2 REST](https://docs.atlassian.com/bitbucket-server/rest/5.10.2/bitbucket-rest.html), resource heading `GET /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/compare/changes`.
- **Method/path:** `GET /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/compare/changes`.
- **Query/path inputs:** Required `from`, `to`; optional `fromRepo`, `start`, `limit`; derive `fromRepo` only from `fromRepositorySlug` as described for compare commits.
- **Request headers:** `Authorization: Bearer …`; `Accept: application/json`.
- **Request body:** None.
- **Expected success:** 200 JSON change page/envelope.
- **Response preservation:** Preserve change objects and upstream pagination/cap metadata.
- **Permission concept:** REPO_READ.
- **Error mapping / stop conditions:** Do not assume all 5.10.2 installations expose identical maximums; staging verifies limit/cap behavior.
- **Retry behavior:** Bounded read retry only for eligible transient failures; never retry 4xx.
- **Response-size / truncation:** Apply the shared bounded-reader policy. Do not silently truncate a JSON object; return the shared response-too-large error when a complete bounded representation cannot be produced.
- **MCP annotations:** `readOnlyHint=true; destructiveHint=false; idempotentHint=true`; mutation tools require client approval under the project policy.
- **Required tests:** Same/cross repo; query encoding; change response; cap fixture; cursor when present; 400/401/404.
- **Test layers:** unit validation/serialization; request-recording HTTP contract; MCP schema/annotation snapshot; real Bitbucket 5.10.2 host where this section declares a staging gate.

<a id="tool-bitbucket_compare_diff"></a>
### 3.12 `bitbucket_compare_diff`

- **MCP purpose:** Retrieve a structured diff between two refs, optionally across repositories in the configured project.
- **Source:** [bundled exact-resource anchor](../references/jira-6.4.14_bitbucket-5.10.2-rest-api-reference.md#bb-api-compare-diff); [official Bitbucket Server 5.10.2 REST](https://docs.atlassian.com/bitbucket-server/rest/5.10.2/bitbucket-rest.html), resource heading `GET /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/compare/diff/{path:.*}`.
- **Method/path:** `GET /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/compare/diff/{path:.*}`.
- **Query/path inputs:** Required `from`, `to`; optional `fromRepo`, `srcPath`, `contextLines`, `whitespace`. Use no path suffix for whole comparison.
- **Request headers:** `Authorization: Bearer …`; `Accept: application/json`.
- **Request body:** None.
- **Expected success:** 200 JSON diff.
- **Response preservation:** Preserve structured diff and truncation metadata exactly.
- **Permission concept:** REPO_READ.
- **Error mapping / stop conditions:** The official 5.10.2 description and concrete host behavior must be contract-tested for response envelope and any hard cap; do not infer cursor behavior from a generic “paged” label alone.
- **Retry behavior:** Bounded read retry only for eligible transient failures; never retry 4xx.
- **Response-size / truncation:** Apply the shared bounded-reader policy. Preserve upstream cap/truncation independently from MCP-layer truncation.
- **MCP annotations:** `readOnlyHint=true; destructiveHint=false; idempotentHint=true`; mutation tools require client approval under the project policy.
- **Required tests:** Whole/path diff; cross repo; moved path; whitespace/context; upstream truncation; response-shape staging fixture; 400/401/404.
- **Test layers:** unit validation/serialization; request-recording HTTP contract; MCP schema/annotation snapshot; real Bitbucket 5.10.2 host where this section declares a staging gate.

<a id="tool-bitbucket_commit_file"></a>
### 3.13 `bitbucket_commit_file`

- **MCP purpose:** Create or update exactly one file and create one commit.
- **Source:** [bundled exact-resource anchor](../references/jira-6.4.14_bitbucket-5.10.2-rest-api-reference.md#bb-api-commit-file); [official Bitbucket Server 5.10.2 REST](https://docs.atlassian.com/bitbucket-server/rest/5.10.2/bitbucket-rest.html), resource heading `PUT /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/browse/{path:.*}`.
- **Method/path:** `PUT /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/browse/{path:.*}`.
- **Query/path inputs:** None.
- **Request headers:** `Authorization: Bearer …`; generated `Content-Type: multipart/form-data; boundary=…`; `Accept: application/json`.
- **Request body:** `multipart/form-data`: required `content`, `branch`; optional/contractual `message`, `sourceCommitId`, `sourceBranch`. The MCP accepts exactly one of text/base64 input, decodes it, then sends bytes in field `content`.
- **Expected success:** 200 JSON commit.
- **Response preservation:** Preserve returned commit; add MCP metadata `singleFileCommit:true`.
- **Permission concept:** REPO_WRITE.
- **Error mapping / stop conditions:** For update, require `sourceCommitId` under the project safety policy. For a new branch require `sourceBranch`. Map 409 details separately for existing file, unchanged content, and stale source commit. Send one PUT only; no blind retry.
- **Retry behavior:** No blind retry. At most one mutation request; a follow-up GET is allowed only where explicitly stated.
- **Response-size / truncation:** Apply the shared bounded-reader policy. Do not silently truncate a JSON object; return the shared response-too-large error when a complete bounded representation cannot be produced.
- **MCP annotations:** `readOnlyHint=false; destructiveHint=true; idempotentHint=false`; mutation tools require client approval under the project policy.
- **Required tests:** Create/update/new branch; multipart names and bytes; invalid/missing fields; stale commit; unchanged; existing file; one PUT under reset; content absent from logs.
- **Test layers:** unit validation/serialization; request-recording HTTP contract; MCP schema/annotation snapshot; real Bitbucket 5.10.2 host where this section declares a staging gate.

<a id="tool-bitbucket_list_pull_requests"></a>
### 3.14 `bitbucket_list_pull_requests`

- **MCP purpose:** List pull requests to/from a repository with native state/ref/order and participant filters.
- **Source:** [bundled exact-resource anchor](../references/jira-6.4.14_bitbucket-5.10.2-rest-api-reference.md#bb-api-list-pull-requests); [official Bitbucket Server 5.10.2 REST](https://docs.atlassian.com/bitbucket-server/rest/5.10.2/bitbucket-rest.html), resource heading `GET /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/pull-requests`.
- **Method/path:** `GET /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/pull-requests`.
- **Query/path inputs:** Optional `direction` (`INCOMING|OUTGOING`), `at` (fully qualified ref), `state` (`OPEN|DECLINED|MERGED|ALL`), `order` (`OLDEST|NEWEST`), `withAttributes`, `withProperties`, `start`, `limit`. Participant filters serialize consecutively as `username.N`, optional `role.N` (`AUTHOR|REVIEWER|PARTICIPANT`) and `approved.N`; N starts at 1, has no gaps, maximum 10.
- **Request headers:** `Authorization: Bearer …`; `Accept: application/json`.
- **Request body:** None.
- **Expected success:** 200 JSON page.
- **Response preservation:** Preserve PR IDs, `version`, state flags, refs, author/reviewers/participants, properties, attributes and page metadata.
- **Permission concept:** REPO_READ.
- **Error mapping / stop conditions:** Reject more than 10 participant filters rather than relying on the server to silently drop extras.
- **Retry behavior:** Bounded read retry only for eligible transient failures; never retry 4xx.
- **Response-size / truncation:** Apply the shared bounded-reader policy. Do not silently truncate a JSON object; return the shared response-too-large error when a complete bounded representation cannot be produced.
- **MCP annotations:** `readOnlyHint=true; destructiveHint=false; idempotentHint=true`; mutation tools require client approval under the project policy.
- **Required tests:** Every enum/default; fully qualified `at`; participant indexing/gaps/max; page cursor; malformed filters; 401/404.
- **Test layers:** unit validation/serialization; request-recording HTTP contract; MCP schema/annotation snapshot; real Bitbucket 5.10.2 host where this section declares a staging gate.

<a id="tool-bitbucket_get_pull_request"></a>
### 3.15 `bitbucket_get_pull_request`

- **MCP purpose:** Retrieve current pull-request state and optimistic-lock version.
- **Source:** [bundled exact-resource anchor](../references/jira-6.4.14_bitbucket-5.10.2-rest-api-reference.md#bb-api-get-pull-request); [official Bitbucket Server 5.10.2 REST](https://docs.atlassian.com/bitbucket-server/rest/5.10.2/bitbucket-rest.html), resource heading `GET /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/pull-requests/{pullRequestId}`.
- **Method/path:** `GET /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/pull-requests/{pullRequestId}`.
- **Query/path inputs:** None.
- **Request headers:** `Authorization: Bearer …`; `Accept: application/json`.
- **Request body:** None.
- **Expected success:** 200 JSON pull request.
- **Response preservation:** Preserve the full PR, especially `id`, `version`, `state`, `open`, `closed`, refs, participants and links.
- **Permission concept:** REPO_READ.
- **Error mapping / stop conditions:** Never strip or normalize away `version`; transition tools depend on it.
- **Retry behavior:** Bounded read retry only for eligible transient failures; never retry 4xx.
- **Response-size / truncation:** Apply the shared bounded-reader policy. Do not silently truncate a JSON object; return the shared response-too-large error when a complete bounded representation cannot be produced.
- **MCP annotations:** `readOnlyHint=true; destructiveHint=false; idempotentHint=true`; mutation tools require client approval under the project policy.
- **Required tests:** 200 full shape/version; numeric ID validation; 401/404; no body; read annotation.
- **Test layers:** unit validation/serialization; request-recording HTTP contract; MCP schema/annotation snapshot; real Bitbucket 5.10.2 host where this section declares a staging gate.

<a id="tool-bitbucket_get_pull_request_activities"></a>
### 3.16 `bitbucket_get_pull_request_activities`

- **MCP purpose:** Retrieve heterogeneous PR activity including comments, approvals, rescopes, merges and plugin-defined activity shapes.
- **Source:** [bundled exact-resource anchor](../references/jira-6.4.14_bitbucket-5.10.2-rest-api-reference.md#bb-api-get-pr-activities); [official Bitbucket Server 5.10.2 REST](https://docs.atlassian.com/bitbucket-server/rest/5.10.2/bitbucket-rest.html), resource heading `GET /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/pull-requests/{pullRequestId}/activities`.
- **Method/path:** `GET /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/pull-requests/{pullRequestId}/activities`.
- **Query/path inputs:** Optional `fromId`, `fromType`, `start`, `limit`. `fromType` is required when `fromId` is present and must be `COMMENT` or `ACTIVITY`.
- **Request headers:** `Authorization: Bearer …`; `Accept: application/json`.
- **Request body:** None.
- **Expected success:** 200 JSON page.
- **Response preservation:** Preserve unknown activity fields for forward/plugin compatibility; preserve paging and activity IDs.
- **Permission concept:** REPO_READ.
- **Error mapping / stop conditions:** Do not decode to a closed enum that discards unknown activity variants.
- **Retry behavior:** Bounded read retry only for eligible transient failures; never retry 4xx.
- **Response-size / truncation:** Apply the shared bounded-reader policy. Do not silently truncate a JSON object; return the shared response-too-large error when a complete bounded representation cannot be produced.
- **MCP annotations:** `readOnlyHint=true; destructiveHint=false; idempotentHint=true`; mutation tools require client approval under the project policy.
- **Required tests:** No cursor; valid COMMENT/ACTIVITY cursor; `fromId` without type rejected; unknown activity shape preserved; next cursor; 400/401/404.
- **Test layers:** unit validation/serialization; request-recording HTTP contract; MCP schema/annotation snapshot; real Bitbucket 5.10.2 host where this section declares a staging gate.

<a id="tool-bitbucket_get_pull_request_commits"></a>
### 3.17 `bitbucket_get_pull_request_commits`

- **MCP purpose:** List commits in a pull request.
- **Source:** [bundled exact-resource anchor](../references/jira-6.4.14_bitbucket-5.10.2-rest-api-reference.md#bb-api-get-pr-commits); [official Bitbucket Server 5.10.2 REST](https://docs.atlassian.com/bitbucket-server/rest/5.10.2/bitbucket-rest.html), resource heading `GET /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/pull-requests/{pullRequestId}/commits`.
- **Method/path:** `GET /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/pull-requests/{pullRequestId}/commits`.
- **Query/path inputs:** Optional `withCounts`, `start`, `limit`.
- **Request headers:** `Authorization: Bearer …`; `Accept: application/json`.
- **Request body:** None.
- **Expected success:** 200 JSON page.
- **Response preservation:** Preserve commit objects, page metadata, and optional `authorCount`/`totalCount`.
- **Permission concept:** REPO_READ.
- **Error mapping / stop conditions:** Use `nextPageStart`; do not infer all commits are returned by a single page.
- **Retry behavior:** Bounded read retry only for eligible transient failures; never retry 4xx.
- **Response-size / truncation:** Apply the shared bounded-reader policy. Do not silently truncate a JSON object; return the shared response-too-large error when a complete bounded representation cannot be produced.
- **MCP annotations:** `readOnlyHint=true; destructiveHint=false; idempotentHint=true`; mutation tools require client approval under the project policy.
- **Required tests:** Counts true/false; cursor; empty page; 401/404; schema.
- **Test layers:** unit validation/serialization; request-recording HTTP contract; MCP schema/annotation snapshot; real Bitbucket 5.10.2 host where this section declares a staging gate.

<a id="tool-bitbucket_get_pull_request_changes"></a>
### 3.18 `bitbucket_get_pull_request_changes`

- **MCP purpose:** Retrieve PR changed paths for all, unreviewed, or explicit-range scope.
- **Source:** [bundled exact-resource anchor](../references/jira-6.4.14_bitbucket-5.10.2-rest-api-reference.md#bb-api-get-pr-changes); [official Bitbucket Server 5.10.2 REST](https://docs.atlassian.com/bitbucket-server/rest/5.10.2/bitbucket-rest.html), resource heading `GET /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/pull-requests/{pullRequestId}/changes`.
- **Method/path:** `GET /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/pull-requests/{pullRequestId}/changes`.
- **Query/path inputs:** Optional `changeScope` (`ALL|UNREVIEWED|RANGE`), `sinceId`, `untilId`, `withComments`, `limit`. Require both IDs for RANGE. Do not expose meaningful `start`: 5.10.2 ignores it.
- **Request headers:** `Authorization: Bearer …`; `Accept: application/json`.
- **Request body:** None.
- **Expected success:** 200 JSON one-page change envelope.
- **Response preservation:** Preserve change objects and properties such as `changeScope`/`unreviewedCommits`; expose `upstreamHardCap` when the server truncates/limits results.
- **Permission concept:** REPO_READ.
- **Error mapping / stop conditions:** The endpoint returns at most one page, ignores `start`, and caps results by request limit/internal maximum. Never promise a follow-up cursor.
- **Retry behavior:** Bounded read retry only for eligible transient failures; never retry 4xx.
- **Response-size / truncation:** Apply the shared bounded-reader policy. Preserve upstream cap/truncation independently from MCP-layer truncation.
- **MCP annotations:** `readOnlyHint=true; destructiveHint=false; idempotentHint=true`; mutation tools require client approval under the project policy.
- **Required tests:** ALL/UNREVIEWED/RANGE; missing range ID; comments toggle; hard cap; assert start omitted/ignored; 401/404.
- **Test layers:** unit validation/serialization; request-recording HTTP contract; MCP schema/annotation snapshot; real Bitbucket 5.10.2 host where this section declares a staging gate.

<a id="tool-bitbucket_get_pull_request_diff"></a>
### 3.19 `bitbucket_get_pull_request_diff`

- **MCP purpose:** Retrieve whole-PR or path-specific structured diff, optionally for a range.
- **Source:** [bundled exact-resource anchor](../references/jira-6.4.14_bitbucket-5.10.2-rest-api-reference.md#bb-api-get-pr-diff); [official Bitbucket Server 5.10.2 REST](https://docs.atlassian.com/bitbucket-server/rest/5.10.2/bitbucket-rest.html), resource heading `GET /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/pull-requests/{pullRequestId}/diff/{path:.*}`.
- **Method/path:** `GET /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/pull-requests/{pullRequestId}/diff/{path:.*}`.
- **Query/path inputs:** Optional `contextLines`, `diffType`, `sinceId`, `srcPath`, `untilId`, `whitespace`, `withComments`. Require the appropriate hash pair for RANGE/COMMIT modes.
- **Request headers:** `Authorization: Bearer …`; `Accept: application/json`.
- **Request body:** None.
- **Expected success:** 200 JSON diff.
- **Response preservation:** Preserve nested diff/hunk/segment/line/comment data and every `truncated` flag.
- **Permission concept:** REPO_READ.
- **Error mapping / stop conditions:** Not paged; server hard-caps lines and subsequent pages cannot be requested. Separate upstream hard cap from MCP response cap.
- **Retry behavior:** Bounded read retry only for eligible transient failures; never retry 4xx.
- **Response-size / truncation:** Apply the shared bounded-reader policy. Preserve upstream cap/truncation independently from MCP-layer truncation.
- **MCP annotations:** `readOnlyHint=true; destructiveHint=false; idempotentHint=true`; mutation tools require client approval under the project policy.
- **Required tests:** Whole/path; EFFECTIVE/RANGE/COMMIT; moved file; comments; whitespace; hard cap; nested truncation; 400/401/404.
- **Test layers:** unit validation/serialization; request-recording HTTP contract; MCP schema/annotation snapshot; real Bitbucket 5.10.2 host where this section declares a staging gate.

<a id="tool-bitbucket_check_pull_request_mergeability"></a>
### 3.20 `bitbucket_check_pull_request_mergeability`

- **MCP purpose:** Check conflicts and merge-check vetoes before a merge mutation.
- **Source:** [bundled exact-resource anchor](../references/jira-6.4.14_bitbucket-5.10.2-rest-api-reference.md#bb-api-check-pr-mergeability); [official Bitbucket Server 5.10.2 REST](https://docs.atlassian.com/bitbucket-server/rest/5.10.2/bitbucket-rest.html), resource heading `GET /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/pull-requests/{pullRequestId}/merge`.
- **Method/path:** `GET /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/pull-requests/{pullRequestId}/merge`.
- **Query/path inputs:** None.
- **Request headers:** `Authorization: Bearer …`; `Accept: application/json`.
- **Request body:** None.
- **Expected success:** 200 JSON mergeability object; upstream may return conflict/state errors for invalid PR states.
- **Response preservation:** Preserve `canMerge`, `conflicted`, `outcome`, and every veto summary/detail.
- **Permission concept:** REPO_READ.
- **Error mapping / stop conditions:** A 200 with `canMerge:false` is a successful precheck, not a transport error. Do not discard veto details.
- **Retry behavior:** Bounded read retry only for eligible transient failures; never retry 4xx.
- **Response-size / truncation:** Apply the shared bounded-reader policy. Do not silently truncate a JSON object; return the shared response-too-large error when a complete bounded representation cannot be produced.
- **MCP annotations:** `readOnlyHint=true; destructiveHint=false; idempotentHint=true`; mutation tools require client approval under the project policy.
- **Required tests:** Mergeable; conflict; one/multiple vetoes; non-open PR; 401/404/409; read-only annotation.
- **Test layers:** unit validation/serialization; request-recording HTTP contract; MCP schema/annotation snapshot; real Bitbucket 5.10.2 host where this section declares a staging gate.

<a id="tool-bitbucket_create_pull_request"></a>
### 3.21 `bitbucket_create_pull_request`

- **MCP purpose:** Create a pull request between two branches in the same repository hierarchy.
- **Source:** [bundled exact-resource anchor](../references/jira-6.4.14_bitbucket-5.10.2-rest-api-reference.md#bb-api-create-pull-request); [official Bitbucket Server 5.10.2 REST](https://docs.atlassian.com/bitbucket-server/rest/5.10.2/bitbucket-rest.html), resource heading `POST /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/pull-requests`.
- **Method/path:** `POST /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/pull-requests`.
- **Query/path inputs:** None.
- **Request headers:** `Authorization: Bearer …`; `Content-Type: application/json`; `Accept: application/json`.
- **Request body:** JSON with `title`, optional `description`, `fromRef`, `toRef`, optional `reviewers`. Construct refs as `refs/heads/...` exactly once. Repository objects are built from configured project and approved source/target slugs; reviewer entries use `{"user":{"name":...}}`.
- **Expected success:** 201 JSON pull request.
- **Response preservation:** Preserve created PR including `id`, `version`, refs, reviewers and links.
- **Permission concept:** REPO_READ on both source and target repositories according to the 5.10.2 endpoint.
- **Error mapping / stop conditions:** Send one POST. Preserve distinct 409 reasons: unresolved reviewer, same branches, target already up-to-date, or an existing PR.
- **Retry behavior:** No blind retry. At most one mutation request; a follow-up GET is allowed only where explicitly stated.
- **Response-size / truncation:** Apply the shared bounded-reader policy. Do not silently truncate a JSON object; return the shared response-too-large error when a complete bounded representation cannot be produced.
- **MCP annotations:** `readOnlyHint=false; destructiveHint=false; idempotentHint=false`; mutation tools require client approval under the project policy.
- **Required tests:** Same/cross repository hierarchy; ref normalization; reviewer payload; 201; each 409 case; permission; ambiguous response one POST.
- **Test layers:** unit validation/serialization; request-recording HTTP contract; MCP schema/annotation snapshot; real Bitbucket 5.10.2 host where this section declares a staging gate.

<a id="tool-bitbucket_add_pull_request_comment"></a>
### 3.22 `bitbucket_add_pull_request_comment`

- **MCP purpose:** Add a general comment, reply, file comment, or line comment.
- **Source:** [bundled exact-resource anchor](../references/jira-6.4.14_bitbucket-5.10.2-rest-api-reference.md#bb-api-add-pr-comment); [official Bitbucket Server 5.10.2 REST](https://docs.atlassian.com/bitbucket-server/rest/5.10.2/bitbucket-rest.html), resource heading `POST /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/pull-requests/{pullRequestId}/comments`.
- **Method/path:** `POST /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/pull-requests/{pullRequestId}/comments`.
- **Query/path inputs:** None.
- **Request headers:** `Authorization: Bearer …`; `Content-Type: application/json`; `Accept: application/json`.
- **Request body:** General: `{"text":...}`. Reply: add `parent:{"id":...}`. File/line: add `anchor`. Anchor may include `diffType`, `fromHash`, `toHash`, `path`, optional `srcPath`; line anchor additionally requires `line`, `lineType` (`ADDED|REMOVED|CONTEXT`) and `fileType` (`FROM|TO`).
- **Expected success:** 201 JSON comment.
- **Response preservation:** Preserve comment `id`, `version`, text, author, dates, nested replies/tasks, permitted operations and anchor-related data returned by upstream.
- **Permission concept:** REPO_READ; endpoint may also add the caller as watcher/participant behavior described upstream.
- **Error mapping / stop conditions:** Validate comment modes as mutually coherent. Do not send a partially populated anchor. `srcPath` is required for copy/move where applicable; non-backcompat anchor combinations require `diffType`. Send one POST and never log text.
- **Retry behavior:** No blind retry. At most one mutation request; a follow-up GET is allowed only where explicitly stated.
- **Response-size / truncation:** Apply the shared bounded-reader policy. Do not silently truncate a JSON object; return the shared response-too-large error when a complete bounded representation cannot be produced.
- **MCP annotations:** `readOnlyHint=false; destructiveHint=false; idempotentHint=false`; mutation tools require client approval under the project policy.
- **Required tests:** General/reply/file/line; every line/file enum; incomplete hashes/path/line rejected; move srcPath; 201; 400/401/404; one POST; text redaction.
- **Test layers:** unit validation/serialization; request-recording HTTP contract; MCP schema/annotation snapshot; real Bitbucket 5.10.2 host where this section declares a staging gate.

<a id="tool-bitbucket_set_pull_request_review_status"></a>
### 3.23 `bitbucket_set_pull_request_review_status`

- **MCP purpose:** Change only the configured service user’s own participant review status.
- **Source:** [bundled exact-resource anchor](../references/jira-6.4.14_bitbucket-5.10.2-rest-api-reference.md#bb-api-set-pr-review-status); [official Bitbucket Server 5.10.2 REST](https://docs.atlassian.com/bitbucket-server/rest/5.10.2/bitbucket-rest.html), resource heading `PUT /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/pull-requests/{pullRequestId}/participants/{userSlug}`.
- **Method/path:** `PUT /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/pull-requests/{pullRequestId}/participants/{userSlug}`.
- **Query/path inputs:** None. URL `userSlug` comes only from `BITBUCKET_USER_SLUG`.
- **Request headers:** `Authorization: Bearer …`; `Content-Type: application/json`; `Accept: application/json`.
- **Request body:** JSON contains `user:{"name": identity}`, `approved` and `status`. Status is exactly `APPROVED`, `NEEDS_WORK`, or `UNAPPROVED`; set `approved=true` only for APPROVED. The exact identity value accepted in `user.name` must be verified on the target 5.10.2 host because configuration currently stores a slug.
- **Expected success:** 201 JSON participant.
- **Response preservation:** Preserve participant role/status/approved flag and `lastReviewedCommit`.
- **Permission concept:** REPO_READ.
- **Error mapping / stop conditions:** No caller-supplied user identity. Current-user author is rejected. Mandatory staging gate confirms whether `BITBUCKET_USER_SLUG` is valid for both URL slug and body `user.name`; stop implementation/release if not.
- **Retry behavior:** No blind retry. At most one mutation request; a follow-up GET is allowed only where explicitly stated.
- **Response-size / truncation:** Apply the shared bounded-reader policy. Do not silently truncate a JSON object; return the shared response-too-large error when a complete bounded representation cannot be produced.
- **MCP annotations:** `readOnlyHint=false; destructiveHint=false; idempotentHint=false`; mutation tools require client approval under the project policy.
- **Required tests:** All three statuses and approved mapping; identity absent; impersonation property rejected; author 409; 201; 400/401/404/409; staging identity test.
- **Test layers:** unit validation/serialization; request-recording HTTP contract; MCP schema/annotation snapshot; real Bitbucket 5.10.2 host where this section declares a staging gate.

<a id="tool-bitbucket_merge_pull_request"></a>
### 3.24 `bitbucket_merge_pull_request`

- **MCP purpose:** Merge an open PR using optimistic locking and an optional mergeability precheck.
- **Source:** [bundled exact-resource anchor](../references/jira-6.4.14_bitbucket-5.10.2-rest-api-reference.md#bb-api-merge-pull-request); [official Bitbucket Server 5.10.2 REST](https://docs.atlassian.com/bitbucket-server/rest/5.10.2/bitbucket-rest.html), resource heading `POST /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/pull-requests/{pullRequestId}/merge`.
- **Method/path:** `POST /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/pull-requests/{pullRequestId}/merge`.
- **Query/path inputs:** Required resolved `version`. Use caller `expectedVersion` unchanged; otherwise GET PR once immediately before mutation.
- **Request headers:** `Authorization: Bearer …`; `Content-Type: application/json`; `Accept: application/json`.
- **Request body:** None for the documented 5.10.2 endpoint.
- **Expected success:** 200 JSON merged pull request.
- **Response preservation:** Preserve merged PR and version/state metadata. On 409 optionally GET current state once for explanation, without replaying POST.
- **Permission concept:** REPO_WRITE.
- **Error mapping / stop conditions:** Default precheck calls mergeability GET and stops before POST on conflict/veto. A 409 can mean conflict, veto, stale version, or invalid/non-open state; classify from sanitized upstream details and context, never flatten all to stale version.
- **Retry behavior:** No blind retry. At most one mutation request; a follow-up GET is allowed only where explicitly stated.
- **Response-size / truncation:** Apply the shared bounded-reader policy. Do not silently truncate a JSON object; return the shared response-too-large error when a complete bounded representation cannot be produced.
- **MCP annotations:** `readOnlyHint=false; destructiveHint=true; idempotentHint=false`; mutation tools require client approval under the project policy.
- **Required tests:** Precheck pass/conflict/veto; expected/auto version; exact query; 200; each 409 category; request count one; safe explanatory GET; destructive annotation.
- **Test layers:** unit validation/serialization; request-recording HTTP contract; MCP schema/annotation snapshot; real Bitbucket 5.10.2 host where this section declares a staging gate.

<a id="tool-bitbucket_decline_pull_request"></a>
### 3.25 `bitbucket_decline_pull_request`

- **MCP purpose:** Decline an open pull request using optimistic locking.
- **Source:** [bundled exact-resource anchor](../references/jira-6.4.14_bitbucket-5.10.2-rest-api-reference.md#bb-api-decline-pull-request); [official Bitbucket Server 5.10.2 REST](https://docs.atlassian.com/bitbucket-server/rest/5.10.2/bitbucket-rest.html), resource heading `POST /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/pull-requests/{pullRequestId}/decline`.
- **Method/path:** `POST /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/pull-requests/{pullRequestId}/decline`.
- **Query/path inputs:** Required resolved `version`; caller expected version wins, otherwise one immediate GET.
- **Request headers:** `Authorization: Bearer …`; `Accept: application/json`.
- **Request body:** None.
- **Expected success:** 200 response; preserve a JSON PR if supplied by the host, otherwise a successful empty result plus resolved version.
- **Response preservation:** Preserve returned state when present. On 409 optionally refresh current PR once for explanation.
- **Permission concept:** REPO_READ per the 5.10.2 endpoint documentation.
- **Error mapping / stop conditions:** 409 may be stale version or non-OPEN state. Never replay the POST. Staging contract test records actual success content type/body for the target host because the reference’s success representation is sparse.
- **Retry behavior:** No blind retry. At most one mutation request; a follow-up GET is allowed only where explicitly stated.
- **Response-size / truncation:** Apply the shared bounded-reader policy. Do not silently truncate a JSON object; return the shared response-too-large error when a complete bounded representation cannot be produced.
- **MCP annotations:** `readOnlyHint=false; destructiveHint=true; idempotentHint=false`; mutation tools require client approval under the project policy.
- **Required tests:** Expected/auto version; open success; empty/JSON success fixture; stale; wrong state; 401/404/409; one POST; staging response-shape gate.
- **Test layers:** unit validation/serialization; request-recording HTTP contract; MCP schema/annotation snapshot; real Bitbucket 5.10.2 host where this section declares a staging gate.

<a id="tool-bitbucket_reopen_pull_request"></a>
### 3.26 `bitbucket_reopen_pull_request`

- **MCP purpose:** Reopen a declined pull request using optimistic locking.
- **Source:** [bundled exact-resource anchor](../references/jira-6.4.14_bitbucket-5.10.2-rest-api-reference.md#bb-api-reopen-pull-request); [official Bitbucket Server 5.10.2 REST](https://docs.atlassian.com/bitbucket-server/rest/5.10.2/bitbucket-rest.html), resource heading `POST /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/pull-requests/{pullRequestId}/reopen`.
- **Method/path:** `POST /rest/api/1.0/projects/{projectKey}/repos/{repositorySlug}/pull-requests/{pullRequestId}/reopen`.
- **Query/path inputs:** Required resolved `version`; caller expected version wins, otherwise one immediate GET.
- **Request headers:** `Authorization: Bearer …`; `Accept: application/json`.
- **Request body:** None.
- **Expected success:** 200 JSON reopened pull request.
- **Response preservation:** Preserve reopened PR and version/state. On 409 optionally refresh once for explanation.
- **Permission concept:** REPO_READ.
- **Error mapping / stop conditions:** 409 may mean not declined or stale version. Never replay the POST.
- **Retry behavior:** No blind retry. At most one mutation request; a follow-up GET is allowed only where explicitly stated.
- **Response-size / truncation:** Apply the shared bounded-reader policy. Do not silently truncate a JSON object; return the shared response-too-large error when a complete bounded representation cannot be produced.
- **MCP annotations:** `readOnlyHint=false; destructiveHint=true; idempotentHint=false`; mutation tools require client approval under the project policy.
- **Required tests:** Expected/auto version; declined success; not-declined; stale; 401/404/409; one POST; destructive annotation.
- **Test layers:** unit validation/serialization; request-recording HTTP contract; MCP schema/annotation snapshot; real Bitbucket 5.10.2 host where this section declares a staging gate.

## 4. Mandatory real-host compatibility gates

Before release against the internal Bitbucket Server 5.10.2 host, record sanitized request/response fixtures for:

- Bearer PAT acceptance through the configured reverse proxy and context path.
- Commit-specific changes and diff response caps and truncation indicators.
- Compare diff response envelope and server-specific cap behavior.
- Multipart single-file create, update, and new-branch behavior.
- PR decline success body/content type.
- Participant status: URL `userSlug` versus request `user.name`; do not release the tool until one configured identity works for both or the configuration contract is explicitly revised.
- 409 error bodies for merge conflict, merge veto, stale version, invalid state, file stale-write, and duplicate/existing resources.

## 5. Completion scan

- Exactly 26 tool sections exist.
- Every section includes method/path, parameters, headers, body, success, response, permission, errors, retry, truncation, annotations, and tests.
- Every local source link resolves to a stable anchor.
- No endpoint field or enum exists only because an implementation agent inferred it.
- Mutations have a one-request assertion.
- Paged tools test non-contiguous `nextPageStart`; one-page/hard-cap tools do not fabricate a cursor.
