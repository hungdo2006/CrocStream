# PRD — CrocStream Desktop (MVP v1, proposed)

> **Cách đọc:** tài liệu này quy định sản phẩm cần làm gì và người dùng sẽ trải
> nghiệm ra sao. Nếu mới tham gia, đọc [hướng dẫn nhập môn](GETTING_STARTED.md)
> trước. MVP tập trung vào chia sẻ màn hình 1:1 trên Windows và số đo thực tế;
> các hành vi bên dưới là yêu cầu dự kiến, chưa phải tính năng đã triển khai.

**Status:** desktop screen-sharing prototype in development; [G0 feasibility PASS](../app/G0_CLOSEOUT.md), with [G1 framing/config PASS](../app/G1_CLOSEOUT.md) in `app/internal/protocol`. No screen-sharing application is runnable yet. The repository contains pristine upstream `croc/`, planning `docs/`, isolated G0 artifacts and the pure-Go application module in `app/` (D-12). Local/remote >600s secure duplex, minimal Windows native build/runtime and dependency-notice review passed at G0. The latest G0 network measured 48–51ms mean application RTT; old-network limits remain documented. Real capture, media/UI, performance and packaging acceptance belong to later gates. C-01/C-02 were owner-confirmed for G1 on 2026-11-16; other provisional choices remain subject to their checkpoints. Paths are repository-root relative.

