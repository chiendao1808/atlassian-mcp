Set-StrictMode -Version 2.0
$ErrorActionPreference = 'Stop'

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RepoRoot = Split-Path -Parent $ScriptDir
$Installer = Join-Path $RepoRoot 'scripts\install-from-remote.ps1'
$PowerShellExe = (Get-Command powershell.exe).Source
$TmpRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("atlassian-mcp-ps-installer-tests-{0}" -f ([guid]::NewGuid().ToString('N')))

# The installer persists these under the real Windows 'User' scope (HKCU\Environment), not under any
# per-test $env:HOME override, so the whole suite snapshots and restores them to avoid leaving test
# values behind on the machine running these tests.
$PersistedEnvKeys = @(
    'ATLASSIAN_TLS_VERIFY', 'JIRA_BASE_URL', 'JIRA_CA_FILE',
    'BITBUCKET_BASE_URL', 'BITBUCKET_PROJECT_KEY', 'BITBUCKET_USER_SLUG',
    'BITBUCKET_CA_FILE', 'BITBUCKET_BEARER_TOKEN'
)

function Save-PersistedEnv {
    $snapshot = @{}
    foreach ($key in $PersistedEnvKeys) {
        $snapshot[$key] = [Environment]::GetEnvironmentVariable($key, 'User')
    }
    $snapshot
}

function Restore-PersistedEnv($Snapshot) {
    foreach ($key in $PersistedEnvKeys) {
        [Environment]::SetEnvironmentVariable($key, $Snapshot[$key], 'User')
    }
}

# Minimal test harness: every test runs the real installer with fake external tools and asserts observable output.
function Fail($Message) {
    throw "FAIL: $Message"
}

# Asserts that installer-created files exist at the stable public paths.
function Assert-File($Path) {
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
        Fail "missing file: $Path"
    }
}

# Asserts temporary worktrees are removed after successful remote installs.
function Assert-PathMissing($Path) {
    if (Test-Path -LiteralPath $Path) {
        Fail "path still exists: $Path"
    }
}

# Asserts generated content without coupling tests to private installer helpers.
function Assert-Contains($Path, $Text) {
    $content = Get-Content -LiteralPath $Path -Raw
    if ($content.IndexOf($Text, [System.StringComparison]::Ordinal) -lt 0) {
        Fail "missing '$Text' in $Path"
    }
}

# Guards the no-secret contract by checking generated artifacts for forbidden values.
function Assert-NotContains($Path, $Text) {
    if (-not (Test-Path -LiteralPath $Path)) {
        return
    }
    $content = Get-Content -LiteralPath $Path -Raw
    if ($content.IndexOf($Text, [System.StringComparison]::Ordinal) -ge 0) {
        Fail "unexpected '$Text' in $Path"
    }
}

# Counts managed-block markers to prove re-runs replace rather than append duplicate config.
function Assert-Count($Path, $Text, $Want) {
    $content = Get-Content -LiteralPath $Path -Raw
    $got = 0
    $index = 0
    while ($true) {
        $index = $content.IndexOf($Text, $index, [System.StringComparison]::Ordinal)
        if ($index -lt 0) {
            break
        }
        $got++
        $index += $Text.Length
    }
    if ($got -ne $Want) {
        Fail "count for '$Text' in $Path = $got, want $Want"
    }
}

# Extracts the fake clone destination from the recorded git command.
function Get-ClonedSourceDir($Log) {
    $line = Get-Content -LiteralPath $Log | Where-Object { $_ -like 'git clone *' } | Select-Object -First 1
    if ([string]::IsNullOrEmpty($line)) {
        Fail 'missing git clone log line'
    }
    if ($line -notmatch '(?<path>[A-Z]:\\.*atlassian-mcp-src-[0-9a-f]+)$') {
        Fail "could not parse clone destination: $line"
    }
    $Matches.path
}

