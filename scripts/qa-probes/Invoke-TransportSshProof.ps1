# Automated G0 proof: local host -> public EC2 relay -> EC2 loopback viewer.
# Secrets travel only through private stdin and the authenticated SSH channel.
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [ValidatePattern('^[a-zA-Z0-9.-]+$')]
    [string]$Server,
    [Parameter(Mandatory = $true)]
    [string]$IdentityFile,
    [Parameter(Mandatory = $true)]
    [string]$KnownHostsFile,
    [Parameter(Mandatory = $true)]
    [string]$Executable,
    [Parameter(Mandatory = $true)]
    [ipaddress]$ClientAddress,
    [ValidatePattern('^[a-zA-Z0-9_.-]+$')]
    [string]$User = 'administrator',
    [ValidateRange(1, 1800)]
    [int]$DurationSeconds = 600,
    [ValidateRange(20, 100)]
    [int]$DataIntervalMilliseconds = 20
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$identity = (Resolve-Path -LiteralPath $IdentityFile).Path
$knownHosts = (Resolve-Path -LiteralPath $KnownHostsFile).Path
$exe = (Resolve-Path -LiteralPath $Executable).Path
$ssh = (Get-Command ssh.exe).Source
$scp = (Get-Command scp.exe).Source
$runID = [guid]::NewGuid().ToString('N')
$remoteDir = "C:/g0/crocstream-$runID"
$remoteExe = "$remoteDir/g0poc.exe"
$ruleName = "CrocStream-G0-$runID"
$clientIP = $ClientAddress.ToString()
if ($ClientAddress.AddressFamily -ne [Net.Sockets.AddressFamily]::InterNetwork) {
    throw 'An IPv4 SSH-client source address is required.'
}
$sshOptions = @('-i', $identity, '-o', 'IdentitiesOnly=yes', '-o', 'BatchMode=yes',
    '-o', 'StrictHostKeyChecking=yes', '-o', "UserKnownHostsFile=$knownHosts",
    '-o', 'ConnectTimeout=15', '-o', 'ServerAliveInterval=15',
    '-o', 'ServerAliveCountMax=3')
$target = "$User@$Server"
$children = [Collections.Generic.List[Diagnostics.Process]]::new()
$password = $null
$invite = $null

