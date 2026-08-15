Set-StrictMode -Version 2.0
$ErrorActionPreference = 'Stop'

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RepoRoot = Split-Path -Parent $ScriptDir
$Installer = Join-Path $RepoRoot 'scripts\install-from-remote.ps1'
$PowerShellExe = (Get-Command powershell.exe).Source
$TmpRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("atlassian-mcp-ps-installer-tests-{0}" -f ([guid]::NewGuid().ToString('N')))

# The installer writes Windows User env vars, so snapshot and restore the managed keys.
$PersistedEnvKeys = @(
    'ATLASSIAN_TLS_VERIFY', 'JIRA_BASE_URL', 'JIRA_CA_FILE', 'JIRA_USERNAME', 'JIRA_PASSWORD',
    'CONFLUENCE_BASE_URL', 'CONFLUENCE_CA_FILE', 'CONFLUENCE_USERNAME', 'CONFLUENCE_PASSWORD',
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

function Fail($Message) {
    throw "FAIL: $Message"
}

function Assert-File($Path) {
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
        Fail "missing file: $Path"
    }
}

function Assert-Contains($Path, $Text) {
    $content = Get-Content -LiteralPath $Path -Raw
    if ($content.IndexOf($Text, [System.StringComparison]::Ordinal) -lt 0) {
        Fail "missing '$Text' in $Path"
    }
}

function Assert-NotContains($Path, $Text) {
    if (-not (Test-Path -LiteralPath $Path)) {
        return
    }
    $content = Get-Content -LiteralPath $Path -Raw
    if ($content.IndexOf($Text, [System.StringComparison]::Ordinal) -ge 0) {
        Fail "unexpected '$Text' in $Path"
    }
}

function Assert-Count($Path, $Text, $Want) {
    $content = Get-Content -LiteralPath $Path -Raw
    $got = 0
    $index = 0
    while ($true) {
        $index = $content.IndexOf($Text, $index, [System.StringComparison]::Ordinal)
        if ($index -lt 0) { break }
        $got++
        $index += $Text.Length
    }
    if ($got -ne $Want) {
        Fail "count for '$Text' in $Path = $got, want $Want"
    }
}

function Remove-RepoRootModuleAnalysisCache {
    $rootCache = Join-Path $RepoRoot 'Microsoft'
    if (-not (Test-Path -LiteralPath $rootCache)) {
        return
    }
    $allowed = @(
        $rootCache,
        (Join-Path $rootCache 'Windows'),
        (Join-Path $rootCache 'Windows\PowerShell'),
        (Join-Path $rootCache 'Windows\PowerShell\ModuleAnalysisCache')
    )
    $actual = @(Get-ChildItem -LiteralPath $rootCache -Force -Recurse | ForEach-Object { $_.FullName })
    foreach ($path in $actual) {
        if ($path -notin $allowed) {
            Fail "unexpected file under generated PowerShell cache tree: $path"
        }
    }
    Remove-Item -LiteralPath $rootCache -Recurse -Force -ErrorAction Stop
}

function New-FakeRelease($Dir) {
    New-Item -ItemType Directory -Force -Path $Dir | Out-Null
    Set-Content -LiteralPath (Join-Path $Dir 'latest.json') -Value '{"tag_name":"v1.2.3"}' -Encoding ASCII
    foreach ($version in @('1.2.3', '9.9.9')) {
        $asset = "atlassian-mcp_${version}_windows_amd64.exe"
        $assetPath = Join-Path $Dir $asset
        Set-Content -LiteralPath $assetPath -Value "atlassian-mcp $version" -Encoding ASCII
        $hash = (Get-FileHash -Algorithm SHA256 -Path $assetPath).Hash.ToLowerInvariant()
        Set-Content -LiteralPath (Join-Path $Dir "atlassian-mcp_${version}_checksums.txt") -Value "$hash  $asset" -Encoding ASCII
    }
}

