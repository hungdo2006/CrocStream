# Read-only G0 inventory. This is not a transport feasibility test.
[CmdletBinding()]
param(
    [string]$RepoRoot,
    [switch]$VerifyUpstream,
    [string]$ToolRoot
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
if ([string]::IsNullOrWhiteSpace($RepoRoot)) {
    $RepoRoot = Join-Path $PSScriptRoot '..\..'
}
$root = (Resolve-Path -LiteralPath $RepoRoot).Path
if ([string]::IsNullOrWhiteSpace($ToolRoot)) {
    $ToolRoot = Join-Path $env:LOCALAPPDATA 'Temp\opencode\crocstream-g0'
}
$git = Get-Command git -CommandType Application -ErrorAction Stop
$pin = '552f40fd2d7593316118370bec0a93478d48e715'
$expectedTree = '0e401898d47eaef76e62263483943e3aa789ad3a'

function Read-Git {
    param([string[]]$GitArguments)
    # Trust only this invocation; do not modify the user's global Git config.
    $result = @(& $git.Source -c "safe.directory=$root" -C $root @GitArguments)
    if ($LASTEXITCODE -ne 0) {
        throw "Git inventory command failed: $($GitArguments -join ' ')"
    }
    return $result
}

if (-not (Test-Path -LiteralPath (Join-Path $root 'croc\go.mod'))) {
    throw 'Expected the CrocStream repository with croc/go.mod.'
}

$head = (Read-Git -GitArguments @('rev-parse', 'HEAD')) -join ''
$tree = (Read-Git -GitArguments @('rev-parse', 'HEAD:croc')) -join ''
$changes = @(Read-Git -GitArguments @('status', '--porcelain', '--untracked-files=all', '--', 'croc/'))
$goMod = [System.IO.File]::ReadAllText((Join-Path $root 'croc\go.mod'))
$goMinimum = [regex]::Match($goMod, '(?m)^go\s+(\S+)').Groups[1].Value

$tools = @(
    foreach ($name in @('go', 'wails3', 'node', 'npm', 'git', 'gcc', 'clang')) {
        $command = Get-Command $name -ErrorAction SilentlyContinue | Select-Object -First 1
        [pscustomobject]@{
            Name = $name
            FoundOnPath = ($null -ne $command)
            Source = $(if ($null -ne $command) { $command.Source } else { $null })
        }
    }
)

$commonPaths = @(
    foreach ($path in @(
        'C:\Program Files\Go\bin\go.exe',
        'C:\Go\bin\go.exe',
        (Join-Path $env:USERPROFILE 'go\bin\wails3.exe')
    )) {
        [pscustomobject]@{ Path = $path; Exists = (Test-Path -LiteralPath $path) }
    }
)

$upstream = [ordered]@{
    Status = 'NOT RUN (use -VerifyUpstream for read-only GitHub lookup)'
    URL = "https://api.github.com/repos/schollz/croc/git/commits/$pin"
    Tree = $null
}
if ($VerifyUpstream) {
    try {
        $response = Invoke-RestMethod -Uri $upstream.URL -TimeoutSec 30 -Headers @{
            Accept = 'application/vnd.github+json'
            'User-Agent' = 'CrocStream-G0-inventory'
        }
        $upstream.Tree = $response.tree.sha
        $upstream.Status = $(if ($response.sha -eq $pin -and $response.tree.sha -eq $expectedTree) {
            'MATCH'
        } else {
            'MISMATCH'
        })
    } catch {
        # Do not include an arbitrary server response or environment values.
        $upstream.Status = 'BLOCKED (GitHub lookup failed)'
    }
}

[pscustomobject]@{
    TimestampUTC = [DateTime]::UtcNow.ToString('o')
    OS = [System.Runtime.InteropServices.RuntimeInformation]::OSDescription
    Architecture = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString()
    PowerShell = $PSVersionTable.PSVersion.ToString()
    RepositoryHead = $head
    UpstreamCommit = $pin
    ExpectedUpstreamTree = $expectedTree
    RecordedSnapshotTree = $tree
    SnapshotTreeMatches = ($tree -eq $expectedTree)
    UpstreamWorkingTreeClean = ($changes.Count -eq 0)
    UpstreamChanges = $changes
    NestedCrocGit = (Test-Path -LiteralPath (Join-Path $root 'croc\.git'))
    GoMinimumFromSnapshot = $goMinimum
    Tools = $tools
    CommonToolPaths = $commonPaths
    PortableTools = @(
        foreach ($relative in @('go1.27.4\go\bin\go.exe', 'bin\wails3.exe')) {
            $path = Join-Path $ToolRoot $relative
            [pscustomobject]@{ Path = $path; Exists = (Test-Path -LiteralPath $path) }
        }
    )
    UpstreamLookup = $upstream
    Scope = 'Inventory only; no install, build, peer handshake or relay test.'
} | ConvertTo-Json -Depth 6

if ($tree -ne $expectedTree -or $changes.Count -ne 0 -or $upstream.Status -eq 'MISMATCH') {
    throw 'Snapshot provenance check failed; inspect the inventory before proceeding.'
}
if ($VerifyUpstream -and $upstream.Status -ne 'MATCH') {
    throw 'Online provenance verification is blocked; do not count this as a pass.'
}
