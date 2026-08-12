# Execution Plan: Release AMD64 Artifacts

> **For agentic workers:** Use `superpowers:executing-plans` or
> `superpowers:subagent-driven-development` to execute this plan in order and
> keep this file current as evidence changes.

Date: 2026-08-11

## Status

Active - release-ID draft validation/publish fix is in place and local static
checks passed. A user-directed temporary diagnostic bypass disables Debian
package generation and Debian tag draft validation, allowing a hosted
no-Debian tag/draft workflow test. This is not the production release contract:
Debian generation, Debian SBOM expectations, and `.deb` checks must be restored
before an actual release. GitHub-hosted artifact validation, protected release
approval, and external same-tag compatibility gates have not run after this
fix.

## Outcome

Tags matching `v*` produce a draft GitHub release through GoReleaser, validate
the exact draft assets on GitHub-hosted Ubuntu and Windows runners, and publish
the release only after the automated checks and the separately recorded
organizational compatibility gates pass.

Acceptance criteria:

- GoReleaser uploads only these amd64 delivery artifacts: one Windows `.exe`,
  one raw Linux executable, and one Debian `.deb`.
- Every uploaded asset starts with `atlassian-mcp_`; the release also contains
  one SHA-256 checksum manifest and an SPDX JSON SBOM for each delivery
  artifact.
- The Windows and Linux binaries report `atlassian-mcp <tag-without-v>` from
  `--version`; the `.deb` installs the same Linux binary at
  `/usr/bin/atlassian-mcp`.
- GitHub-hosted CI runs Go unit, race, and existing contract tests, ShellCheck,
  Bash installer tests, PSScriptAnalyzer, Pester, GoReleaser configuration and
  snapshot checks, checksum verification, SBOM parsing, artifact inventory
  checks, and platform execution checks.
- CI rejects Darwin, arm64, archives, RPMs, container images, or any other
  GoReleaser-uploaded asset.
- The release remains a draft until the pinned organizational Claude Code and
  Codex smoke evidence and internal Jira Server 6.4.14 / Bitbucket Server
  5.10.2 staging evidence are recorded and a protected `release` environment is
  approved.
- No local Windows or Ubuntu validation is required; GitHub-hosted runners are
  the platform proof requested for this work.

GitHub automatically displays source-code zip and tar archives for every tag.
Those platform-generated links are not GoReleaser-uploaded assets and are
excluded from the asset inventory above because GitHub does not provide a
supported switch to remove them.

## Context

Repository authority and verified current state:

- `AGENTS.md` and `docs/WORKFLOW.md` require one durable active plan for this
  coordinated change and behavior-appropriate proof before completion.
- `docs/specs/SPECS.md`, Task 27 requires release packaging, SHA-256 checksums,
  SBOM, changelog, Go unit/race/contract tests, ShellCheck,
  PSScriptAnalyzer/Pester, Claude Code/Codex smoke tests, internal staging
  contracts, and the forbidden-name check.
- `docs/specs/SPECS.md`, Section 14.3 identifies real-host-only compatibility
  cases. They cannot be proven by mocked GitHub-hosted CI.
- `cmd/atlassian-mcp/main.go` currently declares
  `const version = "0.1.0"`; a constant cannot receive Go linker `-X` version
  injection.
- `go.mod` requires Go `1.25.0`.
- No `.goreleaser.yaml`, `.goreleaser.yml`, or `.github` release workflow is
  present.
- `README.md` documents source-build installers and their existing prebuilt
  `--binary` / `-Binary` inputs, but it does not document downloadable release
  assets.
- `scripts/install-from-remote.sh`, `scripts/install-from-remote.ps1`,
  `tests/install-from-remote_test.sh`, and
  `tests/install-from-remote.Tests.ps1` are the existing cross-platform
  installer surfaces.
- `internal/app/naming_test.go` is the existing repository-authoritative
  forbidden credential-name guard. No additional superseded-name list is
  present, so this plan does not invent one.
- Repository searches found no Debian Maintainer identity/email and no License
  value or license file. The user has since supplied both `.deb` metadata
  values; this does not add or authorize a repository license file.

Current GoReleaser v2 concepts used by this plan are documented in the official
build, binary archive, nFPM, checksum, SBOM, and changelog references:

- <https://goreleaser.com/customization/builds/builders/go/>
- <https://goreleaser.com/customization/package/archives/>
- <https://goreleaser.com/customization/package/nfpm/>
- <https://goreleaser.com/customization/package/checksum/>
- <https://goreleaser.com/customization/sbom/>
- <https://goreleaser.com/customization/publish/changelog/>

## Scope

In scope:

- GoReleaser v2 configuration for exactly `linux_amd64_v1` and
  `windows_amd64_v1`.
- Raw Windows and Linux binaries, one Debian package, SHA-256 checksums, SPDX
  JSON SBOMs, and Git-derived GitHub release notes.
- Linker-injected release version with the existing `0.1.0` value retained as
  the source-checkout fallback.
- One GitHub Actions workflow for pull-request/default-branch validation,
  draft tag releases, draft-asset validation, protected approval, and final
  publication.
- GitHub-hosted Ubuntu and Windows artifact execution/inspection.
- Existing Go, Bash, PowerShell, installer, contract, and forbidden-name test
  surfaces.