# Fake git/go fail if called because release installs must not build source.
function New-Fakes($Dir) {
    New-Item -ItemType Directory -Force -Path $Dir | Out-Null
    @'
param([Parameter(ValueFromRemainingArguments = $true)][string[]]$Args)
Add-Content -LiteralPath $env:FAKE_LOG -Value ("git {0}" -f ($Args -join ' '))
exit 99
'@ | Set-Content -LiteralPath (Join-Path $Dir 'git.ps1') -Encoding ASCII
    @'
param([Parameter(ValueFromRemainingArguments = $true)][string[]]$Args)
Add-Content -LiteralPath $env:FAKE_LOG -Value ("go {0}" -f ($Args -join ' '))
exit 99
'@ | Set-Content -LiteralPath (Join-Path $Dir 'go.ps1') -Encoding ASCII
    @'
param([Parameter(ValueFromRemainingArguments = $true)][string[]]$Args)
Add-Content -LiteralPath $env:FAKE_LOG -Value ("icacls {0}" -f ($Args -join ' '))
exit 0
'@ | Set-Content -LiteralPath (Join-Path $Dir 'icacls.ps1') -Encoding ASCII
    @'
param([Parameter(ValueFromRemainingArguments = $true)][string[]]$Args)
Add-Content -LiteralPath $env:FAKE_LOG -Value ("claude {0}" -f ($Args -join ' '))
if ($Args[0] -eq 'mcp' -and $Args[1] -eq 'remove') { exit 1 }
exit 0
'@ | Set-Content -LiteralPath (Join-Path $Dir 'claude.ps1') -Encoding ASCII
}