# Creates fake external commands so installer behavior can be tested without network, Go, or host ACL changes.
function New-Fakes($Dir) {
    New-Item -ItemType Directory -Force -Path $Dir | Out-Null
    @'
param([Parameter(ValueFromRemainingArguments = $true)][string[]]$Args)
Add-Content -LiteralPath $env:FAKE_LOG -Value ("git {0}" -f ($Args -join ' '))
if ($Args[0] -eq 'clone') {
    Write-Error ("Cloning into '{0}'..." -f $Args[$Args.Count - 1])
    $dest = $Args[$Args.Count - 1]
    New-Item -ItemType Directory -Force -Path (Join-Path $dest 'cmd\atlassian-mcp') | Out-Null
    Set-Content -LiteralPath (Join-Path $dest 'go.mod') -Value 'module example.com/atlassian-mcp' -Encoding ASCII
}
if ($Args[0] -eq 'rev-parse') {
    Write-Output 'mocked-ref'
}
exit 0
'@ | Set-Content -LiteralPath (Join-Path $Dir 'git.ps1') -Encoding ASCII
    @'
param([Parameter(ValueFromRemainingArguments = $true)][string[]]$Args)
Add-Content -LiteralPath $env:FAKE_LOG -Value ("go {0}" -f ($Args -join ' '))
if ($Args[0] -eq 'test') {
    Write-Output '?    github.com/chiendao1808/atlassian-mcp/cmd/atlassian-mcp    [no test files]'
}
if ($Args[0] -eq 'build') {
    $out = ''
    for ($i = 0; $i -lt $Args.Count; $i++) {
        if ($Args[$i] -eq '-o') {
            $out = $Args[$i + 1]
        }
    }
    if ([string]::IsNullOrEmpty($out)) {
        exit 1
    }
    Set-Content -LiteralPath $out -Value 'fake binary' -Encoding ASCII
}
exit 0
'@ | Set-Content -LiteralPath (Join-Path $Dir 'go.ps1') -Encoding ASCII
    @'
param([Parameter(ValueFromRemainingArguments = $true)][string[]]$Args)
Add-Content -LiteralPath $env:FAKE_LOG -Value ("icacls {0}" -f ($Args -join ' '))
exit 0
'@ | Set-Content -LiteralPath (Join-Path $Dir 'icacls.ps1') -Encoding ASCII
    @'
param([Parameter(ValueFromRemainingArguments = $true)][string[]]$Args)
Add-Content -LiteralPath $env:FAKE_LOG -Value ("claude {0}" -f ($Args -join ' '))
if ($Args[0] -eq 'mcp' -and $Args[1] -eq 'remove') {
    exit 1
}
exit 0
'@ | Set-Content -LiteralPath (Join-Path $Dir 'claude.ps1') -Encoding ASCII
}

# Runs one isolated installer invocation with HOME, USERPROFILE, PATH, and project/install directories scoped to the case.
function Invoke-InstallerCase($Name, [string[]]$Arguments, [hashtable]$ExtraEnv) {
    $caseDir = Join-Path $TmpRoot $Name
    $fakeBin = Join-Path $caseDir 'bin'
    New-Item -ItemType Directory -Force -Path (Join-Path $caseDir 'home'), (Join-Path $caseDir 'install'), (Join-Path $caseDir 'project') | Out-Null
    New-Fakes $fakeBin

    $oldPath = $env:PATH
    $oldHome = $env:HOME
    $oldUserProfile = $env:USERPROFILE
    $oldFakeLog = $env:FAKE_LOG
    $oldExecutionPolicy = $env:PSExecutionPolicyPreference
    $oldPathext = $env:PATHEXT
    try {
        $env:PATH = "$fakeBin;$oldPath"
        $env:PATHEXT = ".PS1;$oldPathext"
        $env:HOME = Join-Path $caseDir 'home'
        $env:USERPROFILE = Join-Path $caseDir 'home'
        $env:FAKE_LOG = Join-Path $caseDir 'commands.log'
        $env:PSExecutionPolicyPreference = 'Bypass'
        if ($ExtraEnv) {
            foreach ($key in $ExtraEnv.Keys) {
                Set-Item -LiteralPath "Env:$key" -Value $ExtraEnv[$key]
            }
        }

        $baseArgs = @(
            '-NoProfile',
            '-ExecutionPolicy', 'Bypass',
            '-File', $Installer,
            '-InstallDir', (Join-Path $caseDir 'install'),
            '-ProjectDir', (Join-Path $caseDir 'project'),
            '-Scope', 'Project',
            '-NonInteractive'
        )
        $oldErrorActionPreference = $ErrorActionPreference
        $ErrorActionPreference = 'Continue'
        $output = & $PowerShellExe @baseArgs @Arguments *>&1
        $exitCode = $LASTEXITCODE
        $ErrorActionPreference = $oldErrorActionPreference
        [pscustomobject]@{
            ExitCode = $exitCode
            Output = ($output | Out-String)
            CaseDir = $caseDir
            Log = Join-Path $caseDir 'commands.log'
        }
    } finally {
        $env:PATH = $oldPath
        $env:HOME = $oldHome
        $env:USERPROFILE = $oldUserProfile
        $env:FAKE_LOG = $oldFakeLog
        $env:PSExecutionPolicyPreference = $oldExecutionPolicy
        $env:PATHEXT = $oldPathext
    }
}