- README instructions for selecting, verifying, installing, and running the
  amd64 assets while retaining the existing source-build installer path.
- An operational separation between automated release checks and external
  organizational/staging compatibility gates.

Out of scope:

- Docker or any container image.
- arm64 or any non-amd64 architecture.
- macOS release artifacts.
- RPM, APK, MSI, MSIX, Homebrew, Scoop, or other package formats.
- Apt/Yum/other package repositories.
- An auto-updater or update-notification feature.
- Artifact signing, provenance attestations, or key management; checksums and
  SBOMs do not provide signature authenticity.
- Changing installer behavior; the existing prebuilt-binary inputs remain the
  integration point.
- Adding a repository `LICENSE` file. The approved `MIT` value authorizes
  `.deb` metadata only; a repository license file still requires separate
  explicit authorization and exact approved text.
- Making internal Jira/Bitbucket hosts or organizational Claude/Codex clients
  reachable from GitHub-hosted runners.
- Inventing new forbidden aliases that are not listed by repository authority.

## Approach

Use the fewest release surfaces that cover the approved outcome:

| File | Responsibility |
|---|---|
| `cmd/atlassian-mcp/main.go` | Make the existing version value linker-injectable while preserving local-build behavior. |
| `.goreleaser.yaml` | Define the two amd64 binaries, raw uploads, `.deb`, checksums, SBOMs, changelog, draft release, and deterministic names. |
| `.github/workflows/release.yml` | Run quality gates, build snapshots, create draft releases, validate exact assets on hosted runners, and publish after protected approval. |
| `README.md` | Document asset selection, checksum verification, installation, version check, and source-build fallback. |
| `docs/plans/active/release-amd64-artifacts.md` | Track decisions, progress, automated evidence, external evidence, recovery, and the final result. |

Do not add standalone release-verification scripts in this iteration. The
verification commands have one caller, GitHub Actions, so keeping their short
platform-specific commands in the workflow avoids duplicated orchestration.
Add scripts only if a second consumer is approved later.

### Artifact contract

Use deterministic names with these forms, with `<version>` equal to the tag
without the leading `v`:

- `atlassian-mcp_<version>_windows_amd64.exe`
- `atlassian-mcp_<version>_linux_amd64`
- `atlassian-mcp_<version>_linux_amd64.deb`
- `atlassian-mcp_<version>_checksums.txt`
- `atlassian-mcp_<version>_windows_amd64.sbom.json`
- `atlassian-mcp_<version>_linux_amd64.sbom.json`
- `atlassian-mcp_<version>_linux_amd64.deb.sbom.json`

GoReleaser `archives.formats: [binary]` publishes the executables directly,
not inside `.zip` or `.tar.gz` archives. The checksum manifest covers the two
executables and the `.deb`; CI parses all three SBOM documents independently.

### Automated checks versus external gates

| Gate | Execution surface | Evidence | Publication effect |
|---|---|---|---|
| Go unit/contract and race tests | GitHub-hosted Ubuntu | Job logs and check conclusion | Blocks draft creation. |
| ShellCheck and Bash installer tests | GitHub-hosted Ubuntu | Job logs and check conclusion | Blocks draft creation. |
| PSScriptAnalyzer and Pester | GitHub-hosted Windows | Job logs and check conclusion | Blocks draft creation. |
| GoReleaser config/snapshot, inventory, checksums, SBOMs, Linux raw binary, and `.deb` | GitHub-hosted Ubuntu | Job logs plus Actions artifacts or draft assets | Blocks draft creation/publication. |
| Windows `.exe` execution and version | GitHub-hosted Windows | Job logs from the downloaded snapshot/draft asset | Blocks draft publication. |
| Claude Code smoke on the organization's pinned version | Organization-managed client environment | Version, tested commit/tag, result, date, and evidence link recorded for the protected release approval | External gate; not claimed as GitHub-hosted automation. |
| Codex smoke on the organization's pinned version | Organization-managed client environment | Version, tested commit/tag, result, date, and evidence link recorded for the protected release approval | External gate; not claimed as GitHub-hosted automation. |
| Jira Server 6.4.14 and Bitbucket Server 5.10.2 contracts | Internal staging environment | Server versions, tested commit/tag, sanitized result, date, and evidence link | External staging gate; not claimed as mocked CI proof. |

The GitHub `release` environment must require an authorized reviewer. That
reviewer approves only after all four external evidence records refer to the
same tag being published. Client versions, host URLs, credentials, and raw
responses must not be placed in the workflow, release assets, or public logs.

## Configuration, Compatibility, And Deployment

- `.goreleaser.yaml` starts with `version: 2` and `project_name:
  atlassian-mcp`.
- The single Go build points at `./cmd/atlassian-mcp`, sets `binary:
  atlassian-mcp`, `CGO_ENABLED=0`, and declares only
  `linux_amd64_v1` and `windows_amd64_v1` targets.
- Build flags use `-trimpath`; linker flags set `main.version` from
  `{{ .Version }}`. The source fallback remains `0.1.0` for ordinary local
  builds.
- The binary archive format is `binary`, with an upload name template that
  includes project, version, OS, and architecture. No archive files are
  produced.
- The one nFPM entry consumes the Linux build, sets `formats: [deb]`, installs
  into `/usr/bin`, sets the approved Maintainer exactly as recorded in
  **Decisions**, and sets the License exactly to `MIT`. It declares no
  package-manager dependencies unless executable inspection proves one is
  necessary.