function Invoke-InstallerCase($Name, [string[]]$Arguments, [hashtable]$ExtraEnv = @{}) {
    $caseDir = Join-Path $TmpRoot $Name
    $fakeBin = Join-Path $caseDir 'bin'
    $releaseDir = Join-Path $caseDir 'release'
    $moduleCache = Join-Path $caseDir 'ps-module-cache\ModuleAnalysisCache'
    New-Item -ItemType Directory -Force -Path (Join-Path $caseDir 'home'), (Join-Path $caseDir 'install'), (Join-Path $caseDir 'project') | Out-Null
    New-Fakes $fakeBin
    New-FakeRelease $releaseDir
    if ($ExtraEnv.ContainsKey('PRECREATE_INSTALLED_BINARY')) {
        Set-Content -LiteralPath (Join-Path $caseDir 'install\atlassian-mcp.exe') -Value $ExtraEnv['PRECREATE_INSTALLED_BINARY'] -Encoding ASCII
    }
    if ($ExtraEnv.ContainsKey('REMOVE_FAKE_CLAUDE')) {
        Remove-Item -LiteralPath (Join-Path $fakeBin 'claude.ps1') -Force
    }

    $oldPath = $env:PATH
    $oldHome = $env:HOME
    $oldUserProfile = $env:USERPROFILE
    $oldFakeLog = $env:FAKE_LOG
    $oldFakeReleaseDir = $env:FAKE_RELEASE_DIR
    $oldBadChecksum = $env:FAKE_CHECKSUM_MISMATCH
    $oldExecutionPolicy = $env:PSExecutionPolicyPreference
    $oldPathext = $env:PATHEXT
    $oldModuleAnalysisCache = $env:PSModuleAnalysisCachePath
    try {
        if ($ExtraEnv.ContainsKey('PATH_VALUE')) {
            $env:PATH = $ExtraEnv['PATH_VALUE']
        } else {
            $env:PATH = "$fakeBin;$oldPath"
        }
        $env:PATHEXT = ".PS1;$oldPathext"
        $env:HOME = Join-Path $caseDir 'home'
        $env:USERPROFILE = Join-Path $caseDir 'home'
        $env:FAKE_LOG = Join-Path $caseDir 'commands.log'
        $env:FAKE_RELEASE_DIR = $releaseDir
        $env:PSExecutionPolicyPreference = 'Bypass'
        # Scope Windows PowerShell's module-analysis cache to the case temp tree so tests never dirty the repo root.
        $env:PSModuleAnalysisCachePath = $moduleCache
        foreach ($key in $ExtraEnv.Keys) {
            if ($key -in @('PRECREATE_INSTALLED_BINARY', 'REMOVE_FAKE_CLAUDE', 'PATH_VALUE')) {
                continue
            }
            Set-Item -LiteralPath "Env:$key" -Value $ExtraEnv[$key]
        }

        $baseArgs = @()
        if ($Arguments -notcontains '-InstallDir') {
            $baseArgs += @('-InstallDir', (Join-Path $caseDir 'install'))
        }
        if ($Arguments -notcontains '-ProjectDir') {
            $baseArgs += @('-ProjectDir', (Join-Path $caseDir 'project'))
        }
        if ($Arguments -notcontains '-Scope') {
            $baseArgs += @('-Scope', 'Project')
        }
        $baseArgs += '-NonInteractive'
        $runner = Join-Path $caseDir 'runner.ps1'
        $installerLiteral = $Installer.Replace("'", "''")
        $allArgs = @($baseArgs) + @($Arguments)
        $commandParts = foreach ($arg in $allArgs) {
            if ($arg.StartsWith('-')) {
                $arg
            } else {
                "'{0}'" -f ($arg.Replace("'", "''"))
            }
        }
        $runnerPreamble = @"
`$InstallerPath = '$installerLiteral'
`$InstallerCommand = "& '`$InstallerPath' $($commandParts -join ' ')"
"@
        $runnerBody = @'
function Invoke-WebRequest {
    param([string]$Uri, [string]$OutFile, [switch]$UseBasicParsing, [int]$TimeoutSec)
    Add-Content -LiteralPath $env:FAKE_LOG -Value "web timeout=$TimeoutSec $Uri"
    if ($Uri -like '*/releases/latest') {
        return [pscustomobject]@{ Content = (Get-Content -LiteralPath (Join-Path $env:FAKE_RELEASE_DIR 'latest.json') -Raw) }
    }
    $name = Split-Path -Leaf $Uri
    if ($name -like '*_checksums.txt' -and $env:FAKE_CHECKSUM_MISMATCH -eq '1') {
        $asset = $name.Replace('_checksums.txt', '_windows_amd64.exe')
        Set-Content -LiteralPath $OutFile -Value "0000000000000000000000000000000000000000000000000000000000000000  $asset" -Encoding ASCII
        return
    }
    Copy-Item -LiteralPath (Join-Path $env:FAKE_RELEASE_DIR $name) -Destination $OutFile -Force
}
try {
    Invoke-Expression $InstallerCommand
    if (-not $?) { exit 1 }
    exit 0
} catch {
    Write-Error $_
    exit 1
}
'@
        Set-Content -LiteralPath $runner -Value ($runnerPreamble + [Environment]::NewLine + $runnerBody) -Encoding ASCII
        $oldErrorActionPreference = $ErrorActionPreference
        $ErrorActionPreference = 'Continue'
        $output = & $PowerShellExe -NoProfile -ExecutionPolicy Bypass -File $runner *>&1
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
        $env:FAKE_RELEASE_DIR = $oldFakeReleaseDir
        $env:FAKE_CHECKSUM_MISMATCH = $oldBadChecksum
        $env:PSExecutionPolicyPreference = $oldExecutionPolicy
        $env:PATHEXT = $oldPathext
        $env:PSModuleAnalysisCachePath = $oldModuleAnalysisCache
    }
}

function Invoke-InstallerSuccess($Name, [string[]]$Arguments, [hashtable]$ExtraEnv = @{}) {
    $result = Invoke-InstallerCase $Name $Arguments $ExtraEnv
    if ($result.ExitCode -ne 0) {
        Fail "$Name failed with $($result.ExitCode): $($result.Output)"
    }
    $result
}

function Test-DefaultReleaseDownloadVerifiesAndInstallsWithoutSourceBuild {
    $result = Invoke-InstallerSuccess 'default-release' @(
        '-Agents', 'None',
        '-EnableJira',
        '-JiraBaseUrl', 'https://jira.internal.example.com/jira'
    )
    Assert-File (Join-Path $result.CaseDir 'install\atlassian-mcp.exe')
    Assert-Contains (Join-Path $result.CaseDir 'install\atlassian-mcp.exe') 'atlassian-mcp 1.2.3'
    Assert-Contains $result.Log 'releases/latest'
    Assert-Contains $result.Log 'web timeout=120'
    Assert-Contains $result.Log 'atlassian-mcp_1.2.3_windows_amd64.exe'
    Assert-Contains $result.Log 'atlassian-mcp_1.2.3_checksums.txt'
    Assert-NotContains $result.Log 'git clone'
    Assert-NotContains $result.Log 'go build'
}

