param(
    [string]$SourceRepoUrl = '',
    [string]$SourceRef = 'main',
    [int]$SourceCloneDepth = 1,
    [switch]$KeepSource,
    [string]$Binary = '',
    [string]$InstallDir = (Join-Path $HOME '.local\bin'),
    [ValidateSet('Claude', 'Codex', 'Both', 'None')]
    [string]$Agents = '',
    [ValidateSet('Local', 'Project', 'User')]
    [string]$Scope = 'User',
    [string]$ProjectDir = (Get-Location).Path,
    [switch]$EnableJira,
    [string]$JiraBaseUrl = '',
    [string]$JiraCaFile = '',
    [string]$JiraUsername = '',
    [string]$JiraPasswordEnv = 'JIRA_PASSWORD',
    [switch]$EnableBitbucket,
    [string]$BitbucketBaseUrl = '',
    [string]$BitbucketProjectKey = '',
    [string]$BitbucketUserSlug = '',
    [string]$BitbucketTokenEnv = 'BITBUCKET_BEARER_TOKEN',
    [string]$BitbucketCaFile = '',
    [ValidateSet('true', 'false')]
    [string]$AtlassianTlsVerify = 'false',
    [switch]$SkipTests,
    [switch]$DryRun,
    [switch]$Replace,
    [switch]$NonInteractive
)

Set-StrictMode -Version 2.0
$ErrorActionPreference = 'Stop'

$Marker = 'atlassian-mcp managed block'
$Script:Backups = @()
$Script:SourceDir = ''
$Script:InstallSucceeded = $false

# Emits a validation or execution failure with the stable installer name for callers and tests.
function Die($Message) {
    Write-Error "install-from-remote.ps1: $Message"
    exit 1
}

# Runs an external command and preserves its real exit status as an installer failure.
function Invoke-Checked($Command, [string[]]$Arguments) {
    $oldErrorActionPreference = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    try {
        & $Command @Arguments 2>&1 | ForEach-Object { Write-Host $_ }
        $exitCode = $LASTEXITCODE
    } finally {
        $ErrorActionPreference = $oldErrorActionPreference
    }
    if ($exitCode -ne 0) {
        throw "$Command failed with exit code $exitCode"
    }
}

