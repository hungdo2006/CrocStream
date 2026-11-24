# Collect dependency source notices for the linked Windows application and
# installed frontend packages; no screen pixels, runtime tokens or credentials.
[CmdletBinding()]
param([string]$OutputDirectory = '../docs/archive-gates/licenses/g3/reviewed-20261116')
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$repo = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$root = Join-Path $repo 'app'
$output = [IO.Path]::GetFullPath((Join-Path $root $OutputDirectory))
if (Test-Path -LiteralPath $output) { throw 'Refusing to overwrite an existing notice snapshot.' }
. (Join-Path $repo 'scripts/toolchain/Use-Toolchain.ps1')
# Go emits concatenated JSON objects; use its template to enumerate module roots.
$modules = & go -C $root list -buildvcs=false -deps -f '{{if .Module}}{{.Module.Path}}|{{.Module.Version}}|{{.Module.Dir}}{{end}}' .
if ($LASTEXITCODE -ne 0) { throw 'Unable to list dependency modules.' }
$records = [Collections.Generic.List[object]]::new()
$text = [Text.StringBuilder]::new()
foreach ($line in ($modules | Where-Object { $_ -and -not $_.StartsWith('crocstream.local/app|') } | Sort-Object -Unique)) {
    $parts = $line -split '\|', 3
    $source = $parts[2]
    $notices = @(Get-ChildItem -LiteralPath $source -Recurse -File |
        Where-Object { $_.Name -match '^(LICENSE|LICENCE|COPYING|NOTICE)([._-].*)?$' })
    if ($notices.Count -eq 0) { throw "Missing license: $($parts[0])" }
    foreach ($notice in $notices) {
        $relative = $notice.FullName.Substring($source.Length).TrimStart('\','/')
        $hash = (Get-FileHash -LiteralPath $notice.FullName -Algorithm SHA256).Hash
        $records.Add([pscustomobject]@{ ecosystem='go'; name=$parts[0]; version=$parts[1]; file=$relative; sha256=$hash })
        [void]$text.AppendLine("=== $($parts[0])@$($parts[1]) / $relative ===")
        [void]$text.AppendLine([IO.File]::ReadAllText($notice.FullName))
    }
}
$paths = & node -e "const fs=require('fs'); console.log(JSON.stringify(Object.keys(JSON.parse(fs.readFileSync(process.argv[1],'utf8')).packages).filter(Boolean)))" (Join-Path $root 'frontend/package-lock.json') | ConvertFrom-Json
foreach ($packagePath in $paths) {
    $source = Join-Path $root ("frontend/" + $packagePath)
    if (-not (Test-Path -LiteralPath $source)) { continue } # Optional non-Windows packages.
    $package = Get-Content -LiteralPath (Join-Path $source 'package.json') -Raw | ConvertFrom-Json
    $notices = @(Get-ChildItem -LiteralPath $source -File |
        Where-Object { $_.Name -match '^(LICENSE|LICENCE|COPYING|NOTICE)([._-].*)?$' })
    foreach ($notice in $notices) {
        $hash = (Get-FileHash -LiteralPath $notice.FullName -Algorithm SHA256).Hash
        $records.Add([pscustomobject]@{ ecosystem='npm'; name=$package.name; version=$package.version; file=$notice.Name; sha256=$hash })
        [void]$text.AppendLine("=== $($package.name)@$($package.version) / $($notice.Name) ===")
        [void]$text.AppendLine([IO.File]::ReadAllText($notice.FullName))
    }
    if ($notices.Count -eq 0) {
        $extra = @()
        switch ($package.name) {
            '@wailsio/runtime' { $extra = @((Join-Path $env:GOMODCACHE 'github.com/wailsapp/wails/v3@v3.1.0/LICENSE')) }
            '@humanfs/types' { $extra = @((Join-Path $source 'README.md'), (Join-Path $root 'frontend/node_modules/@humanfs/core/LICENSE')) }
            '@rolldown/binding-win32-x64-msvc' { $extra = @((Join-Path $root 'frontend/node_modules/rolldown/LICENSE')) }
            'esrecurse' { $extra = @((Join-Path $source 'esrecurse.js')) }
            'imurmurhash' { $extra = @((Join-Path $source 'README.md')) }
            'natural-compare' { $extra = @((Join-Path $source 'README.md'), (Join-Path $source 'index.js')) }
        }
        if ($extra.Count -eq 0) { throw "Unreviewed missing notice: $($package.name)" }
        foreach ($file in $extra) {
            $records.Add([pscustomobject]@{ ecosystem='npm-supplement'; name=$package.name; version=$package.version; file=[IO.Path]::GetFileName($file); source=$file.Replace($root,'app').Replace($env:GOMODCACHE,'GOMODCACHE'); sha256=(Get-FileHash $file -Algorithm SHA256).Hash })
            [void]$text.AppendLine("=== $($package.name)@$($package.version) / supplemental $([IO.Path]::GetFileName($file)) ===")
            [void]$text.AppendLine([IO.File]::ReadAllText($file))
        }
    }
}
foreach ($file in @('callable.js','nanoid.js','utils.js')) {
    $source = Join-Path $root ("frontend/node_modules/@wailsio/runtime/dist/" + $file)
    $records.Add([pscustomobject]@{ ecosystem='npm-inline'; name='@wailsio/runtime'; version='3.1.0'; file=$file; sha256=(Get-FileHash $source -Algorithm SHA256).Hash })
    [void]$text.AppendLine("=== @wailsio/runtime@3.1.0 / dist/$file (inline notices/source) ===")
    [void]$text.AppendLine([IO.File]::ReadAllText($source))
}
[void][IO.Directory]::CreateDirectory($output)
[IO.File]::WriteAllText((Join-Path $output 'THIRD_PARTY_NOTICES.txt'), $text.ToString(), [Text.UTF8Encoding]::new($false))
[IO.File]::WriteAllText((Join-Path $output 'inventory.json'), ($records | ConvertTo-Json -Depth 6), [Text.UTF8Encoding]::new($false))
"Collected $($records.Count) notice records; review scope and supplemental provenance before redistribution."
