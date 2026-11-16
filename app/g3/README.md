# G3 verification helpers

G3 captures real desktop pixels **only after explicit operator consent**.
UI proof presses Start; native lifecycle test requires a consent environment
variable. No helper saves screenshots, JPEGs, WebSocket payloads or tokens.

## Origin probe

`originprobe` is capture-free. It sends a WebSocket upgrade to a random
loopback observation path, records only its Origin, then closes the window.
Observe both embedded assets and dev-served assets. The probe endpoint never
upgrades to a media connection.

From `app/`, after `. .\g0\Use-Toolchain.ps1`:

```powershell
go build -tags production -buildvcs=false -o "$env:GOBIN\g3-origin-production.exe" ./g3/originprobe
& "$env:GOBIN\g3-origin-production.exe"
go build -buildvcs=false -o "$env:GOBIN\g3-origin-dev.exe" ./g3/originprobe
& "$env:GOBIN\g3-origin-dev.exe" -dev
```

## Actual WebView UI proof

Build the regular frontend first. A `g3test` build adds a test-only local CDP
port through Wails `WindowsOptions`; ordinary builds contain no such debug
configuration. `ui-proof.mjs` selects the primary/default display already shown
in UI, presses Start, observes numeric UI/process measurements, presses Stop,
checks canvas clearing and invokes native application Quit.

```powershell
go build -tags production,g3test -buildvcs=false -o "$env:GOBIN\crocstream-g3-ui-test.exe" .
node g3/ui-proof.mjs "$env:GOBIN\crocstream-g3-ui-test.exe" 600 packaged
# Close the native application while capture is still active:
node g3/ui-proof.mjs "$env:GOBIN\crocstream-g3-ui-test.exe" 3 packaged close-live
# Exercise the real Wails dev command and its own watcher process tree:
node g3/ui-proof.mjs "$env:GOBIN\wails3.exe" 5 dev-cli
```

The harness temporarily opens CDP on loopback for its own child process and
closes that process in `finally`. Diagnostics contain process-local Go heap,
allocation counters, goroutines, Win32 handles/GDI objects and working set/
private bytes. WebView subprocess memory is not included in those Go-process
numbers. UI FPS samples measure the actual canvas path, without storing pixels.

## Actual native lifecycle test

```powershell
$env:CROCSTREAM_CAPTURE_CONSENT='1'
go test -v -count=1 -timeout=120s ./internal/capture -run '^TestActualCapture100Lifecycles$'
Remove-Item Env:CROCSTREAM_CAPTURE_CONSENT
```

Normal `go test` skips actual display reads. Synthetic DIB allocation tests do
not require this variable. Tests never write image fixtures to disk.

## Notice collection

`Collect-Notices.ps1` records full license/notice text for linked Go modules
and installed npm packages, with SHA-256 inventory. It refuses overwrite.
Generated snapshots require review; a `package.json license field only` entry
is not equivalent to a complete binary redistribution notice.
