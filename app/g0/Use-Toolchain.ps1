# Dot-source to configure only the current PowerShell process.
[CmdletBinding()]
param([string]$ToolRoot)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
if ([string]::IsNullOrWhiteSpace($ToolRoot)) {
    $ToolRoot = Join-Path $env:LOCALAPPDATA 'Temp\opencode\crocstream-g0'
}
$ToolRoot = [System.IO.Path]::GetFullPath($ToolRoot)
$goRoot = Join-Path $ToolRoot 'go1.27.4\go'
if (-not (Test-Path -LiteralPath (Join-Path $goRoot 'bin\go.exe'))) {
    throw 'Portable Go is missing. Run Install-Toolchain.ps1 -Component Go first.'
}
$env:GOROOT = $goRoot
$env:GOPATH = Join-Path $ToolRoot 'gopath'
$env:GOMODCACHE = Join-Path $ToolRoot 'gomodcache'
$env:GOCACHE = Join-Path $ToolRoot 'gocache'
$env:GOBIN = Join-Path $ToolRoot 'bin'
$env:GOTOOLCHAIN = 'local'
$env:GOWORK = 'off'
$env:GOENV = 'off'
$env:GOFLAGS = '-mod=readonly'
$env:GOPROXY = 'https://proxy.golang.org'
$env:GOSUMDB = 'sum.golang.org'
$env:GOPRIVATE = ''
$env:GONOSUMDB = ''
$env:GONOPROXY = ''
$env:TEMP = Join-Path $ToolRoot 'tmp'
$env:TMP = $env:TEMP
foreach ($path in @($env:GOPATH, $env:GOMODCACHE, $env:GOCACHE, $env:GOBIN, $env:TEMP)) {
    [void][System.IO.Directory]::CreateDirectory($path)
}
$env:PATH = "$(Join-Path $goRoot 'bin');$env:GOBIN;$env:PATH"
