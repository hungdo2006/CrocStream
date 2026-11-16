# G0-only, conservative notice inventory for compiled Windows package closures.
# Generates data, never edits dependencies or accepts a missing module license.
[CmdletBinding()]
param([Parameter(Mandatory = $true)][string]$OutputDirectory)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$root = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$output = [IO.Path]::GetFullPath($OutputDirectory)
if (Test-Path -LiteralPath $output) { throw 'Output directory already exists; refusing to overwrite.' }
if ($env:GOOS -and $env:GOOS -ne 'windows') { throw 'This inventory is Windows-only.' }
$goRoot = (& go env GOROOT | Out-String).Trim()
if ($LASTEXITCODE -ne 0) { throw 'Go toolchain unavailable.' }

function Field {
    param($Object, [string]$Name)
    if ($null -eq $Object) { return $null }
    $property = $Object.PSObject.Properties[$Name]
    if ($null -ne $property) { return $property.Value }
    return $null
}

$packages = @{}
foreach ($target in @(@('app/g0/buildprobe', '.'), @('app/g0/poc', './cmd/g0poc'))) {
    foreach ($race in @($false, $true)) {
        $arguments = @('-C', (Join-Path $root $target[0]), 'list', '-buildvcs=false', '-deps')
        if ($race) { $arguments += '-race' }
        $format = '{{.ImportPath}}|{{.Dir}}|{{with .Module}}{{.Path}}{{end}}|{{with .Module}}{{.Version}}{{end}}|{{with .Module}}{{.Dir}}{{end}}|{{with .Module}}{{.Main}}{{end}}|{{range .GoFiles}}{{.}};{{end}}{{range .CgoFiles}}{{.}};{{end}}{{range .SFiles}}{{.}};{{end}}|{{range .EmbedFiles}}{{.}};{{end}}'
        $raw = @(& go @arguments -f $format $target[1])
        if ($LASTEXITCODE -ne 0) { throw 'Compiled package inventory failed.' }
        foreach ($line in $raw) {
            $fields = $line -split '\|', 8
            if ($fields.Count -ne 8) { throw 'Unexpected Go package inventory format.' }
            $module = $null
            if ($fields[2]) {
                $module = [pscustomobject]@{ Path=$fields[2]; Version=$fields[3];
                    Dir=$fields[4]; Main=($fields[5] -eq 'true') }
            }
            $pkg = [pscustomobject]@{ ImportPath=$fields[0]; Dir=$fields[1]; Module=$module;
                GoFiles=@($fields[6] -split ';' | Where-Object { $_ });
                EmbedFiles=@($fields[7] -split ';' | Where-Object { $_ }) }
            if ($packages.ContainsKey($pkg.ImportPath)) {
                $previous = $packages[$pkg.ImportPath]
                if ($previous.Dir -ne $pkg.Dir) { throw 'One import path resolved to multiple directories.' }
                $pkg.GoFiles = @(($previous.GoFiles + $pkg.GoFiles) | Sort-Object -Unique)
                $pkg.EmbedFiles = @(($previous.EmbedFiles + $pkg.EmbedFiles) | Sort-Object -Unique)
            }
            $packages[$pkg.ImportPath] = $pkg
        }
    }
}

$modules = @{}
$documents = @{}
$inline = @{}
$embeds = [Collections.Generic.List[object]]::new()
function Add-Document {
    param([string]$Path, [string]$Origin)
    $full = (Resolve-Path -LiteralPath $Path).Path
    if (!$documents.ContainsKey($full)) {
        $documents[$full] = [pscustomobject]@{ path=$full; origin=$Origin }
    }
}
function Is-NoticeName {
    param([string]$Name)
    return $Name -match '^(LICENSE|LICENCE|COPYING|NOTICE|AUTHORS|PATENTS|COPYRIGHT)([._-].*)?$'
}

