# Local/manual demo; no secrets in arguments, logs or files.
[CmdletBinding()]
param([string]$ListenAddress='127.0.0.1:9009')
$ErrorActionPreference='Stop'
$exe=Join-Path $PSScriptRoot 'crocstream-relay.exe'
if(-not(Test-Path -LiteralPath $exe)){throw 'crocstream-relay.exe is missing beside this script.'}
Write-Host "Relay listen address: $ListenAddress"
$secret=Read-Host 'Relay password (at least 12 characters; enter the same in both apps)' -AsSecureString
$ptr=[Runtime.InteropServices.Marshal]::SecureStringToBSTR($secret)
$process=$null
try {
    $plain=[Runtime.InteropServices.Marshal]::PtrToStringBSTR($ptr)
    if($plain.Length -lt 12 -or $plain.Length -gt 1024 -or $plain -match '[\r\n]' -or $plain.Trim() -ne $plain){throw 'Invalid password length or whitespace.'}
    $info=New-Object Diagnostics.ProcessStartInfo
    $info.FileName=$exe
    $info.UseShellExecute=$false
    $info.RedirectStandardInput=$true
    $process=[Diagnostics.Process]::Start($info)
    $process.StandardInput.WriteLine($ListenAddress)
    $process.StandardInput.WriteLine($plain)
    $process.StandardInput.Flush()
    $plain=$null
    [void](Read-Host 'Press Enter to stop the relay')
} finally {
    [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($ptr)
    $secret.Dispose()
    $plain=$null
    if($process){
        try{$process.StandardInput.Close()}catch{}
        if(-not $process.WaitForExit(3000)){$process.Kill();$process.WaitForExit()}
        $process.Dispose()
    }
}
