[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [ValidateSet('host', 'viewer')]
    [string]$Mode,
    [Parameter(Mandatory = $true)]
    [string]$Executable,
    [string]$RelayAddress,
    [string]$Invite,
    [ValidateRange(1, 30)]
    [int]$DurationMinutes = 10
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

function Read-SecretText {
    param([Parameter(Mandatory = $true)][string]$Prompt)
    $secure = Read-Host -Prompt $Prompt -AsSecureString
    $bstr = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($secure)
    try {
        return [Runtime.InteropServices.Marshal]::PtrToStringBSTR($bstr)
    } finally {
        [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($bstr)
    }
}

$exePath = (Resolve-Path -LiteralPath $Executable).Path
if ($Mode -eq 'host' -and [string]::IsNullOrWhiteSpace($RelayAddress)) {
    throw '-RelayAddress is required for host mode.'
}
if ($Mode -eq 'viewer' -and [string]::IsNullOrWhiteSpace($Invite)) {
    $Invite = Read-Host -Prompt 'Paste the private CrocStream invitation'
}
if ([string]::IsNullOrWhiteSpace($RelayAddress)) {
    $RelayAddress = '127.0.0.1:9009'
}

$relayPassword = Read-SecretText -Prompt 'Relay password (input is hidden)'
$durationNanoseconds = [int64]$DurationMinutes * 60 * 1000000000
$setup = [ordered]@{
    Address  = $RelayAddress
    Password = $relayPassword
    Invite   = $Invite
    Duration = $durationNanoseconds
}
$json = $setup | ConvertTo-Json -Compress
$psi = [Diagnostics.ProcessStartInfo]::new()
$psi.FileName = $exePath
$psi.Arguments = "-mode $Mode"
$psi.UseShellExecute = $false
$psi.RedirectStandardInput = $true
$psi.RedirectStandardOutput = $true
$psi.CreateNoWindow = $true
$process = [Diagnostics.Process]::new()
$process.StartInfo = $psi

try {
    if (-not $process.Start()) {
        throw 'Could not start the G0 peer process.'
    }
    $process.StandardInput.WriteLine($json)
    $process.StandardInput.Close()
    $json = $null
    $setup.Password = $null
    $setup.Invite = $null

    if ($Mode -eq 'host') {
        $line = $process.StandardOutput.ReadLine()
        if ([string]::IsNullOrWhiteSpace($line)) {
            throw 'Host did not return an invitation.'
        }
        $hostMessage = $line | ConvertFrom-Json
        if ([string]::IsNullOrWhiteSpace($hostMessage.Invite)) {
            throw 'Host returned an empty invitation.'
        }
        Write-Host ''
        Write-Host 'PRIVATE INVITATION (do not post in logs or chat):' -ForegroundColor Yellow
        Write-Host $hostMessage.Invite -ForegroundColor Yellow
        Write-Host ''
        Write-Host 'Copy it to the viewer machine. Waiting for the test result...' -ForegroundColor Cyan
    }

    while (($line = $process.StandardOutput.ReadLine()) -ne $null) {
        Write-Output $line
    }
    $process.WaitForExit()
    if ($process.ExitCode -ne 0) {
        throw "G0 peer exited with code $($process.ExitCode)."
    }
} finally {
    if ($process -and -not $process.HasExited) {
        $process.Kill()
        $process.WaitForExit()
    }
    if ($relayPassword) {
        $relayPassword = $null
    }
    if ($Invite) {
        $Invite = $null
    }
    if ($process) {
        $process.Dispose()
    }
}