# Escapes paths for Codex TOML basic strings.
function Escape-TomlString($Value) {
    return ([string]$Value).Replace('\', '\\').Replace('"', '\"')
}

# Escapes paths for Claude JSON strings without relying on newer PowerShell JSON formatting behavior.
function Escape-JsonString($Value) {
    return ([string]$Value).Replace('\', '\\').Replace('"', '\"')
}

# Validates non-secret service URLs before persisting them as environment variables.
function Require-ServiceUrl($Name, $Value) {
    if ($Value -notmatch '^https?://') {
        Die "$Name must be an http or https URL"
    }
    if ($Value -match '[?#]') {
        Die "$Name must not include query or fragment"
    }
    $authority = ([string]$Value -replace '^[^:]+://', '') -replace '/.*$', ''
    if ($authority -match '@') {
        Die "$Name must not include embedded credentials"
    }
}

# Rejects credential-bearing HTTPS source URLs before Git can log or persist them.
function Reject-EmbeddedSourceCredentials($Value) {
    if ($Value -match '^https?://[^/?#]*@') {
        Die '-SourceRepoUrl must not include embedded credentials'
    }
}

# Ensures the Bitbucket token indirection variable name is safe to look up and reference.
function Validate-TokenEnvName($Value) {
    if ($Value -notmatch '^[A-Za-z_][A-Za-z0-9_]*$') {
        Die '-BitbucketTokenEnv must be an environment variable name'
    }
}

# Reads an environment variable from the current process, then from the persisted user environment.
function Get-EnvValue($Name) {
    $value = [Environment]::GetEnvironmentVariable($Name)
    if ([string]::IsNullOrEmpty($value)) {
        $value = [Environment]::GetEnvironmentVariable($Name, 'User')
    }
    return $value
}

# Resolves the home directory consistently for Windows PowerShell and PowerShell 7 hosts.
function Get-HomeDir {
    if (-not [string]::IsNullOrEmpty($env:HOME)) {
        return $env:HOME
    }
    if (-not [string]::IsNullOrEmpty($env:USERPROFILE)) {
        return $env:USERPROFILE
    }
    Die 'HOME or USERPROFILE is required'
}

# Applies a restrictive ACL to newly created agent config files when Windows exposes icacls.
function Grant-CurrentUserModify($Path) {
    if (-not (Get-Command icacls -ErrorAction SilentlyContinue)) {
        return
    }
    $user = [System.Security.Principal.WindowsIdentity]::GetCurrent().Name
    if ([string]::IsNullOrEmpty($user)) {
        return
    }
    & icacls $Path '/grant:r' "${user}:(M)" | Out-Null
}

# Applies a restrictive ACL to newly created agent config files when Windows exposes icacls.
function Protect-NewFileAcl($Path, [bool]$WasCreated) {
    if (-not $WasCreated) {
        return
    }
    if (-not (Get-Command icacls -ErrorAction SilentlyContinue)) {
        return
    }
    & icacls $Path '/inheritance:r' | Out-Null
    Grant-CurrentUserModify $Path
}

# Records config state so a later agent-config failure can restore every earlier write.
function Backup-Config($Path) {
    $backup = "$Path.bak.$PID"
    $exists = Test-Path -LiteralPath $Path -PathType Leaf
    if ($exists) {
        Copy-Item -LiteralPath $Path -Destination $backup -Force
    } else {
        $parent = Split-Path -Parent $backup
        if (-not (Test-Path -LiteralPath $parent)) {
            New-Item -ItemType Directory -Force -Path $parent | Out-Null
        }
        Set-Content -LiteralPath $backup -Value '' -Encoding ASCII
    }
    $Script:Backups += [pscustomobject]@{ Path = $Path; Backup = $backup; Exists = $exists }
}

# Restores backed-up config files in reverse order after a partial configuration failure.
function Rollback-Configs {
    for ($i = $Script:Backups.Count - 1; $i -ge 0; $i--) {
        $entry = $Script:Backups[$i]
        if ($entry.Exists) {
            Move-Item -LiteralPath $entry.Backup -Destination $entry.Path -Force -ErrorAction SilentlyContinue
        } else {
            Remove-Item -LiteralPath $entry.Path -Force -ErrorAction SilentlyContinue
            Remove-Item -LiteralPath $entry.Backup -Force -ErrorAction SilentlyContinue
        }
    }
}

# Deletes successful-install backup files once every selected agent config has been written.
function Cleanup-Backups {
    foreach ($entry in $Script:Backups) {
        Remove-Item -LiteralPath $entry.Backup -Force -ErrorAction SilentlyContinue
    }
}

# Replaces an existing file with a sibling temp file without depending on Move-Item overwrite behavior.
function Move-FileIntoPlace($From, $To) {
    if (Test-Path -LiteralPath $To -PathType Leaf) {
        Set-ItemProperty -LiteralPath $To -Name IsReadOnly -Value $false -ErrorAction SilentlyContinue
        Grant-CurrentUserModify $To
        $replaceBackup = "$To.replace.$PID"
        [System.IO.File]::Replace(
            [System.IO.Path]::GetFullPath($From),
            [System.IO.Path]::GetFullPath($To),
            [System.IO.Path]::GetFullPath($replaceBackup)
        )
        Remove-Item -LiteralPath $replaceBackup -Force -ErrorAction SilentlyContinue
        return
    }
    Move-Item -LiteralPath $From -Destination $To -Force
}

# Copies through a sibling temp file and renames into place so readers never observe partial files.
function Copy-Atomically($From, $To) {
    if (Test-Path -LiteralPath $To -PathType Container) {
        throw "$To is a directory"
    }
    $parent = Split-Path -Parent $To
    if (-not (Test-Path -LiteralPath $parent)) {
        New-Item -ItemType Directory -Force -Path $parent | Out-Null
    }
    $tmp = "$To.tmp.$PID"
    Copy-Item -LiteralPath $From -Destination $tmp -Force
    Move-FileIntoPlace $tmp $To
}

# Writes generated content atomically, optionally with rollback backup and new-file ACL restriction.
function Write-FileAtomically($Content, $To, [bool]$Backup) {
    if (Test-Path -LiteralPath $To -PathType Container) {
        throw "$To is a directory"
    }
    $created = -not (Test-Path -LiteralPath $To)
    if ($Backup) {
        Backup-Config $To
    }
    $parent = Split-Path -Parent $To
    if (-not (Test-Path -LiteralPath $parent)) {
        New-Item -ItemType Directory -Force -Path $parent | Out-Null
    }
    $tmp = "$To.tmp.$PID"
    Set-Content -LiteralPath $tmp -Value $Content -Encoding ASCII
    Move-FileIntoPlace $tmp $To
    Protect-NewFileAcl $To $created
}

# Broadcasts WM_SETTINGCHANGE so processes that cache their own environment snapshot (notably
# explorer.exe, which supplies the starting environment for anything launched from the Start Menu
# or a desktop shortcut) refresh it immediately. [Environment]::SetEnvironmentVariable(...,'User')
# only updates the registry; without this broadcast, apps launched via Explorer after this install
# can still inherit Explorer's stale environment snapshot until the next logon, even though a
# terminal opened directly (cmd.exe, powershell.exe) already reads the registry fresh on its own.
function Broadcast-EnvironmentChange {
    if (-not ('AtlassianMcpInstaller.NativeMethods' -as [type])) {
        Add-Type -Namespace AtlassianMcpInstaller -Name NativeMethods -MemberDefinition @'
[DllImport("user32.dll", SetLastError = true, CharSet = CharSet.Auto)]
public static extern IntPtr SendMessageTimeout(IntPtr hWnd, uint Msg, UIntPtr wParam, string lParam, uint fuFlags, uint uTimeout, out UIntPtr lpdwResult);
'@
    }
    $result = [UIntPtr]::Zero
    [void][AtlassianMcpInstaller.NativeMethods]::SendMessageTimeout([IntPtr]0xffff, 0x1A, [UIntPtr]::Zero, 'Environment', 0x2, 5000, [ref]$result)
}

# Resolves the module config, plus the Bitbucket token, into the fixed environment variable names
# the binary reads (internal/config, internal/jira, internal/bitbucket). Single source of truth for
# both Set-PersistedConfigEnv (Windows User env, for ambient-inheriting hosts and manual terminal use)
# and Codex's per-server [mcp_servers.atlassian.env] table (Codex's own MCP child-process launcher does
# not inherit the parent's ambient environment, confirmed via Codex's own logs showing the binary
# starting with every module disabled even after a full restart and an environment-change broadcast;
# Codex requires env vars declared explicitly per server, exactly like its own node_repl entry does).
function Get-ResolvedConfigEnv {
    $envVars = [ordered]@{ ATLASSIAN_TLS_VERIFY = $AtlassianTlsVerify.ToLowerInvariant() }
    if ($EnableJira) {
        $envVars['JIRA_BASE_URL'] = $JiraBaseUrl
    }
    if (-not [string]::IsNullOrEmpty($JiraCaFile)) {
        $envVars['JIRA_CA_FILE'] = $JiraCaFile
    }
    if (-not [string]::IsNullOrEmpty($JiraUsername)) {
        # Optional: lets jira_authenticate skip its chat-input step (ADR-0004) and, when
        # combined with the process starting with these already set, authenticate
        # automatically at startup (ADR-0005). Resolved via -JiraPasswordEnv indirection so
        # the password itself is never a command-line argument, mirroring -BitbucketTokenEnv.
        $jiraPassword = Get-EnvValue $JiraPasswordEnv
        if ([string]::IsNullOrEmpty($jiraPassword)) {
            Die "JIRA password environment variable $JiraPasswordEnv is not set"
        }
        $envVars['JIRA_USERNAME'] = $JiraUsername
        $envVars['JIRA_PASSWORD'] = $jiraPassword
    }
    if ($EnableBitbucket) {
        $envVars['BITBUCKET_BASE_URL'] = $BitbucketBaseUrl
        $envVars['BITBUCKET_PROJECT_KEY'] = $BitbucketProjectKey
        if (-not [string]::IsNullOrEmpty($BitbucketUserSlug)) {
            $envVars['BITBUCKET_USER_SLUG'] = $BitbucketUserSlug
        }
        if (-not [string]::IsNullOrEmpty($BitbucketCaFile)) {
            $envVars['BITBUCKET_CA_FILE'] = $BitbucketCaFile
        }
        $bitbucketToken = Get-EnvValue $BitbucketTokenEnv
        if ([string]::IsNullOrEmpty($bitbucketToken)) {
            Die "BITBUCKET token environment variable $BitbucketTokenEnv is not set"
        }
        $envVars['BITBUCKET_BEARER_TOKEN'] = $bitbucketToken
    }
    return $envVars
}

# Persists the resolved config as User environment variables so hosts that do inherit ambient
# environment (confirmed for Claude Code) and manual terminal runs of the binary pick it up.
# Processes already running at install time will not see these until restarted; Broadcast-
# EnvironmentChange only refreshes processes launched afterward (e.g. via Explorer), not ones
# already running.
function Set-PersistedConfigEnv($EnvVars) {
    foreach ($key in $EnvVars.Keys) {
        [Environment]::SetEnvironmentVariable($key, $EnvVars[$key], 'User')
    }
    Broadcast-EnvironmentChange
}

# Produces Codex TOML with only the installer-managed block replaced. EnvVars is written as an
# explicit [mcp_servers.atlassian.env] table -- required because Codex's MCP launcher does not
# pass its own ambient environment through to spawned stdio servers (see Get-ResolvedConfigEnv).
# This does put the Bitbucket token in Codex's config file, unlike the Claude Code path; that is a
# deliberate, Codex-specific exception forced by Codex's launcher, not a general policy change. The
# file keeps the same current-user-only ACL as the persisted User environment variable it mirrors.
function New-CodexConfig($Path, $Command, $EnvVars) {
    $body = @()
    if (Test-Path -LiteralPath $Path -PathType Leaf) {
        $skip = $false
        foreach ($line in Get-Content -LiteralPath $Path) {
            if ($line -eq "# BEGIN $Marker") {
                $skip = $true
                continue
            }
            if ($line -eq "# END $Marker") {
                $skip = $false
                continue
            }
            if (-not $skip) {
                $body += $line
            }
        }
    }
    $body += "# BEGIN $Marker"
    $body += '[mcp_servers.atlassian]'
    $body += 'command = "{0}"' -f (Escape-TomlString $Command)
    $body += 'args = []'
    if ($EnvVars.Count -gt 0) {
        $body += '[mcp_servers.atlassian.env]'
        foreach ($key in $EnvVars.Keys) {
            $body += '{0} = "{1}"' -f $key, (Escape-TomlString $EnvVars[$key])
        }
    }
    $body += "# END $Marker"
    return ($body -join [Environment]::NewLine)
}

# Produces Claude MCP JSON and refuses unmanaged existing content unless -Replace is supplied.
# Only used for -Scope Project; -Scope Local/User register through the claude CLI instead.
function New-ClaudeConfig($Path, $Command) {
    if ((Test-Path -LiteralPath $Path) -and -not $Replace) {
        if (Test-Path -LiteralPath $Path -PathType Container) {
            throw "refusing to replace directory Claude config at $Path"
        }
        $existing = Get-Content -LiteralPath $Path -Raw
        if ($existing.IndexOf('atlassian-mcp managed by install-from-remote.ps1', [System.StringComparison]::Ordinal) -lt 0) {
            Die "refusing to replace unmanaged Claude config at $Path; use -Replace"
        }
    }
    return @"
{
  "_comment": "atlassian-mcp managed by install-from-remote.ps1",
  "mcpServers": {
    "atlassian": {
      "command": "$(Escape-JsonString $Command)"
    }
  }
}
"@
}

# Ensures the Claude Code CLI is present before it is used to register the atlassian MCP server.
function Require-ClaudeCli {
    if (-not (Get-Command claude -ErrorAction SilentlyContinue)) {
        Die 'claude CLI is required for -Scope Local/User; install it, use -Scope Project, or select -Agents Codex'
    }
}

# Registers/updates the atlassian MCP server via the Claude Code CLI so the entry lands in Claude's
# real config store instead of a hand-written file (writing directly to e.g. ~/.claude/settings.json
# does not register an MCP server with Claude Code).
function Configure-ClaudeCli($Command) {
    Require-ClaudeCli
    $claudeScope = $Scope.ToLowerInvariant()
    $oldErrorActionPreference = $ErrorActionPreference
    Push-Location $ProjectDir
    try {
        # Native stderr must be drained under $ErrorActionPreference='Continue'; otherwise Windows
        # PowerShell 5.1 turns any stderr line from claude.exe into a terminating NativeCommandError.
        $ErrorActionPreference = 'Continue'
        & claude mcp remove atlassian --scope $claudeScope 2>&1 | Out-Null
        $ErrorActionPreference = $oldErrorActionPreference
        Invoke-Checked 'claude' @('mcp', 'add', 'atlassian', '--scope', $claudeScope, '--', $Command)
        $ErrorActionPreference = 'Continue'
        & claude mcp get atlassian --scope $claudeScope 2>&1 | Out-Null
        $getExitCode = $LASTEXITCODE
        $ErrorActionPreference = $oldErrorActionPreference
        if ($getExitCode -ne 0) {
            Write-Warning 'could not verify atlassian MCP registration via claude mcp get'
        }
    } finally {
        $ErrorActionPreference = $oldErrorActionPreference
        Pop-Location
    }
}

# Resolves user, local, and project agent config paths for the selected scope.
function Get-ConfigPaths {
    $homeDir = Get-HomeDir
    if ($Scope -eq 'User') {
        return [pscustomobject]@{
            Codex = Join-Path $homeDir '.codex\config.toml'
        }
    }
    if ($Scope -eq 'Local') {
        return [pscustomobject]@{
            Codex = Join-Path $ProjectDir '.codex\config.toml'
        }
    }
    return [pscustomobject]@{
        Codex = Join-Path $ProjectDir '.codex\config.toml'
        Claude = Join-Path $ProjectDir '.mcp.json'
    }
}

# Configures selected agent files idempotently; caller rolls back any partial failure.
function Configure-Agents($Command, $EnvVars) {
    $paths = Get-ConfigPaths
    if ($Agents -eq 'Codex' -or $Agents -eq 'Both') {
        Write-FileAtomically (New-CodexConfig $paths.Codex $Command $EnvVars) $paths.Codex $true
    }
    if ($Agents -eq 'Claude' -or $Agents -eq 'Both') {
        if ($Scope -eq 'Project') {
            Write-FileAtomically (New-ClaudeConfig $paths.Claude $Command) $paths.Claude $true
        } else {
            Configure-ClaudeCli $Command
        }
    }
}

# Clones a provider-neutral remote, fetches the requested ref, and verifies the worktree has HEAD.
function Clone-Source {
    $Script:SourceDir = Join-Path ([System.IO.Path]::GetTempPath()) ("atlassian-mcp-src-{0}" -f ([guid]::NewGuid().ToString('N')))
    Invoke-Checked 'git' @('clone', '--depth', ([string]$SourceCloneDepth), $SourceRepoUrl, $Script:SourceDir)
    Push-Location $Script:SourceDir
    try {
        & git @('fetch', '--depth', ([string]$SourceCloneDepth), 'origin', $SourceRef) | Out-Null
        Invoke-Checked 'git' @('checkout', $SourceRef)
        Invoke-Checked 'git' @('rev-parse', '--verify', 'HEAD')
    } finally {
        Pop-Location
    }
}

# Runs repository tests unless skipped, then builds cmd/atlassian-mcp into a temporary executable.
function Build-Binary {
    $out = Join-Path ([System.IO.Path]::GetTempPath()) ("atlassian-mcp-build-{0}.exe" -f ([guid]::NewGuid().ToString('N')))
    Push-Location $Script:SourceDir
    try {
        if (-not $SkipTests) {
            Invoke-Checked 'go' @('test', './...')
        }
        Invoke-Checked 'go' @('build', '-o', $out, './cmd/atlassian-mcp')
    } finally {
        Pop-Location
    }
    return $out
}

# Removes cloned source after install, unless -KeepSource was requested for debugging.
function Cleanup-Source {
    if (-not $KeepSource -and -not [string]::IsNullOrEmpty($Script:SourceDir)) {
        $sourceDir = $Script:SourceDir
        Remove-Item -LiteralPath $sourceDir -Recurse -Force -ErrorAction SilentlyContinue
        if (Test-Path -LiteralPath $sourceDir) {
            Write-Warning "could not clean cloned source $sourceDir"
        } else {
            Write-Host "cleaned cloned source $sourceDir"
            $Script:SourceDir = ''
        }
    }
}

try {
    if ([string]::IsNullOrEmpty($Binary) -and [string]::IsNullOrEmpty($SourceRepoUrl)) {
        Die '-SourceRepoUrl is required unless -Binary is used'
    }
    if (-not [string]::IsNullOrEmpty($SourceRepoUrl)) {
        Reject-EmbeddedSourceCredentials $SourceRepoUrl
        if ($SourceRepoUrl -notmatch '^(https?://|git@|ssh://)') {
            Die '-SourceRepoUrl must be HTTPS or SSH'
        }
    }
    if (-not [string]::IsNullOrEmpty($Binary) -and -not (Test-Path -LiteralPath $Binary -PathType Leaf)) {
        Die '-Binary must point to a readable file'
    }
    if ([string]::IsNullOrEmpty($Agents)) {
        if ($NonInteractive) {
            Die '-Agents is required with -NonInteractive'
        }
        $Agents = Read-Host 'Select coding agents (Claude/Codex/Both/None)'
    }
    if ($Agents -notin @('Claude', 'Codex', 'Both', 'None')) {
        Die '-Agents must be Claude, Codex, Both, or None'
    }
    if (-not $EnableJira -and -not $EnableBitbucket) {
        Die 'select at least one module with -EnableJira or -EnableBitbucket'
    }
    if ($EnableJira) {
        if ([string]::IsNullOrEmpty($JiraBaseUrl)) {
            Die '-JiraBaseUrl is required with -EnableJira'
        }
        Require-ServiceUrl '-JiraBaseUrl' $JiraBaseUrl
    }
    if (-not [string]::IsNullOrEmpty($JiraUsername)) {
        if (-not $EnableJira) {
            Die '-JiraUsername requires -EnableJira'
        }
        Validate-TokenEnvName $JiraPasswordEnv
        if ($NonInteractive -and [string]::IsNullOrEmpty((Get-EnvValue $JiraPasswordEnv))) {
            Die "$JiraPasswordEnv is required for non-interactive installs when -JiraUsername is set"
        }
    }
    if ($EnableBitbucket) {
        if ([string]::IsNullOrEmpty($BitbucketBaseUrl)) {
            Die '-BitbucketBaseUrl is required with -EnableBitbucket'
        }
        if ([string]::IsNullOrEmpty($BitbucketProjectKey)) {
            Die '-BitbucketProjectKey is required with -EnableBitbucket'
        }
        Require-ServiceUrl '-BitbucketBaseUrl' $BitbucketBaseUrl
        Validate-TokenEnvName $BitbucketTokenEnv
        if ($NonInteractive -and [string]::IsNullOrEmpty((Get-EnvValue $BitbucketTokenEnv))) {
            Die "$BitbucketTokenEnv is required for non-interactive Bitbucket installs"
        }
    }

    if ($DryRun) {
        Write-Host 'dry-run: validated installer arguments'
        exit 0
    }

    if (-not [string]::IsNullOrEmpty($Binary)) {
        $builtBinary = $Binary
    } else {
        Clone-Source
        $builtBinary = Build-Binary
    }

    $installedBinary = Join-Path $InstallDir 'atlassian-mcp.exe'
    Copy-Atomically $builtBinary $installedBinary
    $envVars = Get-ResolvedConfigEnv
    Set-PersistedConfigEnv $envVars

    if ($Agents -ne 'None') {
        try {
            Configure-Agents $installedBinary $envVars
            Cleanup-Backups
        } catch {
            Rollback-Configs
            Die ("failed to configure selected agents: {0}" -f $_.Exception.Message)
        }
    }

    Write-Host "installed atlassian-mcp to $installedBinary"
    Write-Host 'restart Claude Code, Codex, or your terminal session to pick up the newly persisted environment variables'
    $Script:InstallSucceeded = $true
    Cleanup-Source
} finally {
    if (-not $Script:InstallSucceeded) {
        Cleanup-Source
    }
}
