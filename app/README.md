# CrocStream application core

`go.mod` defines the pure-Go application module, `crocstream.local/app`.
G1 implements media framing/configuration in `internal/protocol`. G2 adds
`internal/frame`, `internal/delta` and `internal/telemetry`, with the approved
LZ4-frame dependency pinned in `go.mod` / `go.sum`.
The isolated G0 modules in `g0/buildprobe` and
`g0/poc` remain separate from application package checks.

Gate evidence: [G0 closeout](G0_CLOSEOUT.md), [G1 closeout](G1_CLOSEOUT.md),
[G2 closeout](G2_CLOSEOUT.md), [G3 closeout](G3_CLOSEOUT.md),
[G4 closeout](G4_CLOSEOUT.md). Dependency notice: [THIRD_PARTY_NOTICES](THIRD_PARTY_NOTICES.md).
Wire contract: [media protocol v1](../docs/ARCHITECTURE.md#media-protocol-v1).

## Run checks

From the repository root, using the existing portable toolchain:

```powershell
. .\app\g0\Use-Toolchain.ps1
go -C app test -count=1 -timeout=180s ./...
go -C app test -race -count=1 -timeout=600s ./...
go -C app vet ./...
go -C app build ./...
```

Run each fuzz target separately:

```powershell
foreach ($target in @('FuzzDecodeHeader', 'FuzzDecodeConfig', 'FuzzDecodePacket', 'FuzzReadStream')) {
    go -C app test ./internal/protocol -run '^$' -fuzz "^${target}$" -fuzztime=30s -parallel=2
    if ($LASTEXITCODE -ne 0) { throw "Fuzz failed: $target" }
}
```

The executable now provides G3 local preview and G4 Share/Watch; remote G4
synthetic acceptance passed on the measured EC2 route. See build/run instructions below; `go build ./...` alone
does not regenerate frontend assets or Wails bindings.

## Protocol API and ownership

- `EncodeHeader` / `DecodeHeader`: exact 18-byte BigEndian envelope. These allow
  the header-only zero-length KEY fixture; `Header.Validate` enforces valid
  packet lengths, IDs, control timestamps and direction.
- `EncodeConfig` / `DecodeConfig`: exact 11-byte CONFIG. Supported pairs are
  640×360 and 1280×720; key interval 1–120, default 30. `Config.Limits` gives
  checked raw-image and decompressed tile-record budgets.
- `DecodePacket`: exactly one in-memory packet, with no trailing bytes. It
  borrows the input payload; it does not enforce stream ordering.
- `NewReader(input, direction)` / `ReadPacket`: one reader per incoming stream;
  validates header/session before bounded allocation. Returned payload belongs
  to the caller and is not reused by subsequent reads.
- After each video packet, the codec must validate the complete LZ4 frame,
  decompressed size and record before `ResolveVideo(true)`. On codec failure,
  retain the previous image and call `ResolveVideo(false)`, then request KEY
  under the session's 1/s PLI limit. Reading ahead is rejected until resolution.
- `ErrNeedKeyframe` means the reader consumed one structurally bounded video
  packet that could not continue the reference sequence. It discards using
  4-KiB scratch space, without decoding or allocating the advertised payload.
  Request a rate-limited KEY and keep reading. Replay protection tracks the
  highest consumed video ID independently of the last successfully decoded ID.
- Other wire/I/O errors are terminal and sticky. END is delivered once, then
  `io.EOF`; clean transport EOF also terminates. Never scan for another magic
  byte or reuse a reader after disconnect.
- `NewWriter(output, direction)` / `WritePacket`: synchronous, single owner.
  A session-owned serialized writer must order bounded control/video queues.
  Packet validation occurs before I/O; state commits after the entire write.
  Any I/O error terminates the writer/session; never retry a partial packet.
  The payload is borrowed only until the call returns.
- `NextFrameID` rejects uint32 exhaustion instead of wrapping. The session must
  end and authenticate anew, with fresh CONFIG/KEY1 and reader/writer instances.

Direction names describe the **sender** of the half-stream: broadcaster writer
and viewer reader use `FromBroadcaster`; viewer writer and broadcaster reader
use `FromViewer`. CONFIG is the first broadcaster packet, with END permitted
for early abort. PING/PONG can follow CONFIG before video; the first video is
always KEY1. Reverse controls need no reverse CONFIG.

Reader/Writer own no socket or goroutine. The session owns `context.Context`,
closes the duplex connection or applies deadlines to unblock I/O, serializes
access and handles END in either direction by stopping the whole session.
Closing only one half is not application-level shutdown.

G2 supplies bounded LZ4 decoding, canonical reference bytes, tile validation
and telemetry (below). G3 supplies Windows capture, local bridge and the
React/Wails preview; G4 implements the actual secure transport integration, PLI
limiting, Ping/Pong timing, serialized queues and session-wide lifecycle.
The G1 tests use synthetic opaque payloads and explicit mock codec acceptance;
they do not claim successful media decoding.

## G2 frame and codec API

`frame.Normalize(ctx, config, src, stride, frame.RGBA|frame.BGRA)` validates
stride/bounds, copies rows, skips padding, swaps channels if needed, and forces
alpha=255. It expects straight, top-down pixel samples; capture must explicitly
adapt premultiplied or bottom-up native buffers. Returned pixels are owned by
the caller.

Create `delta.NewEncoder(config, tau)` and `delta.NewDecoder(config)` once per
session. Supported tau is an integer 0–255 (`delta.DefaultTau=4`); tau is local
encoder policy, not an additional CONFIG field.

Sender:

1. Session writes CONFIG via `protocol.Writer`.
2. Pass a normalized sample to `encoder.Emit(ctx, writer, rgba, timestampMs)`.
   Only sample admission occurs before encoding; never drop an encoded delta.
3. Emit selects scheduled KEY IDs `1, 1+interval, ...`, forced KEY from
   `RequestKeyframe()`, or KEY when dirty fraction is **strictly above 65%**.
   Extra keys do not shift the periodic schedule.
4. Compare each tile's RGB MAE with the **last transmitted canonical image**.
   DELTA changes only selected tiles; KEY replaces the image losslessly.
5. Successful synchronous write commits the image and ID. Any writer error
   permanently terminates that encoder; create new state only after a new
   authenticated session. Cancellation before writing consumes no frame ID.
   The session remains responsible for closing the transport on blocked I/O.
6. The returned `Measurement` reports full packet bytes, measured baseline
   bytes, raw record size, encode elapsed time and baseline elapsed time.
   `DirtyTilesKnown=false` on scheduled/PLI/initial KEY because no comparison
   was needed. No fabricated dirty count should be presented.

Receiver:

1. Read and validate CONFIG; construct the decoder from that exact config.
2. Pass each video packet to `decoder.Decode(ctx, packet)`.
3. Call `reader.ResolveVideo(err == nil)` after Decode. On a decode failure,
   retain the last good displayed pixels and request a rate-limited KEY.
4. If the reader returns `ErrNeedKeyframe` before reaching the decoder, also
   call `decoder.Invalidate()`. Both layers then require a forward KEY.
5. `Snapshot()` returns a copy: receiver result is pixels, last good ID, and
   valid-reference flag. Invalid reference still retains display pixels.

Single-owner synchronous codec objects expose no borrowed reference buffers.
Input slices must not be mutated concurrently. `PacketWriter` must synchronously
consume the packet and must not retain/mutate its payload.

Compression uses one standard LZ4 frame per video packet: Fast, independent
64-KiB blocks, content size, block/content checksums, concurrency=1. A fresh
full-frame compression of the same normalized capture supplies the fair
baseline. On KEY, those same measured full-frame bytes are the actual packet;
on DELTA, the baseline remains the raw captured sample rather than the
thresholded canonical image. Optional cost-aware fallback is not enabled.

Decoder validates exactly one standard frame before allocation/decompression:
no legacy/skippable/dictionary/concatenated frames or trailing bytes. All
standard block sizes and dependent blocks within a self-contained frame are
accepted. Library checksums/content size and application raw caps are checked.
Output stops at cap+1; partial output never reaches the reference. DELTA records
require sorted unique in-range indices, exact tile lengths and opaque alpha.
All tiles validate before a staged reference commits.

Memory is bounded by configured image/record sizes and library block buffers;
there are no codec goroutines or application pools. Working copies are allocated
per emission/decoding, so allocation rate is nonzero; see measured benchmarks.
Both supported widths are multiples of 32: only the bottom row is partial
(8 or 16 pixels high), including its rightmost tile.

## G2 telemetry API

`telemetry.New(nil)` uses `time.Now` under a mutex; tests can supply a clock.
Call `Record(Event{...})` on actual observations:

- `Captured`, `Queued`, `Dropped`, `Rendered`: separate one-event counters.
- `Emitted`: after Emit succeeds; supply `VideoType`, `Bytes`, `BaselineBytes`,
  `CodecTime=EncodeTime`, `BaselineTime` from its measurement.
- `Received`: each complete framed video packet, **before** decoding, with
  `VideoType`/`Bytes`. A corrupt body still counts as received.
- `Decoded`: only after successful Decode, with its `CodecTime=DecodeTime`;
  does not add received bytes again.

`Snapshot()` uses the exact `(now-1s, now]` window. All wire sizes include the
18-byte header; CONFIG/PLI/PING/PONG/END and transport overhead are excluded.
KEY/DELTA means include headers. `BaselineKnown`, `SavingsKnown` and mean-known
flags must control empty UI states. Negative and zero savings are valid.
`Complete=false` means the bounded 4096-event window overflowed; surface the
measurement gap instead of presenting partial rates as exact. `Record` reports
that overflow explicitly. Production callers should use monotonic time and
handle `ErrClock` if an injected clock moves backwards.

Decode FPS and rendered FPS are independent from received-frame counts.
Durations measure elapsed codec execution (including baseline within encode),
not process CPU usage. RTT/decode-to-render/E2E latency are not inferred from
these counters; show N/A until their respective measurement paths exist.
The session/UI will sample snapshots around every 500ms; Meter starts no timer.

G2 fuzz and isolated microbenchmarks:

```powershell
foreach ($target in @('FuzzDecodeDelta', 'FuzzCompressedFrame', 'FuzzDecoderAtomicity')) {
    go -C app test ./internal/delta -run '^$' -fuzz "^${target}$" -fuzztime=30s -fuzzminimizetime=1s -parallel=2
    if ($LASTEXITCODE -ne 0) { throw "Fuzz failed: $target" }
}
go -C app test ./internal/delta -run '^$' -bench '^BenchmarkCodec$' -benchmem -benchtime=2s -count=1
```

The benchmark alternates prebuilt synthetic inputs, includes encode, full
baseline and decode, and excludes capture/transport/UI. Results are not a
screen-sharing FPS or end-to-end latency acceptance test.

## G3 desktop preview — build and run

Requirements verified on this Windows machine: Go 1.27.4, Wails 3.1.0,
Node 24.22.1/npm 12.2.1, WebView2 157.0.4341.46. Existing tool setup is
`g0/Use-Toolchain.ps1`; no frontend global tool install is needed.

From `app/`:

```powershell
. .\g0\Use-Toolchain.ps1
$env:WAILS_EXP_USE_WAKE='1'
wails3 build
.\bin\crocstream.exe
```

Wails 3.1.0's `wails.yaml` manifest route requires the process-local Wake
opt-in above. It installs locked npm dependencies with `npm ci --ignore-scripts`,
generates service bindings, runs the TypeScript/Vite build, generates Windows
resources and builds `bin/crocstream.exe`. Generated files live under
`.wails/`, `frontend/bindings`, `frontend/dist` and `bin`, all ignored.
`frontend/dist-bootstrap.txt` allows Go package analysis for binding generation
before the first frontend build. Build order is important on a fresh checkout.

Development:

```powershell
. .\g0\Use-Toolchain.ps1
$env:WAILS_EXP_USE_WAKE='1'
wails3 dev -host 127.0.0.1 -port 9245
```

The fixed port is part of the tested dev-Origin contract; do not casually
change it without updating/testing that contract. Ctrl+C ends the dev watcher;
closing the app window alone leaves the watcher active by Wails design.
Dev uses a fresh CSP nonce for Vite's React-refresh bootstrap; the packaged
HTML permits only self-hosted scripts and has no dev nonce/debug port.

Operation:

1. Choose a display and 1280×720 or 640×360 at 15/24/30 FPS.
2. **Bắt đầu preview** is explicit consent to capture that display. GDI has
   no separate OS picker prompt; no screen read happens at app startup.
3. Preview is JPEG on this computer only; render/capture/JPEG FPS are measured.
   Window source aspect ratio is retained with black bars as needed.
4. **Dừng preview** clears canvas and closes native capture, HTTP listener and
   WebSocket. Settings unlock after cleanup. A timeout remains Stopping,
   reports the delay and prevents creating an overlapping session.

Token and ephemeral endpoint are returned only through the trusted Wails
Start binding. The token is placed in a WebSocket subprotocol offer, never a
URL/query. Bridge checks exact Host, exact Origin and token before upgrade,
serves only `/preview`, caps sockets/clients/queues and closes on Stop.
Production Origin is `http://wails.localhost`; dev Origin is
`http://wails.localhost:9245`. Missing, null and foreign Origins are rejected.

Frontend checks, after binding generation:

```powershell
cd frontend
npm ci --ignore-scripts --no-audit --no-fund
npm run typecheck
npm run lint
npm run test
npm run build
```

Frontend lifecycle and rendering are shared by local preview and Share/Watch:
`src/hooks/useSession.ts` owns the command/poll coordinator,
`src/hooks/useFrameStream.ts` owns the binary WebSocket and canvas, and
`src/hooks/useDisplays.ts` owns display enumeration. Stop coalesces concurrent
requests and follows pending Start; RPC revisions and backend generations reject
stale results. The backend state names and the UI labels/selectors are retained.
The display queue decodes only the newest pending JPEG on a browser animation
turn, closes every bitmap, and publishes render counts to React every 500 ms.
Display reconnect never rejoins the authenticated media session.

`npm run test:ui` runs Playwright against the actual React frontend using
synthetic JPEGs and deferred desktop bindings. It uses an already installed
Microsoft Edge and starts its own fixed-port Vite server. It does not capture a
display or contact a relay; screenshots/video/traces are disabled. Unit tests
run separately under Vitest. Native WebView2/capture acceptance remains in the
G3–G6 harnesses below.

Actual capture test/probe/UI automation: [g3/README](g3/README.md).
Evidence, command failures and measured limits: [G3_CLOSEOUT](G3_CLOSEOUT.md).
No frames are written to disk. Numeric diagnostics exclude WebView2 subprocess
memory; they must not be presented as total system/application-tree RSS.

## G4 Share/Watch — local desktop and remote synthetic proof

The production UI has three Home choices: local preview (G3), Share and Watch.
Share requires a configured, owner-authorized relay endpoint/password and
explicit screen selection/Start; it shows an invitation only after relay
preparation. Watch accepts the upstream-derived six-word invitation without a
locally invented regex. Both peers authenticate through the reused G0 adapter;
only media CONFIG/KEY/DELTA is displayed as authenticated/live. Stop/Leave is
idempotent and clears code/token, capture and local bridge. UI currently exposes
the G2 0–255/default-4 tau range and 1–120/default-30 emitted-frame KEY interval.

Build/run with the local app Go module and existing portable tools (see G3
instructions above). Generated Wails bindings are refreshed during build.

Local two-process synthetic protocol proof:

```powershell
. .\g0\Use-Toolchain.ps1
go build -race -buildvcs=false -o "$env:GOBIN\g4proof.exe" ./g4/proof
& "$env:GOBIN\g4proof.exe" -duration 30s
```

Desktop local two-process proof is in [`app/g4/README.md`](g4/README.md) and
uses actual capture only after explicit Start. The peer-process proof uses
synthetic RGBA through an owned upstream croc relay. Both run only on loopback.

**Remote G4 PASS:** after the owner restarted EC2 and supplied `47.128.220.28`,
strict SSH matched the previous G0 host key. A >=600s two-machine synthetic
media/control/reference proof passed, and independent SSH inspection confirmed
run-owned processes/rules/listener were cleaned. See
[remote report](G4_REMOTE_PROOF.md) and [SSH helper](g4/Invoke-SshProof.ps1).
Real screen capture to EC2 remains outside the local-only capture permission.
The test does not establish Internet 24 FPS or remote capture-to-render latency.
For a changed IP on the same trusted instance, `-PinnedHostKeyAlias` selects
the previously verified known-host entry without accepting an untrusted new key.

## G5 UX and measured dashboard

Share/Watch now show session uptime, last accepted KEY/DELTA kind, waiting
capture separately from emitted video, explicit empty/incomplete measurements,
and accessible descriptions for rates, means, baseline and negative savings.
Retry countdown follows the backend's 2s/5-per-minute budget. Local display
failure offers a rate-limited manual reconnect without restarting network
decode. Back-to-Home waits for pending Start and resource cleanup.

Actual Windows local 720p/24-FPS two-app stress ran 600s: viewer render p50
24 FPS / p95 25 FPS, Stop 141ms. These measurements apply to the named desktop
dataset, not arbitrary video/Internet conditions. Optional decode→render is
N/A; true E2E is hidden. See [G5 closeout](G5_CLOSEOUT.md) for exact binaries,
tests, resource limits and remaining manual cases; [reproduce](g5/README.md).

## G6 Windows portable and demo

The final local artifact is
`release/g6-final-20261117/CrocStream-0.0.1-windows-amd64.zip`
(approximately 7.72 MiB) with adjacent SHA-256. Extract the entire ZIP and run
`crocstream.exe`; Go/Node/Wails are not required on the target PC. Microsoft
Edge WebView2 Evergreen Runtime is required separately. This prototype is
unsigned; no installer, signing certificate or automatic updates are bundled.

Share/Watch use a compact UI with Vietnamese main labels; three primary
measurements sit under the canvas, and codec metrics remain in **Chi tiết
luồng**. The optional `crocstream-relay.exe`/`Start-DemoRelay.ps1` run a manual
loopback/private-LAN relay using the same pinned upstream implementation.
No credentials are accepted on command-line arguments or saved in the package.

Build the ZIP with existing approved tools (from `app/`):

```powershell
. .\g0\Use-Toolchain.ps1
.\g6\Build-Portable.ps1 -OutputDirectory release/new-build
```

The destination must be fresh. The build installs nothing, checks production
tags, gathers conservative attribution, creates an allowlisted manifest and
checksums, then packages the files. Do not distribute partial build folders.
Use [DEMO](g6/DEMO.md) for operator instructions,
[KNOWN_ISSUES](g6/KNOWN_ISSUES.md) for limits, and
[G6 closeout](G6_CLOSEOUT.md) for actual commands, artifact hashes, production
two-app UI proof and license scope. Engineering-only production acceptance
requires an empty clipboard and local capture consent:

```powershell
.\g6\Smoke-Package.ps1 -Directory <freshly-extracted-package>
.\g6\Test-ProductionUI.ps1 -Directory <freshly-extracted-package>
```

The latter uses Windows UI Automation with the exact shipped binaries,
temporarily copies the invitation in memory/clipboard and sends screen media
only over loopback. No screenshots, traces or frames are saved.
