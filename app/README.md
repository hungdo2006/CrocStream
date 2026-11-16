# CrocStream application core

`go.mod` defines the pure-Go application module, `crocstream.local/app`.
G1 implements media framing/configuration in `internal/protocol`, using only
the Go standard library. The isolated G0 modules in `g0/buildprobe` and
`g0/poc` remain separate from application package checks.

Gate evidence: [G0 closeout](G0_CLOSEOUT.md), [G1 closeout](G1_CLOSEOUT.md).
Wire contract: [media protocol v1](../docs/ARCHITECTURE.md#media-protocol-v1).

## Run checks

From the repository root, using the existing portable toolchain:

```powershell
. .\app\g0\Use-Toolchain.ps1
go -C app test -count=1 -timeout=60s ./...
go -C app test -race -count=1 -timeout=120s ./...
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
and telemetry. G4 supplies the actual secure transport integration, PLI
limiting, Ping/Pong timing, serialized queues and session-wide lifecycle.
The G1 tests use synthetic opaque payloads and explicit mock codec acceptance;
they do not claim successful media decoding.
