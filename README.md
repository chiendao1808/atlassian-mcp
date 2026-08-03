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

### Set required environment variables (Bash, user scope)

The installer never accepts secrets as arguments; it reads the Bitbucket bearer token from the environment variable named by `--bitbucket-token-env` (default `BITBUCKET_BEARER_TOKEN`) only at wrapper runtime. Set it once for your user account so it survives new shells/terminals, then export it in the current shell before running the installer:

```bash
# Persist for the user account: append to the shell profile loaded on login
echo "export BITBUCKET_BEARER_TOKEN='<your-bitbucket-token>'" >> "$HOME/.bashrc"   # bash
# echo "export BITBUCKET_BEARER_TOKEN='<your-bitbucket-token>'" >> "$HOME/.profile"  # sh/other login shells

# Load it into the current shell (or open a new terminal)
source "$HOME/.bashrc"

# Confirm it is visible to the process the installer will run in
echo "$BITBUCKET_BEARER_TOKEN"
```

Fetch it from the canonical GitHub raw URL. Use a release tag or full commit SHA for `INSTALLER_REF` in production:

```bash
INSTALLER_REF='v1.0.0'
INSTALLER_URL="https://raw.githubusercontent.com/chiendao1808/atlassian-mcp/${INSTALLER_REF}/scripts/install-from-remote.sh"

curl -fsSL "$INSTALLER_URL" |
  bash -s -- \
    --source-repo-url https://github.com/chiendao1808/atlassian-mcp.git \
    --source-ref "$INSTALLER_REF" \
    --source-clone-depth 1 \
    --install-dir "$HOME/.local/bin" \
    --agents both \
    --scope user \
    --project-dir "$(pwd)" \
    --enable-jira \
    --jira-base-url https://jira.internal.example.com/jira \
    --jira-ca-file /etc/ssl/certs/jira-internal-ca.pem \
    --enable-bitbucket \
    --bitbucket-base-url https://bitbucket.internal.example.com \
    --bitbucket-project-key ABC \
    --bitbucket-user-slug jane.doe \
    --bitbucket-token-env BITBUCKET_BEARER_TOKEN \
    --bitbucket-ca-file /etc/ssl/certs/bitbucket-internal-ca.pem \
    --atlassian-tls-verify false \
    --non-interactive
    # --keep-source        # omit unless debugging the cloned checkout
    # --binary /path/to/atlassian-mcp   # alternative to --source-repo-url
    # --skip-tests         # omit to run `go test ./...` before building
    # --dry-run            # omit to actually install; add to only validate args
    # --replace            # add only to overwrite an unmanaged Claude config
```

Do not put credentials in the raw URL or `--source-repo-url`. Use SSH, a Git credential helper, or runtime environment variables for private access and secrets.

### Bash installer arguments

