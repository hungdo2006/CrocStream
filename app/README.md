# CrocStream application core

`go.mod` defines the pure-Go application module, `crocstream.local/app`.
G1 implements media framing/configuration in `internal/protocol`. G2 adds
`internal/frame`, `internal/delta` and `internal/telemetry`, with the approved
LZ4-frame dependency pinned in `go.mod` / `go.sum`.
The isolated G0 modules in `g0/buildprobe` and
`g0/poc` remain separate from application package checks.

Gate evidence: [G0 closeout](G0_CLOSEOUT.md), [G1 closeout](G1_CLOSEOUT.md),
[G2 closeout](G2_CLOSEOUT.md). Dependency notice: [THIRD_PARTY_NOTICES](THIRD_PARTY_NOTICES.md).
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

There is no screen-sharing executable yet. `go build ./...` checks the library
on the current platform; the desktop executable belongs to later gates.

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
and telemetry (below). G4 supplies the actual secure transport integration, PLI
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