- The checksum algorithm is explicitly `sha256` with one deterministic
  manifest.
- Two SBOM entries catalog the binary artifacts and Debian package separately
  with Syft's SPDX JSON output. CI installs a pinned Syft release before
  GoReleaser runs.
- Changelog generation uses GoReleaser's `git` implementation without filters,
  so release notes do not silently omit repository changes.
- GoReleaser's source-archive, Docker, signing, package-repository, and
  publisher integrations remain disabled/absent.
- GoReleaser creates releases as drafts. Only the final protected job changes
  the same draft release to published; it does not rebuild or replace validated
  assets.
- The workflow uses Go `1.25.x`, pins third-party actions and release tools to
  reviewed immutable versions, and grants `contents: write` only to the jobs
  that create, list, validate, or update draft GitHub releases. Pull-request
  jobs remain read-only and never run with `pull_request_target`.
- The tag must match `v<semantic-version>`. `v` is removed for binary/package
  version output; invalid tags fail before GoReleaser runs.
- Existing source-build installation remains compatible. README release
  examples download an asset, verify it, then either install it directly or
  pass it to the existing `--binary` / `-Binary` option.

## Progress

- [x] Verify repository workflow, Task 27, current version source, Go version,
  installer/test surfaces, and absence of release configuration.
- [x] Verify the target is the workspace plan document
  `docs/plans/active/release-amd64-artifacts.md`.
- [x] Record the approved amd64-only artifact scope and separate automated
  checks from external compatibility gates.
- [x] Record the approved Debian Maintainer as
  `chiendao1808 <chiendao1808@gmail.com>` in **Decisions**.
- [x] Record the approved Debian License metadata exactly as `MIT` in
  **Decisions** and change plan status to Active.
- [x] Change `cmd/atlassian-mcp/main.go` from a version constant to a
  linker-injectable package variable with a short comment explaining the
  release override. Do not alter CLI output or server-version propagation.
- [x] Add `.goreleaser.yaml` with only the two approved targets, raw binary
  uploads, one `.deb`, SHA-256 checksums, three SPDX JSON SBOMs, Git changelog,
  and draft release mode.
- [ ] Run `goreleaser check` in GitHub-hosted CI; expect a valid v2
  configuration with no unrecognized/deprecated keys.
- [x] Add `.github/workflows/release.yml` with read-only Ubuntu/Windows quality
  jobs for pull requests and the default branch.
- [x] In the Ubuntu quality job, run `go test ./...`, `go test -race ./...`,
  `shellcheck scripts/install-from-remote.sh tests/install-from-remote_test.sh`,
  and `bash tests/install-from-remote_test.sh`. The existing Go suite includes
  HTTP/MCP contract and forbidden-name coverage.
- [x] In the Windows quality job, install pinned PSScriptAnalyzer and Pester
  versions, run `Invoke-ScriptAnalyzer` against the PowerShell installer and
  test file with errors failing the job, then run
  `Invoke-Pester tests/install-from-remote.Tests.ps1`.
- [x] Build a GoReleaser snapshot on pull requests/default-branch pushes and
  upload the generated `dist` assets as a short-retention Actions artifact for
  the hosted validation jobs.
- [x] On Ubuntu, assert the snapshot has exactly the three delivery artifact
  types plus checksum/SBOM evidence, verify checksums, parse every SBOM as SPDX
  JSON, execute the Linux raw binary, inspect the `.deb` control data and file
  list, install it in the runner, and verify its `--version` output.
- [x] On Windows, download the same snapshot, assert there is exactly one
  amd64 `.exe`, execute `--version`, and compare the exact output to the
  snapshot version.
- [x] Add the `v*` tag path: validate semantic tag shape, run the same quality
  jobs, use GoReleaser to create a draft release, and upload no asset outside
  the artifact contract.
- [x] Download the exact draft-release assets on GitHub-hosted Ubuntu and
  Windows runners and repeat inventory, checksum, SBOM, Debian, and executable
  validation against the tag version.
- [ ] Configure the protected GitHub `release` environment with required
  reviewers. Keep the publication job waiting for approval after automated
  draft-asset validation.
- [ ] Record same-tag Claude Code, Codex, Jira 6.4.14, and Bitbucket 5.10.2
  evidence in the protected environment approval record without secrets or
  internal response bodies.
- [x] Publish the already validated draft with `gh release edit <tag>
  --draft=false`; do not rerun GoReleaser or replace assets in this job.
- [x] Update `README.md` with the amd64 support boundary, exact asset-selection
  rules, SHA-256 verification commands for Bash and PowerShell, `.deb` and raw
  binary installation, Windows execution, `--version` checks, SBOM purpose,
  and the existing source-build fallback.
- [x] Review the workflow's effective permissions, action/tool pins, asset
  retention, absence of secret output, and lack of unrequested publishers.
- [x] CR-001: serialize same-ref release workflow runs and re-download draft
  assets after protected approval, verifying them against the validation job's
  uploaded SHA-256 manifest before publishing the existing draft.
- [x] CR-002: make the Windows quality job print PSScriptAnalyzer
  error-severity diagnostics and explicitly fail when any are present.