# Executes a case expected to pass and returns its isolated paths.
function Invoke-InstallerSuccess($Name, [string[]]$Arguments, [hashtable]$ExtraEnv = @{}) {
    $result = Invoke-InstallerCase $Name $Arguments $ExtraEnv
    if ($result.ExitCode -ne 0) {
        Fail "$Name failed with $($result.ExitCode): $($result.Output)"
    }
    $result
}

function Test-HttpsRemotesCheckoutTestBuildAndInstallAtomically {
    foreach ($url in @(
        'https://github.com/acme/atlassian-mcp.git',
        'https://gitlab.com/acme/atlassian-mcp.git',
        'https://bitbucket.internal.example.com/scm/prj/atlassian-mcp.git'
    )) {
        $name = 'remote-' + ($url -replace '[^A-Za-z0-9]', '-')
        $result = Invoke-InstallerSuccess $name @(
            '-SourceRepoUrl', $url,
            '-SourceRef', 'v1.2.3',
            '-Agents', 'Codex',
            '-EnableJira',
            '-JiraBaseUrl', 'https://jira.internal.example.com/jira'
        )
        Assert-Contains $result.Log 'git clone'
        Assert-Contains $result.Log $url
        Assert-Contains $result.Log 'git fetch'
        Assert-Contains $result.Log 'git checkout v1.2.3'
        Assert-Contains $result.Log 'go test ./...'
        Assert-Contains $result.Log 'go build -o'
        Assert-File (Join-Path $result.CaseDir 'install\atlassian-mcp.exe')
        Assert-PathMissing (Get-ClonedSourceDir $result.Log)
        if ($result.Output -notmatch 'cleaned cloned source') {
            Fail "remote install did not report source cleanup: $($result.Output)"
        }
    }
}

function Test-KeepSourcePreservesCloneForDebugging {
    $result = Invoke-InstallerSuccess 'keep-source' @(
        '-SourceRepoUrl', 'https://github.com/acme/atlassian-mcp.git',
        '-KeepSource',
        '-Agents', 'None',
        '-EnableJira',
        '-JiraBaseUrl', 'https://jira.internal.example.com/jira'
    )
    $sourceDir = Get-ClonedSourceDir $result.Log
    try {
        if (-not (Test-Path -LiteralPath $sourceDir -PathType Container)) {
            Fail "kept source was removed: $sourceDir"
        }
    } finally {
        Remove-Item -LiteralPath $sourceDir -Recurse -Force -ErrorAction SilentlyContinue
    }
}

function Test-SshRemoteIsPassedToGitWithoutProviderRewrite {
    $result = Invoke-InstallerSuccess 'ssh' @(
        '-SourceRepoUrl', 'git@gitlab.internal:tools/atlassian-mcp.git',
        '-Agents', 'None',
        '-EnableJira',
        '-JiraBaseUrl', 'https://jira.internal.example.com/jira'
    )
    Assert-Contains $result.Log 'git@gitlab.internal:tools/atlassian-mcp.git'
}

