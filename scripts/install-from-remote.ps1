param(
    [string]$ReleaseTag = '',
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
    [switch]$EnableConfluence,
    [string]$ConfluenceBaseUrl = '',
    [string]$ConfluenceCaFile = '',
    [string]$ConfluenceUsername = '',
    [string]$ConfluencePasswordEnv = 'CONFLUENCE_PASSWORD',
    [switch]$EnableBitbucket,
    [string]$BitbucketBaseUrl = '',
    [string]$BitbucketProjectKey = '',
    [string]$BitbucketUserSlug = '',
    [string]$BitbucketTokenEnv = 'BITBUCKET_BEARER_TOKEN',
    [string]$BitbucketCaFile = '',
    [ValidateSet('true', 'false')]
    [string]$AtlassianTlsVerify = 'false',
    [switch]$DryRun,
    [switch]$Replace,
    [switch]$NonInteractive
)

Set-StrictMode -Version 2.0
$ErrorActionPreference = 'Stop'

$Marker = 'atlassian-mcp managed block'
$ManagedConfluenceEnvKeys = @('CONFLUENCE_BASE_URL', 'CONFLUENCE_CA_FILE', 'CONFLUENCE_USERNAME', 'CONFLUENCE_PASSWORD')
$Script:Backups = @()
$Script:DownloadDir = ''
$Script:InstallSucceeded = $false
$ReleaseDownloadTimeoutSec = 120

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

