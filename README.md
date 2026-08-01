# Atlassian MCP

This repository contains the initial Go implementation for `atlassian-mcp` plus the planning artifacts that define the remaining release work.


## Canonical repository

Implementation repository:

```text
https://github.com/chiendao1808/atlassian-mcp.git
```

Raw installer URL patterns:

```text
https://raw.githubusercontent.com/chiendao1808/atlassian-mcp/<ref>/scripts/install-from-remote.sh
https://raw.githubusercontent.com/chiendao1808/atlassian-mcp/<ref>/scripts/install-from-remote.ps1
```

Use a release tag or full commit SHA for `<ref>` in production. Installer examples use the same repository as the default source:

```text
--source-repo-url https://github.com/chiendao1808/atlassian-mcp.git
-SourceRepoUrl 'https://github.com/chiendao1808/atlassian-mcp.git'
```


## Build and run

```bash
go test ./...
go build ./cmd/atlassian-mcp
./atlassian-mcp --version
```

Runtime uses MCP stdio. Protocol messages are written to `stdout`; logs and startup warnings are written to `stderr`.

Jira tools are registered when `JIRA_BASE_URL` is statically valid. Call `jira_authenticate` once per MCP process session before using Jira issue tools.

Bitbucket configuration is isolated from Jira startup. The Bitbucket module includes the shared REST client foundation for later repository and pull-request tools, but business tools are implemented by the follow-up Bitbucket tasks.

## Contents

```text
atlassian-mcp/
├── README.md
├── CHECKSUMS.sha256
├── manifest.json
├── docs/
│   └── atlassian-mcp-implementation-plan.md
└── references/
    └── jira-6.4.14_bitbucket-5.10.2-rest-api-reference.md
```

## Current naming only

The plan uses only:

- `atlassian-mcp`
- `scripts/install-from-remote.sh`
- `scripts/install-from-remote.ps1`
- `--source-repo-url` / `-SourceRepoUrl`
- `--source-ref` / `-SourceRef`
- explicit `jira-*` and `bitbucket-*` target parameters

No draft aliases or backward-compatibility layer is included.

## Bash installer bootstrap

The Bash installer is committed at the stable repository-root path:

```text
scripts/install-from-remote.sh
```

Fetch it from the canonical GitHub raw URL. Use a release tag or full commit SHA for `INSTALLER_REF` in production:

```bash
INSTALLER_REF='v1.0.0'
INSTALLER_URL="https://raw.githubusercontent.com/chiendao1808/atlassian-mcp/${INSTALLER_REF}/scripts/install-from-remote.sh"

curl -fsSL "$INSTALLER_URL" |
  bash -s -- \
    --source-repo-url https://github.com/chiendao1808/atlassian-mcp.git \
    --source-ref "$INSTALLER_REF" \
    --agents both \
    --enable-jira \
    --jira-base-url https://jira.internal.example.com/jira \
    --atlassian-tls-verify false \
    --non-interactive
```

Do not put credentials in the raw URL or `--source-repo-url`. Use SSH, a Git credential helper, or runtime environment variables for private access and secrets.

## PowerShell installer bootstrap

The PowerShell installer is committed at the stable repository-root path:

```text
scripts/install-from-remote.ps1
```

Fetch it from the canonical GitHub raw URL to a temporary file, then run that file explicitly:

```powershell
$INSTALLER_REF = 'v1.0.0'
$InstallerUrl = "https://raw.githubusercontent.com/chiendao1808/atlassian-mcp/${INSTALLER_REF}/scripts/install-from-remote.ps1"
$InstallerFile = Join-Path $env:TEMP 'install-from-remote.ps1'

Invoke-WebRequest -Uri $InstallerUrl -OutFile $InstallerFile

powershell.exe -NoProfile -ExecutionPolicy Bypass -File $InstallerFile `
  -SourceRepoUrl 'https://github.com/chiendao1808/atlassian-mcp.git' `
  -SourceRef $INSTALLER_REF `
  -Agents Both `
  -EnableJira `
  -JiraBaseUrl 'https://jira.internal.example.com/jira' `
  -AtlassianTlsVerify false `
  -NonInteractive
```

PowerShell 7 users may replace `powershell.exe` with `pwsh`. Do not put credentials in the raw URL or `-SourceRepoUrl`; use SSH, a Git credential helper, or runtime environment variables for private access and secrets.
