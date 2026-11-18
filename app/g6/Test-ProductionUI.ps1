# Drives the EXACT extracted production binaries via Windows UI Automation.
# No CDP, test API, screenshots, frame persistence or secret-bearing output.
[CmdletBinding()]
param([Parameter(Mandatory=$true)][string]$Directory)
Set-StrictMode -Version Latest
$ErrorActionPreference='Stop'
Add-Type -AssemblyName UIAutomationClient
Add-Type -AssemblyName System.Windows.Forms
Add-Type -TypeDefinition @'
using System;
using System.Runtime.InteropServices;
public static class CrocStreamG6Window {
    [DllImport("user32.dll")] public static extern bool SetForegroundWindow(IntPtr hWnd);
}
'@
$labels=[IO.File]::ReadAllText((Join-Path $PSScriptRoot 'ui-labels.json'))|ConvertFrom-Json
$root=(Resolve-Path -LiteralPath $Directory).Path
$phase='setup'
$apps=[Collections.Generic.List[Diagnostics.Process]]::new()
$relay=$null
$clipboardOwned=$false
$rng=[Security.Cryptography.RandomNumberGenerator]::Create()
$private=New-Object byte[] 32
$rng.GetBytes($private);$rng.Dispose()
$password=[Convert]::ToBase64String($private)
[Array]::Clear($private,0,$private.Length)
$children=[Collections.Generic.HashSet[int]]::new()