- [x] CR-003: validate the complete GoReleaser `artifacts.json` uploadable
  artifact inventory against the exact expected asset names before staging the
  snapshot artifact for hosted validation.
- [x] CR-003 follow-up: inspect every GoReleaser `artifacts.json` entry by
  artifact category, internal type, ID, and name before filename staging, so
  unexpected uploadable artifacts or publishers are rejected even when their
  names do not start with `atlassian-mcp_`.
- [x] CR-004: remove unnecessary `actions: write` from
  `validate-draft-ubuntu`; superseded on 2026-08-12 by the user-approved
  `contents: write` scope needed for draft release listing by release ID.
- [x] Fix GitHub Actions run 31509587046 Ubuntu ShellCheck failure by replacing
  the Bash installer test's `grep | wc -l` count pipeline with a
  ShellCheck-clean `grep -F -c` count that still treats no matches as zero.
- [x] Fix GitHub Actions run 31509587046 Windows PSScriptAnalyzer failure by
  invoking PSScriptAnalyzer separately for the two fixed PowerShell paths,
  collecting all error-severity diagnostics, printing them, and failing only
  when the collected count is non-zero.
- [x] Fix the hosted Syft download failure by changing the shared
  `SYFT_VERSION` workflow env from `1.29.0` to the existing release tag
  `v1.29.0`, which feeds both current `download-syft` call sites.
- [x] Temporarily comment out the GoReleaser Debian nFPM block, Debian
  checksum entry, and Debian SBOM entry at the user's request to diagnose the
  hosted Ubuntu snapshot failure.
- [x] Temporarily remove Debian package and Debian SBOM expectations from the
  snapshot artifact inventory and `validate-snapshot-ubuntu` job while keeping
  raw Linux, Windows, checksum, and binary SBOM checks.
- [x] Leave the tag-only draft and release validation jobs unchanged so any tag
  release remains blocked until the Debian package path is restored.
- [x] Fix GitHub Actions run 31554869192 `Validate draft on Ubuntu` failure by
  passing the explicit repository to all no-checkout GitHub CLI release calls:
  Ubuntu draft download, Windows draft download, publish re-download, and
  publish edit.
- [x] Fix GitHub Actions run 31555579728 draft validator failures by replacing
  release-by-tag draft access with exact-one draft release-ID lookup, paginated
  asset-ID inventory checks, asset-ID downloads, and a release-ID REST publish
  patch.
- [x] Temporarily comment Debian `.deb` and Debian SBOM expectations out of
  both tag draft validators, and comment the Ubuntu draft `.deb`
  metadata/content/install checks, at the user's request. Keep release-ID and
  asset-ID validation, checksum validation, binary SBOM parsing, raw Linux,
  Windows executable, strict inventory, and protected publish checks intact.
- [x] Fix the hosted workflow parser failure caused by trailing commas after
  the active Windows binary-SBOM entry in both PowerShell draft-validation
  `$expectedFiles` arrays while Debian SBOM lines are temporarily commented.
- [ ] Record automated job URLs and external evidence links in **Validation**
  and **Result**. Keep the plan active if a first tagged release has not yet
  passed all gates.
- [ ] After one release satisfies all acceptance criteria, record limitations
  and move this file to `docs/plans/completed/` in the same completion change.

## Risks And Recovery

- Risk: the `.deb` License metadata is mistaken for authorization to add a
  repository license file. Mitigation: keep `MIT` limited to the nFPM metadata,
  do not create `LICENSE`, and have CI inspect the approved Maintainer and
  License in the built control data before publication.
- Risk: target expansion occurs through GoReleaser defaults. Mitigation: use an
  explicit two-target list and an exact asset inventory assertion that rejects
  all unexpected uploads.
- Risk: a hard-coded `0.1.0` ships under a later tag. Mitigation: linker
  injection plus exact `--version` checks on both binaries and the installed
  `.deb`.
- Risk: snapshot proof differs from published bits. Mitigation: release to a
  draft, download and validate those exact assets on both hosted operating
  systems, serialize same-ref release workflows, upload a trusted validation
  hash manifest, re-download the draft after protected approval, and publish
  only if the post-approval assets still match the validation hashes.
- Risk: checksums are mistaken for signatures. Mitigation: README states that
  SHA-256 detects accidental or post-selection changes but does not establish
  publisher identity; signing remains explicitly out of scope.
- Risk: GitHub-hosted mock tests are overstated as legacy-server or client
  compatibility. Mitigation: separate job evidence from protected external
  gates and require same-tag organizational/staging records.
- Risk: untrusted pull-request code obtains release permissions. Mitigation:
  use `pull_request`, keep pull-request and snapshot validation permissions
  read-only, condition release operations on tag-only jobs, and scope
  `contents: write` narrowly.
- Risk: a release publishes before environment protection is configured.
  Mitigation: repository administrators configure required reviewers before
  enabling the `v*` trigger; verify protection with a harmless pre-release tag
  or workflow dispatch that stops at the approval gate.
- Recovery before publication: leave the GitHub release as a draft, correct the
  source/configuration on a new commit, move or create a new pre-release tag,
  rerun all checks, and delete the superseded draft only after confirming the
  replacement. Do not reuse a published semantic version.
- Recovery after an invalid publication: immediately delete the GitHub release
  assets/release to stop distribution, retain evidence of why it was withdrawn,
  revert the release change if necessary, fix on a new commit, and publish a
  new patch-version tag. Do not silently replace assets under the old tag.
