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

`<ref>` is a specific release tag, a branch, or a full commit SHA -- `raw.githubusercontent.com` has no built-in "latest". The bootstrap examples below resolve a literal `latest` to the newest published release tag via the GitHub Releases API before building this URL; pin `<ref>` to a specific tag or SHA instead for a reproducible, audited production install. Installer examples use the same repository as the default source:

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

`jira_authenticate` accepts `username`/`password` as tool input, or falls back to the `JIRA_USERNAME`/`JIRA_PASSWORD` environment variables when either field is omitted ([ADR-0004](docs/decisions/0004-jira-credential-env-fallback.md)). When both variables are already set at process startup, the server also authenticates automatically in the background right after startup, so no `jira_authenticate` call is needed at all in that case ([ADR-0005](docs/decisions/0005-jira-auto-authenticate-on-startup.md)). See [docs/security.md](docs/security.md) for the tradeoffs of each source.

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

When `--agents claude`/`both` is combined with the default `--scope user` (or `--scope local`), the installer registers the server by shelling out to `claude mcp add` instead of writing a config file, so the `claude` CLI must already be installed and on `PATH`. Only `--scope project` writes a `.mcp.json` file directly.

The installer generates a runtime wrapper (`atlassian-mcp-run`) that sets module config and resolves the Bitbucket token/Jira password from their indirection variable names at its own runtime, then execs the real binary. Claude Code and manual terminal runs use this wrapper, and it works regardless of whatever ambient environment the launching process has, since the wrapper resolves everything itself. **Codex is the one exception**: Codex's MCP launcher does not pass its own ambient environment through to spawned stdio servers, so it would never see the indirection variable the wrapper looks for. The installer therefore registers Codex directly against the built binary (not the wrapper) and writes an explicit `[mcp_servers.atlassian.env]` table into `config.toml` with the *resolved* values -- this is the one place this installer puts secret values (the Bitbucket token and, if `--jira-username` is set, the Jira password) directly into an agent config file, and only for Codex.

### Set required environment variables (Bash, user scope)

The installer never accepts secrets as arguments directly on the command line; it reads them through env var name *indirection* only while resolving installer configuration:

- The Bitbucket bearer token, from the variable named by `--bitbucket-token-env` (default `BITBUCKET_BEARER_TOKEN`).
- The Jira password, from the variable named by `--jira-password-env` (default `JIRA_PASSWORD`, only resolved when `--jira-username` is set).

Set them once for your user account so they survive new shells/terminals, then export them in the current shell before running the installer:

```bash
# Persist for the user account: append to the shell profile loaded on login
echo "export BITBUCKET_BEARER_TOKEN='<your-bitbucket-token>'" >> "$HOME/.bashrc"   # bash
echo "export JIRA_PASSWORD='<your-jira-password>'" >> "$HOME/.bashrc"
# echo "export BITBUCKET_BEARER_TOKEN='<your-bitbucket-token>'" >> "$HOME/.profile"  # sh/other login shells
# echo "export JIRA_PASSWORD='<your-jira-password>'" >> "$HOME/.profile"

# Load them into the current shell (or open a new terminal)
source "$HOME/.bashrc"

# Confirm they are visible to the process the installer will run in
echo "$BITBUCKET_BEARER_TOKEN"
echo "$JIRA_PASSWORD"
```

Skip the Jira line entirely if you do not want `jira_authenticate` to have a credential fallback (see [ADR-0004](docs/decisions/0004-jira-credential-env-fallback.md)) or automatic startup authentication (see [ADR-0005](docs/decisions/0005-jira-auto-authenticate-on-startup.md)) -- omitting `--jira-username` from the install command below leaves that feature off.

Fetch it from the canonical GitHub raw URL. `raw.githubusercontent.com` has no built-in notion of "latest" -- resolve it to the newest published release tag via the GitHub Releases API first, then use that resolved tag for both the raw fetch and `--source-ref`. Pin `INSTALLER_REF` to a specific release tag or full commit SHA instead of `latest` if you need a reproducible, audited install:

