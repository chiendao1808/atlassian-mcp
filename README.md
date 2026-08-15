# Atlassian MCP

`atlassian-mcp` is a Go MCP stdio server for Jira, Confluence, and Bitbucket Server/Data Center APIs. The supported installer path is binary-first: installers download the published GitHub release executable, verify it with the release SHA-256 manifest, then register the binary with Claude Code and/or Codex.

## Canonical Repository

Implementation repository:

```text
https://github.com/chiendao1808/atlassian-mcp.git
```

Installer raw URLs:

```text
https://raw.githubusercontent.com/chiendao1808/atlassian-mcp/<ref>/scripts/install-from-remote.sh
https://raw.githubusercontent.com/chiendao1808/atlassian-mcp/<ref>/scripts/install-from-remote.ps1
```

The examples below use `main` only because the binary-first installer change is
currently unreleased. `main` is a moving bootstrap ref: after the next release,
replace it with that release's immutable tag or a full commit SHA. The release
binary pin can still target the already published `v1.0.4` assets.

## Tools

The current registered Jira, Confluence, and Bitbucket tools are listed in [docs/tools/catalog.md](docs/tools/catalog.md). Detailed module notes remain in [docs/tools/jira.md](docs/tools/jira.md) and [docs/tools/confluence.md](docs/tools/confluence.md).

## Release Artifacts

GitHub Releases publish amd64 assets only:

- `atlassian-mcp_<version>_linux_amd64`
- `atlassian-mcp_<version>_windows_amd64.exe`
- `atlassian-mcp_<version>_checksums.txt`
- SPDX JSON SBOM files for the raw Linux and Windows binaries

The installer supports Windows amd64 and Linux amd64 raw executables. Debian package generation and Debian SBOM assets are temporarily disabled in the current published inventory, so Linux installs should use the raw binary for now. ARM, macOS, Docker, RPM, package repositories, auto-update, signing, and provenance attestations are outside the current support boundary.

SHA-256 verification detects corrupted or changed files after selecting a release asset. It does not prove publisher identity because the checksum manifest is downloaded from the same GitHub release.

## Bash Install

Install the latest stable release:

```bash
INSTALLER_REF='main'
INSTALLER_URL="https://raw.githubusercontent.com/chiendao1808/atlassian-mcp/${INSTALLER_REF}/scripts/install-from-remote.sh"

curl -fsSL "$INSTALLER_URL" |
  bash -s -- \
    --install-dir "$HOME/.local/bin" \
    --agents both \
    --scope user \
    --project-dir "$(pwd)" \
    --enable-jira \
    --jira-base-url https://jira.internal.example.com/jira \
    --jira-ca-file /etc/ssl/certs/jira-internal-ca.pem \
    --jira-username svc-atlassian-mcp \
    --jira-password-env JIRA_PASSWORD \
    --enable-confluence \
    --confluence-base-url https://confluence.internal.example.com/confluence \
    --confluence-ca-file /etc/ssl/certs/confluence-internal-ca.pem \
    --confluence-username svc-atlassian-mcp \
    --confluence-password-env CONFLUENCE_PASSWORD \
    --enable-bitbucket \
    --bitbucket-base-url https://bitbucket.internal.example.com \
    --bitbucket-project-key ABC \
    --bitbucket-user-slug jane.doe \
    --bitbucket-token-env BITBUCKET_BEARER_TOKEN \
    --bitbucket-ca-file /etc/ssl/certs/bitbucket-internal-ca.pem \
    --atlassian-tls-verify false \
    --non-interactive
```

Pin an exact release binary:

```bash
curl -fsSL "$INSTALLER_URL" |
  bash -s -- \
    --release-tag v1.0.4 \
    --install-dir "$HOME/.local/bin" \
    --agents none \
    --enable-jira \
    --jira-base-url https://jira.internal.example.com/jira \
    --non-interactive
```

Install a local/offline binary instead of downloading from GitHub:

```bash
bash scripts/install-from-remote.sh \
  --binary /path/to/atlassian-mcp \
  --install-dir "$HOME/.local/bin" \
  --agents none \
  --enable-jira \
  --jira-base-url https://jira.internal.example.com/jira \
  --non-interactive
```