function Start-Child {
    param([string]$File, [string[]]$Arguments)
    # These arguments contain paths/options only, never passwords/invitations.
    $quoted = foreach ($arg in $Arguments) {
        if ($arg -match '["\r\n]' -or $arg.EndsWith('\')) {
            throw 'Unsupported native argument quoting.'
        }
        '"' + $arg + '"'
    }
    $psi = [Diagnostics.ProcessStartInfo]::new()
    $psi.FileName = $File
    $psi.Arguments = $quoted -join ' '
    $psi.UseShellExecute = $false
    $psi.CreateNoWindow = $true
    $psi.RedirectStandardInput = $true
    $psi.RedirectStandardOutput = $true
    $psi.RedirectStandardError = $true
    $p = [Diagnostics.Process]::new()
    $p.StartInfo = $psi
    if (!$p.Start()) { throw 'Could not start proof process.' }
    $children.Add($p)
    # Drain diagnostics privately. Never forward arbitrary upstream errors.
    $p | Add-Member -NotePropertyName ErrorDrain -NotePropertyValue $p.StandardError.ReadToEndAsync()
    return $p
}

function Send-Setup {
    param([Diagnostics.Process]$Process, [object]$Setup)
    $Process.StandardInput.WriteLine(($Setup | ConvertTo-Json -Compress -Depth 8))
    $Process.StandardInput.Close()
}

function Read-Line {
    param([Diagnostics.Process]$Process, [int]$TimeoutSeconds)
    $task = $Process.StandardOutput.ReadLineAsync()
    if (!$task.Wait($TimeoutSeconds * 1000)) { throw 'Proof output timed out.' }
    if ([string]::IsNullOrWhiteSpace($task.Result)) { throw 'Proof output missing.' }
    return $task.Result
}

function Finish-Child {
    param([Diagnostics.Process]$Process, [int]$TimeoutSeconds = 20)
    if (!$Process.WaitForExit($TimeoutSeconds * 1000)) { throw 'Proof process exit timed out.' }
    if ($Process.ExitCode -ne 0) { throw 'Proof process returned a failure exit code.' }
    if (!$Process.ErrorDrain.Wait(5000)) { throw 'Diagnostic drain timed out.' }
    # A race diagnostic must not be silently accepted even with a custom exit policy.
    if ($Process.ErrorDrain.Result -match 'DATA RACE') { throw 'Race detector reported a race.' }
}

function Invoke-Remote {
    param([string]$Script)
    $encoded = [Convert]::ToBase64String([Text.Encoding]::Unicode.GetBytes(
        "`$ErrorActionPreference='Stop'; `$ProgressPreference='SilentlyContinue';`n" + $Script))
    $p = Start-Child $ssh ($sshOptions + @($target, 'powershell.exe',
        '-NoProfile', '-NonInteractive', '-EncodedCommand', $encoded))
    $p.StandardInput.Close()
    $output = $p.StandardOutput.ReadToEndAsync()
    if (!$output.Wait(45000)) { throw 'Remote management command timed out.' }
    Finish-Child $p
    return $output.Result
}

try {
    $setupScript = @'
if (Get-NetTCPConnection -State Listen -LocalPort 9009 -ErrorAction SilentlyContinue) {
    throw 'Relay port is already in use; refusing to replace an existing service.'
}
New-Item -ItemType Directory -Path '__DIR__' -ErrorAction Stop | Out-Null
New-NetFirewallRule -Name '__RULE__' -DisplayName 'CrocStream G0 temporary relay' `
    -Direction Inbound -Action Allow -Protocol TCP -LocalPort 9009 `
    -RemoteAddress '__SOURCE__' -Profile Any | Out-Null
'@
    $null = Invoke-Remote ($setupScript.Replace('__DIR__', $remoteDir).
        Replace('__RULE__', $ruleName).Replace('__SOURCE__', $clientIP))

    $copy = Start-Child $scp ($sshOptions + @($exe, "${target}:$remoteExe"))
    $copy.StandardInput.Close()
    Finish-Child $copy 120
    $localHash = (Get-FileHash -LiteralPath $exe -Algorithm SHA256).Hash
    $remoteHash = (Invoke-Remote "(Get-FileHash -LiteralPath '$remoteExe' -Algorithm SHA256).Hash").Trim()
    if ($localHash -ne $remoteHash) { throw 'Transferred executable SHA-256 mismatch.' }

    $random = New-Object byte[] 32
    $rng = [Security.Cryptography.RandomNumberGenerator]::Create()
    try { $rng.GetBytes($random) } finally { $rng.Dispose() }
    $password = [Convert]::ToBase64String($random)
    [Array]::Clear($random, 0, $random.Length)
    $durationNS = [int64]$DurationSeconds * 1000000000
    $intervalNS = [int64]$DataIntervalMilliseconds * 1000000
    $relay = Start-Child $ssh ($sshOptions + @($target, $remoteExe, '-mode', 'relay'))
    Send-Setup $relay @{ Address = '0.0.0.0:9009'; Password = $password; Duration = $durationNS }
    $ready = (Read-Line $relay 20) | ConvertFrom-Json
    if ($ready.event -ne 'relay_ready') { throw 'Remote relay did not become ready.' }

    $hostPeer = Start-Child $exe @('-mode', 'host')
    Send-Setup $hostPeer @{ Address = "${Server}:9009"; Password = $password;
        Duration = $durationNS; SendInterval = $intervalNS }
    $invitation = (Read-Line $hostPeer 35) | ConvertFrom-Json
    $invite = $invitation.Invite
    if ([string]::IsNullOrWhiteSpace($invite)) { throw 'Host invitation missing.' }
    $invitation = $null
    $viewer = Start-Child $ssh ($sshOptions + @($target, $remoteExe, '-mode', 'viewer'))
    Send-Setup $viewer @{ Address = '127.0.0.1:9009'; Password = $password;
        Invite = $invite; Duration = $durationNS; SendInterval = $intervalNS }
    $invite = $null
    $password = $null
    Write-Output (@{ event = 'started'; route = 'local-host-public-ec2-relay-ec2-viewer';
        duration_seconds = $DurationSeconds; data_interval_ms = $DataIntervalMilliseconds;
        run_id = $runID; executable_sha256 = $localHash } |
        ConvertTo-Json -Compress)

    $hostResultTask = $hostPeer.StandardOutput.ReadLineAsync()
    $viewerResultTask = $viewer.StandardOutput.ReadLineAsync()
    if (!$hostResultTask.Wait(($DurationSeconds + 55) * 1000) -or
        !$viewerResultTask.Wait(20000)) { throw 'Peer result timed out.' }
    Finish-Child $hostPeer
    Finish-Child $viewer
    $results = @(($hostResultTask.Result | ConvertFrom-Json), ($viewerResultTask.Result | ConvertFrom-Json))
    $verifier = Start-Child $exe @('-mode', 'verify', '-duration', "${DurationSeconds}s")
    Send-Setup $verifier $results
    $verified = (Read-Line $verifier 60) | ConvertFrom-Json
    Finish-Child $verifier
    if ($verified.status -ne 'RESULTS_VERIFIED') { throw 'Independent result verification failed.' }
    foreach ($peerResult in $verified.results) {
        if ($peerResult.data_interval_ms -ne $DataIntervalMilliseconds) {
            throw 'Peer pacing differs from the requested diagnostic interval.'
        }
    }
    $status = if ($DurationSeconds -ge 600) { 'REMOTE_RELAY_PASS' } else { 'REMOTE_SMOKE_PASS' }
    Write-Output (@{ status = $status; route = 'local-host-public-ec2-relay-ec2-viewer';
        run_id = $runID; executable_sha256 = $localHash; results = $verified.results } |
        ConvertTo-Json -Compress -Depth 8)
} finally {
    $invite = $null
    $password = $null
    # Only terminate executables in this invocation's unique directory.
    $cleanupScript = @'
$owned = '__EXE__'.Replace('/', '\')
Get-CimInstance Win32_Process | Where-Object { $_.ExecutablePath -eq $owned } |
    ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction Stop }
Get-NetFirewallRule -Name '__RULE__' -ErrorAction SilentlyContinue |
    Remove-NetFirewallRule -ErrorAction Stop
if (Get-CimInstance Win32_Process | Where-Object { $_.ExecutablePath -eq $owned }) {
    throw 'Owned remote process remains.'
}
'@
    try {
        $null = Invoke-Remote ($cleanupScript.Replace('__EXE__', $remoteExe).Replace('__RULE__', $ruleName))
        Write-Output '{"event":"remote_cleanup_complete"}'
    } finally {
        foreach ($child in $children) {
            try {
                if (!$child.HasExited) { $child.Kill(); $child.WaitForExit() }
            } finally { $child.Dispose() }
        }
    }
}