| Argument | Required | Sample value | Default | Description |
| --- | --- | --- | --- | --- |
| `--source-repo-url` | Yes, unless `--binary` is set | `https://github.com/chiendao1808/atlassian-mcp.git` | *(empty)* | Provider-neutral Git remote to clone and build `cmd/atlassian-mcp` from. Must not contain embedded credentials. |
| `--binary` | No (alternative to `--source-repo-url`) | `/path/to/atlassian-mcp` | *(empty)* | Path to a prebuilt binary to install directly, skipping clone and build. |
| `--source-ref` | No | `v1.0.0` | `main` | Git ref (branch, tag, or commit) checked out after cloning. |
| `--source-clone-depth` | No | `1` | `1` | Depth passed to `git clone`/`git fetch` for the source checkout. |
| `--keep-source` | No (flag) | — | disabled (source is cleaned up after a successful install) | Keeps the temporary cloned checkout on disk after install, for debugging. |
| `--install-dir` | No | `/home/me/.local/bin` | `$HOME/.local/bin` | Directory the built/provided binary and the generated wrapper script are installed into. |
| `--agents` | Yes, unless run interactively | `both` (`claude`\|`codex`\|`both`\|`none`) | prompted on a TTY; error with `--non-interactive` | Selects which coding agent config(s) to write: Codex TOML, Claude MCP JSON, both, or none. |
| `--scope` | No | `user` (`local`\|`project`\|`user`) | `user` | Chooses whether agent configs are written to the user home directory or to `--project-dir`. |
| `--project-dir` | No | `/home/me/projects/atlassian-mcp` | current working directory | Project directory used to resolve agent config paths when `--scope` is `local`/`project`. |
| `--enable-jira` | No (flag; at least one of `--enable-jira`/`--enable-bitbucket` is required) | — | disabled | Enables the Jira module and writes `JIRA_BASE_URL`/`JIRA_CA_FILE` into the wrapper. |
| `--jira-base-url` | Yes, if `--enable-jira` | `https://jira.internal.example.com/jira` | *(empty)* | Base URL of the Jira instance. Must be a plain http(s) URL with no query, fragment, or embedded credentials. |
| `--jira-ca-file` | No | `/etc/ssl/certs/jira-internal-ca.pem` | *(empty)* | Path to a custom CA bundle for validating the Jira server's TLS certificate. |
| `--enable-bitbucket` | No (flag; at least one of `--enable-jira`/`--enable-bitbucket` is required) | — | disabled | Enables the Bitbucket module and writes its base URL/project key/token indirection into the wrapper. |
| `--bitbucket-base-url` | Yes, if `--enable-bitbucket` | `https://bitbucket.internal.example.com` | *(empty)* | Base URL of the Bitbucket instance. Must be a plain http(s) URL with no query, fragment, or embedded credentials. |
| `--bitbucket-project-key` | Yes, if `--enable-bitbucket` | `ABC` | *(empty)* | Bitbucket project key that scopes the repository/pull-request tools. |
| `--bitbucket-user-slug` | No | `jane.doe` | *(empty)* | Bitbucket user slug used where tools need to identify the acting user. |
| `--bitbucket-token-env` | No | `BITBUCKET_BEARER_TOKEN` | `BITBUCKET_BEARER_TOKEN` | Name of the environment variable the wrapper reads the Bitbucket bearer token from at runtime; the token value itself is never written to config. |
| `--bitbucket-ca-file` | No | `/etc/ssl/certs/bitbucket-internal-ca.pem` | *(empty)* | Path to a custom CA bundle for validating the Bitbucket server's TLS certificate. |
| `--atlassian-tls-verify` | No | `false` (`true`\|`false`) | `false` | Controls whether the wrapper enables TLS certificate verification for Jira/Bitbucket requests. |
| `--skip-tests` | No (flag) | — | disabled (`go test ./...` runs before build) | Skips running the repository test suite before building the binary from source. |
| `--dry-run` | No (flag) | — | disabled | Validates all arguments and exits without cloning, building, installing, or writing any config. |
| `--replace` | No (flag) | — | disabled (refuses to overwrite an unmanaged Claude config) | Allows overwriting an existing Claude MCP config file that wasn't previously managed by this installer. |
| `--non-interactive` | No (flag) | — | disabled (prompts for `--agents` if omitted) | Disables the interactive `--agents` prompt and any other terminal prompts; missing required values become hard errors. |
| `-h`, `--help` | No (flag) | — | disabled | Prints usage text and exits. |

## PowerShell installer bootstrap

The PowerShell installer is committed at the stable repository-root path:

```text
scripts/install-from-remote.ps1
```

### Set required environment variables (PowerShell, user scope)

The installer never accepts secrets as arguments; it reads the Bitbucket bearer token from the environment variable named by `-BitbucketTokenEnv` (default `BITBUCKET_BEARER_TOKEN`) only at wrapper runtime. Persist it in the Windows per-user environment store so it survives new PowerShell/terminal sessions, then reload the current session before running the installer:

```powershell
# Persist for the user account (Windows User environment store)
[Environment]::SetEnvironmentVariable('BITBUCKET_BEARER_TOKEN', '<your-bitbucket-token>', 'User')

# Load it into the current session (or open a new terminal)
$env:BITBUCKET_BEARER_TOKEN = [Environment]::GetEnvironmentVariable('BITBUCKET_BEARER_TOKEN', 'User')

# Confirm it is visible to the process the installer will run in
$env:BITBUCKET_BEARER_TOKEN
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
  -SourceCloneDepth 1 `
  -InstallDir (Join-Path $HOME '.local\bin') `
  -Agents Both `
  -Scope User `
  -ProjectDir (Get-Location).Path `
  -EnableJira `
  -JiraBaseUrl 'https://jira.internal.example.com/jira' `
  -JiraCaFile 'C:\certs\jira-internal-ca.pem' `
  -EnableBitbucket `
  -BitbucketBaseUrl 'https://bitbucket.internal.example.com' `
  -BitbucketProjectKey 'ABC' `
  -BitbucketUserSlug 'jane.doe' `
  -BitbucketTokenEnv 'BITBUCKET_BEARER_TOKEN' `
  -BitbucketCaFile 'C:\certs\bitbucket-internal-ca.pem' `
  -AtlassianTlsVerify false `
  -NonInteractive
  # -KeepSource        # omit unless debugging the cloned checkout
  # -Binary 'C:\path\to\atlassian-mcp.exe'   # alternative to -SourceRepoUrl
  # -SkipTests         # omit to run `go test ./...` before building
  # -DryRun            # omit to actually install; add to only validate args
  # -Replace           # add only to overwrite an unmanaged Claude config
