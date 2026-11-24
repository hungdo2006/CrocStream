# Regression checks for this reviewed G0 dependency/notice snapshot.
[CmdletBinding()]
param([Parameter(Mandatory = $true)][string]$Directory)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$dir = (Resolve-Path -LiteralPath $Directory).Path
$inventory = Get-Content -LiteralPath (Join-Path $dir 'inventory.json') -Raw | ConvertFrom-Json
$bundlePath = Join-Path $dir 'THIRD_PARTY_NOTICES.txt'
$bundle = [IO.File]::ReadAllText($bundlePath)
if ($inventory.modules.Count -ne 21 -or $inventory.embeds.Count -ne 21 -or
    $inventory.notices.Count -ne 52 -or $inventory.inline_notice_count -ne 135) {
    throw 'Dependency or notice scope changed; review and update the expected snapshot.'
}
foreach ($module in $inventory.modules) {
    $pattern = '^' + [regex]::Escape($module.path + '@' + $module.version + '/') +
        '(LICENSE|LICENCE|COPYING)[^/]*$'
    if (!($inventory.notices | Where-Object { $_.origin -match $pattern })) {
        throw "Root license missing for $($module.path)"
    }
}
foreach ($text in @(
    'Creative Commons Attribution 4.0 International Public License',
    'Copyright 2017 Andrey Sitnik', 'ISC License (ISC)',
    'Winc-attribution-MIT', 'W32-attribution-BSD', 'clipboard-attribution-BSD',
    'LLVM Exceptions to the Apache 2.0 License', 'modified-upstream-schedule-provenance',
    'golang.org/x/sys@v0.50.0/windows/race0.go'
)) {
    if (!$bundle.Contains($text)) { throw "Required attribution missing: $text" }
}
foreach ($notice in $inventory.notices | Where-Object { $_.PSObject.Properties['url'] }) {
    $saved = Join-Path $dir ($notice.origin + '.txt')
    if ((Get-FileHash -LiteralPath $saved -Algorithm SHA256).Hash -ne $notice.sha256) {
        throw 'Downloaded license fingerprint differs.'
    }
}
$before = (Get-FileHash -LiteralPath $bundlePath -Algorithm SHA256).Hash
try {
    & (Join-Path $PSScriptRoot 'Audit-Licenses.ps1') -OutputDirectory $dir
    throw 'Audit unexpectedly accepted an existing directory.'
} catch {
    if ($_.Exception.Message -notlike 'Output directory already exists*') { throw }
}
if ((Get-FileHash -LiteralPath $bundlePath -Algorithm SHA256).Hash -ne $before) {
    throw 'Overwrite guard changed an existing bundle.'
}
Write-Output 'PASS: G0 notice closure, nested attributions, ordinary/race coverage, downloaded hashes and overwrite guard.'
