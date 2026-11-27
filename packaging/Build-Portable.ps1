# Reproducible portable-package recipe. Installs nothing and refuses overwrite.
[CmdletBinding()]
param([string]$Version='0.0.4',[string]$OutputDirectory='release',[string]$RelayConfig='')
Set-StrictMode -Version Latest
$ErrorActionPreference='Stop'
if($Version -notmatch '^\d+\.\d+\.\d+$'){throw 'Version must be numeric x.y.z.'}
$repo=Split-Path -Parent $PSScriptRoot
$appRoot=Join-Path $repo 'app'
$manifestVersion=(Select-String -LiteralPath (Join-Path $appRoot 'wails.yaml') -Pattern '^  version: (.+)$').Matches.Groups[1].Value
if($Version -ne $manifestVersion){throw 'Package version must match wails.yaml.'}
. (Join-Path $repo 'scripts/toolchain/Use-Toolchain.ps1')
$output=if([IO.Path]::IsPathRooted($OutputDirectory)){[IO.Path]::GetFullPath($OutputDirectory)}else{[IO.Path]::GetFullPath((Join-Path $appRoot $OutputDirectory))}
$name="CrocStream-$Version-windows-amd64"
$stage=Join-Path $output $name
$archive=Join-Path $output "$name.zip"
foreach($path in @($stage,$archive,"$archive.sha256")){if(Test-Path -LiteralPath $path){throw 'Release destination exists; choose a fresh OutputDirectory.'}}
Push-Location $appRoot
try {
    $env:WAILS_EXP_USE_WAKE='1'
    # Clear GOFLAGS' caller-supplied build tags: release must not carry test APIs/CDP.
    $env:GOFLAGS='-mod=readonly'
    & wails3 build
    if($LASTEXITCODE -ne 0){throw 'Windows build failed.'}
    $binary=Join-Path $appRoot 'bin/crocstream.exe'
    $binaryInfo=(& go version -m $binary) -join "`n"
    if($LASTEXITCODE -ne 0 -or $binaryInfo -match 'g3test|g4test|-race=true' -or $binaryInfo -notmatch '-tags=production'){
        throw 'Release build tags are not production-only.'
    }
    $relay=Join-Path $env:GOBIN 'crocstream-g6-relay.exe'
    & go build -trimpath -buildvcs=false -ldflags '-s -w' -o $relay ./cmd/demo-relay
    if($LASTEXITCODE -ne 0){throw 'Demo relay build failed.'}
    $noticeRelative="release/$name-notice-review"
    $noticeRoot=Join-Path $appRoot $noticeRelative
    if(-not(Test-Path -LiteralPath $noticeRoot)){
        & (Join-Path $repo 'scripts/notices/Collect-Notices.ps1') -OutputDirectory $noticeRelative
    }
    [void][IO.Directory]::CreateDirectory((Join-Path $stage 'licenses'))
    Copy-Item -LiteralPath $binary -Destination (Join-Path $stage 'crocstream.exe')
    Copy-Item -LiteralPath $relay -Destination (Join-Path $stage 'crocstream-relay.exe')
    foreach($file in @('Start-DemoRelay.ps1','Verify-Package.ps1','README.md','DEMO.md','KNOWN_ISSUES.md','NOTICE.txt')){
        Copy-Item -LiteralPath (Join-Path $PSScriptRoot $file) -Destination (Join-Path $stage $file)
    }
    if(-not [string]::IsNullOrWhiteSpace($RelayConfig)){
        $relayConfigPath=if([IO.Path]::IsPathRooted($RelayConfig)){[IO.Path]::GetFullPath($RelayConfig)}else{[IO.Path]::GetFullPath((Join-Path $repo $RelayConfig))}
        if(-not(Test-Path -LiteralPath $relayConfigPath -PathType Leaf)){throw "Relay config file not found: $relayConfigPath"}
        $relayInfo=Get-Item -LiteralPath $relayConfigPath
        if($relayInfo.Length -gt 4096){throw 'Relay config exceeds maximum size.'}
        $relayObj=Get-Content -LiteralPath $relayConfigPath -Raw | ConvertFrom-Json
        $relayProps=@($relayObj.PSObject.Properties.Name | Sort-Object)
        if(@(Compare-Object $relayProps @('address','password')).Count -or [string]::IsNullOrWhiteSpace($relayObj.address) -or [string]::IsNullOrWhiteSpace($relayObj.password)){
            throw 'Relay config JSON must contain only non-empty address and password fields.'
        }
        Copy-Item -LiteralPath $relayConfigPath -Destination (Join-Path $stage 'relay.json')
    }
    Copy-Item -LiteralPath (Join-Path $noticeRoot 'THIRD_PARTY_NOTICES.txt') -Destination (Join-Path $stage 'licenses/THIRD_PARTY_NOTICES.txt')
    Copy-Item -LiteralPath (Join-Path $noticeRoot 'inventory.json') -Destination (Join-Path $stage 'licenses/notice-inventory.json')
    Copy-Item -LiteralPath (Join-Path $repo 'docs/archive-gates/licenses/g0/reviewed-20261116/THIRD_PARTY_NOTICES.txt') -Destination (Join-Path $stage 'licenses/G0_ATTRIBUTIONS.txt')
    $goLicense=Join-Path $env:GOROOT 'LICENSE'
    if(-not(Test-Path -LiteralPath $goLicense)){throw 'Go runtime LICENSE missing.'}
    Copy-Item -LiteralPath $goLicense -Destination (Join-Path $stage 'licenses/GO_LICENSE.txt')
    Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'NOTICE_SCOPE.md') -Destination (Join-Path $stage 'licenses/NOTICE_SCOPE.md')
    $utf8=[Text.UTF8Encoding]::new($false)
    [IO.File]::WriteAllText((Join-Path $stage 'build-info.txt'),($binaryInfo+"`n"+((& go version -m $relay)-join "`n")), $utf8)
    $deps=& go list -buildvcs=false -deps -f '{{if .Module}}{{.Module.Path}}|{{.Module.Version}}{{end}}' . ./cmd/demo-relay
    if($LASTEXITCODE -ne 0){throw 'Cannot inventory linked dependencies.'}
    $modules=@($deps|Where-Object {$_}|Sort-Object -Unique|ForEach-Object {$p=$_ -split '\|',2;[ordered]@{path=$p[0];version=$p[1]}})
    $frontendJson=& node (Join-Path $PSScriptRoot 'runtime-deps.mjs') (Join-Path $appRoot 'frontend/package-lock.json')
    if($LASTEXITCODE -ne 0){throw 'Cannot read frontend runtime pins.'}
    $frontend=@($frontendJson|ConvertFrom-Json)
    if(@($frontend|Where-Object {-not $_.version}).Count){throw 'A frontend runtime pin is missing.'}
    $dependencyInfo=[ordered]@{go_modules=$modules;frontend_runtime=$frontend;upstream_commit='552f40fd2d7593316118370bec0a93478d48e715';notice_scope='conservative superset; development tools not distributed'}
    [IO.File]::WriteAllText((Join-Path $stage 'dependencies.json'),($dependencyInfo|ConvertTo-Json -Depth 6),$utf8)
    $files=@(Get-ChildItem -LiteralPath $stage -File -Recurse | Sort-Object FullName | ForEach-Object {
        [ordered]@{path=$_.FullName.Substring($stage.Length+1).Replace('\','/');bytes=$_.Length;sha256=(Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash}
    })
    $head=& git -c "safe.directory=$($repo.Replace('\','/'))" -C $repo rev-parse HEAD
    $package=[ordered]@{format=1;product='CrocStream';version=$Version;platform='windows/amd64';created_at_utc=[DateTime]::UtcNow.ToString('o');
        source_head=$head;source_state='working tree G6; not an assertion of committed sources';signed=$false;files=$files}
    [IO.File]::WriteAllText((Join-Path $stage 'manifest.json'),($package|ConvertTo-Json -Depth 6),$utf8)
    & (Join-Path $stage 'Verify-Package.ps1') -Directory $stage
    Compress-Archive -LiteralPath $stage -DestinationPath $archive -CompressionLevel Optimal
    $hash=(Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash
    [IO.File]::WriteAllText("$archive.sha256","$hash  $name.zip`n",$utf8)
    [ordered]@{status='PORTABLE_PACKAGE_BUILT';archive=$archive;sha256=$hash;bytes=(Get-Item $archive).Length}|ConvertTo-Json -Compress
} finally {Pop-Location}