function Test-EmbeddedCredentialsAreRejectedBeforeGit {
    $result = Invoke-InstallerCase 'credential-url' @(
        '-SourceRepoUrl', 'https://user:pass@github.com/acme/atlassian-mcp.git',
        '-Agents', 'None',
        '-EnableJira',
        '-JiraBaseUrl', 'https://jira.internal.example.com/jira'
    ) @{}
    if ($result.ExitCode -eq 0) {
        Fail 'credential URL unexpectedly succeeded'
    }
    if ($result.Output -notmatch 'embedded credentials') {
        Fail "credential URL error did not mention embedded credentials: $($result.Output)"
    }
    if (Test-Path -LiteralPath $result.Log) {
        Fail 'git should not run for credential URL'
    }
}

function Test-ModuleValidationAndNonSecretConfig {
    $result = Invoke-InstallerSuccess 'both' @(
        '-Binary', (Join-Path $RepoRoot 'go.mod'),
        '-SkipTests',
        '-Agents', 'Both',
        '-EnableJira',
        '-JiraBaseUrl', 'https://jira.internal.example.com/jira',
        '-EnableBitbucket',
        '-BitbucketBaseUrl', 'https://bitbucket.internal.example.com/bitbucket',
        '-BitbucketProjectKey', 'PRJ',
        '-BitbucketUserSlug', 'svc-atlassian-mcp',
        '-BitbucketTokenEnv', 'BITBUCKET_SECRET_ENV',
        '-AtlassianTlsVerify', 'true'
    ) @{ BITBUCKET_SECRET_ENV = 'super-secret-token' }

    $codex = Join-Path $result.CaseDir 'project\.codex\config.toml'
    $claude = Join-Path $result.CaseDir 'project\.mcp.json'
    # Codex's MCP launcher does not inherit ambient environment (see Get-ResolvedConfigEnv), so the
    # resolved, non-indirected values are expected in its config -- unlike Claude's, which is not
    # touched here and must stay secret-free.
    Assert-NotContains $codex 'BITBUCKET_SECRET_ENV'
    Assert-NotContains $claude 'BITBUCKET_SECRET_ENV'
    Assert-Contains $codex '[mcp_servers.atlassian.env]'
    Assert-Contains $codex 'BITBUCKET_BEARER_TOKEN = "super-secret-token"'
    Assert-NotContains $claude 'super-secret-token'
    Assert-Contains $codex 'args = []'
    if ((Get-Content -LiteralPath $codex -Raw).IndexOf('powershell.exe', [System.StringComparison]::Ordinal) -ge 0) {
        Fail "codex config should invoke the binary directly, not powershell.exe: $codex"
    }
    Assert-Contains $codex 'atlassian-mcp.exe'
    Assert-Contains $claude 'atlassian-mcp.exe'

    if ([Environment]::GetEnvironmentVariable('ATLASSIAN_TLS_VERIFY', 'User') -ne 'true') {
        Fail 'ATLASSIAN_TLS_VERIFY was not persisted as a User environment variable'
    }
    if ([Environment]::GetEnvironmentVariable('JIRA_BASE_URL', 'User') -ne 'https://jira.internal.example.com/jira') {
        Fail 'JIRA_BASE_URL was not persisted as a User environment variable'
    }
    if ([Environment]::GetEnvironmentVariable('BITBUCKET_BASE_URL', 'User') -ne 'https://bitbucket.internal.example.com/bitbucket') {
        Fail 'BITBUCKET_BASE_URL was not persisted as a User environment variable'
    }
    if ([Environment]::GetEnvironmentVariable('BITBUCKET_PROJECT_KEY', 'User') -ne 'PRJ') {
        Fail 'BITBUCKET_PROJECT_KEY was not persisted as a User environment variable'
    }
    if ([Environment]::GetEnvironmentVariable('BITBUCKET_USER_SLUG', 'User') -ne 'svc-atlassian-mcp') {
        Fail 'BITBUCKET_USER_SLUG was not persisted as a User environment variable'
    }
    if ([Environment]::GetEnvironmentVariable('BITBUCKET_BEARER_TOKEN', 'User') -ne 'super-secret-token') {
        Fail 'BITBUCKET_BEARER_TOKEN was not resolved from BITBUCKET_SECRET_ENV and persisted'
    }

    $missing = Invoke-InstallerCase 'missing-project-key' @(
        '-Binary', (Join-Path $RepoRoot 'go.mod'),
        '-Agents', 'None',
        '-EnableBitbucket',
        '-BitbucketBaseUrl', 'https://bitbucket.internal.example.com/bitbucket',
        '-BitbucketTokenEnv', 'BITBUCKET_SECRET_ENV'
    ) @{ BITBUCKET_SECRET_ENV = 'token' }
    if ($missing.ExitCode -eq 0 -or $missing.Output -notmatch 'BitbucketProjectKey') {
        Fail "missing Bitbucket project key was not rejected: $($missing.Output)"
    }
}

