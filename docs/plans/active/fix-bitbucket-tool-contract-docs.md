# Execution Plan: Fix Bitbucket Tool Contract And Docs

Date: 2026-08-01

## Status

Active

## Outcome

Bring the implemented Bitbucket MCP tools, API documentation, and related operator docs back into alignment with `docs/specs/SPECS.md` section 10 and `docs/specs/bitbucket-tool-implementation-guide.md`, with explicit validation for the contract gaps found during review.

## Context

- `AGENTS.md`
- `docs/WORKFLOW.md`
- `docs/specs/SPECS.md` section 10 and Tasks 6-11
- `docs/specs/bitbucket-tool-implementation-guide.md`
- `docs/specs/bitbucket-tool-test-matrix.md`
- `docs/references/jira-6.4.14_bitbucket-5.10.2-rest-api-reference.md`
- `internal/bitbucket/tools/`
- `internal/bitbucket/client/`
- `README.md`
- `docs/plans/completed/build-bitbucket-tools.md`

## Scope

In scope:

- Fix mismatches between Bitbucket tool handlers and the endpoint-level implementation guide.
- Add or update focused tests for fixed schema/query/body behavior.
- Add grouped Bitbucket tool documentation covering all 26 tools, or the exact per-tool files required by `SPECS.md`.
- Update stale user-facing documentation that still says Bitbucket business tools are future work.
- Record unresolved real-host compatibility gates as explicit limitations when they cannot be verified locally.

Out of scope:

- Real Bitbucket Server 5.10.2 staging execution unless credentials and host access are provided in the session.
- New Bitbucket tools beyond the 26-tool registry in `SPECS.md`.
- Atomic multi-file commits.
- Broad model package refactors unless required by tests or endpoint contract.
- Installer work unrelated to Bitbucket tool documentation or contract behavior.

## Approach

1. Reconfirm the 26-tool registry against `SPECS.md`, the implementation guide, and the matrix.
2. Fix high-risk request-shape mismatches first:
   - Compare tools must serialize `fromRepositorySlug` as `fromRepo={BITBUCKET_PROJECT_KEY}/{slug}`.
   - `bitbucket_commit_file` must enforce the documented safety policy for `sourceCommitId` on update and `sourceBranch` on new-branch creation, or the contract must be revised before implementation.
   - `bitbucket_set_pull_request_review_status` must send the documented participant body including `user:{"name": identity}` once the identity contract is confirmed or explicitly revised.
3. Add missing schema/query coverage for documented inputs:
   - `ignoreMissing` on commit list.
   - `since`, `withComments`, and `autoSrcPath` on commit diff.
   - PR list participant filters plus `withAttributes` and `withProperties`.
   - PR activities `fromId` and `fromType`.
   - PR changes `changeScope` and no meaningful `start` exposure.
   - PR diff `diffType` and `withComments`.
   - PR comment anchor fields `diffType`, `fromHash`, and `toHash`.
4. Add focused request-recording tests for each changed API shape and mutation one-request invariant.
5. Create grouped docs under `docs/tools/` that cover every Bitbucket tool with purpose, inputs, endpoint, permissions/approval, safety notes, response shape, and source links.
6. Update `README.md` and plan/result docs so they no longer describe Bitbucket business tools as future work.

## Risks And Recovery

- Risk: Some guide requirements depend on real Bitbucket Server 5.10.2 behavior, especially participant identity and response bodies. Mitigation: stop before inventing behavior; document unresolved gates or request credentials/access.
- Risk: Tightening `bitbucket_commit_file` safety may reject existing local usage patterns. Mitigation: preserve create mode and require explicit mode/safety fields only where the documented contract requires them.
- Risk: Expanding schemas may expose unsupported enum values. Mitigation: validate only enum values documented by the guide and reference.
- Recovery: Revert the files touched by this plan and restore the last passing state from `go test ./internal/bitbucket/...`; generated docs can be removed independently if implementation changes are rejected.

## Progress

- [x] Review `SPECS.md`, Bitbucket guide, matrix, and current tool code for missing documentation/API coverage.
- [ ] Decide whether to implement or revise the `bitbucket_commit_file` update/new-branch safety contract.
- [ ] Decide whether `BITBUCKET_USER_SLUG` is also the valid `user.name` value for review-status payloads, or revise configuration before release.
- [ ] Fix compare `fromRepo` serialization and tests.
- [ ] Fix missing documented query/body inputs and tests.
- [ ] Add grouped Bitbucket tool documentation for all 26 tools.
- [ ] Update stale README/plan documentation.
- [ ] Run focused and repository validation.

## Decisions

- 2026-08-01: Keep this as a separate active plan from `build-task-1-14.md` because that plan is Task 5 foundation-focused, while this work concerns Tasks 6-11 tool contracts and documentation.
- 2026-08-01: Treat `docs/specs/bitbucket-tool-implementation-guide.md` as the endpoint-level authority; do not resolve mismatches by weakening docs without an explicit source-backed reason.
- 2026-08-01: Grouped documentation is acceptable only if every one of the 26 tools has an explicit entry and source link.

## Validation

- Focused proof:
  - `go test ./internal/bitbucket/...`
  - Focused request-recording tests for compare, commit file, review status, PR filters, PR diff/change/comment anchors, and registry/schema annotations.
- Integration or end-to-end proof:
  - Build binary and inspect/list MCP tools when local client support is available.
  - Real Bitbucket Server 5.10.2 staging checks only when host/token/repository access is available.
- Repository-required checks:
  - `go test ./...`
  - Markdown link/path sanity check for new Bitbucket tool docs.

## Result

Complete after implementation. Record verified contract fixes, docs added, validation commands, unresolved real-host gates, and any intentionally revised contract before moving this plan to `docs/plans/completed/`.
