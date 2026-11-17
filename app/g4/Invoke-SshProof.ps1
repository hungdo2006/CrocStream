# Synthetic G4 media: local host -> authorized public EC2 relay -> EC2 viewer.
# The proof never instantiates a real capture source. Secrets use private stdin.
[CmdletBinding()]
param(
    [Parameter(Mandatory=$true)][ValidatePattern('^[a-zA-Z0-9.-]+$')][string]$Server,
    [Parameter(Mandatory=$true)][string]$IdentityFile,
    [Parameter(Mandatory=$true)][string]$KnownHostsFile,
    [Parameter(Mandatory=$true)][string]$Executable,
    [Parameter(Mandatory=$true)][ipaddress]$ClientAddress,
    # Reuse an already trusted key when the owner confirms the same instance's IP changed.
    [ValidatePattern('^[a-zA-Z0-9.-]+$')][string]$PinnedHostKeyAlias,
    [ValidatePattern('^[a-zA-Z0-9_.-]+$')][string]$User='administrator',
    [ValidateRange(5,1800)][int]$DurationSeconds=600
)
Set-StrictMode -Version Latest
$ErrorActionPreference='Stop'
$identity=(Resolve-Path -LiteralPath $IdentityFile).Path
$knownHosts=(Resolve-Path -LiteralPath $KnownHostsFile).Path
$exe=(Resolve-Path -LiteralPath $Executable).Path
$ssh=(Get-Command ssh.exe).Source
$scp=(Get-Command scp.exe).Source
$runID=[guid]::NewGuid().ToString('N')
$remoteDir="C:/g4/crocstream-$runID"
$remoteExe="$remoteDir/g4proof.exe"
$rule="CrocStream-G4-$runID"
$source=$ClientAddress.ToString()
if($ClientAddress.AddressFamily -ne [Net.Sockets.AddressFamily]::InterNetwork){throw 'IPv4 client source required.'}
$sshOptions=@('-i',$identity,'-o','IdentitiesOnly=yes','-o','BatchMode=yes','-o','StrictHostKeyChecking=yes',
    '-o',"UserKnownHostsFile=$knownHosts",'-o','ConnectTimeout=15','-o','ServerAliveInterval=15','-o','ServerAliveCountMax=3')