function Test-NonInteractiveBitbucketRequiresTokenEnvValue {
    Remove-Item Env:UNSET_BITBUCKET_TOKEN -ErrorAction SilentlyContinue
    $result = Invoke-InstallerCase 'missing-token' @(
        '-Binary', (Join-Path $RepoRoot 'go.mod'),
        '-Agents', 'None',
        '-EnableBitbucket',
        '-BitbucketBaseUrl', 'https://bitbucket.internal.example.com/bitbucket',
        '-BitbucketProjectKey', 'PRJ',
        '-BitbucketTokenEnv', 'UNSET_BITBUCKET_TOKEN'
    ) @{}
    if ($result.ExitCode -eq 0 -or $result.Output -notmatch 'UNSET_BITBUCKET_TOKEN') {
        Fail "missing token env was not rejected: $($result.Output)"
    }
}

function Test-AgentConfigEscapesBinaryPathForTomlAndJson {
    $caseDir = Join-Path $TmpRoot 'config-escaping'
    $installDir = Join-Path $caseDir 'install space\slash'
    New-Item -ItemType Directory -Force -Path (Join-Path $caseDir 'home'), $installDir, (Join-Path $caseDir 'project'), (Join-Path $caseDir 'bin') | Out-Null
    New-Fakes (Join-Path $caseDir 'bin')

    $oldPath = $env:PATH
    $oldHome = $env:HOME
    $oldUserProfile = $env:USERPROFILE
    $oldFakeLog = $env:FAKE_LOG
    $oldPathext = $env:PATHEXT
    try {
        $env:PATH = "$(Join-Path $caseDir 'bin');$oldPath"
        $env:PATHEXT = ".PS1;$oldPathext"
        $env:HOME = Join-Path $caseDir 'home'
        $env:USERPROFILE = Join-Path $caseDir 'home'
        $env:FAKE_LOG = Join-Path $caseDir 'commands.log'
        $oldErrorActionPreference = $ErrorActionPreference
        $ErrorActionPreference = 'Continue'
        $output = & $PowerShellExe -NoProfile -ExecutionPolicy Bypass -File $Installer `
            -Binary (Join-Path $RepoRoot 'go.mod') `
            -InstallDir $installDir `
            -ProjectDir (Join-Path $caseDir 'project') `
            -Scope Project `
            -Agents Both `
            -EnableJira `
            -JiraBaseUrl 'https://jira.internal.example.com/jira' `
            -NonInteractive *>&1
        $exitCode = $LASTEXITCODE
        $ErrorActionPreference = $oldErrorActionPreference
        if ($exitCode -ne 0) {
            Fail "config escaping failed: $($output | Out-String)"
        }
    } finally {
        $env:PATH = $oldPath
        $env:HOME = $oldHome
        $env:USERPROFILE = $oldUserProfile
        $env:FAKE_LOG = $oldFakeLog
        $env:PATHEXT = $oldPathext
    }

    Assert-Contains (Join-Path $caseDir 'project\.codex\config.toml') 'args = []'
    Assert-Contains (Join-Path $caseDir 'project\.codex\config.toml') 'install space\\slash\\atlassian-mcp.exe'
    Assert-Contains (Join-Path $caseDir 'project\.mcp.json') 'install space\\slash\\atlassian-mcp.exe'
}