- Recovery for workflow/config regressions: revert `.goreleaser.yaml`,
  `.github/workflows/release.yml`, the version-variable change, and README
  release documentation as one coherent release-pipeline rollback; existing
  source-build installers remain available because they are not modified.

## Decisions

- 2026-08-11: Publish amd64 only: raw Windows `.exe`, raw Linux executable,
  Debian `.deb`, SHA-256 checksums, and SBOMs. Docker, arm64, RPM, package
  repositories, and auto-update behavior are excluded.
- 2026-08-11: Use GitHub-hosted Ubuntu and Windows runners as the requested
  platform validation; no local Windows/Ubuntu proof is required.
- 2026-08-11: Keep Task 27's pinned Claude/Codex smoke tests and Jira/Bitbucket
  staging contracts as protected external gates. Automated mocked checks do not
  satisfy or replace them.
- 2026-08-11: Use one workflow and inline verification commands because the
  commands have one consumer. Avoid standalone release-script scaffolding.
- 2026-08-11: Publish executables with GoReleaser's binary archive format, not
  compressed archives, to match the approved raw-artifact contract.
- 2026-08-11: Create a draft first and validate its downloaded assets before
  publication, avoiding a rebuild between validation and release.
- 2026-08-11: Preserve `0.1.0` as the local-build fallback and inject the tag
  version only during release builds.
- 2026-08-11: Treat GitHub-generated source-code links as platform behavior,
  not additional uploaded release artifacts.
- 2026-08-11: Do not infer a repository license from dependencies, authorship,
  repository visibility, or package defaults.
- 2026-08-11: Set the Debian Maintainer metadata exactly to
  `chiendao1808 <chiendao1808@gmail.com>` as approved by the user.
- 2026-08-11: The phrase `LICENSE mặc định` does not identify a license and is
  not authority to choose one; no default SPDX license can be inferred.
- 2026-08-11: Set the Debian License metadata exactly to `MIT` as approved by
  the user. This authorizes `.deb` metadata only and does not authorize adding
  a repository `LICENSE` file.
- 2026-08-11: Pin GitHub Actions steps to resolved action commit IDs with
  version comments; release tools are pinned through GoReleaser `v2.17.1` and
  Syft `v1.29.0`.
- 2026-08-11: Stage snapshot assets from GoReleaser `artifacts.json` before
  uploading the Actions artifact because local binary archive paths stay under
  target directories while release upload names are flattened.
- 2026-08-11: Use workflow-level concurrency keyed by workflow name and Git ref
  with `cancel-in-progress: false`, so a second run for the same tag cannot
  replace draft assets while the first run is waiting for validation, approval,
  or publication.
- 2026-08-11: Treat the Ubuntu draft-validation job's uploaded
  `validated-assets.sha256` and `validated-assets.txt` as the trusted
  post-validation manifest. The protected publish job must re-download the
  draft assets after approval and compare them with that manifest before
  calling `gh release edit --draft=false`.
- 2026-08-11: PSScriptAnalyzer remains limited to `-Severity Error`; the
  workflow prints those diagnostics and fails only when error-severity results
  exist.
- 2026-08-11: PSScriptAnalyzer 1.24.0 rejects the workflow's previous
  two-element object array supplied to `Invoke-ScriptAnalyzer -Path`, so the
  Windows quality job analyzes each fixed path separately and then evaluates
  the aggregated diagnostics.
- 2026-08-11: ShellCheck SC2126 rejects the Bash installer's test-only
  `grep | wc -l` count pipeline; `grep -F -c` is the canonical count form, with
  explicit no-match normalization because `grep` exits 1 after printing zero.
- 2026-08-11: The pinned `download-syft` action constructs the Syft release URL
  from the supplied version string; the hosted failure requested
  `https://github.com/anchore/syft/releases/1.29.0` and received 404. The
  verified Syft release is tagged `v1.29.0`, so the workflow env must include
  the leading `v`.
- 2026-08-11: Snapshot staging is allowed only after the complete GoReleaser
  `artifacts.json` manifest matches the approved release shape: `Metadata`
  `metadata.json` (`internal_type` 35), build-only `Binary` records for
  `atlassian-mcp` and `atlassian-mcp.exe` (`internal_type` 4), raw-binary
  upload records with ID `raw-binaries` (`internal_type` 2), the Debian package
  with ID `deb` (`internal_type` 6), exactly two binary SBOMs with ID
  `binary-sboms` (`internal_type` 28), one Debian SBOM with ID `deb-sbom`
  (`internal_type` 28), and the checksum manifest (`internal_type` 12).
  Unexpected manifest entries are rejected even when their names do not start
  with `atlassian-mcp_`.
- 2026-08-11: Superseded on 2026-08-12: `validate-draft-ubuntu` no longer
  retains only `contents: read`; both draft validators now use the
  user-approved `contents: write` scope to list draft releases by release ID.
- 2026-08-12: At the user's request, temporarily disable Debian package
  generation and snapshot-only Debian validation to isolate the hosted Ubuntu
  failure. This is not a release-contract change; restore the commented
  GoReleaser Debian package, checksum, SBOM entries, and snapshot validations
  after diagnosis. Tag-only draft/release validation remains intentionally
  unchanged and therefore blocks releases while Debian assets are disabled.