The Bash installer creates `atlassian-mcp` plus an `atlassian-mcp-run` wrapper. Claude/manual runs use the wrapper so token/password indirection is resolved at runtime. Codex is registered directly to the binary with explicit env values in `config.toml` because Codex does not pass its ambient environment to spawned MCP stdio servers.

## PowerShell Install

Install the latest stable release:

```powershell
$INSTALLER_REF = 'main'
$InstallerUrl = "https://raw.githubusercontent.com/chiendao1808/atlassian-mcp/${INSTALLER_REF}/scripts/install-from-remote.ps1"
$InstallerFile = Join-Path $env:TEMP 'install-from-remote.ps1'

Invoke-WebRequest -Uri $InstallerUrl -OutFile $InstallerFile

powershell.exe -NoProfile -ExecutionPolicy Bypass -File $InstallerFile `
  -InstallDir (Join-Path $HOME '.local\bin') `
  -Agents Both `
  -Scope User `
  -ProjectDir (Get-Location).Path `
  -EnableJira `
  -JiraBaseUrl 'https://jira.internal.example.com/jira' `
  -JiraCaFile 'C:\certs\jira-internal-ca.pem' `
  -JiraUsername 'svc-atlassian-mcp' `
  -JiraPasswordEnv 'JIRA_PASSWORD' `
  -EnableConfluence `
  -ConfluenceBaseUrl 'https://confluence.internal.example.com/confluence' `
  -ConfluenceCaFile 'C:\certs\confluence-internal-ca.pem' `
  -ConfluenceUsername 'svc-atlassian-mcp' `
  -ConfluencePasswordEnv 'CONFLUENCE_PASSWORD' `
  -EnableBitbucket `
  -BitbucketBaseUrl 'https://bitbucket.internal.example.com' `
  -BitbucketProjectKey 'ABC' `
  -BitbucketUserSlug 'jane.doe' `
  -BitbucketTokenEnv 'BITBUCKET_BEARER_TOKEN' `
  -BitbucketCaFile 'C:\certs\bitbucket-internal-ca.pem' `
  -AtlassianTlsVerify false `
  -NonInteractive
```

Pin an exact release binary:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File $InstallerFile `
  -ReleaseTag v1.0.4 `
  -InstallDir (Join-Path $HOME '.local\bin') `
  -Agents None `
  -EnableJira `
  -JiraBaseUrl 'https://jira.internal.example.com/jira' `
  -NonInteractive
```

Install a local/offline binary instead of downloading from GitHub:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts\install-from-remote.ps1 `
  -Binary 'C:\path\to\atlassian-mcp.exe' `
  -InstallDir (Join-Path $HOME '.local\bin') `
  -Agents None `
  -EnableJira `
  -JiraBaseUrl 'https://jira.internal.example.com/jira' `
  -NonInteractive
```

Restart Claude Code, Codex, and open terminals after a Windows install so they pick up newly persisted User environment variables. Codex also needs restart because it reads `config.toml` at startup.

## Credentials And Module Config

Installers never accept secret values directly as command-line arguments. They read secrets through env-var name indirection:

- `BITBUCKET_BEARER_TOKEN`, or the variable named by `--bitbucket-token-env` / `-BitbucketTokenEnv`.
- `JIRA_PASSWORD`, or the variable named by `--jira-password-env` / `-JiraPasswordEnv` when a Jira username is configured.
- `CONFLUENCE_PASSWORD`, or the variable named by `--confluence-password-env` / `-ConfluencePasswordEnv` when a Confluence username is configured.

At least one module must be enabled with `--enable-jira`, `--enable-confluence`, or `--enable-bitbucket` (PowerShell: `-EnableJira`, `-EnableConfluence`, `-EnableBitbucket`). Jira and Confluence support explicit authenticate tools and environment fallback; Bitbucket reads its bearer token from runtime config.

## Build And Local Verification

For source development:

```bash
go test ./...
go build ./cmd/atlassian-mcp
./atlassian-mcp --version
```

Runtime uses MCP stdio. Protocol messages are written to `stdout`; logs and startup warnings are written to `stderr`.

## Changelog

### Unreleased

- Switched PowerShell and Bash installers to download verified release executables by default.
- Kept explicit local/offline `-Binary` / `--binary` installation for development, air-gapped, and rollback cases.
- Added binary-first installation guide updates and the tool catalog reference.

### v1.0.4

- Published amd64 release artifacts for Windows and Linux with SHA-256 checksum manifests and SBOM assets.