function Test-ReleaseTagPinsExactAsset {
    $result = Invoke-InstallerSuccess 'pinned-release' @(
        '-ReleaseTag', 'v9.9.9',
        '-Agents', 'None',
        '-EnableJira',
        '-JiraBaseUrl', 'https://jira.internal.example.com/jira'
    )
    Assert-Contains (Join-Path $result.CaseDir 'install\atlassian-mcp.exe') 'atlassian-mcp 9.9.9'
    Assert-Contains $result.Log 'download/v9.9.9/atlassian-mcp_9.9.9_windows_amd64.exe'
}

function Test-ChecksumMismatchFailsWithoutReplacingDestination {
    $result = Invoke-InstallerCase 'checksum-mismatch' @(
        '-Agents', 'None',
        '-EnableJira',
        '-JiraBaseUrl', 'https://jira.internal.example.com/jira'
    ) @{ FAKE_CHECKSUM_MISMATCH = '1'; PRECREATE_INSTALLED_BINARY = 'old binary' }
    if ($result.ExitCode -eq 0 -or $result.Output -notmatch 'checksum mismatch') {
        Fail "checksum mismatch was not rejected: $($result.Output)"
    }
    Assert-Contains (Join-Path $result.CaseDir 'install\atlassian-mcp.exe') 'old binary'
}

function Test-BinaryOverrideKeepsConfigBehaviorAndSkipsDownload {
    $result = Invoke-InstallerSuccess 'binary-config' @(
        '-Binary', (Join-Path $RepoRoot 'go.mod'),
        '-Agents', 'Both',
        '-EnableJira',
        '-JiraBaseUrl', 'https://jira.internal.example.com/jira',
        '-JiraUsername', 'jira-svc',
        '-JiraPasswordEnv', 'JIRA_SECRET_ENV',
        '-EnableConfluence',
        '-ConfluenceBaseUrl', 'https://confluence.internal.example.com/confluence',
        '-ConfluenceUsername', 'confluence-svc',
        '-ConfluencePasswordEnv', 'CONFLUENCE_SECRET_ENV',
        '-EnableBitbucket',
        '-BitbucketBaseUrl', 'https://bitbucket.internal.example.com/bitbucket',
        '-BitbucketProjectKey', 'PRJ',
        '-BitbucketTokenEnv', 'BITBUCKET_SECRET_ENV'
    ) @{ BITBUCKET_SECRET_ENV = 'super-secret-token'; JIRA_SECRET_ENV = 'super-secret-password'; CONFLUENCE_SECRET_ENV = 'super-secret-confluence-password' }

    Assert-NotContains $result.Log 'web '
    $codex = Join-Path $result.CaseDir 'project\.codex\config.toml'
    $claude = Join-Path $result.CaseDir 'project\.mcp.json'
    Assert-Contains $codex 'BITBUCKET_BEARER_TOKEN = "super-secret-token"'
    Assert-Contains $codex 'JIRA_PASSWORD = "super-secret-password"'
    Assert-Contains $codex 'CONFLUENCE_PASSWORD = "super-secret-confluence-password"'
    Assert-NotContains $claude 'super-secret-token'
}

function Test-ServiceBaseUrlsRejectEmbeddedCredentials {
    $jira = Invoke-InstallerCase 'jira-credential-url' @(
        '-Binary', (Join-Path $RepoRoot 'go.mod'),
        '-Agents', 'None',
        '-EnableJira',
        '-JiraBaseUrl', 'https://user:pass@jira.internal.example.com/jira'
    )
    if ($jira.ExitCode -eq 0 -or $jira.Output -notmatch 'embedded credentials') {
        Fail "credential Jira URL was not rejected: $($jira.Output)"
    }

    $bitbucket = Invoke-InstallerCase 'bitbucket-credential-url' @(
        '-Binary', (Join-Path $RepoRoot 'go.mod'),
        '-Agents', 'None',
        '-EnableBitbucket',
        '-BitbucketBaseUrl', 'https://user:pass@bitbucket.internal.example.com/bitbucket',
        '-BitbucketProjectKey', 'PRJ',
        '-BitbucketTokenEnv', 'BITBUCKET_TOKEN_FOR_URL_TEST'
    ) @{ BITBUCKET_TOKEN_FOR_URL_TEST = 'token-value' }
    if ($bitbucket.ExitCode -eq 0 -or $bitbucket.Output -notmatch 'embedded credentials') {
        Fail "credential Bitbucket URL was not rejected: $($bitbucket.Output)"
    }
}