# Ensures a credential indirection variable name is safe to look up and reference.
function Validate-TokenEnvName($Value) {
    if ($Value -notmatch '^[A-Za-z_][A-Za-z0-9_]*$') {
        Die 'credential environment variable name must be valid'
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

# Resolves module config and configured credential env-name indirection into fixed environment
# variable names the binary reads (internal/config, internal/jira, internal/confluence,
# internal/bitbucket). Single source of truth for
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
    if ($EnableConfluence) {
        $envVars['CONFLUENCE_BASE_URL'] = $ConfluenceBaseUrl
    }
    if (-not [string]::IsNullOrEmpty($ConfluenceCaFile)) {
        $envVars['CONFLUENCE_CA_FILE'] = $ConfluenceCaFile
    }
    if (-not [string]::IsNullOrEmpty($ConfluenceUsername)) {
        # Optional: lets confluence_authenticate use environment fallback and, when paired with
        # a password at process startup, lets Confluence auto-authenticate without a tool call.
        # The password is resolved by env-name indirection so it is never accepted as an argument.
        $confluencePassword = Get-EnvValue $ConfluencePasswordEnv
        if ([string]::IsNullOrEmpty($confluencePassword)) {
            Die "CONFLUENCE password environment variable $ConfluencePasswordEnv is not set"
        }
        $envVars['CONFLUENCE_USERNAME'] = $ConfluenceUsername
        $envVars['CONFLUENCE_PASSWORD'] = $confluencePassword
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
# Confluence keys are installer-managed as a set, so omitted values are cleared to prevent
# stale credentials or stale static config from surviving a reinstall.
# Processes already running at install time will not see these until restarted; Broadcast-
# EnvironmentChange only refreshes processes launched afterward (e.g. via Explorer), not ones
# already running.
function Set-PersistedConfigEnv($EnvVars) {
    foreach ($key in $ManagedConfluenceEnvKeys) {
        if (-not $EnvVars.Contains($key)) {
            [Environment]::SetEnvironmentVariable($key, $null, 'User')
        }
    }
    foreach ($key in $EnvVars.Keys) {
        [Environment]::SetEnvironmentVariable($key, $EnvVars[$key], 'User')
    }
    Broadcast-EnvironmentChange
}

# Produces Codex TOML with only the installer-managed block replaced. EnvVars is written as an
# explicit [mcp_servers.atlassian.env] table -- required because Codex's MCP launcher does not
# pass its own ambient environment through to spawned stdio servers (see Get-ResolvedConfigEnv).
# This does put configured Bitbucket/Jira/Confluence secret values in Codex's config file, unlike the
# Claude Code path; that is a deliberate, Codex-specific exception forced by Codex's launcher, not a
# general policy change. The
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

# Resolves the release asset suffix supported by this installer.
# Deliberately avoids System.Runtime.InteropServices.RuntimeInformation: on some Windows
# PowerShell 5.1 (Desktop/.NET Framework) hosts that type's IsOSPlatform() resolves but
# OSArchitecture throws PropertyNotFoundStrict under Set-StrictMode -Version 2.0, because the
# facade assembly backing the type can be partially implemented depending on installed .NET
# Framework version/GAC state. System.Environment has existed unchanged since .NET Framework 4.0,
# so it works identically on every PowerShell version this script supports.
function Get-ReleasePlatform {
    # $IsWindows is a PowerShell Core (6+) automatic variable and does not exist in Windows
    # PowerShell (5.1, Desktop edition); Desktop edition only ever runs on Windows, so its
    # absence implies Windows rather than indicating an unknown platform.
    $isWindowsHost = if (Test-Path Variable:\IsWindows) { $IsWindows } else { $true }
    if (-not $isWindowsHost -or -not [Environment]::Is64BitOperatingSystem) {
        Die ("unsupported platform: {0} (supported: Windows amd64)" -f [Environment]::OSVersion.VersionString)
    }
    return 'windows_amd64.exe'
}

# Uses GitHub's latest stable release API unless the caller pins an exact tag.
function Resolve-ReleaseTag {
    if (-not [string]::IsNullOrEmpty($ReleaseTag)) {
        return $ReleaseTag
    }
    $response = Invoke-WebRequest -UseBasicParsing -TimeoutSec $ReleaseDownloadTimeoutSec -Uri 'https://api.github.com/repos/chiendao1808/atlassian-mcp/releases/latest'
    $payload = $response.Content | ConvertFrom-Json
    if ([string]::IsNullOrEmpty($payload.tag_name)) {
        Die 'could not resolve latest GitHub release tag'
    }
    return [string]$payload.tag_name
}

# Downloads the platform release binary and verifies it against the release checksum manifest.
function Download-ReleaseBinary {
    $platform = Get-ReleasePlatform
    $tag = Resolve-ReleaseTag
    $version = $tag.TrimStart('v')
    $asset = "atlassian-mcp_${version}_${platform}"
    $checksumAsset = "atlassian-mcp_${version}_checksums.txt"
    $baseUrl = "https://github.com/chiendao1808/atlassian-mcp/releases/download/$tag"
    $Script:DownloadDir = Join-Path ([System.IO.Path]::GetTempPath()) ("atlassian-mcp-release-{0}" -f ([guid]::NewGuid().ToString('N')))
    New-Item -ItemType Directory -Force -Path $Script:DownloadDir | Out-Null

    $assetPath = Join-Path $Script:DownloadDir $asset
    $checksumPath = Join-Path $Script:DownloadDir $checksumAsset
    Invoke-WebRequest -UseBasicParsing -TimeoutSec $ReleaseDownloadTimeoutSec -Uri "$baseUrl/$asset" -OutFile $assetPath
    Invoke-WebRequest -UseBasicParsing -TimeoutSec $ReleaseDownloadTimeoutSec -Uri "$baseUrl/$checksumAsset" -OutFile $checksumPath
    $pattern = '(^|\s)\*?{0}$' -f [regex]::Escape($asset)
    $line = Get-Content -LiteralPath $checksumPath | Where-Object { $_ -match $pattern } | Select-Object -First 1
    if ([string]::IsNullOrEmpty($line)) {
        Die "checksum entry not found for $asset"
    }
    $expected = (($line -split '\s+')[0]).ToLowerInvariant()
    $actual = (Get-FileHash -Algorithm SHA256 -Path $assetPath).Hash.ToLowerInvariant()
    if ($actual -ne $expected) {
        Die "checksum mismatch for $asset"
    }
    return $assetPath
}

# Removes temporary release downloads after success or failure.
function Cleanup-Download {
    if (-not [string]::IsNullOrEmpty($Script:DownloadDir)) {
        Remove-Item -LiteralPath $Script:DownloadDir -Recurse -Force -ErrorAction SilentlyContinue
        $Script:DownloadDir = ''
    }
}

try {
    if (-not [string]::IsNullOrEmpty($Binary) -and -not (Test-Path -LiteralPath $Binary -PathType Leaf)) {
        Die '-Binary must point to a readable file'
    }
    if (-not [string]::IsNullOrEmpty($ReleaseTag) -and $ReleaseTag -notmatch '^v[0-9]+[.][0-9]+[.][0-9]+([-+][A-Za-z0-9._-]+)?$') {
        Die '-ReleaseTag must look like v1.2.3'
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
    Validate-TokenEnvName $JiraPasswordEnv
    Validate-TokenEnvName $ConfluencePasswordEnv
    Validate-TokenEnvName $BitbucketTokenEnv
    if (-not $EnableJira -and -not $EnableConfluence -and -not $EnableBitbucket) {
        Die 'select at least one module with -EnableJira, -EnableConfluence, or -EnableBitbucket'
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
        if ([string]::IsNullOrEmpty((Get-EnvValue $JiraPasswordEnv))) {
            Die "$JiraPasswordEnv is required when -JiraUsername is set"
        }
    }
    if ($EnableConfluence) {
        if ([string]::IsNullOrEmpty($ConfluenceBaseUrl)) {
            Die '-ConfluenceBaseUrl is required with -EnableConfluence'
        }
        Require-ServiceUrl '-ConfluenceBaseUrl' $ConfluenceBaseUrl
    }
    if (-not [string]::IsNullOrEmpty($ConfluenceUsername)) {
        if (-not $EnableConfluence) {
            Die '-ConfluenceUsername requires -EnableConfluence'
        }
        if ([string]::IsNullOrEmpty((Get-EnvValue $ConfluencePasswordEnv))) {
            Die "$ConfluencePasswordEnv is required when -ConfluenceUsername is set"
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
        if ([string]::IsNullOrEmpty((Get-EnvValue $BitbucketTokenEnv))) {
            Die "$BitbucketTokenEnv is required for Bitbucket installs"
        }
    }

    if ($DryRun) {
        Write-Host 'dry-run: validated installer arguments'
        exit 0
    }

    if (-not [string]::IsNullOrEmpty($Binary)) {
        $builtBinary = $Binary
    } else {
        $builtBinary = Download-ReleaseBinary
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
    Cleanup-Download
} finally {
    if (-not $Script:InstallSucceeded) {
        Cleanup-Download
    }
}