- 2026-08-12: GitHub Actions run 31554869192 failed in `Validate draft on
  Ubuntu` because `validate-draft-ubuntu`, `validate-draft-windows`, and
  `publish-release` do not run `actions/checkout`, so GitHub CLI cannot infer
  the repository from `.git`. Keep those jobs checkout-free and pass
  `--repo` from `GITHUB_REPOSITORY` on the four `gh release` callers instead.
- 2026-08-12: GitHub Actions run 31555579728 showed `Create draft release`
  succeeded but both draft validators failed with `release not found`; passing
  `--repo` was insufficient because tag-based release download resolves
  published releases, not draft releases.
- 2026-08-12: The user approved scoped validator permission elevation to
  `contents: write` for `validate-draft-ubuntu` and `validate-draft-windows`
  only, so their `GITHUB_TOKEN` can list the intended draft release. Other job
  permissions remain unchanged.
- 2026-08-12: Draft validation and protected publication resolve exactly one
  draft release by `draft == true` and exact `GITHUB_REF_NAME`, enumerate all
  release and asset pages, require the exact expected asset names, download by
  asset ID, and publish with a release-ID REST `PATCH` that sets only `draft`
  to `false`.
- 2026-08-12: At the user's request, extend the temporary Debian bypass from
  snapshot validation to tag draft validation so hosted run 31561263646 can be
  retested against the current five-asset draft. Keep the Debian lines
  commented in place as the restoration map. Restore Debian package generation,
  Debian SBOM expectations, draft `.deb` checks, and the production seven-asset
  contract before any actual release.
- 2026-08-12: PowerShell array entries cannot keep a trailing comma before a
  commented Debian SBOM line. During the temporary Debian bypass, the active
  Windows binary-SBOM entry is the final `$expectedFiles` element in both
  Windows draft validators; when restoring the commented Debian SBOM item, add
  that comma back to the Windows SBOM entry in the same edit.

Promote a lasting release-support policy to `docs/decisions/` only if the
implemented behavior establishes a broader architectural rule beyond this
task-local amd64 release contract.

## Open Questions

None for implementation. The Debian Maintainer and License metadata decisions
are resolved. The `MIT` decision applies only to `.deb` metadata; adding a
repository `LICENSE` file remains unauthorized and out of scope.

Release-time prerequisites that do not change the implementation architecture
but still block publication are: the organization's pinned Claude Code and
Codex version numbers, access to the internal staging hosts, authorized test
credentials, evidence locations, and a configured required reviewer for the
GitHub `release` environment.

## Validation

Focused proof:

- `goreleaser check`
- A CI-only linker check that builds with a sentinel version and expects exact
  `atlassian-mcp <sentinel>` output.
- Snapshot asset inventory, SHA-256 verification, SPDX JSON parsing, Linux
  binary execution, Debian metadata/content/install checks, and Windows `.exe`
  execution on GitHub-hosted runners.

Integration or end-to-end proof:

- `go test ./...`
- `go test -race ./...`
- `bash tests/install-from-remote_test.sh`
- `Invoke-Pester tests/install-from-remote.Tests.ps1`
- Exact downloaded draft-release validation on GitHub-hosted Ubuntu and
  Windows before publication.
- Same-tag Claude Code and Codex MCP smoke evidence on the organization's
  pinned versions.
- Same-tag contract/smoke evidence against internal Jira Server 6.4.14 and
  Bitbucket Server 5.10.2 staging hosts.

Repository-required checks:

- `shellcheck scripts/install-from-remote.sh tests/install-from-remote_test.sh`
- `Invoke-ScriptAnalyzer` on `scripts/install-from-remote.ps1` and
  `tests/install-from-remote.Tests.ps1`, with analyzer findings failing CI.
- Existing repository forbidden-name guards through the Go suite, including
  `internal/app/naming_test.go`; expand only from a repository-approved list.
- `git diff --check` during implementation review.
- Inspect the release asset API response and fail unless every uploaded asset
  matches the exact artifact contract.

Local validation on Windows:

- `codegraph sync .` passed; the index was already up to date before
  implementation inspection.
- `goreleaser check` passed with local GoReleaser `2.17.1`.
- `go test ./...` passed after rerunning with a longer timeout.
- `go test -race ./...` passed.
- `go build -trimpath -ldflags "-X main.version=9.8.7-local" -o
  $env:TEMP/atlassian-mcp-linker-check.exe ./cmd/atlassian-mcp`, then
  `--version`, returned `atlassian-mcp 9.8.7-local`.
- `bash tests/install-from-remote_test.sh` passed through Git Bash at
  `C:\Program Files\Git\bin\bash.exe`.
- `Invoke-Pester tests/install-from-remote.Tests.ps1` through Windows
  PowerShell exited 0 and printed the installer test `PASS` markers. The local
  bundled Pester module was `3.4.0`; CI installs pinned Pester `5.7.1`.
- `goreleaser release --snapshot --clean --skip=publish --skip=sbom` passed.
  This proved the two cross-built raw binary names, `.deb` name, checksum
  manifest generation, and linker version propagation without publishing.
- A local snapshot with a temporary fake `syft` command outside the repository
  found and fixed an invalid Debian SBOM document template that referenced
  `.Format`, which package SBOM templates did not expose.
