# Architecture — CrocStream Desktop v1 (proposed)

> **Cách đọc:** bắt đầu từ quyết định/blocker, rồi các pipeline để hiểu luồng dữ
> liệu. Phần media protocol là hợp đồng kỹ thuật chi tiết cho người làm
> protocol/codec. Thuật ngữ và sơ đồ nhập môn nằm trong
> [GETTING_STARTED](GETTING_STARTED.md). Thiết kế ở đây chưa phải bằng chứng chạy
> thực tế; G0–G6 có closeout PASS, gồm remote synthetic G4, local stress G5 và portable Windows G6.

## Contents
- [Reference source](#reference-source)
- [Decisions and blockers](#decisions-and-blockers)
- [Pending clarifications](#pending-clarifications)
- [Principles and module layout](#principles)
- [Local WebView bridge](#local-webview-media-bridge)
- [Media protocol v1](#media-protocol-v1)
- [Telemetry contract](#telemetry-contract)
- [Sources and licensing](#sources-and-licensing)

## Reference source
The reference is `croc/` (relative to the repository root) at **552f40fd2d7593316118370bec0a93478d48e715**, after tag v11.6.5; `src/version/version.go` reports 11.6.5 and `go.mod` requires Go 1.27.3. The source snapshot is included with provenance in [UPSTREAM.md](../UPSTREAM.md). The owner chose one repository containing upstream source, documents and future application code; the former standalone checkout was backed up outside the workspace. No screen-sharing application or old experiment evidence is retained. The initial survey and later local/remote/native/notice evidence culminate in [G0 PASS](../app/G0_CLOSEOUT.md); this does not validate the future media application.

Source inspection, not runtime feasibility evidence:
- `src/croc/croc.go`: Send and transfer lifecycle depend on file metadata.
- `src/comm/comm.go`: framed message API, not directly the proposed `io.ReadWriteCloser`; access to `Connection()` alone does not provide authenticated media transport.
- `src/pakekey/pakekey.go`: transcript-bound PAKE with mutual confirmation; accepted purposes are transfer, local probe and SSH, not CrocStream.
- `src/crypt/crypt.go`: cryptographic primitives do not constitute a complete authenticated session.
- `src/tailcattransport/`: exported Listen/Accept/Dial data-plane facilities require a session key and authenticated offer exchange. `src/sshshare/` supplies SSH-specific authentication/lifecycle, not a generic CrocStream API.
- `web/`: upstream browser file-transfer/SSH client, not Wails or CrocStream.

## Decisions and blockers
Updated 2026-11-16. **D-12 and D-07's C-01/C-02 choices are owner-confirmed;
G2 LZ4 dependency is approved; other D entries remain provisional.** Keep IDs stable and distinguish design
requirements from verified implementation.

| ID | Decision | Reason / tradeoff |
| --- | --- | --- |
| D-01 | Separate CrocStream desktop app; preserve upstream file-transfer behavior. | Custom media protocol is not interoperable with stock croc send/receive. |
| D-02 | Windows 10/11 x64 first; macOS/Linux later. | One testable platform before cross-platform claims. |
| D-03 | Proposed Wails v3 stable line, target v3.1.0; Go 1.27.3+ pending toolchain checks; React/TypeScript/Tailwind. Pin exact transitive dependencies after G0. | Recheck versions; v2 event/binding APIs cannot be copied into v3. |
| D-04 | Pin the exact reference commit above, not merely the version string or v11.6.5 tag. | Reproducibility; v9/v10 APIs cannot be assumed. |
| D-05 | Exactly one broadcaster and one viewer per session. | One coherent canonical-reference/PLI sequence. |
| D-06 | RGBA8888 alpha=255, 32×32 tiles, LZ4 frame format, lossless network KEY. G2 pins owner-approved `pierrec/lz4/v4 v4.1.33` (2026-11-16). JPEG only for local UI. | Deterministic sender/viewer reference equality; dependency provenance/notice in `app/THIRD_PARTY_NOTICES.md`. |
| D-07 | Fixed session resolution: only 1280×720 or 640×360; default 1280×720 at 24 FPS, fallback 640×360 at 15 FPS; key interval 1–120 emitted frames, default 30, MAE tau=4, dirty ratio >0.65 triggers KEY. | Resolution pairs and key interval owner-confirmed 2026-11-16 (C-01/C-02); defaults are not performance guarantees. |
| D-08 | Webcam after screen-capture MVP; no audio, cloud storage, accounts, recording or multi-viewer. | Avoid CGO/OpenCV complexity before network proof. |
| D-09 | Do not apply UDP loss management to a reliable TCP byte stream by default. UI may skip display snapshots, never delta decode state. | Gaps imply application drop/corruption or reconnect. |
| D-10 | Optional measured RTT and viewer-local decode→render; no one-way E2E claim without tested clock synchronization. | Unsynchronized host timestamps cannot establish E2E latency. |
| D-11 | Full-frame LZ4 baseline on each emitted sample versus actual video packet bytes in the same window; transport overhead separate. | Fair and honestly labeled comparison. |
| D-12 (owner-confirmed) | Use one CrocStream repository: pristine upstream source in `croc/`, planning documents in `docs/`, future application code in `app/`. Keep one root `.git`; no nested repository or second active checkout. Former custom code and experiments remain removed. | Owner decisions 2026-11-15; supersedes the separate-repository layout. Consolidation does not authorize application implementation or modifying the upstream snapshot. |

### G0 blockers
G0 feasibility closed **PASS** on 2026-11-16; see
[closeout and explicit limits](../app/G0_CLOSEOUT.md).
The IDs below remain stable; PASS is for the measured prototype scope, not
production/capture/UI/installer or an independent security audit.

| ID | Status | Required evidence / decision |
| --- | --- | --- |
| B-01 | PASS G0 | Experimental adapter implements `io.ReadWriteCloser`; local and actual two-machine secure duplex proofs and implementation self-review pass. No general production-readiness claim. |
| B-02 | PASS tested route; network limits retained | Two EC2 remote proofs >600s, hashes/control/cleanup verified. Old network RTT 10.6–11.8s mean; new network at original rate 48–51ms mean, 191ms max. See G0_REMOTE_PROOF and G0_LATENCY_FOLLOWUP; old-path root cause unisolated. |
| B-03 | PASS G0 self-review/tests | Actual PAKE, transcript, mutual confirmation, directional AEAD and negative isolation tests pass; exact bounds/nonce/admission limitations documented in closeout. Not an independent audit. |
| B-04 | PASS G0 minimal native feasibility | Go 1.27.4/Wails 3.1.0/croc/x-sys/Win32-GDI combined builds and synthetic native runtime tests pass, including CGO=0; doctor and 21-module notice review pass. Actual display capture remains G3. |
| B-05 | PASS G0 option A | Owner-approved experimental adapter and small noticed derivative retained; maintenance/regression contract documented. No relay-provider switch, full fork or production deployment approval inferred. |

G0 follow-up, 2026-11-16: the owner approved portable tools and pursuing the
croc-based adapter design/review (option A). Go/Wails installation and a minimal
combined native link/runtime probe and license review succeeded; B-04 is closed
at G0's dependency-feasibility level, not G3 real-capture acceptance.
The experimental [adapter contract](../app/G0_ADAPTER_DESIGN.md) is now
implemented in `app/g0/poc` with local B-01/B-03 test evidence; production/security
review remains distinct from those tests. B-02 has an owner-provided EC2 path
and >600s proofs on two network conditions; the latest original-load run measured
48–51ms mean application RTT. The old path's high RTT remains in the
[remote report](../app/G0_REMOTE_PROOF.md); follow-up in
[G0_LATENCY_FOLLOWUP](../app/G0_LATENCY_FOLLOWUP.md). Local [runtime report](../app/G0_LOCAL_PROOF.md). Initial survey:
[G0_FEASIBILITY](../app/G0_FEASIBILITY.md); current checks:
[G0_TOOLCHAIN](../app/G0_TOOLCHAIN.md). No transport blocker is closed by tool installation.

Deferred owner choices: webcam versus macOS/Linux priority; installer format; self-hosted versus public demo relay; Vietnamese-only versus bilingual UI; named hardware/network before absolute performance acceptance; whether the academic demo requires raw-capture losslessness (`tau=0`) or tolerates skipped changes (`tau=4`). Both tau modes must preserve sender/viewer canonical equality.

### Change control and risks
Update the existing decision ID, affected product requirements, protocol, tests and roadmap together. Document backward incompatibility; do not reuse protocolVersion=1 after changing wire bytes.

G0 must quantify internal API/fork maintenance costs, relay capacity/policy and security assumptions. Avoid low-latency/P2P claims without evidence. High-entropy LZ4 keys, local JPEG encoding and full-frame baseline computation require separate profiling; static desktops do not establish universal performance.

## Pending clarifications
These are pre-existing ambiguities. C-01/C-02 were explicitly resolved by the
owner on 2026-11-16 when authorizing G1. At G3, the owner also confirmed C-03
and C-05, and authorized an exact observed-Origin policy for C-04.
Resolve affected items before implementation and update decision/spec/tests together.

| ID | Existing requirements | Proposed resolution to confirm |
| --- | --- | --- |
| C-01 — RESOLVED | PRD previously described maximum 30 versus CONFIG 1–120. | Owner confirmed 1–120 emitted frames, default 30; 30 is not the maximum. Wire layout/version unchanged. |
| C-02 — RESOLVED | Product lists two resolution pairs; CONFIG listed width/height independently. | Owner confirmed only (640,360) and (1280,720); reject mixed pairs. Wire layout/version unchanged. |
| C-03 — RESOLVED | Architecture previously suggested app-lifetime listener versus session shutdown. | Owner confirmed session-owned loopback listener/WebSockets/queue; Start creates, Stop closes. Cryptographic 256-bit token generated at launch and additionally rotated each session. |
| C-04 — RESOLVED for Windows | Owner authorized exact Origin allowlist, reject missing/null/foreign Origin. | Actual Windows WebView2 probe and application UI: embedded `http://wails.localhost`, dev `http://wails.localhost:9245`. Production accepts only embedded Origin; fixed-port dev accepts only the observed dev Origin. Full dev/app evidence in G3 closeout; other platforms require fresh observations. |
| C-05 — RESOLVED | Basic accessibility delivery priority was ambiguous. | Owner confirmed keyboard navigation, labels, visible focus and accessible status in G3/MVP; enhancements remain later. |

### When to resolve clarifications

These checkpoints organize the existing open questions; they do not approve the
proposed resolutions above.

| Before implementing | Items | Evidence / decision needed |
| --- | --- | --- |
| G1 configuration validation | C-01, C-02 — resolved | Owner confirmed 1–120/default 30 and only the two product resolution pairs on 2026-11-16 |
| G3 local bridge | C-03 confirmed; C-04 observation-based | Session ownership and exact Origin policy implemented; verify full dev/packaged UI and resource cleanup |
| G3 preview UI and G5 full UI | C-05 confirmed | Basic accessible controls implemented; verify keyboard and accessibility tree, record actual screen-reader testing separately |

## Principles
1. **Transport is replaceable**, but its security cannot be weakened; no hard dependency on undocumented `comm.Comm` layout beyond an experimentally verified adapter.
2. **Codec is a pure Go library** with no Wails, relay, capture, UI or disk dependency. Input images have deterministic packed RGBA normalization.
3. Sender and receiver maintain separate canonical references but **must converge byte-for-byte** under a valid sequence; the network payload is not JPEG.
4. **Real-time over backpressure**: bounded latest-only capture before encoder, sequential codec and network write; decode all deltas, optionally skip UI display snapshots.
5. **Wails v3 not v2**: v3 services/events/app/window APIs only. Do not blindly port `runtime.EventsEmit`/v2 `app.go` examples.

## Application layout and future modules

The current single repository contains `croc/` (pristine upstream reference snapshot),
`docs/` (planning documents), G0 artifacts and the application module in `app/`.
G0–G6 are closed PASS; local-preview implementation and evidence are
tracked in [G3_CLOSEOUT](../app/G3_CLOSEOUT.md).
`internal/protocol`, `internal/frame`, `internal/delta` and
`internal/telemetry` are implemented; G2 evidence is in
[G2_CLOSEOUT](../app/G2_CLOSEOUT.md). G3 also implements capture, bridge,
preview, desktop bindings and frontend. Session integration is implemented in G4;
local desktop and remote two-machine synthetic proofs pass. See
[G4_CLOSEOUT](../app/G4_CLOSEOUT.md).

```
AGENTS.md
README.md
docs/{PRD,ARCHITECTURE,TEST_PLAN,ROADMAP}.md
croc/                                 # Existing pristine upstream source
app/                                  # Application core and G0 artifacts
  G0_FEASIBILITY.md                    # Historical survey; final status in closeout
  G0_TOOLCHAIN.md                      # Tool/build/test evidence
  G0_ADAPTER_DESIGN.md                 # Experimental transport contract
  G0_LOCAL_PROOF.md                    # Historical local PoC evidence
  G0_CLOSEOUT.md                       # G0 PASS, notices/native checks and G1 handoff
  G1_CLOSEOUT.md                       # Framing/config implementation and evidence
  G2_CLOSEOUT.md                       # Codec/telemetry evidence and benchmarks
  README.md                           # Application-core commands and API contract
  go.mod                              # Pure Go, approved LZ4 dependency at G2
  internal/protocol/                  # Implemented: header, CONFIG, reader/writer, tests/fuzz
  internal/frame/                     # Implemented: stride/order/alpha normalization
  internal/delta/                     # Implemented: bounded LZ4 KEY/DELTA, canonical state
  internal/telemetry/                 # Implemented: bounded exact rolling 1s measurements
  g0/Inspect-Environment.ps1           # Existing read-only inventory script
  g0/Install-Toolchain.ps1             # Approved portable tool installation
  g0/Use-Toolchain.ps1                 # Process-local environment
  g0/buildprobe/                       # Isolated minimal Go/Wails/croc compile probe
  g0/poc/                              # Experimental transport + CLI harness + tests
  main_windows.go                     # G3 Wails bootstrap; root package for actual v3 build
  wails.yaml                          # Wails v3.1 manifest (Wake opt-in)
  internal/capture/                   # G3 Win32 GDI display capture, consent at Start
  internal/bridge/                    # G3 session-owned binary JPEG WebSocket
  internal/preview/                   # G3 preview lifecycle and bounded sampling
  internal/desktop/                   # G3 Wails service bindings
  internal/session/                   # G4 authenticated role/media/control lifecycle
  internal/transport/                 # G4 factory over the existing G0 module
  g0/poc/duplex/                       # G4 public façade; one crypto source of truth
  g4/                                # Synthetic/local-desktop proofs and remote helper
  G4_CLOSEOUT.md                      # Local desktop + remote synthetic G4 PASS
  G4_REMOTE_PROOF.md                  # >=600s two-machine hashes/control/cleanup
  frontend/src/                       # G3 React/TypeScript canvas preview
  frontend/package.json               # Pinned dependencies + package-lock.json
  g3/                                # Capture-free Origin probe, opt-in verification
  # Everything below is proposed, not implemented:
  internal/app/                       # Wails service methods + app/session coordinator
  build/                             # Wails build assets
```
Adjust the paths to the actual initialized Wails v3 template; app structure is conceptual, not a license to invent outdated framework methods.

## Suggested Go boundaries
```
// Pseudocode interfaces, not a claim about upstream croc's actual APIs.
type Duplex interface { io.Reader; io.Writer; io.Closer }
type TransportFactory interface {
    Host(ctx context.Context, options HostOptions) (listener Listener, err error)
    Join(ctx context.Context, invite string) (duplex Duplex, err error)
}
type Listener interface {
    Invitation() string
    Accept(ctx context.Context) (Duplex, error)
    Close() error
}
type Source interface {
    Open(ctx context.Context, cfg CaptureConfig) error
    Next(ctx context.Context) (Frame, error)
    Close() error
}
type Frame struct { Width, Height int; RGBA []byte; CapturedAt time.Time }
```
Implement transport adapter only from real investigated upstream API; session must support cancellation even when read/write blocked via closable connections/deadlines as supported.

### Transport lifecycle requirements
Host preparation must expose an invitation before waiting for a joiner; it must
not report a peer-authenticated stream at that stage. Accept/Join should return
the duplex stream only after mutual authentication. Define context ownership,
blocked-I/O cancellation, deadlines, close semantics and bounded buffering.
Keep raw sockets and traffic keys private. These are proposed requirements,
not exported APIs in the current upstream checkout.

## Sender pipeline
`Capture Screen` → `packed RGBA` → `latest-only queue (1–2)` → `Delta Encoder (canonical ref)` → `Protocol writer (single writer; CONFIG+KEY+DELTA)` → `secure Duplex transport`.

A **separate** preview sampling path accepts captured frames and may encode JPEG for local WebView. Preview frame processing must not block media writer. It is allowed to drop intermediate previews; it must not mutate reference state. Encoder writes frame ID only for video packets selected/emitted; if sending fails terminate session.

## Viewer pipeline
`Secure Duplex` → `io.ReadFull header+bounded payload` → `Protocol validator` → `LZ4 decoder + reference buffer` → `jpeg preview encoder (local-only)` → `local WebSocket` → `React canvas`.

Receiver processing validates entire DELTA before touching canonical reference. Reader cannot skip codec packets. Render bridge uses latest-only output buffer when UI is slower. Apply lossless codec before UI JPEG sampling to retain valid reference.

## Control plane
- The same duplex channel allows viewer `REQUEST_KEYFRAME` and either side `PING/PONG` concurrently. One **serialized writer goroutine per endpoint** receives outbound video/control packets from bounded queues; priority: END/PLI/PONG before video (while preserving valid ordering and encoder state).
- Send config, keyframe, and periodic keys; viewer requests resync when invalid. PLI only signals sender's **next** emitted KEY, not an immediate copy of a stale raw buffer. PLI is rate-limited.
- Wails service methods: `ListDisplays`, `StartBroadcast`, `StopBroadcast`, `JoinSession`, `LeaveSession`, `GetBridgeEndpoint`, `GetStats`. Their exact signatures derive from v3 app bindings and test doubles; use explicit success/error states. Events carry **only statuses/statistics**, never Base64 frames.

G4's implemented bindings are `ListDisplays`, `StartBroadcast`, `JoinSession`,
`StopSession`, `GetInvitation`, `GetSessionStats`, and G3's local-preview
bindings. Start/Join return only the trusted local bridge endpoint/token plus
generation. Invitation is a separate trusted binding, never included in
telemetry snapshots. UI polls state/metrics around 500ms, not video-rate events.
`StopSession` implements both broadcaster Stop and viewer Leave.

`Preparing → WaitingForViewer → Authenticating → Live` and
`Connecting → Authenticating → AwaitingKeyframe → Watching/Recovering` derive
from actual admission, PAKE progress and successful first KEY write/decode;
the same authenticated byte stream remains in use. `Live` is not viewer render
acknowledgement or one-way latency proof. Pairing is one-shot; Stop clears the
code, closes all session resources, and a new session authenticates afresh.
Join is explicitly rate-limited (2s minimum between attempts, 5/minute) with no
automatic retry loop. Local tests can configure their own private lifecycle
limits without altering the production limit.

G5 exposes the remaining retry budget via sanitized `retryAfterMs`, plus
capture totals, current bounded queue depths, bridge drops and existing codec/
baseline measurements. `meanJPEGMs` is lifetime elapsed JPEG encoding time,
not process CPU; Windows diagnostics expose measured cumulative process CPU.
The last frame kind is the last successfully emitted/decoded video, not a
canvas acknowledgement. No media wire field is added. Back waits for the
pending Start binding before Stop; display-only reconnect uses the same live
session endpoint, is user-triggered/rate-limited, and invalidates old canvas
decode work. Optional LZ4 decode→render stays N/A because JPEG snapshots do not
carry correlated timestamps. See [G5 report](../app/G5_CLOSEOUT.md).

## Local WebView media bridge
- Bind **only** `127.0.0.1:<OS-assigned-port>`; require a cryptographic 256-bit launch/session token on WebSocket upgrade. The listener and upgraded connections belong to the preview session (C-03). Reject absent/null/foreign Origin with an exact mode-specific allowlist (C-04); no wildcard.
- Provide token + ephemeral URL only via trusted app binding to own frontend; do not expose in logs or telemetry. Explicitly close upgrade connections on Stop/exit; bound send queue (1-2), per-client caps and WebSocket write deadline.
- Distinguish `/preview` and `/viewer` channels/roles, or use one tagged binary envelope with validated role; no accidental cross-role data mixing. **Frontend media bytes are independent encoded JPEG frames** (`image/jpeg`) for `createImageBitmap` in this MVP; `createImageBitmap` does **not** parse raw RGBA without a format-specific wrapper. Stop and clear canvases when session ends; close bitmap in `finally` after draw.
- Wails v3 origin may differ in dev vs packaged; test both without permissive `Origin: *` behavior.
- Frontend: `binaryType='blob'`, `createImageBitmap(blob)`, draw into `useRef<HTMLCanvasElement>` with fitted letterboxing; always `bitmap.close()` in `finally`. Keep newest queued display image only and close stale bitmaps.
- On unmount/Stop/Leave, close WS and listeners, invalidate outstanding decode work with generation tokens and clear canvas. Failed image decode reports `Preview unavailable` without collapsing media transport.

G3 handshake: `/preview` only, one authenticated client, token offered via
`Sec-WebSocket-Protocol: crocstream.preview, auth.<token>`, never in URL/query.
Server negotiates only `crocstream.preview`, never echoes the token. Exact
Host and Origin are checked before upgrade. JPEG messages are binary and bounded
to 4 MiB; newest-only one-message queue, one-second write timeout and bounded
HTTP headers/read timeouts. G4 uses a separate immutable role instance for
`/viewer` with its own token/listener; each instance rejects the other path.
Stop closes hijacked sockets as well as the HTTP listener.

G3 capture uses Win32 GDI on a locked worker thread: monitor enumeration,
per-monitor DPI context, bounded top-down 32-bit DIB, aspect-fitted StretchBlt
and G2 RGBA normalization. Black letterboxing preserves source aspect ratio.
Only explicit Start invokes screen reads. GDI surface selection is restored
before deletion; DC acquisition/release remains on its owner thread. Display
removal/geometry change stops the session. Protected content, HDR and secondary
monitor behavior must be reported based on actual tests, not inferred.

## Buffers, concurrency, cancellation
- Use `context.Context` for all worker loops and `errgroup` or WaitGroup for bounded shutdown. Explicit buffer ownership: pool item cannot be released while encoder, decoder, websocket or image encoding still refers to it. `sync.Pool` only for profiled hot paths, not as an unconditional requirement.
- Avoid shared mutable frame buffers between capture/codec/render without immutable handoff or copy. Decode stage must not mutate reference before all checks succeed.
- Single writer serializes frames, PLI, PING/PONG and END. Control priority must preserve CONFIG-before-KEY ordering and prevent video after END; cancellation unblocks blocked I/O.
- Verify lifecycle and concurrency using [TEST_PLAN](TEST_PLAN.md#c-session-backpressure-concurrency).

## Transport feasibility is G0
- Inspect the pinned `github.com/schollz/croc/v11` commit and exact cryptographic/relay handshakes. Do not assume using `crypt.New()` alone reproduces croc security or that grabbing `comm.Comm` automatically bypasses file metadata safely.
- Prove same-machine two-process persistent duplex stream, then two machines via expected relay, including reverse control. Report measured limits, license/attribution and security model. If impossible/unsafe, stop and propose options — no silent fallback to unencrypted plain TCP.

## Packaging and deployment
- Target Wails v3.1.0, toolchain from tested upstream `go.mod` (provisionally Go 1.27.3+), Windows x64 initially. Verify `wails3 doctor`, native screen-capture permission, WebView2, firewall/relay usage and signed/unsigned installer behavior. Tests on actual macOS/Linux and webcam hardware are deferred to subsequent phases.

G6 produces an unsigned, allowlisted portable ZIP using the existing toolchain,
with SHA-256 checksums, manifest, complete conservative notices and demo docs.
The optional `cmd/demo-relay` wraps the same pinned upstream TCP relay,
accepts credentials only over stdin, and permits loopback/private IPv4 binds.
It changes no transport cryptography or media bytes and is not a Windows service.
The actual extracted production apps complete a local Share/Watch session via
Windows UI Automation without test/CDP APIs. WebView2 remains a separately
installed prerequisite. Installer/signing and fresh-machine startup are not
passed by the portable result. See [G6 closeout](../app/G6_CLOSEOUT.md).

## Media protocol v1
**Normative v1 contract, not deployed.** G1 implements framing/configuration;
G2 implements codec/telemetry; G4 integrates application sessions and passes
two-machine synthetic media acceptance on the measured EC2 route. Framing runs
inside an authenticated/encrypted transport; stock croc clients do not
understand it. G0 tested a separate experimental transport record envelope.
The media byte layout below is unchanged. Changing wire bytes requires a
protocol revision and tests.

### Stream and header
One reliable ordered bidirectional byte stream per session, one receive parser and one serialized writer per endpoint. Reverse keyframe requests share that duplex stream.

Packet = exactly **18-byte header + PayloadLength bytes**, BigEndian, no padding. Use `io.ReadFull` for both. Mid-header/payload EOF is fatal; never scan for a later magic byte. Validate type, session/configuration and type-specific lengths **before allocation**. `MAX_PAYLOAD = 8,388,608` (8 MiB); separately cap decompression.

| Offset | Bytes | Field | Validation |
| --- | ---: | --- | --- |
| 0 | 1 | Magic uint8 | `0xC5` |
| 1 | 1 | Type uint8 | One of the types below |
| 2 | 4 | FrameID uint32 | 1-based video IDs; 0 for control/END |
| 6 | 8 | TimestampMs int64 | Broadcaster Unix ms for media; 0 for control/END; not a synchronized latency clock |
| 14 | 4 | PayloadLength uint32 | Exact following byte count, <=8 MiB plus type-specific bounds |

| Type | Direction | FrameID | Payload / rules |
| --- | --- | --- | --- |
| `KEYFRAME 0x01` (KEY) | broadcaster → viewer | >=1 | Independent LZ4 frame of full packed RGBA; first video always KEY id1 |
| `DELTA 0x02` | broadcaster → viewer | >=1 | Independent LZ4 frame of tile record; valid prior reference and sequential video ID required |
| `REQUEST_KEYFRAME 0x03` (PLI) | viewer → broadcaster | 0 | Empty; max 1/s, ignore excess; next emitted video must be KEY |
| `SESSION_CONFIG 0x04` (CONFIG) | broadcaster → viewer | 0 | Exactly 11 bytes; once immediately after handshake, before video; reconnect is a new session |
| `PING 0x05` | either | 0 | 8-byte arbitrary nonce |
| `PONG 0x06` | either | 0 | Promptly echo nonce; initiator measures monotonic RTT |
| `END 0xFF` (EOS) | either | 0 | Empty; best-effort delivery; also honor EOF/cancellation |

Unknown types are fatal in v1; no silent fallback.

The G1 API enforces CONFIG as the first broadcaster packet, with END allowed
for early abort; reverse controls require no reverse CONFIG. After CONFIG,
controls may precede the first KEY. See [API and ownership](../app/README.md).

### SESSION_CONFIG
| Offset | Bytes | Field | Value |
| --- | ---: | --- | --- |
| 0 | 1 | ProtocolVersion uint8 | 1 |
| 1 | 2 | Width uint16 | 640 or 1280 |
| 3 | 2 | Height uint16 | 360 or 720 |
| 5 | 1 | TileSize uint8 | 32 |
| 6 | 1 | PixelFormat uint8 | 1 = packed RGBA8888, alpha=255 |
| 7 | 1 | Compression uint8 | 1 = LZ4 **frame**, not block API |
| 8 | 1 | FPS uint8 | 15, 24 or 30 desired; actual rate measured |
| 9 | 2 | KeyInterval uint16 | 1–120 emitted frames, default 30 (C-01 resolved) |

Reject unsupported values; no silent downscale/rescale. Only resolution pairs
**(640,360)** and **(1280,720)** are permitted; reject mixed pairs (C-02 resolved).
Checked `Width*Height*4 <= MAX_RAW = 3,686,400` (1280×720×4). Larger formats require a new protocol revision and bounds.

### Canonical image and compression
- Normalize native capture stride into contiguous row-major `R,G,B,A`, length `Width*Height*4`, alpha forced to 255 without premultiplied alpha. Any RGB color transformation must be deterministic and identical in tests. Resolution cannot change within a session.
- KEY compresses exact canonical RGBA into one self-contained LZ4 frame. Decoded length must equal `Width*Height*4`; reject overrun, underrun and trailing unsupported data. No JPEG/WebP network reference.
- DELTA compresses the entire record below as one self-contained LZ4 frame. Compression is independent per packet; reconstruction still needs the canonical reference.
- Skipped sub-threshold tiles leave sender reference unchanged. Compare new capture against **transmitted canonical reference**, not the last capture; equality is between peers, not necessarily raw capture when tau>0.

G2 uses standard LZ4 frames with Fast compression, independent 64-KiB blocks,
content size and block/content checksums, concurrency=1. Decoder accepts all
standard block sizes and dependent blocks within a self-contained frame.
Preflight requires exactly one standard frame: no legacy, skippable,
dictionary-dependent, concatenated or trailing data. Decode is bounded to the
configured raw cap plus one overshoot-detection byte; checksums and content size
are checked before committing pixels. This selects permitted LZ4-frame options;
it does not change media v1 header/config/tile bytes.

### DELTA record before compression
```text
uint16 dirtyCount (BE)
repeat dirtyCount times, strictly increasing tileIndex:
    uint16 tileIndex (BE)
    uint8 tileRGBA[effectiveTileWidth * effectiveTileHeight * 4]
```
`tileColumns=ceil(Width/32)`, `tileRows=ceil(Height/32)`, `tileIndex=tileY*tileColumns+tileX` (zero-based).
For each tile: `x=tileX*32`, `y=tileY*32`, `w=min(32,Width-x)`, `h=min(32,Height-y)`. Pack tile rows without a per-tile length field; config/index determine size. There are 920 tiles (40×23) at 1280×720 and 240 (20×12) at 640×360. Empty DELTA is two zero count bytes before compression.

- Require `dirtyCount<=tileCount`, valid strictly ascending unique indices, exact parsed length and no trailing bytes.
- Decompressed cap: `2 + 2*tileCount + Width*Height*4` with overflow checks; stop streaming decompression on overshoot.
- Tile RGB MAE = `sum(|a-b|)/(3*pixelCount)`, excluding alpha; dirty iff `MAE > tau`, default 4.
  G2 uses the equivalent exact integer comparison `sum > tau*3*pixelCount`;
  local encoder tau is integer 0–255. Both supported widths divide by 32, so
  v1 has partial-height bottom tiles only, including the bottom-right tile.
- If `dirtyCount/tileCount > 0.65`, emit lossless KEY. Optional cost-aware fallback may use KEY when encoded DELTA >= encoded KEY, only if both candidates were actually measured.
- After DELTA, update only transmitted dirty tiles; after KEY, replace the entire canonical reference. Commit reference/FrameID only after acceptance by the session writer. A network write failure terminates the session; never continue from uncertain state.

### Ordering and recovery
1. CONFIG then KEY id1 after authentication. Receiver has no reference until a valid KEY decodes.
2. KEY with `FrameID>lastVideoID` independently resets reference, including across ID gaps. Reject non-increasing IDs/replay within a session.
3. DELTA requires a reference and `FrameID=lastVideoID+1`. Validate the full record before mutation or use a staged buffer. On error/gap retain last good image/reference bytes, mark reference invalid, issue rate-limited PLI and await KEY.
4. At most one emitted video per encoded capture tick; only emitted frames acquire IDs. Drop/replace raw captures before encoding, never intermediate DELTA packets or decode operations. UI may skip display snapshots.
5. For interval 30, scheduled KEY IDs are `1,31,61,...`; PLI/dirty fallback may force earlier KEY. Near uint32 overflow terminate and establish a fresh authenticated session; never wrap to 0.
6. On close: best-effort END, cancel workers, close sockets/readers/writers, release screen/camera, clear in-memory codephrase/token. EOF, timeout or cryptographic failure means Disconnected, with no silent reuse of delta state.
7. Bound latest-only capture to 1–2 samples; encoder maintains one canonical sequence and sender serializes writes. No unbounded queues.

G1 integration: `Reader.ResolveVideo` separates payload receipt from codec
acceptance. `lastVideoID`/canonical progress advances only after successful
decode; a separate consumed-wire-ID high-water mark rejects non-increasing
IDs even when earlier payloads failed validation. A recoverable sequence error
discards exactly the bounded payload, invalidates the reference and waits for
a forward KEY. Malformed framing or truncated discard is terminal. Actual
reference bytes and decompression validation are implemented in G2.
After a parser-level `ErrNeedKeyframe`, also invalidate the codec reference.
After codec Decode, pass success/failure to `Reader.ResolveVideo`; only then
read the next video. See [application API](../app/README.md).

Treat even decrypted input as untrusted. Malformed framing, directions, sizes, versions and compressed data must yield bounded errors, never panic or uncontrolled allocation. Room codes, frames, keys and tokens must not enter plaintext logs/telemetry or frame storage. Reuse verified upstream PAKE/E2E mechanisms rather than inventing a handshake. Exact byte fixtures and parser/fuzz obligations are in [TEST_PLAN](TEST_PLAN.md#a-protocol-tests-unit--fuzz-no-network-required).

## Telemetry contract
- `CaptureStats`: captured/queued/dropped samples; `CodecStats`: types, compressed/raw sizes, encode/decode CPU duration; `ProtocolStats`: custom-wire video versus baseline; `TransportStats`: measured RTT and route only when observable.
- Actual video bytes = sum of `18 + compressed payload length` for emitted KEY/DELTA. Exclude CONFIG, PLI/PING/PONG and crypto/relay/network overhead. Label application-level video rate, not NIC traffic.
- Full-frame baseline = independently LZ4-encode each **same emitted sample** as a full frame with the same codec settings, plus 18-byte header. Align actual/baseline in the same rolling 1-second window.
- `savingPct = 100*(1-sum(actualBytes)/sum(baselineBytes))`; may be negative, undefined when denominator is zero. Include baseline CPU cost in benchmarks; label estimates/sampling explicitly.
- `KiB/s = bytes_per_1s/1024` (do not label this KB/s). FPS counters separately track capture, emission, decode and render. Key/Delta mean packet sizes consistently include the header.
- TimestampMs is broadcaster wall clock; do not subtract an unsynchronized viewer clock for E2E latency. Ping/Pong uses initiator monotonic time; decode→render is viewer-local. Unmeasured latency is N/A; MVP hides true E2E.

G2 records successful emission and complete received video separately from
successful decode/render. Baseline and actual emission bytes enter the same
event/window; KEY can reuse its measured full-frame bytes as baseline.
The exact rolling interval is `(now-1s, now]`, with a fixed 4096-event ring.
Overflow is explicit/incomplete, never an exact savings claim. Unknown baseline
or empty window disables savings; negative values remain valid. G2 codec timing
is measured elapsed execution time including baseline, not process CPU usage.
Full process CPU, RTT and render timing require later integration measurements.

## Sources and licensing
The following external observations were recorded on 2026-11-13; the local commit was recorded on 2026-11-15. They are background references, **not fresh release checks or proof of installed tools**. Recheck APIs, build requirements and licenses against the pinned revision at G0.

| Reference | URL / recorded context |
| --- | --- |
| Pinned croc source | https://github.com/schollz/croc/tree/552f40fd2d7593316118370bec0a93478d48e715 |
| Upstream | https://github.com/schollz/croc — MIT; README describes P2P with relay fallback |
| Releases | https://github.com/schollz/croc/releases — v11.6.5 recorded as latest, released 2026-11-07 |
| Module | https://github.com/schollz/croc/blob/main/go.mod — `github.com/schollz/croc/v11`, Go 1.27.3 when checked; main is mutable |
| Packages | https://pkg.go.dev/github.com/schollz/croc/v11/src — comm/crypt/codephrase/pakekey/sshshare existence does not prove duplex integration |
| Wails v3 release | https://wails.io/blog/wails-v3-released/ — v3.0.0 recorded 2026-10-26 |
| Wails changelog | https://wails.io/changelog/ — v3.1.0 recorded 2026-11-01 |
| Installation | https://wails.io/quick-start/installation/ |
| v2→v3 migration | https://v3.wails.io/migration/v2-to-v3/ — incompatible bindings/events |
| Codex AGENTS discovery | https://learn.chatgpt.com/docs/agent-configuration/agents-md |

Verify croc MIT license/notices and licenses of all selected dependencies. Preserve attribution in the redistributed app and any fork; do not copy code of unknown licensing.