function Test-ClaudeCliRegistersScopeLocalAndUser {
    foreach ($scope in @('User', 'Local')) {
        $caseDir = Join-Path $TmpRoot "claude-cli-$scope"
        New-Item -ItemType Directory -Force -Path (Join-Path $caseDir 'home'), (Join-Path $caseDir 'install'), (Join-Path $caseDir 'project'), (Join-Path $caseDir 'bin') | Out-Null
        New-Fakes (Join-Path $caseDir 'bin')

        $oldPath = $env:PATH
        $oldHome = $env:HOME
        $oldUserProfile = $env:USERPROFILE
        $oldFakeLog = $env:FAKE_LOG
        $oldPathext = $env:PATHEXT
        try {
            $env:PATH = "$(Join-Path $caseDir 'bin');$oldPath"
            $env:PATHEXT = ".PS1;$oldPathext"
            $env:HOME = Join-Path $caseDir 'home'
            $env:USERPROFILE = Join-Path $caseDir 'home'
            $env:FAKE_LOG = Join-Path $caseDir 'commands.log'
            $oldErrorActionPreference = $ErrorActionPreference
            $ErrorActionPreference = 'Continue'
            $output = & $PowerShellExe -NoProfile -ExecutionPolicy Bypass -File $Installer `
                -Binary (Join-Path $RepoRoot 'go.mod') `
                -InstallDir (Join-Path $caseDir 'install') `
                -ProjectDir (Join-Path $caseDir 'project') `
                -Scope $scope `
                -Agents Claude `
                -EnableJira `
                -JiraBaseUrl 'https://jira.internal.example.com/jira' `
                -NonInteractive *>&1
            $exitCode = $LASTEXITCODE
            $ErrorActionPreference = $oldErrorActionPreference
            if ($exitCode -ne 0) {
                Fail "claude CLI scope $scope failed: $($output | Out-String)"
            }
        } finally {
            $env:PATH = $oldPath
            $env:HOME = $oldHome
            $env:USERPROFILE = $oldUserProfile
            $env:FAKE_LOG = $oldFakeLog
            $env:PATHEXT = $oldPathext
        }

        $log = Join-Path $caseDir 'commands.log'
        Assert-Contains $log ("claude mcp add atlassian --scope {0} --" -f $scope.ToLowerInvariant())
        Assert-Contains $log ("claude mcp get atlassian --scope {0}" -f $scope.ToLowerInvariant())
        Assert-PathMissing (Join-Path $caseDir 'home\.claude\settings.json')
        Assert-PathMissing (Join-Path $caseDir 'project\.mcp.json')
    }
}

