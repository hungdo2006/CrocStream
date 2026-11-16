# G0 tooling and build probe

These scripts support the owner-approved G0 investigation. The authenticated
duplex PoC is in the separate [poc module](poc/README.md); the tooling and
buildprobe below are not the streaming application.

## Portable tools

Run from `D:\CrocStream` using PowerShell:

```powershell
powershell -NoProfile -File app/g0/Install-Toolchain.ps1 -Component Go
powershell -NoProfile -File app/g0/Install-Toolchain.ps1 -Component Wails
. .\app\g0\Use-Toolchain.ps1
go version
wails3 doctor
```

Tools/cache/binaries go under
`$env:LOCALAPPDATA\Temp\opencode\crocstream-g0`, not into `croc/`. Go is pinned
to 1.27.4; the official Windows amd64 ZIP's size and SHA-256 are checked before
extraction. Wails CLI is installed as the version-qualified Go module
`github.com/wailsapp/wails/v3/cmd/wails3@v3.1.0`, with Go checksum verification.
No system-wide PATH changes or `go env -w` are made.

`Use-Toolchain.ps1` must be **dot-sourced** for settings to apply to subsequent
commands in that terminal. It sets process-local Go paths, temporary directories,
module proxy/checksum verification, `GOTOOLCHAIN=local`, `GOWORK=off` and
`GOFLAGS=-mod=readonly`. Start a new terminal to return to your usual environment.
It does not set or bypass PowerShell execution policy.

Repeated Go installation checks the existing version. Wails installation refuses
to overwrite an existing executable; inspect `go version -m "$env:GOBIN\wails3.exe"`
before deciding to replace it. The scripts do not silently delete partial downloads
or failed installations.

## Inventory

```powershell
powershell -NoProfile -File app/g0/Inspect-Environment.ps1 -VerifyUpstream
```

This checks the committed upstream tree, working-tree changes, tool locations
and optional GitHub commit metadata. A successful inventory is not a passed G0.
Raw baseline `Get-Command` may report no Go/Wails until the portable environment
is loaded; the inventory also lists their explicit portable paths.

## Upstream baseline

After loading the portable environment:

```powershell
go -C croc test -count=1 -timeout=120s ./src/codephrase ./src/pakekey ./src/crypt ./src/comm ./src/tcp
go -C croc test -race -count=1 -timeout=180s ./src/codephrase ./src/pakekey ./src/crypt ./src/comm ./src/tcp
go -C croc test -count=1 -timeout=180s ./...
go -C croc vet ./...
```

Check actual results in [the toolchain report](../G0_TOOLCHAIN.md); these commands
are reproducible checks, not claims they all pass. Upstream tests may have their
own Windows/environment limitations and fixed ports; run the network test jobs
sequentially. Full upstream CLI tests also emit fixture codes and exercise the
clipboard. Keep source unchanged, avoid retaining secret-bearing raw output,
check generated artifacts, and record failures.

## Isolated Go/Wails/croc link probe

`buildprobe/` is a **G0-only module**, separate from future production dependencies.
Its local `replace` uses the verified snapshot, not stock croc tag contents.
The program constructs Wails service options, type-checks the relay API and
checks a Windows-native GDI dependency path, without opening a window/network
connection or reading a display. Native operations copy only a synthetic 2×2
bitmap between memory DCs; real capture acceptance remains G3.

The first dependency resolution was performed explicitly with:

```powershell
$env:GOFLAGS = '-mod=mod'
go -C app/g0/buildprobe mod tidy
$env:GOFLAGS = '-mod=readonly'
```

After dependencies are resolved:

```powershell
go -C app/g0/buildprobe build -buildvcs=false -o "$env:GOBIN\g0-buildprobe.exe" .
& "$env:GOBIN\g0-buildprobe.exe"
go -C app/g0/buildprobe vet ./...
go -C app/g0/buildprobe test -count=1 ./...
go -C app/g0/buildprobe test -race -count=1 ./...
```

A successful build/run only establishes that those imports/signatures link on
this Windows environment. It does not validate Wails window lifecycle, bindings,
real screen capture, WebView2 rendering, protocol isolation or remote relay
operation. The synthetic GDI check is a native build/runtime feasibility probe,
not a screen-streaming implementation.

## G0 license inventory and final decision

See [license scope](licenses/README.md) and [closeout](../G0_CLOSEOUT.md).
`Audit-Licenses.ps1` requires a fresh output directory, does not install
dependencies, and refuses to overwrite an existing review snapshot.

## Design and next step

Read [G0_ADAPTER_DESIGN](../G0_ADAPTER_DESIGN.md) for the proposed purpose,
handshake, record envelope and acceptance matrix. Read
[G0_LOCAL_PROOF](../G0_LOCAL_PROOF.md) for implementation/test evidence.
