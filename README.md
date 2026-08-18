# Atlassian MCP

`atlassian-mcp` is a Go MCP stdio server for Jira, Confluence, and Bitbucket Server/Data Center APIs. The supported installer path is binary-first: installers download the published GitHub release executable, verify it with the release SHA-256 manifest, then register the binary with Claude Code, Codex, Cursor, and Kiro.

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

## Agent Configuration Paths

The installer writes each agent's config to its native location. Cursor and Kiro
store their MCP servers in a JSON file that is merged rather than replaced, so
existing unrelated MCP servers and top-level JSON keys survive every install or
reinstall.

| Installer scope | Claude Code | Codex | Cursor | Kiro |
| --- | --- | --- | --- | --- |
| `user` | registered via the `claude` CLI | `~/.codex/config.toml` | `~/.cursor/mcp.json` | `~/.kiro/settings/mcp.json` |
| `project` | `<project>/.mcp.json` | `<project>/.codex/config.toml` | `<project>/.cursor/mcp.json` | `<project>/.kiro/settings/mcp.json` |
| `local` | registered via the `claude` CLI | `<project>/.codex/config.toml` | `<project>/.cursor/mcp.json` (workspace alias) | `<project>/.kiro/settings/mcp.json` (workspace alias) |

Cursor and Kiro keep the same project/workspace file for both `local` and
`project` scope. References: [Cursor MCP](https://cursor.com/docs/mcp) and
[Kiro MCP configuration](https://kiro.dev/docs/mcp/configuration/).

## Tools

The current registered Jira, Confluence, and Bitbucket tools are listed in [docs/tools/catalog.md](docs/tools/catalog.md). Detailed module notes remain in [docs/tools/jira.md](docs/tools/jira.md), [docs/tools/confluence.md](docs/tools/confluence.md), and [docs/tools/bitbucket.md](docs/tools/bitbucket.md).

## Release Artifacts

GitHub Releases publish amd64 assets only:

- `atlassian-mcp_<version>_linux_amd64`
- `atlassian-mcp_<version>_windows_amd64.exe`
- `atlassian-mcp_<version>_checksums.txt`
- SPDX JSON SBOM files for the raw Linux and Windows binaries

The installer supports Windows amd64 and Linux amd64 raw executables. Debian package generation and Debian SBOM assets are temporarily disabled in the current published inventory, so Linux installs should use the raw binary for now. ARM, macOS, Docker, RPM, package repositories, auto-update, signing, and provenance attestations are outside the current support boundary.

SHA-256 verification detects corrupted or changed files after selecting a release asset. It does not prove publisher identity because the checksum manifest is downloaded from the same GitHub release.

## Credentials And Module Config

Installers never accept secret values directly as command-line arguments. They read secrets through env-var name indirection at install time:

- `BITBUCKET_BEARER_TOKEN`, or the variable named by `--bitbucket-token-env` / `-BitbucketTokenEnv`.
- `JIRA_PASSWORD`, or the variable named by `--jira-password-env` / `-JiraPasswordEnv` when a Jira username is configured.
- `CONFLUENCE_PASSWORD`, or the variable named by `--confluence-password-env` / `-ConfluencePasswordEnv` when a Confluence username is configured.

At least one module must be enabled with `--enable-jira`, `--enable-confluence`, or `--enable-bitbucket` (PowerShell: `-EnableJira`, `-EnableConfluence`, `-EnableBitbucket`). Jira and Confluence support explicit authenticate tools and environment fallback; Bitbucket reads its bearer token from runtime config.

Skip the Jira or Confluence password line entirely if you do not want the matching `*_authenticate` tool to have a credential fallback or automatic startup authentication. Omit `--jira-username`/`-JiraUsername` or `--confluence-username`/`-ConfluenceUsername` to leave that feature off for that module.

Do not put credentials in the raw installer URL or any `--*-base-url`/`-*BaseUrl` argument. Use the env-var-name-indirected variables, or a secrets manager, for private access and secrets. The steps to set those variables are shown per installer type just below, right before each install command.

## Bash Install

### Set Required Environment Variables (Bash)

Set the indirection variables once for your user account so they survive new shells/terminals, then export them into the current shell before running the installer:

```bash
# Persist for the user account: append to the shell profile loaded on login
echo "export BITBUCKET_BEARER_TOKEN='<your-bitbucket-token>'" >> "$HOME/.bashrc"   # bash
echo "export JIRA_PASSWORD='<your-jira-password>'" >> "$HOME/.bashrc"
echo "export CONFLUENCE_PASSWORD='<your-confluence-password>'" >> "$HOME/.bashrc"
# echo "export BITBUCKET_BEARER_TOKEN='<your-bitbucket-token>'" >> "$HOME/.profile"  # sh/other login shells
# echo "export JIRA_PASSWORD='<your-jira-password>'" >> "$HOME/.profile"
# echo "export CONFLUENCE_PASSWORD='<your-confluence-password>'" >> "$HOME/.profile"

# Load them into the current shell (or open a new terminal)
source "$HOME/.bashrc"

# Confirm they are visible to the process the installer will run in
for name in BITBUCKET_BEARER_TOKEN JIRA_PASSWORD CONFLUENCE_PASSWORD; do
  if [ -n "${!name:-}" ]; then
    printf '%s is set\n' "$name"
  else
    printf '%s is missing\n' "$name"
  fi
done
```

Use custom variable names instead of the defaults above by passing `--bitbucket-token-env`, `--jira-password-env`, and/or `--confluence-password-env` with the matching name to `install-from-remote.sh`.

Install the latest stable release. This is the standard, full-argument form — every
supported flag is listed; `--release-tag`/`--binary` (mutually exclusive alternatives to
downloading the latest release) are left out here and shown separately below:

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

Pin an exact release binary instead of the latest one — add `--release-tag` (a minimal
example; combine with the full module/agent args from above as needed):

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

The Bash installer writes the binary and an `atlassian-mcp-run` wrapper. Claude/manual runs use the wrapper so token/password indirection is resolved at runtime. Codex is registered directly to the binary with explicit env values in `config.toml` because Codex does not pass its ambient environment to spawned MCP stdio servers. Cursor and Kiro are registered through their JSON config files; those never contain resolved credentials — the wrapper resolves them at runtime. Configure Cursor and Kiro (`--agents cursor,kiro`) with the module flags from above; the same flags apply:

```bash
curl -fsSL "$INSTALLER_URL" |
  bash -s -- \
    --install-dir "$HOME/.local/bin" \
    --agents cursor,kiro \
    --scope project \
    --project-dir "$(pwd)" \
    --enable-jira \
    --jira-base-url https://jira.internal.example.com/jira \
    --non-interactive
```

`--agents all` registers every supported agent (`claude,codex,cursor,kiro`) in one run; `--agents both` remains Claude + Codex only. The final success line reports the resolved release tag (e.g. `installed atlassian-mcp v1.0.4 to ...`), or `(local binary)` when `--binary` was used instead of a GitHub download.

### Bash Installer Arguments

| Argument | Required | Sample value | Default | Description |
| --- | --- | --- | --- | --- |
| `--release-tag` | No | `v1.0.4` | latest stable release (resolved via the GitHub releases API) | Exact release tag to download and verify. Must look like `v1.2.3`. Ignored when `--binary` is set. |
| `--binary` | No (alternative to `--release-tag`) | `/path/to/atlassian-mcp` | *(empty)* | Path to a prebuilt/offline binary to install directly, skipping the GitHub download and checksum verification. |
| `--install-dir` | No | `/home/me/.local/bin` | `$HOME/.local/bin` | Directory the installed binary and the generated `atlassian-mcp-run` wrapper are written into. |
| `--agents` | Yes, unless run interactively | `both` (`claude`\|`codex`\|`cursor`\|`kiro`, comma-separated) or `both`\|`all`\|`none` | prompted on a TTY; error with `--non-interactive` | Selects which coding agent to register. `both` means Claude + Codex only; `all` means Claude + Codex + Cursor + Kiro; explicit comma-separated lists deduplicate repeated names. For `--scope local`/`user`, Claude is registered via the `claude` CLI (`claude mcp add`); the `claude` CLI must be on `PATH`. Selecting `cursor` or `kiro` requires `jq` so the installer can merge the JSON config safely. |
| `--scope` | No | `user` (`local`\|`project`\|`user`) | `user` | Chooses where agent configs are registered. Codex always writes `config.toml` (user home or `--project-dir`). For Claude: `local`/`user` register via the `claude` CLI; `project` writes `--project-dir/.mcp.json`. Cursor and Kiro write their JSON config in their config directory for every scope; `local` resolves to the project/workspace file (see the scope table above). |
| `--project-dir` | No | `/home/me/projects/atlassian-mcp` | current working directory | Project directory used to resolve agent config paths when `--scope` is `local`/`project`. |
| `--enable-jira` | No (flag; at least one of `--enable-jira`/`--enable-confluence`/`--enable-bitbucket` is required) | — | disabled | Enables the Jira module and writes `JIRA_BASE_URL`/`JIRA_CA_FILE` into the wrapper (and, for Codex, into `config.toml`). |
| `--jira-base-url` | Yes, if `--enable-jira` | `https://jira.internal.example.com/jira` | *(empty)* | Base URL of the Jira instance. Must be a plain http(s) URL with no query, fragment, or embedded credentials. |
| `--jira-ca-file` | No | `/etc/ssl/certs/jira-internal-ca.pem` | *(empty)* | Path to a custom CA bundle for validating the Jira server's TLS certificate. |
| `--jira-username` | No (requires `--enable-jira`) | `svc-atlassian-mcp` | *(empty)* | Jira username resolved into `JIRA_USERNAME`, letting `jira_authenticate` fall back to it and enabling automatic startup authentication when the password is also present. The variable named by `--jira-password-env` must already hold a value for `--non-interactive` installs or whenever `--agents` includes `codex`. Omit entirely to leave both features off. |
| `--jira-password-env` | No | `JIRA_PASSWORD` | `JIRA_PASSWORD` | Name of the environment variable the wrapper reads the Jira password from at runtime when `--jira-username` is set; for Codex, the resolved value is written into `config.toml` instead. |
| `--enable-confluence` | No (flag; at least one of `--enable-jira`/`--enable-confluence`/`--enable-bitbucket` is required) | — | disabled | Enables the Confluence module and writes `CONFLUENCE_BASE_URL`/`CONFLUENCE_CA_FILE` into the wrapper (and, for Codex, into `config.toml`). |
| `--confluence-base-url` | Yes, if `--enable-confluence` | `https://confluence.internal.example.com/confluence` | *(empty)* | Base URL of the Confluence instance. Must be a plain http(s) URL with no query, fragment, or embedded credentials. |
| `--confluence-ca-file` | No | `/etc/ssl/certs/confluence-internal-ca.pem` | *(empty)* | Path to a custom CA bundle for validating the Confluence server's TLS certificate. |
| `--confluence-username` | No (requires `--enable-confluence`) | `svc-atlassian-mcp` | *(empty)* | Confluence username resolved into `CONFLUENCE_USERNAME`, enabling `confluence_authenticate` fallback and automatic startup authentication when the password is also present. Same `--non-interactive`/`codex` password-presence rule as Jira applies. Omit entirely to leave both features off. |
| `--confluence-password-env` | No | `CONFLUENCE_PASSWORD` | `CONFLUENCE_PASSWORD` | Name of the environment variable the wrapper reads the Confluence password from at runtime when `--confluence-username` is set; for Codex, the resolved value is written into `config.toml` instead. |
| `--enable-bitbucket` | No (flag; at least one of `--enable-jira`/`--enable-confluence`/`--enable-bitbucket` is required) | — | disabled | Enables the Bitbucket module and writes its base URL/project key/token indirection into the wrapper (and, for Codex, the resolved values into `config.toml`). |
| `--bitbucket-base-url` | Yes, if `--enable-bitbucket` | `https://bitbucket.internal.example.com` | *(empty)* | Base URL of the Bitbucket instance. Must be a plain http(s) URL with no query, fragment, or embedded credentials. |
| `--bitbucket-project-key` | Yes, if `--enable-bitbucket` | `ABC` | *(empty)* | Bitbucket project key that scopes the repository/pull-request tools. |
| `--bitbucket-user-slug` | No | `jane.doe` | *(empty)* | Bitbucket user slug used where tools need to identify the acting user. |
| `--bitbucket-token-env` | No | `BITBUCKET_BEARER_TOKEN` | `BITBUCKET_BEARER_TOKEN` | Name of the environment variable the wrapper reads the Bitbucket bearer token from at runtime; for Codex, the resolved value is also written into `config.toml`. The named variable must already hold a value for `--non-interactive` installs or whenever `--agents` includes `codex`. |
| `--bitbucket-ca-file` | No | `/etc/ssl/certs/bitbucket-internal-ca.pem` | *(empty)* | Path to a custom CA bundle for validating the Bitbucket server's TLS certificate. |
| `--atlassian-tls-verify` | No | `false` (`true`\|`false`) | `false` | Controls whether the wrapper (and, for Codex, `config.toml`) enables TLS certificate verification for Jira/Confluence/Bitbucket requests. |
| `--dry-run` | No (flag) | — | disabled | Validates all arguments and exits without downloading, installing, or writing any config. |
| `--replace` | No (flag) | — | disabled (refuses to overwrite an unmanaged Claude config) | For `--scope project`, allows overwriting an existing `.mcp.json` that wasn't previously managed by this installer. For Cursor/Kiro JSON, replaces only the conflicting `mcpServers.atlassian` entry, preserving every unrelated root key and server entry. |
| `--non-interactive` | No (flag) | — | disabled (prompts for `--agents` if omitted) | Disables the interactive `--agents` prompt; missing required values become hard errors. |
| `-h`, `--help` | No (flag) | — | disabled | Prints usage text and exits. |

## PowerShell Install

### Set Required Environment Variables (PowerShell)

The installer resolves these indirection variables at install time and persists the resolved values as Windows **User** environment variables (and, for Codex, into `config.toml`), so set the indirection variables in the User store first and reload them into the current session:

```powershell
# Persist for the user account (Windows User environment store)
[Environment]::SetEnvironmentVariable('BITBUCKET_BEARER_TOKEN', '<your-bitbucket-token>', 'User')
[Environment]::SetEnvironmentVariable('JIRA_PASSWORD', '<your-jira-password>', 'User')
[Environment]::SetEnvironmentVariable('CONFLUENCE_PASSWORD', '<your-confluence-password>', 'User')

# Load them into the current session (or open a new terminal)
$env:BITBUCKET_BEARER_TOKEN = [Environment]::GetEnvironmentVariable('BITBUCKET_BEARER_TOKEN', 'User')
$env:JIRA_PASSWORD = [Environment]::GetEnvironmentVariable('JIRA_PASSWORD', 'User')
$env:CONFLUENCE_PASSWORD = [Environment]::GetEnvironmentVariable('CONFLUENCE_PASSWORD', 'User')

# Confirm they are visible to the process the installer will run in
foreach ($name in 'BITBUCKET_BEARER_TOKEN', 'JIRA_PASSWORD', 'CONFLUENCE_PASSWORD') {
    if ([string]::IsNullOrEmpty([Environment]::GetEnvironmentVariable($name))) {
        "$name is missing"
    } else {
        "$name is set"
    }
}
```

Use custom variable names instead of the defaults above by passing `-BitbucketTokenEnv`, `-JiraPasswordEnv`, and/or `-ConfluencePasswordEnv` with the matching name to `install-from-remote.ps1`.

Install the latest stable release. This is the standard, full-argument form — every
supported parameter is listed; `-ReleaseTag`/`-Binary` (mutually exclusive alternatives to
downloading the latest release) are left out here and shown separately below:

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

Pin an exact release binary instead of the latest one — add `-ReleaseTag` (a minimal
example; combine with the full module/agent args from above as needed):

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

Restart Claude Code, Codex, Cursor, Kiro, and open terminals after a Windows install so they pick up newly persisted User environment variables. Codex also needs restart because it reads `config.toml` at startup. Cursor and Kiro read their merged JSON config, which never contains resolved credentials — runtime config comes from the persisted User environment. Configure Cursor and Kiro (`-Agents 'Cursor,Kiro'`) with the module flags from above; the same flags apply; `-Agents All` registers all four agents. The final success line reports the resolved release tag (e.g. `installed atlassian-mcp v1.0.4 to ...`), or `(local binary)` when `-Binary` was used instead of a GitHub download.

### PowerShell Installer Arguments

There is no wrapper script on Windows: the installer resolves all module config, plus the Bitbucket token and optional Jira/Confluence credentials, directly into a fixed set of Windows **User** environment variables (and, for Codex, into `config.toml`) at install time.

| Argument | Required | Sample value | Default | Description |
| --- | --- | --- | --- | --- |
| `-ReleaseTag` | No | `v1.0.4` | latest stable release (resolved via the GitHub releases API) | Exact release tag to download and verify. Must look like `v1.2.3`. Ignored when `-Binary` is set. |
| `-Binary` | No (alternative to `-ReleaseTag`) | `C:\path\to\atlassian-mcp.exe` | *(empty)* | Path to a prebuilt/offline binary to install directly, skipping the GitHub download and checksum verification. |
| `-InstallDir` | No | `C:\Users\me\.local\bin` | `Join-Path $HOME '.local\bin'` | Directory the binary (`atlassian-mcp.exe`) is installed into. Claude/Codex/Cursor/Kiro are registered to run this exe directly. |
| `-Agents` | Yes, unless run interactively | `Both` (`Claude`\|`Codex`\|`Cursor`\|`Kiro`, comma-separated) or `Both`\|`All`\|`None` | prompted on a TTY; error with `-NonInteractive` | Selects which coding agent to register. `Both` means Claude + Codex only; `All` means Claude + Codex + Cursor + Kiro; explicit comma-separated lists deduplicate repeated names. For `-Scope Local`/`User`, Claude is registered via the `claude` CLI (`claude mcp add`); the `claude` CLI must be on `PATH`. |
| `-Scope` | No | `User` (`Local`\|`Project`\|`User`) | `User` | Chooses where agent configs are registered. Codex always writes a TOML file (user home or `-ProjectDir`). For Claude: `Local`/`User` register via the `claude` CLI; `Project` writes `-ProjectDir\.mcp.json`. Cursor and Kiro write their JSON config in their config directory for every scope; `Local` resolves to the project/workspace file (see the scope table above). |
| `-ProjectDir` | No | `C:\Users\me\projects\atlassian-mcp` | current working directory | Project directory used to resolve agent config paths when `-Scope` is `Local`/`Project`. |
| `-EnableJira` | No (switch; at least one of `-EnableJira`/`-EnableConfluence`/`-EnableBitbucket` is required) | — | disabled | Enables the Jira module and persists `JIRA_BASE_URL`/`JIRA_CA_FILE` as User environment variables. |
| `-JiraBaseUrl` | Yes, if `-EnableJira` | `https://jira.internal.example.com/jira` | *(empty)* | Base URL of the Jira instance. Must be a plain http(s) URL with no query, fragment, or embedded credentials. |
| `-JiraCaFile` | No | `C:\certs\jira-internal-ca.pem` | *(empty)* | Path to a custom CA bundle for validating the Jira server's TLS certificate. |
| `-JiraUsername` | No (requires `-EnableJira`) | `svc-atlassian-mcp` | *(empty)* | Jira username resolved into `JIRA_USERNAME`, letting `jira_authenticate` fall back to it and enabling automatic startup authentication when the password is also present. The variable named by `-JiraPasswordEnv` must already hold a value at install time whenever `-JiraUsername` is set. Omit entirely to leave both features off. |
| `-JiraPasswordEnv` | No | `JIRA_PASSWORD` | `JIRA_PASSWORD` | Name of the environment variable the installer reads the Jira password from at install time when `-JiraUsername` is set; the resolved value is persisted to the `JIRA_PASSWORD` User environment variable and, for Codex, written into `config.toml`. |
| `-EnableConfluence` | No (switch; at least one of `-EnableJira`/`-EnableConfluence`/`-EnableBitbucket` is required) | — | disabled | Enables the Confluence module and persists `CONFLUENCE_BASE_URL`/`CONFLUENCE_CA_FILE` as User environment variables. |
| `-ConfluenceBaseUrl` | Yes, if `-EnableConfluence` | `https://confluence.internal.example.com/confluence` | *(empty)* | Base URL of the Confluence instance. Must be a plain http(s) URL with no query, fragment, or embedded credentials. |
| `-ConfluenceCaFile` | No | `C:\certs\confluence-internal-ca.pem` | *(empty)* | Path to a custom CA bundle for validating the Confluence server's TLS certificate. |
| `-ConfluenceUsername` | No (requires `-EnableConfluence`) | `svc-atlassian-mcp` | *(empty)* | Confluence username resolved into `CONFLUENCE_USERNAME`, enabling `confluence_authenticate` fallback and automatic startup authentication when the password is also present. The variable named by `-ConfluencePasswordEnv` must already hold a value at install time whenever `-ConfluenceUsername` is set. Omit entirely to leave both features off. |
| `-ConfluencePasswordEnv` | No | `CONFLUENCE_PASSWORD` | `CONFLUENCE_PASSWORD` | Name of the environment variable the installer reads the Confluence password from at install time when `-ConfluenceUsername` is set; the resolved value is persisted to the `CONFLUENCE_PASSWORD` User environment variable and, for Codex, written into `config.toml`. |
| `-EnableBitbucket` | No (switch; at least one of `-EnableJira`/`-EnableConfluence`/`-EnableBitbucket` is required) | — | disabled | Enables the Bitbucket module and persists its base URL/project key/resolved token as User environment variables. |
| `-BitbucketBaseUrl` | Yes, if `-EnableBitbucket` | `https://bitbucket.internal.example.com` | *(empty)* | Base URL of the Bitbucket instance. Must be a plain http(s) URL with no query, fragment, or embedded credentials. |
| `-BitbucketProjectKey` | Yes, if `-EnableBitbucket` | `ABC` | *(empty)* | Bitbucket project key that scopes the repository/pull-request tools. |
| `-BitbucketUserSlug` | No | `jane.doe` | *(empty)* | Bitbucket user slug used where tools need to identify the acting user. |
| `-BitbucketTokenEnv` | No | `BITBUCKET_BEARER_TOKEN` | `BITBUCKET_BEARER_TOKEN` | Name of the environment variable the installer reads the Bitbucket bearer token from at install time; the resolved value is persisted to the `BITBUCKET_BEARER_TOKEN` User environment variable and, for Codex, also written into `config.toml`. Must already hold a value whenever `-EnableBitbucket` is set. |
| `-BitbucketCaFile` | No | `C:\certs\bitbucket-internal-ca.pem` | *(empty)* | Path to a custom CA bundle for validating the Bitbucket server's TLS certificate. |
| `-AtlassianTlsVerify` | No | `false` (`true`\|`false`) | `false` | Persisted as the `ATLASSIAN_TLS_VERIFY` User environment variable, controlling TLS certificate verification for Jira/Confluence/Bitbucket requests. |
| `-DryRun` | No (switch) | — | disabled | Validates all arguments and exits without downloading, installing, or writing any config. |
| `-Replace` | No (switch) | — | disabled (refuses to overwrite an unmanaged Claude config) | For `-Scope Project`, allows overwriting an existing `.mcp.json` that wasn't previously managed by this installer. For Cursor/Kiro JSON, replaces only the conflicting `mcpServers.atlassian` entry, preserving every unrelated root key and server entry. |
| `-NonInteractive` | No (switch) | — | disabled (prompts for `-Agents` if omitted) | Disables the interactive `-Agents` prompt; missing required values become hard errors. |

Unlike Jira/Confluence username handling in the Bash installer, PowerShell requires the matching password/token environment variable to already hold a value at install time whenever the corresponding username/module switch is set — regardless of `-NonInteractive` or `-Agents`.

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
- Restored the step-by-step credential environment variable setup guide (Bash and PowerShell) that was dropped during the binary-first README rewrite.
- Restored the Bash and PowerShell installer argument reference tables, updated for the current binary-first script interface (`--release-tag`/`-ReleaseTag` replaces the removed source-build flags).
- Expanded the "latest stable release" install examples to show every supported argument, using the installer's native inline invocation style (`bash -s -- \` / `powershell.exe -File ... \``) consistent with the rest of the guide.
- Reordered the guide so each installer's credential environment variable setup steps appear immediately before that installer's install commands, instead of after both.
- Both installers now report the resolved release tag (or `(local binary)` when `--binary`/`-Binary` was used) in their final success message.
- Added Cursor and Kiro agent registration to both installers (`--agents cursor,kiro` / `-Agents 'Cursor,Kiro'`, plus the `all` alias), with JSON config merging that preserves unrelated servers and root keys, and added `docs/cursor.md` and `docs/kiro.md` configuration guides.

### v1.0.4

- Published amd64 release artifacts for Windows and Linux with SHA-256 checksum manifests and SBOM assets.