function Test-ClaudeCliMissingBinaryErrorsClearly {
    $caseDir = Join-Path $TmpRoot 'claude-cli-missing'
    New-Item -ItemType Directory -Force -Path (Join-Path $caseDir 'home'), (Join-Path $caseDir 'install'), (Join-Path $caseDir 'project'), (Join-Path $caseDir 'bin') | Out-Null
    New-Fakes (Join-Path $caseDir 'bin')
    Remove-Item -LiteralPath (Join-Path $caseDir 'bin\claude.ps1') -Force

    $oldPath = $env:PATH
    $oldHome = $env:HOME
    $oldUserProfile = $env:USERPROFILE
    $oldFakeLog = $env:FAKE_LOG
    $oldPathext = $env:PATHEXT
    try {
        # Deliberately excludes $oldPath: a real claude CLI may be installed on the host running
        # these tests, and this case must prove behavior when no claude binary can be found at all.
        $env:PATH = "$(Join-Path $caseDir 'bin');$env:SystemRoot\System32;$env:SystemRoot"
        $env:PATHEXT = ".PS1;$oldPathext"
        $env:HOME = Join-Path $caseDir 'home'
        $env:USERPROFILE = Join-Path $caseDir 'home'
        $env:FAKE_LOG = Join-Path $caseDir 'commands.log'
        $oldErrorActionPreference = $ErrorActionPreference
        $ErrorActionPreference = 'Continue'
        $output = & $PowerShellExe -NoProfile -ExecutionPolicy Bypass -File $Installer `
            -Binary (Join-Path $RepoRoot 'go.mod') `
            -InstallDir (Join-Path $caseDir 'install') `
            -ProjectDir (Join-Path $caseDir 'project') `
            -Scope User `
            -Agents Claude `
            -EnableJira `
            -JiraBaseUrl 'https://jira.internal.example.com/jira' `
            -NonInteractive *>&1
        $exitCode = $LASTEXITCODE
        $ErrorActionPreference = $oldErrorActionPreference
        if ($exitCode -eq 0) {
            Fail 'installer unexpectedly succeeded without claude CLI'
        }
        # Nested error rendering can hard-wrap the message, so tolerate whitespace/newlines between words.
        if ((($output | Out-String) -replace '\s+', ' ') -notmatch 'claude\s*CLI is required') {
            Fail "missing claude CLI error did not mention requirement: $($output | Out-String)"
        }
    } finally {
        $env:PATH = $oldPath
        $env:HOME = $oldHome
        $env:USERPROFILE = $oldUserProfile
        $env:FAKE_LOG = $oldFakeLog
        $env:PATHEXT = $oldPathext
    }
}

function Test-RerunIsIdempotentConfigFailureRollsBackAndRestrictsAcls {
    $result = Invoke-InstallerSuccess 'idem' @(
        '-Binary', (Join-Path $RepoRoot 'go.mod'),
        '-Agents', 'Codex',
        '-EnableJira',
        '-JiraBaseUrl', 'https://jira.internal.example.com/jira'
    )
    $result = Invoke-InstallerSuccess 'idem' @(
        '-Binary', (Join-Path $RepoRoot 'go.mod'),
        '-Agents', 'Codex',
        '-EnableJira',
        '-JiraBaseUrl', 'https://jira.internal.example.com/jira'
    )
    $codex = Join-Path $result.CaseDir 'project\.codex\config.toml'
    Assert-Count $codex 'atlassian-mcp managed block' 2
    Assert-Count $codex 'command =' 1
    Assert-Contains $result.Log 'icacls'

    $rollbackDir = Join-Path $TmpRoot 'rollback'
    New-Item -ItemType Directory -Force -Path (Join-Path $rollbackDir 'home'), (Join-Path $rollbackDir 'install'), (Join-Path $rollbackDir 'project\.codex'), (Join-Path $rollbackDir 'project\.mcp.json') | Out-Null
    Set-Content -LiteralPath (Join-Path $rollbackDir 'project\.codex\config.toml') -Value 'original config' -Encoding ASCII
    New-Fakes (Join-Path $rollbackDir 'bin')

    $oldPath = $env:PATH
    $oldHome = $env:HOME
    $oldUserProfile = $env:USERPROFILE
    $oldFakeLog = $env:FAKE_LOG
    $oldPathext = $env:PATHEXT
    try {
        $env:PATH = "$(Join-Path $rollbackDir 'bin');$oldPath"
        $env:PATHEXT = ".PS1;$oldPathext"
        $env:HOME = Join-Path $rollbackDir 'home'
        $env:USERPROFILE = Join-Path $rollbackDir 'home'
        $env:FAKE_LOG = Join-Path $rollbackDir 'commands.log'
        $oldErrorActionPreference = $ErrorActionPreference
        $ErrorActionPreference = 'Continue'
        $output = & $PowerShellExe -NoProfile -ExecutionPolicy Bypass -File $Installer `
            -Binary (Join-Path $RepoRoot 'go.mod') `
            -InstallDir (Join-Path $rollbackDir 'install') `
            -ProjectDir (Join-Path $rollbackDir 'project') `
            -Scope Project `
            -Agents Both `
            -EnableJira `
            -JiraBaseUrl 'https://jira.internal.example.com/jira' `
            -NonInteractive *>&1
        $exitCode = $LASTEXITCODE
        $ErrorActionPreference = $oldErrorActionPreference
        if ($exitCode -eq 0) {
            Fail 'config failure unexpectedly succeeded'
        }
        if (($output | Out-String) -notmatch 'refusing to replace directory\s+Claude config') {
            Fail "config failure did not include root cause: $($output | Out-String)"
        }
    } finally {
        $env:PATH = $oldPath
        $env:HOME = $oldHome
        $env:USERPROFILE = $oldUserProfile
        $env:FAKE_LOG = $oldFakeLog
        $env:PATHEXT = $oldPathext
    }
    Assert-Contains (Join-Path $rollbackDir 'project\.codex\config.toml') 'original config'
}