- The same fake-Syft check showed `artifacts: binary` did not match the
  delivered raw-binary archive artifacts, so binary SBOMs now catalog the
  `raw-binaries` archive artifacts.
- `goreleaser release --snapshot --clean --skip=publish` passed with a
  temporary fake `syft` command that writes minimal SPDX JSON. This proved
  GoReleaser's SBOM artifact selection and document names:
  `atlassian-mcp_1.0.4-snapshot_linux_amd64.sbom.json`,
  `atlassian-mcp_1.0.4-snapshot_windows_amd64.sbom.json`, and
  `atlassian-mcp_1.0.4-snapshot_linux_amd64.deb.sbom.json`.
- Local staging from `dist/artifacts.json` produced
  `atlassian-mcp_1.0.4-snapshot_linux_amd64`,
  `atlassian-mcp_1.0.4-snapshot_windows_amd64.exe`,
  `atlassian-mcp_1.0.4-snapshot_linux_amd64.deb`, and
  `atlassian-mcp_1.0.4-snapshot_checksums.txt`, plus all three SBOM filenames;
  PowerShell checksum verification passed and the staged Windows executable returned
  `atlassian-mcp 1.0.4-snapshot`.
- A PowerShell equivalent of the new GoReleaser `artifacts.json` inventory
  check passed locally because `jq` is unavailable on this Windows host.
- After CR-003 follow-up, a PowerShell equivalent of the complete-manifest
  inventory check passed locally against a generated snapshot manifest,
  including validation of allowed artifact categories, internal types, IDs,
  and exact expected release asset names. A resumed verification pass initially
  failed when the local check used a hard-coded snapshot version; using
  `dist/metadata.json` as the workflow does fixed the harness mismatch and
  passed against the generated `1.0.4-snapshot` manifest.
- A local publish-reverification simulation generated a trusted
  `validated-assets.sha256` from staged draft files, copied those files to a
  separate publish directory, and `sha256sum -c` verified all seven assets
  before publication logic.
- `git diff --check` passed with line-ending warnings only for existing
  Windows checkout behavior.
- GitHub Actions run 31509587046 produced two red hosted checks that required
  follow-up: Ubuntu ShellCheck reported SC2126 at
  `tests/install-from-remote_test.sh:46`, and Windows PSScriptAnalyzer 1.24.0
  rejected an `Object[]` argument supplied to `Invoke-ScriptAnalyzer -Path` in
  `.github/workflows/release.yml`.
- The Bash count helper now uses `grep -F -c` and normalizes grep's no-match
  exit to count `0`; local Git Bash execution of
  `tests/install-from-remote_test.sh` is the focused behavioral proof.
- The Windows quality job now invokes PSScriptAnalyzer separately for
  `scripts/install-from-remote.ps1` and `tests/install-from-remote.Tests.ps1`,
  aggregates diagnostics, prints them, and throws when the aggregate
  error-severity count is non-zero.
- `C:\Program Files\Git\bin\bash.exe tests/install-from-remote_test.sh` passed
  after the Bash count fix and printed `PASS install-from-remote_test.sh`.
- `go test ./...` passed after the GitHub Actions follow-up fixes.
- `goreleaser check` passed after the GitHub Actions follow-up fixes.
- `git diff --check` passed after the GitHub Actions follow-up fixes with
  line-ending warnings only for the edited workflow, plan, and Bash test files.
- The Syft download root cause came from a hosted GitHub Actions log: the
  pinned `anchore/sbom-action/download-syft` action received `1.29.0`,
  requested `https://github.com/anchore/syft/releases/1.29.0`, and received
  404. The release exists as tag `v1.29.0` (published 2025-07-21), so the
  workflow now sets `SYFT_VERSION: "v1.29.0"` for both existing download
  call sites. A hosted snapshot/release run must prove the actual download.
- Static workflow validation passed: `.github/workflows/release.yml` contains
  `SYFT_VERSION: "v1.29.0"`, contains no old `SYFT_VERSION: "1.29.0"` value,
  and has exactly two `syft-version: ${{ env.SYFT_VERSION }}` references across
  exactly two `download-syft` action call sites.
- `goreleaser check` passed after the Syft tag fix.
- `git diff --check` passed after the Syft tag fix with line-ending warnings
  only for the edited workflow and plan files.
- A temporary Debian snapshot bypass was applied on 2026-08-12 for diagnosis:
  `.goreleaser.yaml` has the Debian nFPM, checksum, and SBOM entries commented
  out, and the snapshot-only workflow path no longer expects or checks `.deb`
  or Debian SBOM assets. The tag-only draft/release validation path was left
  unchanged and is expected to block any tag release until Debian artifacts are
  restored.
- `go run github.com/goreleaser/goreleaser/v2@v2.17.1 check` passed after the
  temporary Debian snapshot bypass; the local Go toolchain downloaded and used
  Go `1.26.5` because GoReleaser v2.17.1 now requires it.
- `git diff --check` passed after the temporary Debian snapshot bypass with
  line-ending warnings only for the edited workflow, GoReleaser config, and
  active plan files.
- Targeted workflow search confirmed remaining Debian references are only in
  tag-only draft validation, so tag releases remain intentionally blocked while
  Debian generation is disabled.
