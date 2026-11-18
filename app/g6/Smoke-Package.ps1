# Runtime smoke for an extracted portable ZIP. Does not click Start/capture.
[CmdletBinding()]
param([Parameter(Mandatory=$true)][string]$Directory)
Set-StrictMode -Version Latest
$ErrorActionPreference='Stop'
$root=(Resolve-Path -LiteralPath $Directory).Path
& (Join-Path $root 'Verify-Package.ps1') -Directory $root
$app=Join-Path $root 'crocstream.exe'
$sig=Get-AuthenticodeSignature -LiteralPath $app
if($sig.Status -ne 'NotSigned'){throw 'Expected the documented unsigned prototype.'}
$p=Start-Process -FilePath $app -WorkingDirectory $root -PassThru
$children=@()
try {
    $deadline=[DateTime]::UtcNow.AddSeconds(30)
    do {
        $p.Refresh()
        if($p.HasExited){throw 'Packaged app exited during startup.'}
        if($p.MainWindowHandle -ne [IntPtr]::Zero){break}
        Start-Sleep -Milliseconds 200
    } while([DateTime]::UtcNow -lt $deadline)
    if($p.MainWindowHandle -eq [IntPtr]::Zero){throw 'Packaged app window did not appear.'}
    $children=@(Get-CimInstance Win32_Process | Where-Object {$_.ParentProcessId -eq $p.Id} | ForEach-Object {$_.ProcessId})
    if(-not $p.CloseMainWindow()){throw 'Could not request normal window close.'}
    if(-not $p.WaitForExit(10000)){throw 'Packaged app did not exit after normal window close.'}
} finally {
    $p.Dispose()
}
foreach($id in $children){
    $deadline=[DateTime]::UtcNow.AddSeconds(5)
    while([DateTime]::UtcNow -lt $deadline -and (Get-Process -Id $id -ErrorAction SilentlyContinue)){Start-Sleep -Milliseconds 100}
    if(Get-Process -Id $id -ErrorAction SilentlyContinue){throw 'An app-owned child process survived shutdown.'}
}

# Verify the packaged helper rejects public/wildcard binds and serves loopback.
function Start-Relay([string]$address,[string]$password){
    $info=[Diagnostics.ProcessStartInfo]::new()
    $info.FileName=Join-Path $root 'crocstream-relay.exe'
    $info.WorkingDirectory=$root
    $info.UseShellExecute=$false
    $info.CreateNoWindow=$true
    $info.RedirectStandardInput=$true
    $info.RedirectStandardOutput=$true
    $info.RedirectStandardError=$true
    $child=[Diagnostics.Process]::Start($info)
    $child.StandardInput.WriteLine($address)
    $child.StandardInput.WriteLine($password)
    $child.StandardInput.Flush()
    return $child
}
$bad=Start-Relay '0.0.0.0:9009' 'smoke-only-private-password'
try {
    if(-not $bad.WaitForExit(5000)){throw 'Relay accepted a wildcard bind.'}
    if($bad.ExitCode -eq 0){throw 'Public/wildcard bind validation failed.'}
    $null=$bad.StandardOutput.ReadToEnd();$null=$bad.StandardError.ReadToEnd()
} finally {if(-not $bad.HasExited){$bad.Kill();$bad.WaitForExit()};$bad.Dispose()}

$probe=[Net.Sockets.TcpListener]::new([Net.IPAddress]::Loopback,0)
$probe.Start();$port=([Net.IPEndPoint]$probe.LocalEndpoint).Port;$probe.Stop()
$address="127.0.0.1:$port"
$relay=Start-Relay $address 'smoke-only-private-password'
$ready=$false
try {
    $deadline=[DateTime]::UtcNow.AddSeconds(5)
    while([DateTime]::UtcNow -lt $deadline -and -not $relay.HasExited){
        $client=[Net.Sockets.TcpClient]::new()
        try{$task=$client.ConnectAsync([Net.IPAddress]::Loopback,$port);if($task.Wait(150)){$ready=$true;break}}catch{}
        finally{$client.Dispose()}
        Start-Sleep -Milliseconds 50
    }
    if(-not $ready){throw 'Packaged relay did not bind the requested loopback port.'}
    $relay.StandardInput.Close()
    if(-not $relay.WaitForExit(5000)){throw 'Relay did not stop when its launcher closed stdin.'}
    $null=$relay.StandardOutput.ReadToEnd();$null=$relay.StandardError.ReadToEnd()
} finally {if(-not $relay.HasExited){$relay.Kill();$relay.WaitForExit()};$relay.Dispose()}
$check=[Net.Sockets.TcpClient]::new()
 $connected=$false
try{$task=$check.ConnectAsync([Net.IPAddress]::Loopback,$port);$connected=$task.Wait(300)}catch{}
finally{$check.Dispose()}
if($connected){throw 'Relay listener survived shutdown.'}
'PORTABLE_RUNTIME_SMOKE_PASS: window started/closed; relay loopback/wildcard/EOF checks; unsigned as documented; no capture performed.'