function Test-DryRunValidatesWithoutSideEffects {
    $result = Invoke-InstallerSuccess 'dry-run' @(
        '-SourceRepoUrl', 'https://github.com/acme/atlassian-mcp.git',
        '-Agents', 'Both',
        '-DryRun',
        '-EnableJira',
        '-JiraBaseUrl', 'https://jira.internal.example.com/jira'
    )
    if (Test-Path -LiteralPath (Join-Path $result.CaseDir 'install\atlassian-mcp.exe')) {
        Fail 'dry-run installed binary'
    }
    if (Test-Path -LiteralPath (Join-Path $result.CaseDir 'project\.codex\config.toml')) {
        Fail 'dry-run wrote codex config'
    }
}

function Test-FinalPathsAndReadmeBootstrapContract {
    Assert-File $Installer
    if (Test-Path -LiteralPath (Join-Path $RepoRoot 'install-from-remote.ps1')) {
        Fail 'root PowerShell installer should not exist'
    }
    Assert-Contains (Join-Path $RepoRoot 'README.md') 'https://raw.githubusercontent.com/chiendao1808/atlassian-mcp/${INSTALLER_REF}/scripts/install-from-remote.ps1'
    Assert-Contains (Join-Path $RepoRoot 'README.md') 'Invoke-WebRequest -Uri $InstallerUrl -OutFile $InstallerFile'
    Assert-Contains (Join-Path $RepoRoot 'README.md') "powershell.exe -NoProfile -ExecutionPolicy Bypass -File `$InstallerFile"
    Assert-NotContains (Join-Path $RepoRoot 'README.md') 'Invoke-Expression'
}

$PersistedEnvSnapshot = Save-PersistedEnv
try {
    New-Item -ItemType Directory -Force -Path $TmpRoot | Out-Null
    $tests = @(
        'Test-HttpsRemotesCheckoutTestBuildAndInstallAtomically',
        'Test-KeepSourcePreservesCloneForDebugging',
        'Test-SshRemoteIsPassedToGitWithoutProviderRewrite',
        'Test-EmbeddedCredentialsAreRejectedBeforeGit',
        'Test-ModuleValidationAndNonSecretConfig',
        'Test-NonInteractiveBitbucketRequiresTokenEnvValue',
        'Test-AgentConfigEscapesBinaryPathForTomlAndJson',
        'Test-ClaudeCliRegistersScopeLocalAndUser',
        'Test-ClaudeCliMissingBinaryErrorsClearly',
        'Test-RerunIsIdempotentConfigFailureRollsBackAndRestrictsAcls',
        'Test-DryRunValidatesWithoutSideEffects',
        'Test-FinalPathsAndReadmeBootstrapContract'
    )
    foreach ($test in $tests) {
        & $test
        Write-Host "ok $test"
    }
    Write-Host 'PASS install-from-remote.Tests.ps1'
} finally {
    Restore-PersistedEnv $PersistedEnvSnapshot
    Remove-Item -LiteralPath $TmpRoot -Recurse -Force -ErrorAction SilentlyContinue
}
