# Bitbucket Tools

The Bitbucket module registers these tools after valid static configuration (`BITBUCKET_BASE_URL`, `BITBUCKET_PROJECT_KEY`, `BITBUCKET_BEARER_TOKEN`):

| Tool | Access | Notes |
|---|---|---|
| `bitbucket_get_repository` | Read-only | Reads repository metadata for the configured project and `repositorySlug`. |
| `bitbucket_list_branches` | Read-only | Lists branches with optional `filterText`, `orderBy` (ALPHABETICAL), and paging (`start`/`limit`). |
| `bitbucket_get_default_branch` | Read-only | Reads the default branch of `repositorySlug`. |
| `bitbucket_create_branch` | Additive write | Creates one branch from `name` and `startPoint`. Requires client approval. |
| `bitbucket_get_file` | Read-only | Reads one file at `path` on `at` ref. Returns text for valid UTF-8, base64 otherwise. |
| `bitbucket_list_commits` | Read-only | Lists commits with optional `since`, `until`, `path`, `followRenames`, `ignoreMissing`, and paging. |
| `bitbucket_get_commit` | Read-only | Reads one commit by `commitId`. |
| `bitbucket_get_commit_changes` | Read-only | Lists changed paths for one commit. Optional `since`, `withComments`, and `limit` (no `start`). |
| `bitbucket_get_commit_diff` | Read-only | Reads structured diff for one commit. Optional `path`, `srcPath`, `contextLines`, `whitespace`, `since`, `withComments`, `autoSrcPath`. |
| `bitbucket_compare_commits` | Read-only | Compares commits between `from` and `to` refs. Optional `fromRepositorySlug` serializes as `fromRepo={projectKey}/{slug}` for cross-repo comparison within the configured project. |
| `bitbucket_compare_changes` | Read-only | Compares changed paths between `from` and `to`. Same `fromRepositorySlug` serialization as compare commits. |
| `bitbucket_compare_diff` | Read-only | Structured diff between `from` and `to`. Optional `path`, `srcPath`, `contextLines`, `whitespace`. Same `fromRepositorySlug` serialization. |
| `bitbucket_commit_file` | Destructive write | Creates or updates exactly one file with one commit via multipart PUT. Required `mode` (`create` or `update`): update requires `sourceCommitId` (prevents silent overwrite of concurrent changes); create rejects it. Optional `sourceBranch` (callers MUST supply when `branch` does not yet exist). Content via exactly one of `content` (text) or `contentBase64`. 409 maps to `BITBUCKET_COMMIT_FILE_CONFLICT`. One PUT only; no retry. |
| `bitbucket_list_pull_requests` | Read-only | Lists PRs with optional `state` (OPEN/DECLINED/MERGED/ALL), `direction` (INCOMING/OUTGOING), `order` (OLDEST/NEWEST), `at`, participant filters (up to 10, serialized as `username.N`/`role.N`/`approved.N`), `withAttributes`, `withProperties`, and paging. |
| `bitbucket_get_pull_request` | Read-only | Reads one PR by `pullRequestId`. Preserves `version` for transition tools. |
| `bitbucket_get_pull_request_activities` | Read-only | Lists PR activities. Optional `fromId`, `fromType` (COMMENT or ACTIVITY; required when `fromId` is set), and paging. |
| `bitbucket_get_pull_request_commits` | Read-only | Lists PR commits. Optional `withCounts` and paging. |
| `bitbucket_get_pull_request_changes` | Read-only | Lists PR changed paths. Optional `changeScope` (ALL/UNREVIEWED/RANGE); RANGE requires both `sinceId` and `untilId`. No `start` exposure. |
| `bitbucket_get_pull_request_diff` | Read-only | Reads PR diff. Optional `diffType` (EFFECTIVE/RANGE/COMMIT); RANGE requires `sinceId`+`untilId`, COMMIT requires `untilId`. Optional `withComments`. |
| `bitbucket_check_pull_request_mergeability` | Read-only | Checks mergeability of one PR. |
| `bitbucket_create_pull_request` | Additive write | Creates one PR from `title`, `description`, `fromBranch`, `toBranch`, optional `fromRepositorySlug` (cross-repo), and `reviewers`. Requires client approval. |
| `bitbucket_add_pull_request_comment` | Additive write | Adds a comment to a PR. Optional `anchor` supports file-level (path only) and line-level (path+line+lineType+fileType) modes with optional `diffType`/`fromHash`/`toHash`. Requires client approval. |
| `bitbucket_set_pull_request_review_status` | Additive write | Sets the configured service user's own review status (`APPROVED`, `NEEDS_WORK`, or `UNAPPROVED`). Body includes `user:{"name": identity}`, `status`, and `approved`. Requires `BITBUCKET_USER_SLUG` config. Identity value correctness is a staging gate. |
| `bitbucket_merge_pull_request` | Destructive write | Merges one PR with optimistic-locking version safety (auto-fetches current version). Irreversible. Requires client approval. |
| `bitbucket_decline_pull_request` | Destructive write | Declines one PR with version safety. Requires client approval. |
| `bitbucket_reopen_pull_request` | Destructive write | Reopens one PR with version safety. Requires client approval. |
| `bitbucket_update_pull_request` | Additive write | Updates title, description, and/or reviewers using auto-preserve semantics: omitted fields are preserved from a pre-PUT GET; version is always resolved automatically. An explicitly empty `reviewers` array clears all reviewers. Requires client approval. |

All tool results use the shared envelope: `success`, `service`, `tool`, `data`, `error`, and optional `meta`.

Mutation tools (create, commit, merge, decline, reopen, update, comment, review status) require client approval under the project policy. Read-only tools require no approval.

The `bitbucket_commit_file` safety policy enforces explicit caller intent: `mode` is required, `sourceCommitId` is mandatory for updates and forbidden for creates, and `sourceBranch` must be supplied by the caller when targeting a branch that does not yet exist (the upstream error is surfaced otherwise).

Cross-repository compare operations accept only a bare repository slug in `fromRepositorySlug`; the tool serializes it as `fromRepo={configuredProjectKey}/{slug}`. Arbitrary project keys, numeric IDs, URLs, or slash-qualified values are rejected before any request.

Local tests cover the documented Bitbucket Server 5.10.2 wire contract. Release against an actual Bitbucket environment remains user-owned: the `user.name` identity value for review-status and 409 exception-name sub-classification for commit_file are staging gates per the implementation guide §4.