function Test-ModuleAndCredentialValidation {
    Remove-Item Env:UNSET_BITBUCKET_TOKEN -ErrorAction SilentlyContinue
    $missingToken = Invoke-InstallerCase 'missing-token' @(
        '-Binary', (Join-Path $RepoRoot 'go.mod'),
        '-Agents', 'None',
        '-EnableBitbucket',
        '-BitbucketBaseUrl', 'https://bitbucket.internal.example.com/bitbucket',
        '-BitbucketProjectKey', 'PRJ',
        '-BitbucketTokenEnv', 'UNSET_BITBUCKET_TOKEN'
    )
    if ($missingToken.ExitCode -eq 0 -or $missingToken.Output -notmatch 'UNSET_BITBUCKET_TOKEN') {
        Fail "missing Bitbucket token env was not rejected: $($missingToken.Output)"
    }

    $missingProject = Invoke-InstallerCase 'missing-project-key' @(
        '-Binary', (Join-Path $RepoRoot 'go.mod'),
        '-Agents', 'None',
        '-EnableBitbucket',
        '-BitbucketBaseUrl', 'https://bitbucket.internal.example.com/bitbucket',
        '-BitbucketTokenEnv', 'BITBUCKET_SECRET_ENV'
    ) @{ BITBUCKET_SECRET_ENV = 'token' }
    if ($missingProject.ExitCode -eq 0 -or $missingProject.Output -notmatch 'BitbucketProjectKey') {
        Fail "missing Bitbucket project key was not rejected: $($missingProject.Output)"
    }

    $noJira = Invoke-InstallerCase 'jira-username-without-enable' @(
        '-Binary', (Join-Path $RepoRoot 'go.mod'),
        '-Agents', 'None',
        '-EnableBitbucket',
        '-BitbucketBaseUrl', 'https://bitbucket.internal.example.com/bitbucket',
        '-BitbucketProjectKey', 'PRJ',
        '-BitbucketTokenEnv', 'BITBUCKET_SECRET_ENV',
        '-JiraUsername', 'jira-svc'
    ) @{ BITBUCKET_SECRET_ENV = 'token' }
    if ($noJira.ExitCode -eq 0 -or $noJira.Output -notmatch '-JiraUsername requires -EnableJira') {
        Fail "-JiraUsername without -EnableJira was not rejected: $($noJira.Output)"
    }

    Remove-Item Env:UNSET_JIRA_PASSWORD -ErrorAction SilentlyContinue
    $missingJiraPassword = Invoke-InstallerCase 'jira-username-missing-password' @(
        '-Binary', (Join-Path $RepoRoot 'go.mod'),
        '-Agents', 'None',
        '-EnableJira',
        '-JiraBaseUrl', 'https://jira.internal.example.com/jira',
        '-JiraUsername', 'jira-svc',
        '-JiraPasswordEnv', 'UNSET_JIRA_PASSWORD'
    )
    if ($missingJiraPassword.ExitCode -eq 0 -or $missingJiraPassword.Output -notmatch 'UNSET_JIRA_PASSWORD') {
        Fail "missing Jira password env was not rejected: $($missingJiraPassword.Output)"
    }

    $noConfluence = Invoke-InstallerCase 'confluence-username-without-enable' @(
        '-Binary', (Join-Path $RepoRoot 'go.mod'),
        '-Agents', 'None',
        '-EnableJira',
        '-JiraBaseUrl', 'https://jira.internal.example.com/jira',
        '-ConfluenceUsername', 'confluence-svc'
    )
    if ($noConfluence.ExitCode -eq 0 -or $noConfluence.Output -notmatch '-ConfluenceUsername requires -EnableConfluence') {
        Fail "-ConfluenceUsername without -EnableConfluence was not rejected: $($noConfluence.Output)"
    }

    Remove-Item Env:UNSET_CONFLUENCE_PASSWORD -ErrorAction SilentlyContinue
    $missingConfluencePassword = Invoke-InstallerCase 'confluence-username-missing-password' @(
        '-Binary', (Join-Path $RepoRoot 'go.mod'),
        '-Agents', 'None',
        '-EnableConfluence',
        '-ConfluenceBaseUrl', 'https://confluence.internal.example.com/confluence',
        '-ConfluenceUsername', 'confluence-svc',
        '-ConfluencePasswordEnv', 'UNSET_CONFLUENCE_PASSWORD'
    )
    if ($missingConfluencePassword.ExitCode -eq 0 -or $missingConfluencePassword.Output -notmatch 'UNSET_CONFLUENCE_PASSWORD') {
        Fail "missing Confluence password env was not rejected: $($missingConfluencePassword.Output)"
    }
}

