# Verification & Test Plan — CrocStream

> **Cách đọc:** đây là danh sách bằng chứng cần thu thập, không phải báo cáo test
> đã chạy. Thành viên mới xem [hướng dẫn nhập môn](../GETTING_STARTED.md); khi nhận
> task, dùng bảng coverage để tìm đúng nhóm test. G0 bắt đầu ở mục D; D.5 chỉ áp
> dụng cho công việc adapter tiếp theo khi được phê duyệt.

## Ground rule
Tests are acceptance evidence. The existence of this plan is not evidence that any test has passed. Codex must report exact commands, OS, Go/Wails versions, observed values, failures and untested areas after every phase.

## Coverage by requirement and gate
Product IDs are defined in [PRD](PRD.md); decisions/blockers and protocol in [ARCHITECTURE](ARCHITECTURE.md); execution in [ROADMAP](ROADMAP.md#gates).

| Requirement / contract | Verification below | Gate |
| --- | --- | --- |
| B-01/B-02/B-03/B-05, D-01/D-04/D-09; secure duplex and isolation | D and D.5 transport proof | G0 / adapter follow-up |
| B-04, D-02/D-03/D-08/D-12; platforms, tools, workspace | D inventory, E platform checks, commands | G0 / G6 / G7 |
| US-03, D-06/D-07; framing, CONFIG, canonical codec | A protocol, B delta correctness | G1 / G2 |
| US-01/US-02, D-05; setup, pairing, UI states | C lifecycle, E GUI/system | G3 / G4 / G5 |
| US-04, D-10/D-11; honest telemetry | F benchmark and metrics | G2 / G5 |
| US-05; failures, cleanup, recovery | A malformed data, C concurrency, D.5 adapter, E GUI | G1–G5 |
| US-06; reproducibility and packaging | D two-process evidence, E installed build, F conditions | G0 / G6 |

[C-01/C-02](ARCHITECTURE.md#pending-clarifications) were owner-resolved on
2026-11-16: key interval 1–120/default 30, only 640×360 and 1280×720.
C-03/C-05 were owner-confirmed for G3; C-04 now uses the actual observed
WebView2 Origins and rejects missing/null/foreign values. Evidence and
remaining manual checks must be recorded in G3 closeout.

## A. Protocol tests (unit + fuzz; no network required)
G1 framing/config implementation and executed commands:
[G1 closeout](../archive-gates/reports/G1_CLOSEOUT.md). Decompression, tile records and canonical
image correctness evidence: [G2 closeout](../archive-gates/reports/G2_CLOSEOUT.md).

- Header: fixed 18 bytes, BigEndian encode/decode fixtures below and [media protocol](ARCHITECTURE.md#media-protocol-v1), all packet types, EOF between any two bytes, invalid magic/type/direction, unsupported version.
- Reject lengths >8 MiB **before allocation**, negative/overflow arithmetic in dimension calculation, oversize decompression and unexpected trailing data. A fuzz target should cover `DecodeHeader`, `DecodeConfig`, `DecodePacket`, `DecodeDelta`; prohibit panics and unbounded memory for any input.
- CONFIG: verify the exact fixture below; reject zero/unsupported dimensions, tile size, codec, FPS and non-11-byte config.
- Full packet roundtrip over `io.Pipe` with a reader returning 1-3 bytes per Read; verify `io.ReadFull` reconstructs correctly. Simulate fragmented writes and sender termination halfway through payload.
- Validate control packets `FrameID=0`, `TimestampMs=0`, correct payload size, invalid PLI lengths, wrong direction.
- CONFIG exactly once before video; accept KeyInterval 1–120 (including 31/120), reject 0/121/65535; accept only 640×360 and 1280×720, reject mixed pairs (C-01/C-02 resolved). First video must be KEY id1. Unknown type/version is fatal, with no magic scanning after truncation.
- Fuzz header, CONFIG, tile records and compressed content even after decryption: wrong direction, invalid LZ4, replayed IDs, length overflow, impossible dimensions, malicious tile indices and compression bombs must not panic or allocate unbounded memory.
- Check uint32 FrameID exhaustion triggers a fresh authenticated session rather than wraparound; KEY resync allows forward ID gaps but rejects non-increasing IDs.
- Failed codec acceptance must retain the last good ID but consume the wire ID:
  test replay of a rejected initial/later KEY and a lower KEY after a discarded
  DELTA gap. Require forward KEY recovery and no payload read-ahead before codec
  resolution. Session reconnect itself remains G4 integration acceptance.

### Exact byte fixtures
- CONFIG: `01 05 00 02 d0 20 01 01 18 00 1e` = version1, 1280×720, tile32, rgba1, lz4frame1, fps24, interval30. Assert exact encoding and decoding.
- Header: `c5 01 00 00 00 01 00 00 00 00 00 00 00 00 00 00 00 00` = first KEY header with zero timestamp/payload length. This is a **header-only encoder fixture, not a valid video packet**.

## B. Delta correctness (unit/property)
1. Image patterns: black/single dirty pixel, white, colorbars, text-on-static-desktop, moving rectangle, full-screen random pixels, 640×360 and 1280×720 with rightmost tiles and partial-height bottom 32×32 tiles. Both widths are divisible by 32; there are no partial-width tiles in supported v1 formats.
2. Frame 1 KEY, then 200 successive frames, every decoded reference **byte-equal** to the sender's canonical transmitted reference, for `tau=0` and `tau=4`. Explicitly test a slowly changing sub-threshold pixel over repeated captures to prove no sender/viewer drift.
3. Tile record: `dirtyCount=0`, max count, sorted/duplicate/unsorted indices, out-of-range, truncated tile bytes, extra bytes, tile at bottom/right edge; malformed cases must leave old reference unchanged.
4. Early KEY via `dirtyRatio>0.65` and via PLI; every scheduled 30 emitted frame interval; wrong/missing IDs force Recovering until KEY; duplicate/replayed FrameID rejected.
5. Property tests: random current/reference frames; apply output to separate decoder; recovered reference matches encoder canonical; compare results to a slow reference implementation.
6. Compression: independent LZ4 **frame** stream for each packet; reject corrupted/truncated frame and decompressed output exceeding cap.
7. G2 implementation tests use generated patterns and an independent slow pixel
   oracle, not captured screens. Require opaque alpha, no partial canonical
   mutation on late record error, single-frame boundaries (including a trailing
   empty frame), no-size compression bombs, false content sizes, cancellation
   during work, failed-writer terminal state and copied snapshots.
8. Baseline test compares each emitted capture's full-frame compression, even
   when thresholded canonical pixels differ. On KEY actual and baseline bytes
   are equal; measurements include headers and baseline cost.

## C. Session, backpressure, concurrency
- In-memory duplex mocked transport with 1 byte reads, delayed/blocked writes, mock disconnect and reverse PLI. Assert one writer, no interleaved headers or partial writes misparsed.
- Slow UI: application decode processes all DELTAs while WS display drops intermediate frames; no drift. Slow encoder/network: capture queue bounded (1-2), latest captured frame may replace earlier captures BEFORE encoding; protocol FrameID remains contiguous.
- Rapid 100x Start/Stop/Join/Leave with context cancellation and `go test -race ./...` (when supported); check goroutine counts/handles before and after. No leaks, resources released on unexpected EOF, wrong code, blocked network or GUI exit.
- PLI flood: at most one request per second; no storms. Multiple concurrent socket attempts and invalid auth rejected. Local bridge only on 127.0.0.1; reject wrong/missing token, foreign Origin and unauthorized paths. The original missing-Origin rejection requirement needs C-04 resolution using dev/packaged WebView observations.
- Test disconnect during handshake, blocked relay, slow viewer, GUI close during capture, local WS reconnect and pooled-buffer ownership. Cancellation must unblock I/O; shutdown target <=3 s under non-hung conditions, with truthful timeout reporting.
- Confirm CONFIG/KEY ordering and END termination despite control priority; no video after END, no interleaved writes. Hold last good image after invalid DELTA, await valid KEY, and reset reference on reconnect.
- Local bridge: OS-assigned port, per-launch >=128-bit random token via trusted app binding only, client caps, bounded queues and write deadline; reject cross-role preview/viewer access. Test dev and packaged Origin behavior without wildcard acceptance.
- Confirm Stop closes session media connections/queues, clears canvases and invalidates stale decode work; final listener ownership follows C-03. WS display dropping must close stale ImageBitmaps; image-decode failure must not kill network transport.
- G3: test missing/null/wrong-scheme/wrong-port Origin, missing/wrong token,
  `/viewer` and query-token rejection, one-client cap, binary ownership,
  latest-only queue, deadline/close and 100 session lifecycles. Native capture
  tests require explicit consent; ordinary tests use synthetic DIB/fake sources.
- G4 local tests cover the actual secure croc duplex carrying media-v1 packets:
  100 Host/Join lifecycles, 100-frame canonical equality, corrupt DELTA atomicity,
  bounded PLI/Pong, correct received-byte counts even on discarded video,
  slow-UI latest-only snapshots without skipping decode, blocked writer/remote
  END, auth progress, wrong invitation, canceled preparation and Join limits.
  Two-desktop-app proof verifies local capture, both canvases, PLI recovery,
  Stop/disconnect/EOS and closed role-isolated bridges. See
  [G4 evidence/status](../archive-gates/reports/G4_CLOSEOUT.md).
- Remote G4 acceptance is not waived by local/G0 success. It passed on
  2026-11-17 with the owner-restarted EC2: synthetic-only two-machine media
  proof >=600s, expected hashes/bytes/IDs/control and independent run-owned
  process/firewall cleanup. Evidence:
  [G4 remote report](../archive-gates/reports/G4_REMOTE_PROOF.md). These remain the reproduction
  criteria; never transmit real screen content under local-only capture permission.

## D. Transport G0 proof (two processes, no GUI)
Execution status (2026-11-16): **G0 PASS** for the named prototype paths; see
[closeout](../archive-gates/reports/G0_CLOSEOUT.md), including native minimal-build and license
evidence. The test requirements below remain the verification contract;
later media/capture/UI tests are not retroactively passed by G0.

- Inventory pinned croc commit `552f40fd2d7593316118370bec0a93478d48e715`, imports, dependency/license details and actual API signatures; build minimal CLI test program that performs **actual upstream-supported authenticated PAKE** and exchange of arbitrary byte streams. Do not use plaintext shortcuts or assume removed toolchains remain installed.
- Both peers send/receive an arbitrary 10 MiB deterministic byte stream (split into multiple packets) and reverse-control Ping/KeyframeRequest equivalents over the same authenticated session; verify hashes and order.
- Repeat via localhost/LAN **and** target real relay path if available; run >=10 min without file-completion metadata or idle close; report measured latency/throughput/reconnect behavior and relay policy/availability separately.
- Evidence: exact code path/function signatures, pinned upstream SHA/tag, relay settings, terminal commands, stdout/stderr redacted, pass/fail and security assumptions. **Failure or ambiguity = G0 No-Go**.

## D.5. Transport adapter acceptance

After an adapter design and implementation workspace are approved, test PAKE
purpose isolation, both handshake directions, wrong passwords, record corruption,
reflection/replay/session isolation, bounded lengths, truncation, duplex tiny
reads/large writes, concurrent writers, partial write progress, relay admission,
Accept/session cancellation, disconnect and 100 local Host/Join lifecycles.
Exercise two real processes and compare regression results against the pinned
upstream baseline. The isolated transport package and harness are in `app/g0/poc`.
Use the [PoC README](../../scripts/qa-probes/TRANSPORT_PROOF.md) for commands and reporting limits.
The G0 inventory script records source provenance and environment metadata.
Historical runtime results are in
[G0_CLOSEOUT](../archive-gates/reports/G0_CLOSEOUT.md).
Local adapter tests do not satisfy G1 media framing or remote acceptance.

## E. GUI/system manual acceptance

G5 executed evidence: [G5 closeout](../archive-gates/reports/G5_CLOSEOUT.md) and
[reproduction harness](../archive-gates/g5/README.md). Actual local two-app 600s stress
and final 26-check UI suite passed; keyboard/accessibility-tree tests are
distinct from unexecuted screen-reader speech and physical-device cases.
The methodology below remains the contract, including explicitly unmeasured
content/platform cases in the report.

G6 portable acceptance: [G6 closeout](../archive-gates/reports/G6_CLOSEOUT.md) records exact ZIP/
binary hashes, the production two-app UIA run, extracted startup/normal close,
relay cancellation, modified/extra-file rejection and notice audit. The
instrumented UI suite separately covers malformed/recovery/canvas behavior.
Installer/signing/SmartScreen download reputation and fresh Windows tests are
NOT RUN; no success is inferred from the tested unsigned portable package.

- On actual Windows 10/11 machine with Wails/WebView2: `wails3 doctor` passes, `wails3 dev` runs, packaged `wails3 build` opens and closes cleanly; verify app startup after installation if installer built.
- Enumerate/select screen, permission handling, preview in broadcaster, code copy, join from another process, viewer canvas first KEY, recovery after injected invalid DELTA, disconnect UI, Stop, room code invalidated.
- Confirm lock/handle is released: stop streaming then another app can capture the screen/camera; observe no orphan relay connections or processes.
- Exercise every [UI state and control](PRD.md#ui-and-interaction), including keyboard and screen-reader access, focus rings/labels, disabled Start/Join, Stop/Leave in active states, safe rate-limited Retry, Back-from-Live cleanup, preserved code input and settings locks. Resolve C-05 delivery priority explicitly.
- Permission denied/missing device, wrong room code, server unreachable, lag, recovery and EOS show actionable non-blocking states. No screen/camera capture before explicit user action; no “Live” before authentication and successful first KEY delivery.
- Verify real dark theme at 960×640 and 1150×760, smaller-size scrolling, correct 16:9 letterboxing, separate local/remote labels, no stale frames on rejoin, and `Preview unavailable` on local image errors.
- Dashboard uses real counters, correct labels/tooltips and empty states (`—`, `N/A`, hidden E2E); no hardcoded room codes, savings, route types or status. Updates ~500 ms with rolling 1-second metrics; no Base64 frame events or frame-rate status events.
- Check secret code rotation requires session recreation, code cleared on Stop, and code/token/keys/frames absent from logs and crash reports; no frame files stored.
- Check WebView2, firewall/relay behavior, signed/unsigned installer behavior and target-machine startup; run frontend typecheck/lint/tests/build and native Windows build where tools permit. Missing results are NOT RUN, not cross-platform evidence.
- Future: webcam actual hardware, macOS capture permission, Linux Wayland/X11 distinction and packaging are **deferred**, not automatically passed.

## F. Benchmark and metrics methodology
- G2 unit checks cover exact one-second cutoff, missing baselines,
  received-but-not-decoded frames, explicit ring overflow/clock errors and
  concurrent recording/snapshot. Synthetic codec microbenchmarks include encode,
  baseline and decode; do not substitute them for full-app timed acceptance.
- Record OS, CPU, RAM, monitor capture source, resolution, fps, exact croc commit, relay route, time window and start/end timestamp. Run at least 2 min after warmup; >=10 min memory/leak check.
- Content scenarios: static desktop (savings likely high), document scroll, moving window, full-screen video/random noise (savings may be low or negative); never promise a universal %.
- Record raw `Captured`, `Emitted`, `Rendered` FPS; Key/Delta sizes, rolling full LZ4 baseline and actual protocol bytes, compression CPU, CPU%, max RSS, allocs/frame, dropped captures, UI frame lag p50/p95, RTT p50/p95 where measured. Include data CSV for actual benchmarks if requested.
- Guard against data fabrication: only show calculated results from logs; mark estimated values and mark unmeasured values N/A.
- Validate the [telemetry contract](ARCHITECTURE.md#telemetry-contract): aligned same-emitted-sample full-frame LZ4 baseline including header, video-only actual bytes, zero-denominator empty state, negative savings and separate transport overhead. Include baseline compute cost; profile local JPEG encoding separately.
- Measure 1280×720/24 FPS target on named Windows hardware with p50/p95; document deviations and lower-resolution/FPS results. Record queue sizes, goroutine counts and memory growth over >=10 minutes; no zero-allocation guarantee. Report LAN recovery and relay performance separately; no public-relay 24 FPS guarantee.

### Synthetic codec baseline

On 2026-11-18, Go 1.27.4 on Windows/amd64 with an Intel Core i7-13650HX measured
one short run at 1280×720. Each operation encoded an input, computed the full-frame
LZ4 baseline and decoded the emitted packet. Capture, JPEG, network, rendering
and process RSS require separate measurements.

| Synthetic input | ms/op | B/op | allocs/op |
| --- | ---: | ---: | ---: |
| Static | 4.83 | 14,630,461 | 34 |
| Moving | 4.87 | 14,623,981 | 37 |
| Noise | 7.19 | 16,828,488 | 25 |

```powershell
go -C app test -run '^$' -bench '^BenchmarkCodec$' -benchtime=1s -benchmem ./internal/delta
```

## Typical verification commands

Run from the repository root with the [Windows toolchain](../../README.md#4-điều-kiện-tiên-quyết).
The verifier runs Go tests, vet and module integrity checks in `app/` (including `internal/crocduplex` and `cmd/qa-probes`)
and `scripts/qa-probes/buildprobe`. `-Race` adds the race suite for each module. Wails regenerates
bindings from Go and runs the frontend build. The browser suite uses synthetic
bindings and images in Edge.

```powershell
powershell -NoProfile -File .\scripts\verify.ps1 -Race
```

For focused checks after the first Wails build, use `npm run typecheck`, `npm run lint`,
`npm run test`, `npm run test:ui` and `npm run build` from `app/frontend`. Run individual
Go fuzz targets using the commands in the [app README](../../app/README.md#run-checks).
Record fuzz durations and retain the distinction between seed coverage and a timed run.
For documentation-only edits, check the Git diff and local Markdown links/anchors.

Real screen capture requires `CROCSTREAM_CAPTURE_CONSENT=1` and owner consent.
Record native WebView2, real capture, remote relay, fresh-machine and spoken screen-reader
tests separately. For skipped checks, record the reason and mark them `NOT RUN`.
