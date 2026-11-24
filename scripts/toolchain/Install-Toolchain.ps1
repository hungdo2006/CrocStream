# Owner approved portable G0 toolchain installation on 2026-11-15.
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [ValidateSet('Go', 'Wails')]
    [string]$Component,
    [string]$ToolRoot
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
if ([string]::IsNullOrWhiteSpace($ToolRoot)) {
    $ToolRoot = Join-Path $env:LOCALAPPDATA 'Temp\opencode\crocstream-g0'
}
$ToolRoot = [System.IO.Path]::GetFullPath($ToolRoot)
[void][System.IO.Directory]::CreateDirectory($ToolRoot)
$version = 'go1.27.4'
$installRoot = Join-Path $ToolRoot $version
$goExe = Join-Path $installRoot 'go\bin\go.exe'

if ($Component -eq 'Go') {
    if (Test-Path -LiteralPath $installRoot) {
        if (-not (Test-Path -LiteralPath $goExe)) {
            throw 'Existing toolchain directory is incomplete; inspect it before retrying.'
        }
        $actualVersion = & $goExe version
        if ($LASTEXITCODE -ne 0 -or $actualVersion -ne 'go version go1.27.4 windows/amd64') {
            throw 'Existing Go version does not match the approved toolchain.'
        }
        Write-Output "Already installed: $actualVersion"
        return
    }
    $releases = Invoke-RestMethod 'https://go.dev/dl/?mode=json&include=all' -TimeoutSec 60
    $release = @($releases | Where-Object { $_.version -eq $version -and $_.stable })
    if ($release.Count -ne 1) { throw 'Pinned stable Go release is not in official metadata.' }
    $archive = @($release[0].files | Where-Object {
        $_.filename -eq "$version.windows-amd64.zip" -and
        $_.os -eq 'windows' -and $_.arch -eq 'amd64' -and $_.kind -eq 'archive'
    })
    if ($archive.Count -ne 1 -or $archive[0].sha256 -notmatch '^[a-f0-9]{64}$') {
        throw 'Invalid archive metadata.'
    }
    $download = Join-Path $ToolRoot "$($archive[0].filename).$([Guid]::NewGuid().ToString('N')).download"
    Invoke-WebRequest -UseBasicParsing -Uri "https://go.dev/dl/$($archive[0].filename)" -OutFile $download -TimeoutSec 300
    $hash = (Get-FileHash -LiteralPath $download -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($hash -ne $archive[0].sha256 -or (Get-Item -LiteralPath $download).Length -ne $archive[0].size) {
        throw 'Go archive checksum/size mismatch; file retained for inspection, not executed.'
    }
    $zip = "$download.zip"
    Move-Item -LiteralPath $download -Destination $zip
    $staging = Join-Path $ToolRoot "$version-staging-$([Guid]::NewGuid().ToString('N'))"
    Expand-Archive -LiteralPath $zip -DestinationPath $staging
    Move-Item -LiteralPath $staging -Destination $installRoot
    Write-Output "Verified official archive SHA256=$hash"
    & $goExe version
    if ($LASTEXITCODE -ne 0) { throw 'Installed Go did not run successfully.' }
    return
}

. (Join-Path $PSScriptRoot 'Use-Toolchain.ps1') -ToolRoot $ToolRoot
$wailsExe = Join-Path $env:GOBIN 'wails3.exe'
if (Test-Path -LiteralPath $wailsExe) {
    throw 'wails3.exe already exists; inspect version before replacing it.'
}
# Version-qualified go install uses an isolated module, not croc/go.mod.
& $goExe install github.com/wailsapp/wails/v3/cmd/wails3@v3.1.0
if ($LASTEXITCODE -ne 0) { throw 'Wails CLI installation failed; inspect the command output.' }
& $goExe version -m $wailsExe
if ($LASTEXITCODE -ne 0) { throw 'Cannot inspect installed Wails CLI module provenance.' }