- GitHub Actions run 31554869192 failed before draft-asset validation with
  `fatal: not a git repository` at `gh release download`. The workflow now
  supplies `--repo` from `GITHUB_REPOSITORY` on the Ubuntu and Windows draft
  downloads, the protected publish re-download, and the protected publish edit;
  a hosted rerun is still required to prove these no-checkout GitHub CLI calls.
- GitHub Actions run 31555579728 created the draft release successfully, then
  both draft validators failed with `release not found`. The root cause is the
  release-by-tag draft access path: draft releases must be selected from the
  release list by release ID before assets can be inventoried or downloaded.
- Static workflow invariant check passed after the release-ID fix: no
  `gh release download` or `gh release edit` commands remain, both draft
  validators have `contents: write`, paginated release lookup and asset-ID API
  use are present, Windows downloads use `Invoke-WebRequest -OutFile`, publish
  uses a REST `PATCH` with `draft=false`, and tag draft validation still expects
  Debian assets while the temporary Debian bypass remains snapshot-only.
- `git diff --check` passed after the release-ID fix with the existing Windows
  checkout line-ending warning only for `.github/workflows/release.yml`.
- Static workflow invariant check passed after the temporary tag draft Debian
  bypass: the Ubuntu draft-download list, Ubuntu draft validation list, Windows
  draft-download list, and Windows executable-validation list keep Debian
  `.deb` and Debian SBOM entries only as comments; the Ubuntu draft `.deb`
  metadata/content/install commands are commented; the Ubuntu draft validation
  step no longer says `deb`; checksum, binary SBOM, raw Linux, Windows
  executable, strict asset-inventory, release-ID/asset-ID, and protected
  publish logic remain active.
- A second targeted workflow static check confirmed no active Debian package
  validation command remains in the draft Ubuntu block while those commands
  stay commented as the restoration map.
- `git diff --check` passed after the temporary tag draft Debian bypass with
  the existing Windows checkout line-ending warning only for
  `.github/workflows/release.yml` and
  `docs/plans/active/release-amd64-artifacts.md`.
- PowerShell Core (`pwsh`) is unavailable locally. A Windows PowerShell parser
  static check passed for the two edited draft-validation `$expectedFiles`
  snippets, confirmed no trailing comma remains before the commented Debian
  SBOM lines, and confirmed both restoration comments mention adding the
  Windows SBOM comma back.
- `git diff --check` passed after the PowerShell trailing-comma fix with the
  existing Windows checkout line-ending warning only for
  `.github/workflows/release.yml` and
  `docs/plans/active/release-amd64-artifacts.md`.

Local limitations and remaining proof:

- Local command availability check found `shellcheck`, `pwsh`, `actionlint`,
  `jq`, and `yq` unavailable on this Windows host.
- `shellcheck` is not installed locally, so ShellCheck remains GitHub-hosted
  CI proof. Re-run the hosted Ubuntu quality job to prove SC2126 is resolved.
- `pwsh` and PSScriptAnalyzer are not installed locally, so PSScriptAnalyzer
  remains GitHub-hosted Windows proof. Re-run the hosted Windows quality job to
  prove the PSScriptAnalyzer 1.24.0 `-Path` invocation is resolved.
- Syft is not installed locally, and `go install` of Syft `v1.29.0` plus
  GoReleaser `v2.17.1` timed out after five minutes. Real Syft SBOM content
  generation, download through `download-syft`, and parsing remain
  GitHub-hosted CI proof; local fake-Syft proof covered only GoReleaser SBOM
  selection and naming.
- Linux raw binary execution, Debian control/file-list inspection, and Debian
  install proof remain GitHub-hosted Ubuntu proof.
- Draft-release asset download, protected release publication, Claude Code,
  Codex, Jira Server 6.4.14, and Bitbucket Server 5.10.2 evidence have not run.
- The exact GitHub Actions concurrency behavior, artifact upload/download, and
  post-approval re-download must still be proven by a hosted tag workflow run.
- GitHub Actions run 31555579728 must be rerun on hosted runners to confirm the
  release-ID and asset-ID API path can validate the intended draft and reach
  the already-intentional Debian tag-validation block while Debian generation
  remains disabled.
- A hosted workflow rerun is still required to prove GitHub's YAML-to-pwsh
  execution path accepts the corrected Windows draft-validation arrays.

## Definition Of Done

- The approved Maintainer and exact License metadata decisions are recorded
  verbatim in **Decisions**.
- The four affected implementation/documentation files match the artifact and
  security contracts above without additional release targets or publishers.
- All automated GitHub-hosted checks pass for the tagged commit.
- The exact draft assets pass Linux, Windows, checksum, SBOM, Debian, naming,
  and version validation.
- Protected same-tag Claude Code, Codex, Jira 6.4.14, and Bitbucket 5.10.2
  evidence is recorded and approved.
- The validated draft is published unchanged and the public asset inventory is
  rechecked.
- README release instructions match the published names and verified commands.
- Recovery steps remain executable, known risks/limitations are recorded, and
  this plan's progress, validation, decisions, and result are reconciled.
- Only then is this plan moved from `docs/plans/active/` to
  `docs/plans/completed/`.

## Result

Implementation added within the approved scope. No tag was pushed, no draft
release was created, and no release was published. Work remains Active until
GitHub-hosted CI, exact draft-asset validation, protected approval, and the
same-tag external compatibility gates pass.
