# Remove local build, test and tool-generated temporary directories.
[CmdletBinding()]
param([switch]$IncludeNodeModules)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$repo = Split-Path -Parent $PSScriptRoot
$targets = @(
    '.playwright-mcp',
    'release',
    'packaging/release',
    'app/.wails',
    'app/bin',
    'app/release',
    'app/frontend/dist',
    'app/frontend/test-results',
    'app/frontend/playwright-report'
)
if ($IncludeNodeModules) {
    $targets += 'app/frontend/node_modules'
}

$removed = [Collections.Generic.List[string]]::new()
foreach ($rel in $targets) {
    $full = Join-Path $repo $rel
    if (Test-Path -LiteralPath $full) {
        Remove-Item -LiteralPath $full -Recurse -Force
        $removed.Add($rel)
    }
}
foreach ($dir in @(Get-ChildItem -LiteralPath (Join-Path $repo 'app') -Directory -Filter 'release-*' -ErrorAction SilentlyContinue)) {
    Remove-Item -LiteralPath $dir.FullName -Recurse -Force
    $removed.Add("app/$($dir.Name)")
}

if ($removed.Count -eq 0) {
    Write-Output 'Workspace is already clean.'
} else {
    Write-Output "Cleaned: $($removed -join ', ')"
}
