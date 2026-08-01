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

# Emits a validation or execution failure with the stable installer name for callers and tests.
function Die($Message) {
    Write-Error "install-from-remote.ps1: $Message"
    exit 1
}

# Runs an external command and preserves its real exit status as an installer failure.
function Invoke-Checked($Command, [string[]]$Arguments) {
    & $Command @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "$Command failed with exit code $LASTEXITCODE"
    }
}

# Escapes a value for generated PowerShell single-quoted literals in the wrapper.
function Escape-PowerShellLiteral($Value) {
    return ([string]$Value).Replace("'", "''")
}

# Escapes paths for Codex TOML basic strings.
function Escape-TomlString($Value) {
    return ([string]$Value).Replace('\', '\\').Replace('"', '\"')
}

# Escapes paths for Claude JSON strings without relying on newer PowerShell JSON formatting behavior.
function Escape-JsonString($Value) {
    return ([string]$Value).Replace('\', '\\').Replace('"', '\"')
}

# Validates non-secret service URLs before writing them into wrapper environment.
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

# Ensures the Bitbucket token indirection can be safely referenced by the wrapper.
function Validate-TokenEnvName($Value) {
    if ($Value -notmatch '^[A-Za-z_][A-Za-z0-9_]*$') {
        Die '-BitbucketTokenEnv must be an environment variable name'
    }
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

# Applies a restrictive ACL to newly created wrapper/config files when Windows exposes icacls.
function Protect-NewFileAcl($Path, [bool]$WasCreated) {
    if (-not $WasCreated) {
        return
    }
    if (-not (Get-Command icacls -ErrorAction SilentlyContinue)) {
        return
    }
    $user = $env:USERNAME
    if ([string]::IsNullOrEmpty($user)) {
        return
    }
    & icacls $Path '/inheritance:r' '/grant:r' "${user}:(R,W)" | Out-Null
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
    Move-Item -LiteralPath $tmp -Destination $To -Force
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
    Move-Item -LiteralPath $tmp -Destination $To -Force
    Protect-NewFileAcl $To $created
}

# Generates the PowerShell wrapper; token values are read only when the wrapper starts.
function Write-Wrapper($Wrapper, $BinaryPath) {
    $lines = @(
        'Set-StrictMode -Version 2.0',
        '$ErrorActionPreference = ''Stop''',
        ('$env:ATLASSIAN_TLS_VERIFY = ''{0}''' -f (Escape-PowerShellLiteral $AtlassianTlsVerify.ToLowerInvariant()))
    )
    if ($EnableJira) {
        $lines += '$env:JIRA_BASE_URL = ''{0}''' -f (Escape-PowerShellLiteral $JiraBaseUrl)
    }
    if (-not [string]::IsNullOrEmpty($JiraCaFile)) {
        $lines += '$env:JIRA_CA_FILE = ''{0}''' -f (Escape-PowerShellLiteral $JiraCaFile)
    }
    if ($EnableBitbucket) {
        $lines += '$env:BITBUCKET_BASE_URL = ''{0}''' -f (Escape-PowerShellLiteral $BitbucketBaseUrl)
        $lines += '$env:BITBUCKET_PROJECT_KEY = ''{0}''' -f (Escape-PowerShellLiteral $BitbucketProjectKey)
        if (-not [string]::IsNullOrEmpty($BitbucketUserSlug)) {
            $lines += '$env:BITBUCKET_USER_SLUG = ''{0}''' -f (Escape-PowerShellLiteral $BitbucketUserSlug)
        }
        if (-not [string]::IsNullOrEmpty($BitbucketCaFile)) {
            $lines += '$env:BITBUCKET_CA_FILE = ''{0}''' -f (Escape-PowerShellLiteral $BitbucketCaFile)
        }
        # The secret remains in the launch environment; config stores only the variable name indirection.
        $lines += 'if ([string]::IsNullOrEmpty([Environment]::GetEnvironmentVariable(''{0}''))) {{ Write-Error ''BITBUCKET token environment variable {0} is not set''; exit 1 }}' -f (Escape-PowerShellLiteral $BitbucketTokenEnv)
        $lines += '$env:BITBUCKET_BEARER_TOKEN = [Environment]::GetEnvironmentVariable(''{0}'')' -f (Escape-PowerShellLiteral $BitbucketTokenEnv)
    }
    $lines += '& ''{0}'' @args' -f (Escape-PowerShellLiteral $BinaryPath)
    $lines += 'exit $LASTEXITCODE'
    Write-FileAtomically ($lines -join [Environment]::NewLine) $Wrapper $false
}

# Produces Codex TOML with only the installer-managed block replaced.
function New-CodexConfig($Path, $Command) {
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
    $body += "# END $Marker"
    return ($body -join [Environment]::NewLine)
}

# Produces Claude MCP JSON and refuses unmanaged existing content unless -Replace is supplied.
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

# Resolves user, local, and project agent config paths for the selected scope.
function Get-ConfigPaths {
    $homeDir = Get-HomeDir
    if ($Scope -eq 'User') {
        return [pscustomobject]@{
            Codex = Join-Path $homeDir '.codex\config.toml'
            Claude = Join-Path $homeDir '.claude\atlassian-mcp.mcp.json'
        }
    }
    return [pscustomobject]@{
        Codex = Join-Path $ProjectDir '.codex\config.toml'
        Claude = Join-Path $ProjectDir '.mcp.json'
    }
}

# Configures selected agent files idempotently; caller rolls back any partial failure.
function Configure-Agents($Command) {
    $paths = Get-ConfigPaths
    if ($Agents -eq 'Codex' -or $Agents -eq 'Both') {
        Write-FileAtomically (New-CodexConfig $paths.Codex $Command) $paths.Codex $true
    }
    if ($Agents -eq 'Claude' -or $Agents -eq 'Both') {
        Write-FileAtomically (New-ClaudeConfig $paths.Claude $Command) $paths.Claude $true
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

# Removes cloned source on exit unless -KeepSource was requested.
function Cleanup-Source {
    if (-not $KeepSource -and -not [string]::IsNullOrEmpty($Script:SourceDir)) {
        Remove-Item -LiteralPath $Script:SourceDir -Recurse -Force -ErrorAction SilentlyContinue
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
    if ($EnableBitbucket) {
        if ([string]::IsNullOrEmpty($BitbucketBaseUrl)) {
            Die '-BitbucketBaseUrl is required with -EnableBitbucket'
        }
        if ([string]::IsNullOrEmpty($BitbucketProjectKey)) {
            Die '-BitbucketProjectKey is required with -EnableBitbucket'
        }
        Require-ServiceUrl '-BitbucketBaseUrl' $BitbucketBaseUrl
        Validate-TokenEnvName $BitbucketTokenEnv
        if ($NonInteractive -and [string]::IsNullOrEmpty([Environment]::GetEnvironmentVariable($BitbucketTokenEnv))) {
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
    $wrapper = Join-Path $InstallDir 'atlassian-mcp-run.ps1'
    Copy-Atomically $builtBinary $installedBinary
    Write-Wrapper $wrapper $installedBinary

    if ($Agents -ne 'None') {
        try {
            Configure-Agents $wrapper
            Cleanup-Backups
        } catch {
            Rollback-Configs
            Die 'failed to configure selected agents'
        }
    }

    Write-Host "installed atlassian-mcp to $installedBinary"
} finally {
    Cleanup-Source
}