function Test-ConfluencePersistedEnvClearsWhenDisabledOrCredentialsOmitted {
    foreach ($key in @('CONFLUENCE_BASE_URL', 'CONFLUENCE_CA_FILE', 'CONFLUENCE_USERNAME', 'CONFLUENCE_PASSWORD')) {
        [Environment]::SetEnvironmentVariable($key, "stale-$key", 'User')
    }
    Invoke-InstallerSuccess 'confluence-disabled-clears' @(
        '-Binary', (Join-Path $RepoRoot 'go.mod'),
        '-Agents', 'None',
        '-EnableJira',
        '-JiraBaseUrl', 'https://jira.internal.example.com/jira'
    ) | Out-Null
    foreach ($key in @('CONFLUENCE_BASE_URL', 'CONFLUENCE_CA_FILE', 'CONFLUENCE_USERNAME', 'CONFLUENCE_PASSWORD')) {
        if (-not [string]::IsNullOrEmpty([Environment]::GetEnvironmentVariable($key, 'User'))) {
            Fail "$key was not cleared when Confluence was disabled"
        }
    }

    foreach ($key in @('CONFLUENCE_CA_FILE', 'CONFLUENCE_USERNAME', 'CONFLUENCE_PASSWORD')) {
        [Environment]::SetEnvironmentVariable($key, "stale-$key", 'User')
    }
    Invoke-InstallerSuccess 'confluence-username-omitted-clears-credentials' @(
        '-Binary', (Join-Path $RepoRoot 'go.mod'),
        '-Agents', 'None',
        '-EnableConfluence',
        '-ConfluenceBaseUrl', 'https://confluence.internal.example.com/confluence'
    ) | Out-Null
    if ([Environment]::GetEnvironmentVariable('CONFLUENCE_BASE_URL', 'User') -ne 'https://confluence.internal.example.com/confluence') {
        Fail 'CONFLUENCE_BASE_URL was not refreshed when Confluence stayed enabled'
    }
    foreach ($key in @('CONFLUENCE_CA_FILE', 'CONFLUENCE_USERNAME', 'CONFLUENCE_PASSWORD')) {
        if (-not [string]::IsNullOrEmpty([Environment]::GetEnvironmentVariable($key, 'User'))) {
            Fail "$key was not cleared when omitted from Confluence reinstall"
        }
    }
}

function Test-AgentConfigEscapesBinaryPathForTomlAndJson {
    $caseDir = Join-Path $TmpRoot 'config-escaping'
    $installDir = Join-Path $caseDir 'install space\slash'
    $result = Invoke-InstallerSuccess 'config-escaping' @(
        '-Binary', (Join-Path $RepoRoot 'go.mod'),
        '-InstallDir', $installDir,
        '-Agents', 'Both',
        '-EnableJira',
        '-JiraBaseUrl', 'https://jira.internal.example.com/jira'
    )
    Assert-Contains (Join-Path $result.CaseDir 'project\.codex\config.toml') 'args = []'
    Assert-Contains (Join-Path $result.CaseDir 'project\.codex\config.toml') 'install space\\slash\\atlassian-mcp.exe'
    Assert-Contains (Join-Path $result.CaseDir 'project\.mcp.json') 'install space\\slash\\atlassian-mcp.exe'
}