```

PowerShell 7 users may replace `powershell.exe` with `pwsh`. Do not put credentials in the raw URL or `-SourceRepoUrl`; use SSH, a Git credential helper, or runtime environment variables for private access and secrets.

### PowerShell installer arguments

| Argument | Required | Sample value | Default | Description |
| --- | --- | --- | --- | --- |
| `-SourceRepoUrl` | Yes, unless `-Binary` is set | `https://github.com/chiendao1808/atlassian-mcp.git` | *(empty)* | Provider-neutral Git remote to clone and build `cmd/atlassian-mcp` from. Must not contain embedded credentials. |
| `-Binary` | No (alternative to `-SourceRepoUrl`) | `C:\path\to\atlassian-mcp.exe` | *(empty)* | Path to a prebuilt binary to install directly, skipping clone and build. |
| `-SourceRef` | No | `v1.0.0` | `main` | Git ref (branch, tag, or commit) checked out after cloning. |
| `-SourceCloneDepth` | No | `1` | `1` | Depth passed to `git clone`/`git fetch` for the source checkout. |
| `-KeepSource` | No (switch) | — | disabled (source is cleaned up after a successful install) | Keeps the temporary cloned checkout on disk after install, for debugging. |
| `-InstallDir` | No | `C:\Users\me\.local\bin` | `Join-Path $HOME '.local\bin'` | Directory the built/provided binary and the generated wrapper script are installed into. |
| `-Agents` | Yes, unless run interactively | `Both` (`Claude`\|`Codex`\|`Both`\|`None`) | prompted on a TTY; error with `-NonInteractive` | Selects which coding agent config(s) to write: Codex TOML, Claude MCP JSON, both, or none. |
| `-Scope` | No | `User` (`Local`\|`Project`\|`User`) | `User` | Chooses whether agent configs are written to the user home directory or to `-ProjectDir`. |
| `-ProjectDir` | No | `C:\Users\me\projects\atlassian-mcp` | current working directory | Project directory used to resolve agent config paths when `-Scope` is `Local`/`Project`. |
| `-EnableJira` | No (switch; at least one of `-EnableJira`/`-EnableBitbucket` is required) | — | disabled | Enables the Jira module and writes `JIRA_BASE_URL`/`JIRA_CA_FILE` into the wrapper. |
| `-JiraBaseUrl` | Yes, if `-EnableJira` | `https://jira.internal.example.com/jira` | *(empty)* | Base URL of the Jira instance. Must be a plain http(s) URL with no query, fragment, or embedded credentials. |
| `-JiraCaFile` | No | `C:\certs\jira-internal-ca.pem` | *(empty)* | Path to a custom CA bundle for validating the Jira server's TLS certificate. |
| `-EnableBitbucket` | No (switch; at least one of `-EnableJira`/`-EnableBitbucket` is required) | — | disabled | Enables the Bitbucket module and writes its base URL/project key/token indirection into the wrapper. |
| `-BitbucketBaseUrl` | Yes, if `-EnableBitbucket` | `https://bitbucket.internal.example.com` | *(empty)* | Base URL of the Bitbucket instance. Must be a plain http(s) URL with no query, fragment, or embedded credentials. |
| `-BitbucketProjectKey` | Yes, if `-EnableBitbucket` | `ABC` | *(empty)* | Bitbucket project key that scopes the repository/pull-request tools. |
| `-BitbucketUserSlug` | No | `jane.doe` | *(empty)* | Bitbucket user slug used where tools need to identify the acting user. |
| `-BitbucketTokenEnv` | No | `BITBUCKET_BEARER_TOKEN` | `BITBUCKET_BEARER_TOKEN` | Name of the environment variable the wrapper reads the Bitbucket bearer token from at runtime; the token value itself is never written to config. |
| `-BitbucketCaFile` | No | `C:\certs\bitbucket-internal-ca.pem` | *(empty)* | Path to a custom CA bundle for validating the Bitbucket server's TLS certificate. |
| `-AtlassianTlsVerify` | No | `false` (`true`\|`false`) | `false` | Controls whether the wrapper enables TLS certificate verification for Jira/Bitbucket requests. |
| `-SkipTests` | No (switch) | — | disabled (`go test ./...` runs before build) | Skips running the repository test suite before building the binary from source. |
| `-DryRun` | No (switch) | — | disabled | Validates all arguments and exits without cloning, building, installing, or writing any config. |
| `-Replace` | No (switch) | — | disabled (refuses to overwrite an unmanaged Claude config) | Allows overwriting an existing Claude MCP config file that wasn't previously managed by this installer. |
| `-NonInteractive` | No (switch) | — | disabled (prompts for `-Agents` if omitted) | Disables the interactive `-Agents` prompt; missing required values become hard errors. |
