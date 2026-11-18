[CmdletBinding()]
param(
    [string]$ToolRoot,
    [switch]$Offline,
    [switch]$Race
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

function Invoke-Check([string]$Program, [string[]]$Arguments) {
    $timer = [Diagnostics.Stopwatch]::StartNew()
    $command = "$Program $($Arguments -join ' ')"
    Write-Host "> $command"
    & $Program @Arguments
    $code = $LASTEXITCODE
    Write-Host ('Exit {0} after {1:N1}s' -f $code, $timer.Elapsed.TotalSeconds)
    if ($code -ne 0) {
        throw "Verification failed ($code) running $command"
    }
}

$repo = Split-Path -Parent $PSScriptRoot
Push-Location -LiteralPath $repo
try {
    . .\app\g0\Use-Toolchain.ps1 -ToolRoot $ToolRoot
    Remove-Item Env:CROCSTREAM_CAPTURE_CONSENT -ErrorAction SilentlyContinue
    $env:WAILS_EXP_USE_WAKE = '1'
    if ($Offline) {
        $env:GOPROXY = 'off'
        $env:npm_config_offline = 'true'
    }
    Write-Host 'Verification uses synthetic media. Native capture consent is disabled.'
    Invoke-Check go @('version')
    Invoke-Check node @('--version')
    Invoke-Check npm.cmd @('--version')
    Invoke-Check wails3 @('version')

    foreach ($module in @('app', 'app/g0/poc', 'app/g0/buildprobe')) {
        Invoke-Check go @('-C', $module, 'test', '-count=1', '-timeout=240s', './...')
        Invoke-Check go @('-C', $module, 'vet', './...')
        Invoke-Check go @('-C', $module, 'mod', 'verify')
        if ($Race) {
            Invoke-Check go @('-C', $module, 'test', '-race', '-count=1', '-timeout=600s', './...')
        }
    }

    Push-Location -LiteralPath app
    $previousCGO = [Environment]::GetEnvironmentVariable('CGO_ENABLED')
    try {
        $env:CGO_ENABLED = '0'
        # Wails installs from the lockfile, generates bindings, and builds the frontend.
        Invoke-Check wails3 @('build')
        Invoke-Check go @('version', '-m', '.\bin\crocstream.exe')
    } finally {
        [Environment]::SetEnvironmentVariable('CGO_ENABLED', $previousCGO, 'Process')
        Pop-Location
    }

    foreach ($check in @('typecheck', 'lint', 'test', 'test:ui')) {
        Invoke-Check npm.cmd @('--prefix', 'app/frontend', 'run', $check)
    }
} finally {
    Pop-Location
}
