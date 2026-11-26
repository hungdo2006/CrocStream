[CmdletBinding()]
param([Parameter(Mandatory=$true)][string]$Directory)
Set-StrictMode -Version Latest
$ErrorActionPreference='Stop'
$root=(Resolve-Path -LiteralPath $Directory).Path
$manifest=Get-Content -LiteralPath (Join-Path $root 'manifest.json') -Raw | ConvertFrom-Json
if($manifest.format -ne 1 -or $manifest.platform -ne 'windows/amd64'){throw 'Invalid package manifest.'}
$required=@('crocstream.exe','crocstream-relay.exe','Start-DemoRelay.ps1','README.md','DEMO.md','KNOWN_ISSUES.md','NOTICE.txt',
    'licenses/THIRD_PARTY_NOTICES.txt','licenses/notice-inventory.json','licenses/G0_ATTRIBUTIONS.txt','licenses/GO_LICENSE.txt','licenses/NOTICE_SCOPE.md',
    'build-info.txt','dependencies.json','Verify-Package.ps1')
$allowed=@($required)+@('relay.json')
$declared=@($manifest.files | ForEach-Object {$_.path})
if($declared.Count -ne @($declared | Sort-Object -Unique).Count){throw 'Duplicate file path in manifest.'}
foreach($req in $required){
    if($req -notin $declared){throw "Missing required manifest entry: $req"}
}
foreach($file in $manifest.files){
    if($file.path -notin $allowed){throw 'Invalid file path in manifest.'}
    $path=Join-Path $root $file.path
    if(-not(Test-Path -LiteralPath $path -PathType Leaf)){throw "Missing package file: $($file.path)"}
    $info=Get-Item -LiteralPath $path
    if($info.Length -ne $file.bytes -or (Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash -ne $file.sha256){
        throw "Package checksum mismatch: $($file.path)"
    }
}
$actual=@(Get-ChildItem -LiteralPath $root -File -Recurse | ForEach-Object {$_.FullName.Substring($root.Length+1).Replace('\','/')})
if(@(Compare-Object (@($declared)+@('manifest.json')|Sort-Object) ($actual|Sort-Object)).Count){throw 'Unexpected file in release directory.'}
'PACKAGE_VERIFIED'