if($PinnedHostKeyAlias){$sshOptions+=@('-o',"HostKeyAlias=$PinnedHostKeyAlias")}
$target="$User@$Server"
$children=[Collections.Generic.List[Diagnostics.Process]]::new()
$password=$null
$invite=$null
$created=$false
function Start-Child {
    param([string]$File,[string[]]$Arguments)
    $quoted=foreach($argument in $Arguments){
        if($argument -match '["\r\n]' -or $argument.EndsWith('\')){throw 'Unsupported native argument quoting.'}
        '"'+$argument+'"'
    }
    $psi=[Diagnostics.ProcessStartInfo]::new()
    $psi.FileName=$File;$psi.Arguments=$quoted -join ' '
    $psi.UseShellExecute=$false;$psi.CreateNoWindow=$true
    $psi.RedirectStandardInput=$true;$psi.RedirectStandardOutput=$true;$psi.RedirectStandardError=$true
    $p=[Diagnostics.Process]::new();$p.StartInfo=$psi
    if(-not $p.Start()){throw 'Failed to start proof child.'}
    $children.Add($p)
    $p | Add-Member -NotePropertyName ErrorDrain -NotePropertyValue $p.StandardError.ReadToEndAsync()
    return $p
}
function Send-Setup {
    param([Diagnostics.Process]$Process,[object]$Setup)
    $Process.StandardInput.WriteLine(($Setup | ConvertTo-Json -Compress -Depth 8))
    $Process.StandardInput.Close()
}
function Read-Line {
    param([Diagnostics.Process]$Process,[int]$Seconds)
    $read=$Process.StandardOutput.ReadLineAsync()
    if(-not $read.Wait($Seconds*1000) -or [string]::IsNullOrWhiteSpace($read.Result)){throw 'Proof output absent/timed out.'}
    return $read.Result
}
function Finish-Child {
    param([Diagnostics.Process]$Process,[int]$Seconds=20)
    if(-not $Process.WaitForExit($Seconds*1000) -or $Process.ExitCode -ne 0){throw 'Proof process failed.'}
    if(-not $Process.ErrorDrain.Wait(5000) -or $Process.ErrorDrain.Result -match 'DATA RACE'){throw 'Peer diagnostic failure.'}
}
function Remote {
    param([string]$Script)
    $encoded=[Convert]::ToBase64String([Text.Encoding]::Unicode.GetBytes("`$ErrorActionPreference='Stop';`$ProgressPreference='SilentlyContinue';`n"+$Script))
    $p=Start-Child $ssh ($sshOptions+@($target,'powershell.exe','-NoProfile','-NonInteractive','-EncodedCommand',$encoded))
    $p.StandardInput.Close()
    $read=$p.StandardOutput.ReadToEndAsync()
    if(-not $read.Wait(45000)){throw 'Remote management timeout.'}
    Finish-Child $p
    return $read.Result
}
try {
    $setup=@'
if(Get-NetTCPConnection -State Listen -LocalPort 9009 -ErrorAction SilentlyContinue){throw 'Port 9009 occupied; refusing replacement.'}
New-Item -ItemType Directory -Path '__DIR__' -ErrorAction Stop | Out-Null
New-NetFirewallRule -Name '__RULE__' -DisplayName 'CrocStream G4 temporary relay' -Direction Inbound -Action Allow -Protocol TCP -LocalPort 9009 -RemoteAddress '__SOURCE__' -Profile Any | Out-Null
'@
    $created=$true
    # A partially completed setup also requires cleanup of these unique names.
    $null=Remote ($setup.Replace('__DIR__',$remoteDir).Replace('__RULE__',$rule).Replace('__SOURCE__',$source))
    $copy=Start-Child $scp ($sshOptions+@($exe,"${target}:$remoteExe"));$copy.StandardInput.Close();Finish-Child $copy 120
    $hash=(Get-FileHash -LiteralPath $exe -Algorithm SHA256).Hash
    $remoteHash=(Remote "(Get-FileHash -LiteralPath '$remoteExe' -Algorithm SHA256).Hash").Trim()
    if($hash -ne $remoteHash){throw 'Transferred executable hash mismatch.'}
    $random=New-Object byte[] 32
    $rng=[Security.Cryptography.RandomNumberGenerator]::Create()
    try{$rng.GetBytes($random)}finally{$rng.Dispose()}
    $password=[Convert]::ToBase64String($random);[Array]::Clear($random,0,$random.Length)
    $durationNS=[int64]$DurationSeconds*1000000000
    $relay=Start-Child $ssh ($sshOptions+@($target,$remoteExe,'-mode','relay'))
    Send-Setup $relay @{Address='0.0.0.0:9009';Password=$password;Duration=$durationNS}
    $ready=(Read-Line $relay 20)|ConvertFrom-Json
    if($ready.event -ne 'relay_ready'){throw 'Relay not ready.'}
    $hostPeer=Start-Child $exe @('-mode','host')
    Send-Setup $hostPeer @{Address="${Server}:9009";Password=$password;Duration=$durationNS}
    $invitation=(Read-Line $hostPeer 35)|ConvertFrom-Json
    $invite=$invitation.Invite
    if([string]::IsNullOrWhiteSpace($invite)){throw 'No invitation.'}
    $invitation=$null
    $viewer=Start-Child $ssh ($sshOptions+@($target,$remoteExe,'-mode','viewer'))
    Send-Setup $viewer @{Address='127.0.0.1:9009';Password=$password;Invite=$invite;Duration=$durationNS}
    $invite=$null;$password=$null
    Write-Output (@{event='started';run_id=$runID;duration_seconds=$DurationSeconds;executable_sha256=$hash;route='local-synthetic-host-public-ec2-relay-ec2-viewer'}|ConvertTo-Json -Compress)
    $hostRead=$hostPeer.StandardOutput.ReadLineAsync();$viewerRead=$viewer.StandardOutput.ReadLineAsync()
    if(-not $hostRead.Wait(($DurationSeconds+55)*1000) -or -not $viewerRead.Wait(20000)){throw 'Peer result timeout.'}
    Finish-Child $hostPeer;Finish-Child $viewer
    $results=@(($hostRead.Result|ConvertFrom-Json),($viewerRead.Result|ConvertFrom-Json))
    $verifier=Start-Child $exe @('-mode','verify','-duration',"${DurationSeconds}s")
    Send-Setup $verifier $results
    $verified=(Read-Line $verifier 30)|ConvertFrom-Json
    Finish-Child $verifier
    if($verified.status -ne 'RESULTS_VERIFIED'){throw 'Media verification failed.'}
    Write-Output (@{status='REMOTE_MEDIA_PASS';run_id=$runID;executable_sha256=$hash;duration_seconds=$DurationSeconds;results=$verified.results;capture='synthetic only'}|ConvertTo-Json -Compress -Depth 8)
} finally {
    $invite=$null;$password=$null
    if($created){
        $cleanup=@'
$owned='__EXE__'.Replace('/','\')
Get-CimInstance Win32_Process | Where-Object {$_.ExecutablePath -eq $owned} | ForEach-Object {Stop-Process -Id $_.ProcessId -Force -ErrorAction Stop}
Get-NetFirewallRule -Name '__RULE__' -ErrorAction SilentlyContinue | Remove-NetFirewallRule -ErrorAction Stop
if(Get-CimInstance Win32_Process | Where-Object {$_.ExecutablePath -eq $owned}){throw 'Owned remote process remains.'}
if(Get-NetFirewallRule -Name '__RULE__' -ErrorAction SilentlyContinue){throw 'Owned remote firewall rule remains.'}
'@
        try{$null=Remote ($cleanup.Replace('__EXE__',$remoteExe).Replace('__RULE__',$rule));Write-Output '{"event":"remote_cleanup_complete"}'}
        finally{foreach($p in $children){try{if(-not $p.HasExited){$p.Kill();$p.WaitForExit()}}finally{$p.Dispose()}}}
    }else{
        foreach($p in $children){try{if(-not $p.HasExited){$p.Kill();$p.WaitForExit()}}finally{$p.Dispose()}}
    }
}