function Test-ClaudeCliRegistersScopeLocalAndUser {
    foreach ($scope in @('User', 'Local')) {
        $result = Invoke-InstallerSuccess "claude-cli-$scope" @(
            '-Binary', (Join-Path $RepoRoot 'go.mod'),
            '-Scope', $scope,
            '-Agents', 'Claude',
            '-EnableJira',
            '-JiraBaseUrl', 'https://jira.internal.example.com/jira'
        )
        Assert-Contains $result.Log ("claude mcp add atlassian --scope {0} --" -f $scope.ToLowerInvariant())
        Assert-Contains $result.Log ("claude mcp get atlassian --scope {0}" -f $scope.ToLowerInvariant())
        if (Test-Path -LiteralPath (Join-Path $result.CaseDir 'home\.claude\settings.json')) {
            Fail "installer wrote a Claude settings file for scope $scope instead of using claude mcp add"
        }
        if (Test-Path -LiteralPath (Join-Path $result.CaseDir 'project\.mcp.json')) {
            Fail "installer wrote .mcp.json for scope $scope instead of using claude mcp add"
        }
    }
}

function Test-ClaudeCliMissingBinaryErrorsClearly {
    $fakeCaseDir = Join-Path $TmpRoot 'claude-cli-missing'
    $pathOnly = "$(Join-Path $fakeCaseDir 'bin');$env:SystemRoot\System32;$env:SystemRoot"
    $result = Invoke-InstallerCase 'claude-cli-missing' @(
        '-Binary', (Join-Path $RepoRoot 'go.mod'),
        '-Scope', 'User',
        '-Agents', 'Claude',
        '-EnableJira',
        '-JiraBaseUrl', 'https://jira.internal.example.com/jira'
    ) @{ REMOVE_FAKE_CLAUDE = '1'; PATH_VALUE = $pathOnly }
    if ($result.ExitCode -eq 0 -or (($result.Output -replace '\s+', ' ') -notmatch 'claude\s*CLI is required')) {
        Fail "missing claude CLI error did not mention requirement: $($result.Output)"
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
    $oldModuleAnalysisCache = $env:PSModuleAnalysisCachePath
    try {
        $env:PATH = "$(Join-Path $rollbackDir 'bin');$oldPath"
        $env:PATHEXT = ".PS1;$oldPathext"
        $env:HOME = Join-Path $rollbackDir 'home'
        $env:USERPROFILE = Join-Path $rollbackDir 'home'
        $env:FAKE_LOG = Join-Path $rollbackDir 'commands.log'
        # Match Invoke-InstallerCase: any PowerShell cache from this subprocess belongs in the test temp tree.
        $env:PSModuleAnalysisCachePath = Join-Path $rollbackDir 'ps-module-cache\ModuleAnalysisCache'
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
        $env:PSModuleAnalysisCachePath = $oldModuleAnalysisCache
    }
    Assert-Contains (Join-Path $rollbackDir 'project\.codex\config.toml') 'original config'
}

function Test-DryRunValidatesWithoutSideEffects {
    $result = Invoke-InstallerSuccess 'dry-run' @(
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
    Assert-Contains (Join-Path $RepoRoot 'README.md') '$INSTALLER_REF = ''main'''
    Assert-Contains (Join-Path $RepoRoot 'README.md') '-ReleaseTag v1.0.4'
    Assert-NotContains (Join-Path $RepoRoot 'README.md') '-SourceRepoUrl'
}

$PersistedEnvSnapshot = Save-PersistedEnv
try {
    New-Item -ItemType Directory -Force -Path $TmpRoot | Out-Null
    $tests = @(
        'Test-DefaultReleaseDownloadVerifiesAndInstallsWithoutSourceBuild',
        'Test-ReleaseTagPinsExactAsset',
        'Test-ChecksumMismatchFailsWithoutReplacingDestination',
        'Test-BinaryOverrideKeepsConfigBehaviorAndSkipsDownload',
        'Test-ServiceBaseUrlsRejectEmbeddedCredentials',
        'Test-ModuleAndCredentialValidation',
        'Test-ConfluencePersistedEnvClearsWhenDisabledOrCredentialsOmitted',
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
    Remove-RepoRootModuleAnalysisCache
}
