# Bitbucket Tool Implementation and Test Matrix — Server 5.10.2

This matrix is a coverage/navigation artifact. The authoritative endpoint contract is `bitbucket-tool-implementation-guide.md`.

## Global requirements

- All 26 schemas require `repositorySlug` and reject unknown outer fields.
- Every tool has unit validation/serialization tests, request-recording HTTP contract tests, and MCP schema/annotation snapshots.
- Genuine paged APIs follow only `nextPageStart`; hard-capped endpoints never fabricate continuation.
- Mutations assert one upstream request and redact token/content/comment/diff sentinels.
- Every row links to a section containing exact source references and test details.

| # | Tool contract | Task | Method/path (after `/rest/api/1.0`) | Success | Permission | Paging/cap | Real-host gate |
|---:|---|---:|---|---|---|---|---|
| 1 | [`bitbucket_get_repository`](bitbucket-tool-implementation-guide.md#tool-bitbucket_get_repository) | 6 | `GET /projects/{projectKey}/repos/{repositorySlug}` | 200 JSON repository. | REPO_READ on the repository. | Single response | Normal contract suite |
| 2 | [`bitbucket_list_branches`](bitbucket-tool-implementation-guide.md#tool-bitbucket_list_branches) | 6 | `GET /projects/{projectKey}/repos/{repositorySlug}/branches` | 200 JSON page. | REPO_READ. | Paged / nextPageStart | Normal contract suite |
| 3 | [`bitbucket_get_default_branch`](bitbucket-tool-implementation-guide.md#tool-bitbucket_get_default_branch) | 6 | `GET /projects/{projectKey}/repos/{repositorySlug}/branches/default` | 200 JSON branch; 204 empty body when the repository has no default branch. | REPO_READ. | Single response | Normal contract suite |
| 4 | [`bitbucket_create_branch`](bitbucket-tool-implementation-guide.md#tool-bitbucket_create_branch) | 6 | `POST /projects/{projectKey}/repos/{repositorySlug}/branches` | 200 JSON branch. | REPO_WRITE. | Single response | Normal contract suite |
| 5 | [`bitbucket_get_file`](bitbucket-tool-implementation-guide.md#tool-bitbucket_get_file) | 7 | `GET /projects/{projectKey}/repos/{repositorySlug}/raw/{path:.*}` | 200 raw response body; upstream JSON error bodies for failures. | REPO_READ. | Single response | Normal contract suite |
| 6 | [`bitbucket_list_commits`](bitbucket-tool-implementation-guide.md#tool-bitbucket_list_commits) | 7 | `GET /projects/{projectKey}/repos/{repositorySlug}/commits` | 200 JSON page. | REPO_READ. | Paged / nextPageStart | Normal contract suite |
| 7 | [`bitbucket_get_commit`](bitbucket-tool-implementation-guide.md#tool-bitbucket_get_commit) | 7 | `GET /projects/{projectKey}/repos/{repositorySlug}/commits/{commitId}` | 200 JSON commit. | REPO_READ. | Single response | Normal contract suite |
| 8 | [`bitbucket_get_commit_changes`](bitbucket-tool-implementation-guide.md#tool-bitbucket_get_commit_changes) | 7 | `GET /projects/{projectKey}/repos/{repositorySlug}/commits/{commitId}/changes` | 200 JSON change page/envelope. | REPO_READ. | Hard-cap / no continuation | Normal contract suite |
| 9 | [`bitbucket_get_commit_diff`](bitbucket-tool-implementation-guide.md#tool-bitbucket_get_commit_diff) | 7 | `GET /projects/{projectKey}/repos/{repositorySlug}/commits/{commitId}/diff/{path:.*}` | 200 JSON diff object. | REPO_READ. | Single response | Normal contract suite |
| 10 | [`bitbucket_compare_commits`](bitbucket-tool-implementation-guide.md#tool-bitbucket_compare_commits) | 7 | `GET /projects/{projectKey}/repos/{repositorySlug}/compare/commits` | 200 JSON page. | REPO_READ for repositories required by the comparison. | Paged / nextPageStart | Normal contract suite |
| 11 | [`bitbucket_compare_changes`](bitbucket-tool-implementation-guide.md#tool-bitbucket_compare_changes) | 7 | `GET /projects/{projectKey}/repos/{repositorySlug}/compare/changes` | 200 JSON change page/envelope. | REPO_READ. | Paged / nextPageStart | Required |
| 12 | [`bitbucket_compare_diff`](bitbucket-tool-implementation-guide.md#tool-bitbucket_compare_diff) | 7 | `GET /projects/{projectKey}/repos/{repositorySlug}/compare/diff/{path:.*}` | 200 JSON diff. | REPO_READ. | Hard-cap / no continuation | Required |
| 13 | [`bitbucket_commit_file`](bitbucket-tool-implementation-guide.md#tool-bitbucket_commit_file) | 8 | `PUT /projects/{projectKey}/repos/{repositorySlug}/browse/{path:.*}` | 200 JSON commit. | REPO_WRITE. | Single response | Normal contract suite |
| 14 | [`bitbucket_list_pull_requests`](bitbucket-tool-implementation-guide.md#tool-bitbucket_list_pull_requests) | 9 | `GET /projects/{projectKey}/repos/{repositorySlug}/pull-requests` | 200 JSON page. | REPO_READ. | Paged / nextPageStart | Normal contract suite |
| 15 | [`bitbucket_get_pull_request`](bitbucket-tool-implementation-guide.md#tool-bitbucket_get_pull_request) | 9 | `GET /projects/{projectKey}/repos/{repositorySlug}/pull-requests/{pullRequestId}` | 200 JSON pull request. | REPO_READ. | Single response | Normal contract suite |
| 16 | [`bitbucket_get_pull_request_activities`](bitbucket-tool-implementation-guide.md#tool-bitbucket_get_pull_request_activities) | 9 | `GET /projects/{projectKey}/repos/{repositorySlug}/pull-requests/{pullRequestId}/activities` | 200 JSON page. | REPO_READ. | Paged / nextPageStart | Normal contract suite |
| 17 | [`bitbucket_get_pull_request_commits`](bitbucket-tool-implementation-guide.md#tool-bitbucket_get_pull_request_commits) | 9 | `GET /projects/{projectKey}/repos/{repositorySlug}/pull-requests/{pullRequestId}/commits` | 200 JSON page. | REPO_READ. | Paged / nextPageStart | Normal contract suite |
| 18 | [`bitbucket_get_pull_request_changes`](bitbucket-tool-implementation-guide.md#tool-bitbucket_get_pull_request_changes) | 9 | `GET /projects/{projectKey}/repos/{repositorySlug}/pull-requests/{pullRequestId}/changes` | 200 JSON one-page change envelope. | REPO_READ. | Hard-cap / no continuation | Normal contract suite |
| 19 | [`bitbucket_get_pull_request_diff`](bitbucket-tool-implementation-guide.md#tool-bitbucket_get_pull_request_diff) | 9 | `GET /projects/{projectKey}/repos/{repositorySlug}/pull-requests/{pullRequestId}/diff/{path:.*}` | 200 JSON diff. | REPO_READ. | Hard-cap / no continuation | Normal contract suite |
| 20 | [`bitbucket_check_pull_request_mergeability`](bitbucket-tool-implementation-guide.md#tool-bitbucket_check_pull_request_mergeability) | 9 | `GET /projects/{projectKey}/repos/{repositorySlug}/pull-requests/{pullRequestId}/merge` | 200 JSON mergeability object; upstream may return conflict/state errors for invalid PR states. | REPO_READ. | Single response | Normal contract suite |
| 21 | [`bitbucket_create_pull_request`](bitbucket-tool-implementation-guide.md#tool-bitbucket_create_pull_request) | 9 | `POST /projects/{projectKey}/repos/{repositorySlug}/pull-requests` | 201 JSON pull request. | REPO_READ on both source and target repositories according to the 5.10.2 endpoint. | Single response | Normal contract suite |
| 22 | [`bitbucket_add_pull_request_comment`](bitbucket-tool-implementation-guide.md#tool-bitbucket_add_pull_request_comment) | 10 | `POST /projects/{projectKey}/repos/{repositorySlug}/pull-requests/{pullRequestId}/comments` | 201 JSON comment. | REPO_READ; endpoint may also add the caller as watcher/participant behavior described upstream. | Single response | Normal contract suite |
| 23 | [`bitbucket_set_pull_request_review_status`](bitbucket-tool-implementation-guide.md#tool-bitbucket_set_pull_request_review_status) | 10 | `PUT /projects/{projectKey}/repos/{repositorySlug}/pull-requests/{pullRequestId}/participants/{userSlug}` | 201 JSON participant. | REPO_READ. | Single response | Required |
| 24 | [`bitbucket_merge_pull_request`](bitbucket-tool-implementation-guide.md#tool-bitbucket_merge_pull_request) | 11 | `POST /projects/{projectKey}/repos/{repositorySlug}/pull-requests/{pullRequestId}/merge` | 200 JSON merged pull request. | REPO_WRITE. | Single response | Normal contract suite |
| 25 | [`bitbucket_decline_pull_request`](bitbucket-tool-implementation-guide.md#tool-bitbucket_decline_pull_request) | 11 | `POST /projects/{projectKey}/repos/{repositorySlug}/pull-requests/{pullRequestId}/decline` | 200 response; preserve a JSON PR if supplied by the host, otherwise a successful empty result plus resolved version. | REPO_READ per the 5.10.2 endpoint documentation. | Single response | Required |
| 26 | [`bitbucket_reopen_pull_request`](bitbucket-tool-implementation-guide.md#tool-bitbucket_reopen_pull_request) | 11 | `POST /projects/{projectKey}/repos/{repositorySlug}/pull-requests/{pullRequestId}/reopen` | 200 JSON reopened pull request. | REPO_READ. | Single response | Normal contract suite |

## Cross-cutting test files

- `internal/bitbucket/client/client_test.go`
- `internal/bitbucket/tools/registry_test.go`
- `tests/contract/bitbucket_client_test.go`
- `tests/contract/bitbucket_repository_branch_test.go`
- `tests/contract/bitbucket_commit_diff_test.go`
- `tests/contract/bitbucket_commit_file_test.go`
- `tests/contract/bitbucket_pull_request_read_test.go`
- `tests/contract/bitbucket_pull_request_mutation_test.go`

## Release assertions

- Registry count and guide section count are both 26.
- Every linked local API anchor resolves.
- No mutation test records more than one mutation request.
- Staging evidence exists for the gates listed in the guide.