```bash
INSTALLER_REF='latest'
if [[ "$INSTALLER_REF" == "latest" ]]; then
  INSTALLER_REF="$(curl -fsSL https://api.github.com/repos/chiendao1808/atlassian-mcp/releases/latest |
    grep -m1 '"tag_name"' | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')"
  [[ -n "$INSTALLER_REF" ]] || { echo "could not resolve latest release tag" >&2; exit 1; }
fi
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
    --jira-username svc-atlassian-mcp \
    --jira-password-env JIRA_PASSWORD \
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
| `--source-ref` | No | `latest` | `main` | Git ref (branch, tag, or commit) checked out after cloning. |
| `--source-clone-depth` | No | `1` | `1` | Depth passed to `git clone`/`git fetch` for the source checkout. |
| `--keep-source` | No (flag) | — | disabled (source is cleaned up after a successful install) | Keeps the temporary cloned checkout on disk after install, for debugging. |
| `--install-dir` | No | `/home/me/.local/bin` | `$HOME/.local/bin` | Directory the built/provided binary and the generated wrapper script are installed into. |
| `--agents` | Yes, unless run interactively | `both` (`claude`\|`codex`\|`both`\|`none`) | prompted on a TTY; error with `--non-interactive` | Selects which coding agent to register: Codex TOML, Claude, both, or none. For `--scope local`/`user`, Claude is registered by invoking the `claude` CLI (`claude mcp add`) instead of writing a config file; the `claude` CLI must be on `PATH`. |
| `--scope` | No | `user` (`local`\|`project`\|`user`) | `user` | Chooses where agent configs are registered. Codex always writes a TOML file (user home or `--project-dir`). For Claude: `local`/`user` register via the `claude` CLI; `project` writes `--project-dir/.mcp.json`. |
| `--project-dir` | No | `/home/me/projects/atlassian-mcp` | current working directory | Project directory used to resolve agent config paths when `--scope` is `local`/`project`. |
| `--enable-jira` | No (flag; at least one of `--enable-jira`/`--enable-bitbucket` is required) | — | disabled | Enables the Jira module and writes `JIRA_BASE_URL`/`JIRA_CA_FILE` into the wrapper (and, for Codex, into `config.toml`). |
| `--jira-base-url` | Yes, if `--enable-jira` | `https://jira.internal.example.com/jira` | *(empty)* | Base URL of the Jira instance. Must be a plain http(s) URL with no query, fragment, or embedded credentials. |
| `--jira-ca-file` | No | `/etc/ssl/certs/jira-internal-ca.pem` | *(empty)* | Path to a custom CA bundle for validating the Jira server's TLS certificate. |
| `--jira-username` | No (requires `--enable-jira`) | `svc-atlassian-mcp` | *(empty)* | Jira username resolved into `JIRA_USERNAME`, letting `jira_authenticate` fall back to it ([ADR-0004](docs/decisions/0004-jira-credential-env-fallback.md)) and enabling automatic startup authentication when the password is also present ([ADR-0005](docs/decisions/0005-jira-auto-authenticate-on-startup.md)). Omit entirely to leave both features off. |
| `--jira-password-env` | No | `JIRA_PASSWORD` | `JIRA_PASSWORD` | Name of the environment variable the wrapper reads the Jira password from at runtime when `--jira-username` is set; for Codex, the resolved value is written into `config.toml` instead (see the note above). |
| `--enable-bitbucket` | No (flag; at least one of `--enable-jira`/`--enable-bitbucket` is required) | — | disabled | Enables the Bitbucket module and writes its base URL/project key/token indirection into the wrapper (and, for Codex, the resolved values into `config.toml`). |
| `--bitbucket-base-url` | Yes, if `--enable-bitbucket` | `https://bitbucket.internal.example.com` | *(empty)* | Base URL of the Bitbucket instance. Must be a plain http(s) URL with no query, fragment, or embedded credentials. |
| `--bitbucket-project-key` | Yes, if `--enable-bitbucket` | `ABC` | *(empty)* | Bitbucket project key that scopes the repository/pull-request tools. |
| `--bitbucket-user-slug` | No | `jane.doe` | *(empty)* | Bitbucket user slug used where tools need to identify the acting user. |
| `--bitbucket-token-env` | No | `BITBUCKET_BEARER_TOKEN` | `BITBUCKET_BEARER_TOKEN` | Name of the environment variable the wrapper reads the Bitbucket bearer token from at runtime; for Codex, the resolved value is also written into `config.toml` (never into Claude's config either way). |
| `--bitbucket-ca-file` | No | `/etc/ssl/certs/bitbucket-internal-ca.pem` | *(empty)* | Path to a custom CA bundle for validating the Bitbucket server's TLS certificate. |
| `--atlassian-tls-verify` | No | `false` (`true`\|`false`) | `false` | Controls whether the wrapper (and, for Codex, `config.toml`) enables TLS certificate verification for Jira/Bitbucket requests. |
| `--skip-tests` | No (flag) | — | disabled (`go test ./...` runs before build) | Skips running the repository test suite before building the binary from source. |
| `--dry-run` | No (flag) | — | disabled | Validates all arguments and exits without cloning, building, installing, or writing any config. |
| `--replace` | No (flag) | — | disabled (refuses to overwrite an unmanaged Claude config) | Only applies to `--scope project`: allows overwriting an existing `.mcp.json` that wasn't previously managed by this installer. Has no effect on `--scope local`/`user`, which register through the `claude` CLI and are idempotent by design. |
| `--non-interactive` | No (flag) | — | disabled (prompts for `--agents` if omitted) | Disables the interactive `--agents` prompt and any other terminal prompts; missing required values become hard errors. |
| `-h`, `--help` | No (flag) | — | disabled | Prints usage text and exits. |

## PowerShell installer bootstrap

The PowerShell installer is committed at the stable repository-root path:

```text
scripts/install-from-remote.ps1
```

When `-Agents Claude`/`Both` is combined with the default `-Scope User` (or `-Scope Local`), the installer registers the server by shelling out to `claude mcp add` instead of writing a config file, so the `claude` CLI must already be installed and on `PATH`. Only `-Scope Project` writes a `.mcp.json` file directly.

There is no wrapper script. The installer registers `atlassian-mcp.exe` directly with Claude/Codex and resolves all non-secret module config, plus the Bitbucket token and (if `-JiraUsername` is set) the Jira credential, into one set of environment variables (`ATLASSIAN_TLS_VERIFY`, `JIRA_BASE_URL`, `JIRA_CA_FILE`, `JIRA_USERNAME`, `JIRA_PASSWORD`, `BITBUCKET_BASE_URL`, `BITBUCKET_PROJECT_KEY`, `BITBUCKET_USER_SLUG`, `BITBUCKET_CA_FILE`, `BITBUCKET_BEARER_TOKEN`). Those values reach the binary two different ways depending on the client, because Claude Code and Codex disagree on whether a spawned MCP server inherits the launching process's environment:

- **Claude Code** spawns the binary inheriting the ambient process environment, so the installer persists this set as Windows **User** environment variables (`[Environment]::SetEnvironmentVariable(..., 'User')`) and that is enough.
- **Codex does not** pass its own ambient environment through to spawned stdio MCP servers (confirmed from Codex's own logs: the binary started with every module disabled until this was fixed). The installer therefore also writes the same values directly into an explicit `[mcp_servers.atlassian.env]` table in `config.toml` — this is the one place this installer deliberately puts secret values (the Bitbucket token and, if configured, the Jira password) into an agent config file, and only for Codex.

**Restart Claude Code, Codex, and any open terminal** after installing/reinstalling — Windows only hands newly persisted User environment variables to processes started *after* the change, not to ones already running. (Codex additionally always needs a restart regardless, since its copy lives in `config.toml`, re-read only at Codex startup.)

### Set required environment variables (PowerShell, user scope)

The installer never accepts secrets as arguments directly on the command line. It reads two secrets through env var name *indirection* only while resolving installer configuration, then writes the resolved values to fixed-name User environment variables (and, for Codex, into `config.toml`) so the binary can read them directly:

- The Bitbucket bearer token, from the variable named by `-BitbucketTokenEnv` (default `BITBUCKET_BEARER_TOKEN`) -> written to `BITBUCKET_BEARER_TOKEN`.
- The Jira password, from the variable named by `-JiraPasswordEnv` (default `JIRA_PASSWORD`, only resolved when `-JiraUsername` is set) -> written to `JIRA_PASSWORD`.

Persist the indirection variables in the Windows per-user environment store so they survive new PowerShell/terminal sessions, then reload the current session before running the installer:

```powershell
# Persist for the user account (Windows User environment store)
[Environment]::SetEnvironmentVariable('BITBUCKET_BEARER_TOKEN', '<your-bitbucket-token>', 'User')
[Environment]::SetEnvironmentVariable('JIRA_PASSWORD', '<your-jira-password>', 'User')

# Load them into the current session (or open a new terminal)
$env:BITBUCKET_BEARER_TOKEN = [Environment]::GetEnvironmentVariable('BITBUCKET_BEARER_TOKEN', 'User')
$env:JIRA_PASSWORD = [Environment]::GetEnvironmentVariable('JIRA_PASSWORD', 'User')

# Confirm they are visible to the process the installer will run in
$env:BITBUCKET_BEARER_TOKEN
$env:JIRA_PASSWORD
```

Skip the Jira line entirely if you do not want `jira_authenticate` to have a credential fallback (see ADR-0004) or automatic startup authentication (see ADR-0005) — omitting `-JiraUsername` from the install command below leaves that feature off.

Fetch it from the canonical GitHub raw URL to a temporary file, then run that file explicitly. `raw.githubusercontent.com` has no built-in notion of "latest" -- resolve it to the newest published release tag via the GitHub Releases API first, then use that resolved tag for both the raw fetch and `-SourceRef`. Pin `$INSTALLER_REF` to a specific release tag or full commit SHA instead of `latest` if you need a reproducible, audited install:

```powershell
$INSTALLER_REF = 'latest'
if ($INSTALLER_REF -eq 'latest') {
    $INSTALLER_REF = (Invoke-RestMethod -Uri 'https://api.github.com/repos/chiendao1808/atlassian-mcp/releases/latest').tag_name
    if ([string]::IsNullOrEmpty($INSTALLER_REF)) {
        throw 'could not resolve latest release tag'
    }
}
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
  -JiraUsername 'svc-atlassian-mcp' `
  -JiraPasswordEnv 'JIRA_PASSWORD' `
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
| `-SourceRef` | No | `latest` | `main` | Git ref (branch, tag, or commit) checked out after cloning. |
| `-SourceCloneDepth` | No | `1` | `1` | Depth passed to `git clone`/`git fetch` for the source checkout. |
| `-KeepSource` | No (switch) | — | disabled (source is cleaned up after a successful install) | Keeps the temporary cloned checkout on disk after install, for debugging. |
| `-InstallDir` | No | `C:\Users\me\.local\bin` | `Join-Path $HOME '.local\bin'` | Directory the built/provided binary (`atlassian-mcp.exe`) is installed into. There is no wrapper script; Claude/Codex are registered to run this exe directly. |
| `-Agents` | Yes, unless run interactively | `Both` (`Claude`\|`Codex`\|`Both`\|`None`) | prompted on a TTY; error with `-NonInteractive` | Selects which coding agent to register: Codex TOML, Claude, both, or none. For `-Scope Local`/`User`, Claude is registered by invoking the `claude` CLI (`claude mcp add`) instead of writing a config file; the `claude` CLI must be on `PATH`. |
| `-Scope` | No | `User` (`Local`\|`Project`\|`User`) | `User` | Chooses where agent configs are registered. Codex always writes a TOML file (user home or `-ProjectDir`). For Claude: `Local`/`User` register via the `claude` CLI; `Project` writes `-ProjectDir\.mcp.json`. |
| `-ProjectDir` | No | `C:\Users\me\projects\atlassian-mcp` | current working directory | Project directory used to resolve agent config paths when `-Scope` is `Local`/`Project`. |
| `-EnableJira` | No (switch; at least one of `-EnableJira`/`-EnableBitbucket` is required) | — | disabled | Enables the Jira module and persists `JIRA_BASE_URL`/`JIRA_CA_FILE` as User environment variables. |
| `-JiraBaseUrl` | Yes, if `-EnableJira` | `https://jira.internal.example.com/jira` | *(empty)* | Base URL of the Jira instance. Must be a plain http(s) URL with no query, fragment, or embedded credentials. |
| `-JiraCaFile` | No | `C:\certs\jira-internal-ca.pem` | *(empty)* | Path to a custom CA bundle for validating the Jira server's TLS certificate. |
| `-JiraUsername` | No (requires `-EnableJira`) | `svc-atlassian-mcp` | *(empty)* | Jira username resolved into `JIRA_USERNAME`, letting `jira_authenticate` fall back to it (ADR-0004) and enabling automatic startup authentication when the password is also present (ADR-0005). Omit entirely to leave both features off. |
| `-JiraPasswordEnv` | No | `JIRA_PASSWORD` | `JIRA_PASSWORD` | Name of the environment variable the installer reads the Jira password from at install time when `-JiraUsername` is set; the resolved value is persisted to the `JIRA_PASSWORD` User environment variable and, for Codex, written into `config.toml` (see the note above `.mcp.json`/`config.toml` handling). |
| `-EnableBitbucket` | No (switch; at least one of `-EnableJira`/`-EnableBitbucket` is required) | — | disabled | Enables the Bitbucket module and persists its base URL/project key/resolved token as User environment variables. |
| `-BitbucketBaseUrl` | Yes, if `-EnableBitbucket` | `https://bitbucket.internal.example.com` | *(empty)* | Base URL of the Bitbucket instance. Must be a plain http(s) URL with no query, fragment, or embedded credentials. |
| `-BitbucketProjectKey` | Yes, if `-EnableBitbucket` | `ABC` | *(empty)* | Bitbucket project key that scopes the repository/pull-request tools. |
| `-BitbucketUserSlug` | No | `jane.doe` | *(empty)* | Bitbucket user slug used where tools need to identify the acting user. |
| `-BitbucketTokenEnv` | No | `BITBUCKET_BEARER_TOKEN` | `BITBUCKET_BEARER_TOKEN` | Name of the environment variable the installer reads the Bitbucket bearer token from at install time; the resolved value is persisted to the `BITBUCKET_BEARER_TOKEN` User environment variable and, for Codex, also written into `config.toml` (never into Claude's config either way). |
| `-BitbucketCaFile` | No | `C:\certs\bitbucket-internal-ca.pem` | *(empty)* | Path to a custom CA bundle for validating the Bitbucket server's TLS certificate. |
| `-AtlassianTlsVerify` | No | `false` (`true`\|`false`) | `false` | Persisted as the `ATLASSIAN_TLS_VERIFY` User environment variable, controlling TLS certificate verification for Jira/Bitbucket requests. |
| `-SkipTests` | No (switch) | — | disabled (`go test ./...` runs before build) | Skips running the repository test suite before building the binary from source. |
| `-DryRun` | No (switch) | — | disabled | Validates all arguments and exits without cloning, building, installing, or writing any config. |
| `-Replace` | No (switch) | — | disabled (refuses to overwrite an unmanaged Claude config) | Only applies to `-Scope Project`: allows overwriting an existing `.mcp.json` that wasn't previously managed by this installer. Has no effect on `-Scope Local`/`User`, which register through the `claude` CLI and are idempotent by design. |
| `-NonInteractive` | No (switch) | — | disabled (prompts for `-Agents` if omitted) | Disables the interactive `-Agents` prompt; missing required values become hard errors. |

### Testing a local build (no GitHub fetch)

To try a change from a local checkout instead of the raw GitHub URL, use `-Binary` with a
binary built from this repo, and run `scripts/install-from-remote.ps1` straight from disk:

```powershell
$RepoRoot = '.\atlassian-mcp'
$LocalBinary = Join-Path $env:TEMP 'atlassian-mcp.exe'
Push-Location $RepoRoot
go build -o $LocalBinary ./cmd/atlassian-mcp
Pop-Location

[Environment]::SetEnvironmentVariable('BITBUCKET_BEARER_TOKEN', '<your-bitbucket-token>', 'User')
[Environment]::SetEnvironmentVariable('JIRA_PASSWORD', '<your-jira-password>', 'User')

powershell.exe -NoProfile -ExecutionPolicy Bypass -File "$RepoRoot\scripts\install-from-remote.ps1" `
  -Binary $LocalBinary `
  -InstallDir (Join-Path $HOME '.local\bin') `
  -Agents Both `
  -Scope User `
  -EnableJira `
  -JiraBaseUrl 'https://jira.internal.example.com/jira' `
  -JiraUsername 'svc-atlassian-mcp' `
  -JiraPasswordEnv 'JIRA_PASSWORD' `
  -EnableBitbucket `
  -BitbucketBaseUrl 'https://bitbucket.internal.example.com' `
  -BitbucketProjectKey 'ABC' `
  -BitbucketTokenEnv 'BITBUCKET_BEARER_TOKEN' `
  -AtlassianTlsVerify false `
  -NonInteractive

# Reload the persisted values into THIS PowerShell session, so `atlassian-mcp.exe` can be
# tested directly here without opening a new terminal. This does not reach Codex/Claude
# Code -- they are separate processes and still need a full restart to see the new config.
foreach ($name in @('ATLASSIAN_TLS_VERIFY', 'JIRA_BASE_URL', 'JIRA_USERNAME', 'JIRA_PASSWORD', 'BITBUCKET_BASE_URL', 'BITBUCKET_PROJECT_KEY', 'BITBUCKET_USER_SLUG', 'BITBUCKET_BEARER_TOKEN')) {
    Set-Item -Path "Env:$name" -Value ([Environment]::GetEnvironmentVariable($name, 'User'))
}
```