foreach ($pkg in $packages.Values) {
    $module = Field $pkg 'Module'
    $isMain = $null -ne $module -and (Field $module 'Main')
    if ($isMain) { continue }
    $base = $goRoot
    $owner = 'Go-standard-library'
    if ($null -ne $module) {
        $base = $module.Dir
        $owner = $module.Path + '@' + $module.Version
        $modules[$owner] = [pscustomobject]@{ path=$module.Path; version=$module.Version }
        $rootNotices = @(Get-ChildItem -LiteralPath $base -File | Where-Object { Is-NoticeName $_.Name })
        if (!($rootNotices | Where-Object { $_.Name -match '^(LICENSE|LICENCE|COPYING)' })) {
            throw "Missing module root license: $owner"
        }
    }
    $base = [IO.Path]::GetFullPath($base).TrimEnd('\')
    $directories = [Collections.Generic.HashSet[string]]::new([StringComparer]::OrdinalIgnoreCase)
    $null = $directories.Add($pkg.Dir)
    foreach ($embed in @(Field $pkg 'EmbedFiles')) {
        if (!$embed) { continue }
        $full = Join-Path $pkg.Dir $embed
        $embeds.Add([pscustomobject]@{ package=$pkg.ImportPath; file=$embed;
            sha256=(Get-FileHash -LiteralPath $full -Algorithm SHA256).Hash })
        $null = $directories.Add((Split-Path $full))
    }
    foreach ($directory in $directories) {
        $cursor = [IO.Path]::GetFullPath($directory)
        while ($cursor -eq $base -or $cursor.StartsWith($base + '\', [StringComparison]::OrdinalIgnoreCase)) {
            foreach ($file in Get-ChildItem -LiteralPath $cursor -File) {
                if (Is-NoticeName $file.Name) {
                    Add-Document $file.FullName ($owner + '/' + $file.FullName.Substring($base.Length + 1).Replace('\','/'))
                }
            }
            if ($cursor -eq $base) { break }
            $cursor = Split-Path $cursor
        }
    }
    # Retain inline notices in selected non-standard source, not just LICENSE at root.
    if ($null -ne $module) {
        foreach ($name in @((Field $pkg 'GoFiles')) + @((Field $pkg 'CgoFiles')) + @((Field $pkg 'SFiles'))) {
            if (!$name) { continue }
            $path = Join-Path $pkg.Dir $name
            $text = [IO.File]::ReadAllText($path)
            $blocks = [regex]::Matches($text, '(?s)/\*.*?\*/|(?m)(?:^[ \t]*//[^\r\n]*\r?\n)+')
            foreach ($block in $blocks) {
                if ($block.Value -notmatch '(?i)copyright|SPDX-License-Identifier|permission is hereby|redistribution and use') { continue }
                $origin = $owner + '/' + $path.Substring($base.Length + 1).Replace('\','/')
                $key = $origin + ':' + $block.Index
                $inline[$key] = [pscustomobject]@{ origin=$origin; text=$block.Value }
            }
        }
    }
}

# Sources of special-case attributions that are not package ancestor LICENSEs.
$wails = Join-Path $env:GOMODCACHE 'github.com/wailsapp/wails/v3@v3.1.0'
foreach ($extra in @(
    @('croc/src/codephrase/wordlists/README.md', 'EFF-wordlist-attribution'),
    @('app/g0/poc/internal/upstreampake/LICENSE', 'modified-upstream-schedule-MIT'),
    @('app/g0/poc/internal/upstreampake/UPSTREAM.md', 'modified-upstream-schedule-provenance')
)) { Add-Document (Join-Path $root $extra[0]) $extra[1] }
Add-Document (Join-Path $goRoot 'src/runtime/race/README') 'Go-race-runtime-provenance'
$nanoid = Join-Path $wails 'internal/runtime/desktop/@wailsio/runtime/src/nanoid.ts'
$nanoidText = [IO.File]::ReadAllText($nanoid)
$header = [regex]::Match($nanoidText, '\A(?:[ \t]*//[^\r\n]*\r?\n|[ \t]*\r?\n)+').Value
if ($header -notmatch 'Copyright 2017 Andrey Sitnik') { throw 'Nanoid notice source changed; review needed.' }
$inline['Wails-embedded-nanoid'] = [pscustomobject]@{ origin='Wails embedded modified nanoid'; text=$header }

New-Item -ItemType Directory -Path $output -ErrorAction Stop | Out-Null
$utf8 = [Text.UTF8Encoding]::new($false)
$records = [Collections.Generic.List[object]]::new()
$bundle = [Text.StringBuilder]::new()
$null = $bundle.AppendLine('CrocStream G0 third-party notices; compiled package closure, ordinary and race Windows builds.')
$null = $bundle.AppendLine('Conservative inventory, not a final desktop release legal opinion. Keep these notices with G0 binaries.')
foreach ($document in $documents.Values | Sort-Object origin) {
    $null = $bundle.AppendLine("`n===== $($document.origin) =====")
    $null = $bundle.AppendLine([IO.File]::ReadAllText($document.path))
    $records.Add([pscustomobject]@{ origin=$document.origin;
        sha256=(Get-FileHash -LiteralPath $document.path -Algorithm SHA256).Hash })
}
foreach ($notice in $inline.Values | Sort-Object origin, text) {
    $null = $bundle.AppendLine("`n===== Inline attribution: $($notice.origin) =====")
    $null = $bundle.AppendLine($notice.text)
}

# License text only: no package installation, executable or remote code execution.
$external = @(
    @('oxc-runtime-MIT', 'https://unpkg.com/@oxc-project/runtime@0.155.0/LICENSE'),
    @('Babel-helper-attribution-MIT', 'https://raw.githubusercontent.com/babel/babel/main/LICENSE'),
    @('Rolldown-generated-runtime-MIT', 'https://unpkg.com/rolldown@1.2.15/LICENSE'),
    @('Chromium-screenmanager-BSD', 'https://raw.githubusercontent.com/chromium/chromium/main/LICENSE'),
    @('Winc-attribution-MIT', 'https://raw.githubusercontent.com/tadvi/winc/master/LICENSE'),
    @('W32-attribution-BSD', 'https://raw.githubusercontent.com/gonutz/w32/master/LICENSE'),
    @('clipboard-attribution-BSD', 'https://raw.githubusercontent.com/atotto/clipboard/master/LICENSE'),
    @('LLVM-race-runtime', 'https://raw.githubusercontent.com/llvm/llvm-project/51bfeff0e4b0757ff773da6882f4d538996c9b04/compiler-rt/LICENSE.TXT')
)
foreach ($entry in $external) {
    $file = Join-Path $output ($entry[0] + '.txt')
    Invoke-WebRequest -UseBasicParsing -Uri $entry[1] -OutFile $file -TimeoutSec 30
    if ((Get-Item -LiteralPath $file).Length -gt 262144) { throw 'Unexpectedly large license response.' }
    $text = [IO.File]::ReadAllText($file)
    if ($text -notmatch 'Permission|Redistribution|Apache License') { throw 'Unrecognized license response; manual review required.' }
    $null = $bundle.AppendLine("`n===== $($entry[0]); source $($entry[1]) =====")
    $null = $bundle.AppendLine($text)
    $records.Add([pscustomobject]@{ origin=$entry[0]; url=$entry[1];
        sha256=(Get-FileHash -LiteralPath $file -Algorithm SHA256).Hash })
}
[IO.File]::WriteAllText((Join-Path $output 'THIRD_PARTY_NOTICES.txt'), $bundle.ToString(), $utf8)
$manifest = [ordered]@{ generated_utc=[datetime]::UtcNow.ToString('o'); scope='G0 Windows ordinary/race package closures';
    modules=@($modules.Values | Sort-Object path); notices=@($records);
    inline_notice_count=$inline.Count; embeds=@($embeds | Sort-Object package,file) }
[IO.File]::WriteAllText((Join-Path $output 'inventory.json'), ($manifest | ConvertTo-Json -Depth 10), $utf8)
[pscustomobject]@{ status='INVENTORY_GENERATED_REVIEW_REQUIRED'; modules=$modules.Count;
    notice_files=$records.Count; inline_notices=$inline.Count; embedded_files=$embeds.Count;
    output=$output } | ConvertTo-Json -Compress