function Find-Element($window,[string]$property,[string]$value){
    $prop=if($property -eq 'id'){[Windows.Automation.AutomationElement]::AutomationIdProperty}else{[Windows.Automation.AutomationElement]::NameProperty}
    $condition=[Windows.Automation.PropertyCondition]::new($prop,$value)
    if($property -eq 'button'){$condition=[Windows.Automation.AndCondition]::new($condition,[Windows.Automation.PropertyCondition]::new([Windows.Automation.AutomationElement]::ControlTypeProperty,[Windows.Automation.ControlType]::Button))}
    return $window.FindFirst([Windows.Automation.TreeScope]::Descendants,$condition)
}
function Wait-Element($window,[string]$property,[string]$value,[int]$seconds=15){
    $end=[DateTime]::UtcNow.AddSeconds($seconds)
    do{$el=Find-Element $window $property $value;if($null -ne $el){return $el};Start-Sleep -Milliseconds 100}while([DateTime]::UtcNow -lt $end)
    throw 'Expected UI state did not appear.'
}
function Click-Element($el){
    $el.SetFocus()
    $invoke=[Windows.Automation.InvokePattern]$el.GetCurrentPattern([Windows.Automation.InvokePattern]::Pattern)
    $invoke.Invoke()
}
function Set-Input($window,[string]$id,[string]$value){
    $el=Wait-Element $window 'id' $id
    $pattern=[Windows.Automation.ValuePattern]$el.GetCurrentPattern([Windows.Automation.ValuePattern]::Pattern)
    $pattern.SetValue($value)
}
function Open-App {
    $p=Start-Process -FilePath (Join-Path $root 'crocstream.exe') -WorkingDirectory $root -PassThru
    $apps.Add($p)
    $end=[DateTime]::UtcNow.AddSeconds(20)
    do{$p.Refresh();if($p.MainWindowHandle -ne [IntPtr]::Zero){break};if($p.HasExited){throw 'Native app exited early'};Start-Sleep -Milliseconds 100}while([DateTime]::UtcNow -lt $end)
    if($p.MainWindowHandle -eq [IntPtr]::Zero){throw 'Native window did not appear'}
    return [Windows.Automation.AutomationElement]::FromHandle($p.MainWindowHandle)
}
function Read-RenderFPS($window){
    $all=$window.FindAll([Windows.Automation.TreeScope]::Descendants,[Windows.Automation.Condition]::TrueCondition)
    foreach($el in $all){
        if($el.Current.Name -match ('^'+[regex]::Escape($labels.render)+': (\d+)\.')){return [int]$Matches[1]}
    }
    return 0
}
try{
    & (Join-Path $root 'Verify-Package.ps1') -Directory $root
    $clipboard=[Windows.Forms.Clipboard]::GetDataObject()
    if($clipboard -and @($clipboard.GetFormats()).Count -gt 0){throw 'Clear the Windows clipboard before this opt-in UI test; it temporarily copies an invitation.'}
    $probe=[Net.Sockets.TcpListener]::new([Net.IPAddress]::Loopback,0)
    $probe.Start();$port=([Net.IPEndPoint]$probe.LocalEndpoint).Port;$probe.Stop()
    $address="127.0.0.1:$port"
    $info=[Diagnostics.ProcessStartInfo]::new()
    $info.FileName=Join-Path $root 'crocstream-relay.exe'
    $info.UseShellExecute=$false;$info.CreateNoWindow=$true
    $info.RedirectStandardInput=$true;$info.RedirectStandardOutput=$true;$info.RedirectStandardError=$true
    $relay=[Diagnostics.Process]::Start($info)
    $relay.StandardInput.WriteLine($address);$relay.StandardInput.WriteLine($password);$relay.StandardInput.Flush()
    $ready=$relay.StandardOutput.ReadLineAsync()
    if(-not $ready.Wait(5000) -or $ready.Result -notlike 'Relay ready*'){throw 'Relay readiness failed'}
    $phase='home'
    $hostWindow=Open-App
    $viewerWindow=Open-App
    Click-Element (Wait-Element $hostWindow 'button' $labels.share)
    Click-Element (Wait-Element $viewerWindow 'button' $labels.watch)
    $phase='host-start'
    Set-Input $hostWindow 'relay' $address
    Set-Input $hostWindow 'relay-password' $password
    Click-Element (Wait-Element $hostWindow 'button' $labels.start)
    $null=Wait-Element $hostWindow 'name' $labels.waiting
    $phase='copy-invitation'
    [void][CrocStreamG6Window]::SetForegroundWindow($apps[0].MainWindowHandle)
    Start-Sleep -Milliseconds 150
    [Windows.Forms.Clipboard]::Clear()
    $clipboardOwned=$true
    Click-Element (Wait-Element $hostWindow 'button' $labels.copy)
    $end=[DateTime]::UtcNow.AddSeconds(5)
    $code=''
    do{
        Start-Sleep -Milliseconds 100
        $code=[Windows.Forms.Clipboard]::GetText()
        # G0 adapter emits cs1- plus upstream GenerateSSH; do not parse its words.
    }while(((-not $code.StartsWith('cs1-')) -or $code.Length -gt 256) -and [DateTime]::UtcNow -lt $end)
    if(-not $code.StartsWith('cs1-') -or $code.Length -gt 256){throw 'Copy did not provide an invitation'}
    $phase='viewer-join'
    Set-Input $viewerWindow 'relay' $address
    Set-Input $viewerWindow 'relay-password' $password
    Set-Input $viewerWindow 'join-code' $code
    $code=''
    Click-Element (Wait-Element $viewerWindow 'button' $labels.join)
    $null=Wait-Element $viewerWindow 'name' $labels.watching 30
    $null=Wait-Element $hostWindow 'name' $labels.live
    $password=''
    $phase='production-render'
    $end=[DateTime]::UtcNow.AddSeconds(10)
    $fps=0
    do{$fps=Read-RenderFPS $viewerWindow;if($fps -gt 0){break};Start-Sleep -Milliseconds 200}while([DateTime]::UtcNow -lt $end)
    if($fps -le 0){throw 'Production viewer render counter did not advance'}
    $phase='production-stop'
    $stop=[Diagnostics.Stopwatch]::StartNew()
    Click-Element (Wait-Element $hostWindow 'button' $labels.stop)
    $null=Wait-Element $hostWindow 'name' $labels.idle 5
    $null=Wait-Element $viewerWindow 'name' $labels.disconnected 5
    $stop.Stop()
    if($stop.ElapsedMilliseconds -gt 3000){throw 'Production Stop exceeded normal target'}
    foreach($p in $apps){
        foreach($child in (Get-CimInstance Win32_Process|Where-Object {$_.ParentProcessId -eq $p.Id})){[void]$children.Add([int]$child.ProcessId)}
        if(-not $p.CloseMainWindow() -or -not $p.WaitForExit(10000)){throw 'Normal native close failed'}
    }
    $relay.StandardInput.Close()
    if(-not $relay.WaitForExit(5000)){throw 'Production relay did not stop'}
    foreach($id in $children){
        $end=[DateTime]::UtcNow.AddSeconds(5)
        while((Get-Process -Id $id -ErrorAction SilentlyContinue) -and [DateTime]::UtcNow -lt $end){Start-Sleep -Milliseconds 100}
        if(Get-Process -Id $id -ErrorAction SilentlyContinue){throw 'Native child process left behind'}
    }
    [ordered]@{status='PRODUCTION_TWO_APP_UI_PASS';observed_at_utc=[DateTime]::UtcNow.ToString('o');viewer_render_fps=$fps;stop_ms=$stop.ElapsedMilliseconds;debug_protocol=$false;capture_route='loopback only';binary_sha256=(Get-FileHash (Join-Path $root 'crocstream.exe')).Hash;children_closed=$children.Count}|ConvertTo-Json -Compress
}catch{
    [ordered]@{status='PRODUCTION_TWO_APP_UI_FAIL';phase=$phase;error_type=$_.Exception.GetType().Name}|ConvertTo-Json -Compress
    exit 1
}finally{
    $password='';$code=''
    if($clipboardOwned){[Windows.Forms.Clipboard]::Clear()}
    foreach($p in $apps){if(-not $p.HasExited){[void]$p.CloseMainWindow();if(-not $p.WaitForExit(5000)){$p.Kill();$p.WaitForExit()}};$p.Dispose()}
    if($relay){if(-not $relay.HasExited){$relay.StandardInput.Close();if(-not $relay.WaitForExit(5000)){$relay.Kill();$relay.WaitForExit()}};$relay.Dispose()}
}