This file owns product behavior and UI acceptance. [ARCHITECTURE](ARCHITECTURE.md) owns technical contracts; [TEST_PLAN](TEST_PLAN.md) owns verification; [ROADMAP](ROADMAP.md) owns execution gates. Unresolved differences are tracked in [clarifications](ARCHITECTURE.md#pending-clarifications), not silently resolved by this consolidation.

## Product outcome
Two separate desktop app processes on two Windows 10/11 x64 machines can join a secret-code 1:1 session. Broadcaster shares one screen; Viewer watches live output reconstructed from a lossless Keyframe + selected dirty tiles. Users can start/stop safely and compare bandwidth against a fair whole-frame compressed baseline.

**User value**: private code-based screen viewing with interpretable real-time metrics and an educational demonstration of delta-frame trade-offs. This is a **technical prototype**, not a production conferencing replacement. The app is not a client for stock croc file-transfer sessions.

## Scope and prioritization
| Priority | Included | Excluded / later |
| --- | --- | --- |
| P0 MVP | Two roles, 1:1, screen capture, 1280×720/640×360, 15/24/30 FPS selector, fixed resolution per session, codephrase join, lossless KEY and tile DELTA, safe stop, errors, core telemetry, Windows package | audio, recording, cloud, account, multi-user |
| P1 | Webcam source, capture device switch when stopped, reliable settings, latency tooltips, relay configuration/self-hosted demo | webcam must not block P0 |
| P2 | macOS/Linux packaging + capture permissions, accessibility, optional detailed benchmarks | cross-platform success is not claimed without real builds |

The screen source is a real display chosen from enumerated displays, with user consent; do not capture screens before Start. For Webcam P1, report missing camera/permission, release device on Stop, test separately per OS.

## Functional requirements and acceptance criteria

### US-01 Broadcaster setup
- Home offers **Share** and **Watch**; Stream page enumerates available screens, desired resolution and target frame rate; entering app does not automatically broadcast.
- On Start: validate selection, request OS capture permission as required, start session in explicit `Preparing → WaitingForViewer → Live` states. Show code only once actual pairing/relay preparation succeeds; display **Waiting for Viewer** honestly.
- Room code can be copied, rotated (only by stopping/recreating a session), never logged or included in crash reports. Input code validation follows actual upstream integration results, not a fabricated regex.
- Preview appears locally when capture works, whether viewer connected or not; preview and remote video are separately labeled. Error “source missing/permission denied” is actionable.

### US-02 Viewer join
- Viewer enters secret code; flow `Idle → Connecting → Authenticating → AwaitingKeyframe → Watching` or explicit error state. Wrong code and relay failure are distinguishable when safely observable; prevent repeated uncontrolled reconnect attempts.
- Viewer renders only after validated CONFIG + KEY. Each valid DELTA reconstructs the sender's canonical reference exactly. Invalid DELTA does not mutate screen/reference; requests keyframe at most once per second.
- Stop/Leave displays a clean idle screen, closes resources, clears the room code and does not leave network or capture goroutines running.

### US-03 Deterministic codec
- Follow the [media protocol](ARCHITECTURE.md#media-protocol-v1): validated CONFIG before video, first video KEY id1, lossless LZ4-frame RGBA and deterministic dirty tiles.
- Default tiles 32×32, RGB MAE threshold `tau=4`, KEY fallback above 65% dirty tiles. Key interval is configurable from **1–120 emitted frames**, default **30** (C-01, owner-confirmed 2026-11-16).
- CONFIG permits only **640×360** or **1280×720**, never mixed width/height pairs (C-02, owner-confirmed 2026-11-16).
- Under backpressure skip capture **before** encoding; do not skip intermediate deltas and continue decoding later frames. No permanent sender/viewer drift, even with noise filtering (`tau=4`).
- No pixel-perfect match to every raw capture is promised when threshold >0; the acceptance proof compares **receiver output to sender canonical transmitted image**. Test `tau=0` where every pixel change should be represented losslessly.

### US-04 Telemetry, measurement, disclosure
- Update presentation every ~500 ms without disturbing video rendering. Show sender emitted FPS, viewer decoded/render FPS, sent/received KiB/s and Key/Delta mean bytes.
- Savings percent follows the [telemetry contract](ARCHITECTURE.md#telemetry-contract), comparing full-frame LZ4 baseline and actual video bytes in the same rolling one-second window. Allow 0% or negative savings; tooltips explain baseline, period and overhead exclusions.
- Show optional measured RTT and viewer-local decode-to-render latency. E2E latency is “N/A” unless clock synchronization was implemented and tested.
- Add session uptime, status and dropped capture samples. Never identify transport as “Direct” or “Relay” without verified mechanism data.

### US-05 Error handling & lifecycle
- All long-running work observes cancellation; `Stop` / window close is idempotent. Close capture, codec worker, local WebSocket server, transport and telemetry; no stale screen lock. Target cleanup within 3 s under non-hung conditions; report timeout rather than falsely declaring success.
- Handle invalid code, inaccessible public relay, sender disconnect, missing device, permission denied, oversized packet, malformed/corrupt delta, invalid protocol version, decoder lag and connection cancellation.
- A receiver with invalid reference holds last good displayed frame, shows Recovering, and requests a new keyframe; on unrecoverable channel loss return to Disconnected (no unsafe reuse of old reference on reconnect).

### US-06 Demo reproducibility
- Include a reproducible demo with two processes (same machine loopback is acceptable for protocol tests; remote machine/relay additionally required for the transport acceptance), fixed resolutions and recorded test conditions.
- Provide commands to build/start/test; no real-world benchmark claims without named machine specs, connection type, duration and sample dataset.

## Non-functional gates (first benchmarks, not guaranteed claims)
- Video target: 24 FPS at 1280×720 screen content on a specified Windows test machine; performance is measured as p50/p95 over a 2-minute demo, and deviations reported.
- Memory: no unbounded growth during a >=10-minute stream; record max RSS, allocations per frame, queue size and goroutine count. No arbitrary zero-allocation promise.
- LAN presentation target: viewer can recover from deliberate decoder invalidation after next KEY / PLI without crash or visually corrupted output. Relay performance is recorded separately; 24 FPS over public relay is **not** guaranteed.
- Build and release criteria: Go unit/fuzz tests, frontend typecheck/lint/build, Windows desktop build on actual Windows, operator steps in README, dependency license notes and no embedded secrets.

## Out of scope
Audio streaming, chat, multi-viewer, credential systems, recording/VOD, remote control, mouse/keyboard input injection, covert capture, analytics upload and compatibility with arbitrary stock croc CLI messages. No proprietary closed-source requirement is assumed; comply with upstream MIT notices.

## Open acceptance dependencies
Croc's persistent duplex stream viability, relay usage/capacity, security handshake and platform build matrix are controlled by [B-01 through B-05](ARCHITECTURE.md#decisions-and-blockers) and [G0](ROADMAP.md#gates). This PRD is not an instruction to bypass the gate.

## UI and interaction

### Design and screens
Desktop-first dark layout: default ~1150×760, minimum ~960×640, scrolling at smaller sizes. Palette: `slate-950` background, `slate-900` cards, `indigo-500` primary, `emerald-500` connected, `amber-400` waiting/recovering, `rose-500` error/stop. System font; `font-mono tabular-nums` for metrics/code. Preserve 16:9 canvas with letterboxing, never stretch.

High-contrast focus rings, full keyboard navigation and accessible labels remain explicit UI requirements. Their delivery priority versus the P2 accessibility entry is unresolved (C-05).

| Screen | Layout and controls |
| --- | --- |
| Home | Accessible `Share Screen` / `Watch Stream` cards; optional relay configuration/status link. No green “relay connected” until probed. |
| Broadcaster | Left: labeled local preview, selected screen, Start/Stop, dashboard. Right: screen/resolution/FPS selectors, advanced keyframe interval and tau, code + Copy only after session preparation, pairing status, actual route only if observable. Dashboard includes video/baseline KiB/s, savings, emitted FPS, key/delta counts and sizes, dropped captures, uptime. |
| Viewer | Code input, explicit connection progress, placeholder until valid KEY, large viewing canvas, Leave. Show bandwidth, decoded/rendered FPS, optional RTT/local decode→render delay and last frame kind. |

### State machine
```text
Home
 ├── Share → Idle → Preparing → WaitingForViewer → Live → Stopping → Idle
 │                  └─────────(Failure)──────────────→ Error → Idle
 └── Watch → Idle → Connecting → Authenticating → AwaitingKeyframe
                           └── (Failure) → Error    └── Watching → Recovering
                                                               ↕ AwaitingKeyframe
                                               Watching/Recovering → Disconnected
                                               Watching/Recovering → Leaving → Idle
```
- Disable Start during Preparing/Live/Stopping and Join during Connecting/Authenticating. Stop/Leave remain available in any active state and are idempotent.
- Back from Live requires Stop and cleanup; window close cancels capture, network and local bridge. Lock settings during Live; source/resolution changes require Stop.
- Show `Waiting for viewer` until authentication and successful first KEY delivery, not merely relay availability.
- Preserve entered code on inline validation errors when useful. Offer Retry only when safe and rate-limited.
- No screen/camera access before explicit user action. Permission denial, missing source, wrong code, unreachable relay, viewer lag, malformed-frame recovery and broadcaster END/EOS need actionable, non-blocking states.
- Use placeholders rather than corrupt frames or misleading 0 ms. On local image-decode failure show `Preview unavailable` and diagnostics without terminating media transport.
- The frontend must prevent stale frames from a prior session appearing after rejoin; implementation and cleanup are specified in the [local bridge](ARCHITECTURE.md#local-webview-media-bridge).

### Telemetry presentation
| UI label | Meaning | Empty state |
| --- | --- | --- |
| `Video protocol rate (KiB/s)` | Application video packet bytes in last 1 s, not NIC traffic | `—` before first frame |
| `Full-frame LZ4 baseline (KiB/s)` | Same emitted samples encoded as full frames with headers | `—` if not measured |
| `Delta savings (%)` | Contract formula; may be negative | `—` when denominator is zero |
| `Capture / emitted / decoded / rendered FPS` | Separate, appropriately named counters; label encoded FPS separately if collected | `—` when idle |
| `Avg Key / Delta (KiB)` | Mean application packet sizes in sample window | `—` without samples |
| `RTT (ms)` | Measured Ping/Pong RTT, not one-way latency | `N/A` |
| `Viewer decode→render (ms)` | Local viewer measurement | `N/A` |
| `True end-to-end latency` | Requires tested cross-host clock synchronization | Hidden in MVP |

Backend UI snapshots every ~500 ms; rates/savings use rolling 1-second windows. Status/statistics events must not run at video frame rate; media uses binary loopback WebSocket. No production mock metrics, room codes, route types or connection states. Estimated/sampled baseline values must be labeled.

UI acceptance procedures, including keyboard/screen-reader use and both window sizes, are in [TEST_PLAN](TEST_PLAN.md#e-guisystem-manual-acceptance).
