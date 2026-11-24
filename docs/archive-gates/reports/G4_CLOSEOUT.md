# G4 closeout — secure two-peer media integration

**Status: G4 PASS — local desktop integration plus >=600s remote synthetic
media/control/reference proof and independent cleanup.**
Date: 2026-11-17.
Starting repository HEAD: `5322d6a` (G3); starting working tree clean.

## Authorization and architecture

The owner approved reuse of the existing G0 croc adapter and its PAKE/AEAD
implementation, with no crypto or media-wire change. The owner approved reuse
of the earlier EC2 setup for remote testing, and approved actual capture only
for a local two-app test. Remote proof therefore uses **synthetic images**.

`app/g0/poc/duplex` exposes the existing `Host`/`Join`/session types without
copying cryptography. `app/internal/transport` adapts that API. G0 additions
only clear the relay password after admission and provide an auth-progress
callback; upstream crypto, PAKE purpose, transport records and media protocol
bytes remain unchanged. Existing G0 regression tests run against the same
adapter module.

`app/internal/session` owns one protocol reader and one serialized writer per
peer. Host sends CONFIG, KEY1 and sequential KEY/DELTA from latest-only raw
captures. Viewer validates/decompresses every video packet before reference
commit, retains last-good pixels on invalid input and sends a rate-limited PLI.
The next emitted capture is forced KEY. Ping/Pong shares the serialized writer;
only a matching outstanding nonce records application RTT. EOS, Stop, EOF and
malformed input terminate the session; no transparent reconnect reuses a stale
reference. A 200ms best-effort END window is followed by socket close to unblock
the peer reader/writer. G0 transport cancellation owns the duplex socket.

JPEG is only each peer's local canvas bridge. Media over croc is authenticated-
encrypted protocol-v1 RGBA/LZ4; no route is labeled Direct/Relay by UI. The
code is shown to host only after relay preparation, cleared on pairing/Stop and
never logged. Relay passwords/codes are never command-line flags or files in
the proof harness. The adapter clears relay admission password from persistent
Listener options once admission succeeds. This is not an independent security
audit; inherited upstream relay admission bounds remain as recorded at G0.

No independent security audit or public-relay compatibility is implied.
Record nonce/session-budget limits remain unchanged. See the G0 closeout for
upstream admission allocation bounds, invitation entropy and key-erasure limits.

## Implemented

- `app/internal/transport`: public Host/Join wrapper over G0 secure duplex;
  relay `host:port` and password validation; auth progress surfaced without raw
  connection/key access.
- `app/internal/session`: role/state lifecycle, CONFIG/codec/telemetry wiring,
  exact frame/reference counters/bytes, RTT, PLI/Pong, bounded channels and
  cancellation/cleanup.
- `app/internal/protocol/stream.go`: `LastReceived` reports only a complete,
  structurally bounded video header+payload, including sequence-discarded
  records; truncated/malformed packets are not counted as received.
- `app/internal/bridge`: same G3 session-owned loopback security, distinct
  `/preview` broadcaster-local and `/viewer` viewer-local role endpoints.
- `app/internal/desktop` and React/Wails: Home Share/Watch/local-preview,
  source/size/FPS/tau/key interval, relay inputs, secret invitation, explicit
  state, canvas and measured capture/emitted/received/decoded/rendered FPS,
  video/baseline KiB/s, Key/Delta means, savings, RTT, drops and uptime.
- `app/g4/proof`: separate host/viewer child processes and an owned loopback
  croc relay; fixed synthetic images, independent expected-frame/hash checks,
  media bytes, recovery, control and cleanup.
- `app/g4/Invoke-SshProof.ps1`: owner-approved remote synthetic proof workflow;
  strict host-key pinning, unique relay/password/firewall rule, transfer hash,
  result validation and cleanup of only run-owned remote resources.

## Verification evidence

### G0 adapter regression

The G0 module remained the implementation source and its tests passed after the
progress/secret-lifetime changes:

| Command | Result |
| --- | --- |
| `go -C app/g0/poc test -count=1 -timeout=120s ./...` | PASS: cmd 1.896s, duplex 2.448s, transport 5.242s, PAKE 1.071s |
| `go -C app/g0/poc test -race -count=1 -timeout=180s ./...` | PASS: cmd 2.835s, duplex 3.066s, transport 16.956s, PAKE 1.681s |
| `go -C app/g0/poc vet ./...`; `go -C app/g0/poc mod verify` | PASS; all modules verified |

New test `TestHandshakeProgressOccursOnlyAtPeerAuthentication` verifies the
phase callback on both roles. `TestRelayPasswordNotRetainedAfterAdmission`
verifies listener admission password is cleared. No crypto/wire change.

### Local authenticated synthetic media

Command (Windows, race-enabled proof peer binary):

```powershell
go build -race -buildvcs=false -o "$env:GOBIN\g4proof-final.exe" ./g4/proof
& "$env:GOBIN\g4proof-final.exe" -duration 30s
```

PASS `LOCAL_MEDIA_PASS`, separate host/viewer OS processes over the actual
upstream croc relay implementation on 127.0.0.1. Both generated expected RGBA
sequences and independent final hashes matched; images were synthetic and
never written to disk.

| Metric | Broadcaster | Viewer |
| --- | ---: | ---: |
| Active duration | 30.468s | 30.474s |
| Emitted / received media packets | 390 | 390 |
| Decoded packets | — | 389 (one consumed during recovery) |
| Keys / Deltas | 14 / 376 | 14 / 375 |
| PLI accepted / requested | 1 | 1 |
| Pongs | 30 | 30 |
| Video bytes sent / received | 118,981 | 118,981 |
| Last ID | 390 | 390 |
| Final synthetic RGBA SHA-256 | `32e9ff4d59a671fd1bf3783f8f3032732ad9d703ccb06bdb6d3a016296ba2373` | same |

This table preserves an earlier 30s checkpoint. The final race-enabled binary
also completed **600s** on the stabilized implementation:

```powershell
& "$env:GOBIN\g4proof-verified.exe" -duration 600s
```

Saved raw sanitized result:
[local-synthetic-600s-20261117.json](../evidence/g4/local-synthetic-600s-20261117.json).
It was fed back to the binary's independent `-mode verify -duration 600s`
verifier: **RESULTS_VERIFIED**.

| Final 600s metric | Host | Viewer |
| --- | ---: | ---: |
| Active seconds | 600.400959 | 600.405236 |
| Emitted / received packets | 6,863 | 6,863 |
| Decoded packets | — | 6,862 |
| Key / Delta | 230 / 6,633 | 230 / 6,632 |
| Total application video bytes | 2,028,915 | 2,028,915 |
| PLI accepted / requested; recoveries | 1 | 1 request / 1 recovery |
| Matching Pong observations | 594 | 599 |
| Final ID | 6,863 | 6,863 |
| Goroutines start → end | 2 → 1 | 2 → 1 |
| Last canonical hash | `adaad170fa45d43ea07a37fba21d6876d591994d055eecd7ce8d2bb586f9f1f7` | same |

This is a correctness/lifecycle proof, not a 24-FPS benchmark: it ran with the
race detector, independent per-image hash work and other verification jobs
sharing the laptop. The synthetic producer blocks on its bounded admission
queue and emitted fewer samples than the requested clock ticks. No post-encode
video packet was dropped. The one failed decode is the deliberate PLI test.

The verifier checks role, minimum duration, contiguous emitted ID accounting,
media types/bytes, successful decode, PLI recovery, bidirectional Ping/Pong,
EOS, finite durations and exact final expected synthetic pixels. `LOCAL_MEDIA_PASS`
is not remote or capture proof.

### Local two-desktop-app integration (actual display capture, loopback relay)

Owner explicitly allowed actual capture for local test only. `g4/ui-proof.mjs`
started two instrumented Wails processes with distinct temporary WebView
profiles and a random-password upstream croc relay bound to loopback. Host
selected the actual primary 1920×1080 display and started at 1280×720/24 FPS;
viewer joined using the private host invitation. The test invalidated viewer
reference and verified PLI → KEY recovery. Pixels remained in RAM. No screenshot,
trace, invitation, password, token or packet payload was persisted/logged.

An earlier 30s desktop checkpoint after G4 state/telemetry/UI integration:

| Metric | Host | Viewer |
| --- | ---: | ---: |
| Measured run | 30.819s | 30.819s |
| Capture / emitted / received / decoded FPS | 24 / 24 / 0 / 0 | 0 / 0 / 24 / 24 |
| Emitted / received packets | 739 | 739 |
| Successfully decoded | — | 738 (one intentional recovery discard) |
| Keys / deltas | 32 / 707 | 32 / 706 |
| PLI accepted / requested; recovery | 1 / 1; host next-Key observed | 1 request; 1 recovery |
| Pongs | 30 | 30 |
| Media bytes | 14,218,633 sent | 14,218,633 received |
| Rolling host video rate | 264.89 KiB/s | — |
| Rolling full-frame baseline | 6,270.97 KiB/s | — |
| Rolling host delta savings | 95.78% | — |
| RTT latest measured sample | 2.115ms | 3.978ms |
| End IDs | 739 | 739 |
| Stop / clear / errors | 186ms / cleared / 0 page errors | cleared / 0 page errors |

One frame can be in flight across the snapshot, so emitted/received/decoded
counters need not match at every instant. These are **same-machine relay**
values, not Internet latency/FPS or a universal savings claim. Actual remote
screen pixels were not sent.

Final desktop rerun, including validated viewer END and post-stop counters:

```powershell
go build -tags production,g3test,g4test -buildvcs=false -o "$env:GOBIN\crocstream-g4-verified.exe" .
node g4/ui-proof.mjs "$env:GOBIN\crocstream-g4-verified.exe" "$env:GOBIN\g4proof-verified.exe" 30
```

**PASS**, raw sanitized output:
[local-desktop-final-20261117.jsonl](../evidence/g4/local-desktop-final-20261117.jsonl).
Paired at 2026-11-17T10:19:55.512Z, total observation including Stop/checks
31.143s. Final settled counters after Stop:

- Host emitted **738** (26 KEY / 712 DELTA); viewer received **738** and
  decoded **737** (26 KEY / 711 DELTA). Both last IDs **738**.
- Exactly **7,254,398 B** sent/received, including media headers only. PLI
  requested/accepted once; viewer recovery count 1; Pong observations 30 each.
- Active snapshot captured/emitted/decoded FPS 24/24/24; received FPS 23 at
  its separate sample boundary. Canvas contained decoded non-black image data;
  the harness did **not** compute a rendered-FPS average for this G4 run.
- Loopback RTT latest sample **0.5155ms** each side; rolling sender savings
  **95.778%**, baseline **6,349.37 KiB/s**, video **268.05 KiB/s** at the active
  snapshot. Separate peer snapshots need not align at every packet boundary.
- Keyboard Stop plus state polling/canvas checks took **590ms**, both canvases
  cleared, viewer observed validated END, page errors **0**. Host Idle,
  viewer Disconnected, invitation empty, both peers quit gracefully and both
  observed bridge listeners refused new connections afterward.
- Host goroutines **12 → 20 → 12**, GDI **12 → 15 → 12**;
  viewer goroutines **12 → 19 → 12**, GDI **12 → 12 → 12**.
  Process handles did not return to startup counts before process exit:
  host 388→487, viewer 394→467. Do not claim a zero-handle-leak proof from
  these samples. The G3 native resource tests remain separate evidence.

All desktop memory/process values refer to each Go process, not aggregate
WebView subprocess RSS. Full UI stress/performance profiling remains G5.

### Tests, UI and build

- `go test -count=1 -timeout=180s ./...`: PASS on prior G4 checkpoint; G4
  session/media local transport and integration packages included.
- Final `go test -race -count=1 -timeout=600s ./...`: **PASS** on the stabilized
  implementation: proof 2.987s, bridge 4.854s, capture 1.836s, delta 413.134s,
  frame 1.851s, preview 3.709s, protocol 2.199s, session 46.926s,
  telemetry 1.590s; no detected races on executed paths.
- Final ordinary `go test -count=1 -timeout=180s ./...`: **PASS**,
  proof 2.168s, bridge 4.578s, capture 1.204s, delta 24.499s, frame 1.156s,
  preview 3.273s, protocol 1.523s, session 14.117s, telemetry 1.190s.
- Final `go vet ./...`; `go mod verify`: **PASS**, all modules verified.
- Final G0 ordinary regression: **PASS**, cmd 1.948s, duplex 2.508s,
  transport 5.208s, PAKE 1.103s; G0 vet **PASS**.
- `go vet ./...`, module verification, `CGO_ENABLED=0` production library/app
  build, Wails Windows build: PASS on the current implementation checkpoints.
- `npm run typecheck`, `npm run lint`, `npm run test` (2/2), `npm run build`:
  PASS after Wails v3 binding regeneration. Wails binds 1 service/10 methods.
- Frontend `/share`/`/watch` path and sample local desktop proof passed; final
  post-metrics UI proof was rerun after the latest `receivedFPS`/average-size
  presentation changes below.
- G0, G1, G2 and G3 closeouts remain applicable; no `croc/` source changed.

Bounded fuzz campaigns on final parser/adapter boundaries:

- `go test ./internal/protocol -run '^$' -fuzz '^FuzzReadStream$'
  -fuzztime=15s -fuzzminimizetime=1s -parallel=2`: **PASS, 743,789 executions**.
- `go -C g0/poc test ./internal/transport -run '^$'
  -fuzz '^FuzzHandshakeBody$' -fuzztime=15s -fuzzminimizetime=1s -parallel=2`:
  **PASS, 206,963 executions**.

## Remote two-machine acceptance — PASS

On 2026-11-17 the owner supplied the restarted instance's new IP
`47.128.220.28`. Its SSH key matched the existing G0 trust record with strict
host-key checking. Both a 30s smoke and a **600.277/600.282s** two-machine run
passed using the same race-enabled binary. The final run emitted/received
**6,839 media packets / 2,021,033 bytes**, matched the final canonical hash and
all counters, exercised reverse PLI/KEY recovery and 600 Pong observations per
peer, delivered END and returned both peers to one goroutine.

Saved output was independently reverified; separate SSH inspection found no
run-owned processes/firewall rules or TCP 9009 listener left. No real screen
was transmitted to EC2. Complete commands, host identity, source/binary hashes,
observations and limits: [G4_REMOTE_PROOF](G4_REMOTE_PROOF.md).
This closes the remote evidence gap and **G4 overall PASS** at prototype scope;
G5 UX/stress, G6 packaging and real remote capture/render performance are not
retroactively passed.

**Historical checkpoint:** the blocker below describes the first local closeout.
It was resolved by the actual remote proof above, not by treating reachability
or smoke alone as completion of the >=600s acceptance.

The owner authorized reuse of EC2 but is updating ingress. The previously
pinned G0 endpoint `54.151.152.78:22` timed out on strict-host-key probes with
10–15s connect bounds. No SSH host-key check was relaxed; no firewall/security
group was changed by the agent and no remote service was started.

At that earlier checkpoint, the owner confirmed they would update AWS ingress.
Remote G4 synthetic proof was **not run / BLOCKED** until the current IP,
the G0-verified host key and TCP 22/9009 source-restricted access were available.
Real screen capture to EC2 was and remains unauthorized for this proof.

The earlier local-only checkpoint did not close G4. The later successful
remote media/control/hash proof and independent cleanup above now close it.

## Reproduction and final artifact provenance

From `app/`, after `. .\g0\Use-Toolchain.ps1`:

```powershell
go test -count=1 -timeout=180s ./...
go test -race -count=1 -timeout=600s ./...
go vet ./...
go mod verify
$env:WAILS_EXP_USE_WAKE='1'
wails3 build
```

Local and remote verification are complete; see the remote report above.
Usage/proof instructions: [g4/README](../../../app/g4/README.md).

| Artifact | SHA-256 |
| --- | --- |
| Ordinary final `app/bin/crocstream.exe` | `07BDBE9AD35E68B8CF9E411AEF9F21F57EFC92EA33BDFA1436B28A650145FDB8` |
| Test-only `crocstream-g4-verified.exe` | `9E7A834D2E46A8B5890AF5C3A51FA8640A9F51465DBE88C233A143D197740AE6` |
| Race-enabled `g4proof-verified.exe` | `203C7186172F36CB0CE83931792C534284502EE46724DD0B2E24FE1905352A37` |
| Saved synthetic 600s JSON | `D121C15D50ED50704EB065536D96FFB09F004C8002D6C83F53D42F6D1EF353A0` |
| Saved desktop JSONL | `AF5FECEDB0C5A22D5B0112657E303817221FAA99B8CE32BB9DD9E5F8B0B3D7E1` |

Ordinary Wails build: **PASS**, 13.2 MiB, 4.3s final build command. It contains
no test CDP or injected-decoder method. The desktop evidence uses the same
capture/codec/session/frontend code with explicit test instrumentation; it is
not falsely described as a UI automation run on the ordinary binary.

## Dependency notices and failures retained

Source minimums selected `coder/websocket 1.8.15` (from G3's 1.8.14) and graph
dependency `godbus/dbus/v5 5.2.4`; the graph is pinned in go.mod/go.sum and uses
the owner-approved G0 dependency closure. See [G4 notice inventory/scope](../../../app/g4/licenses/README.md).
No third-party crypto source was copied or altered in `croc/`.

The notice collector was rerun for the linked G4 application: **178 records**,
with exact inventory hashes and inherited G0/G3 obligations in
`g4/licenses/README.md`. It is not final G6 shipping/legal acceptance.

Intermediate failures were corrected, not relabeled passes:

- Public façade initially returned a typed-nil `*Session` on failed Accept /
  Join, causing controller Close to panic. Wrapper now returns nil interface
  on errors; G0 and application regressions passed.
- Race test found validated END racing an in-flight socket write; a stopped
  reader now unblocks the sole writer after a bounded delay, and only validated
  remote END is treated as normal termination.
- New auth callbacks/capture-option fields initially required test call-site
  updates and a missing atomic import. Those compile errors were corrected.
- Join retry limits caused the 100-cycle lifecycle test to hit the production
  guessing-rate limiter. That test now configures private test-only limits,
  while a separate test exercises the normal 2s cooldown / 5 attempts/minute.
- Slow-UI test previously used a 5s timeout and a blocking synthetic source
  under heavy concurrent race load; it now models latest-only raw admission
  and compares **every emitted** frame to decoded count, under a 15s bound.
- Proof verifier gained explicit RTT checks; its success fixture was updated
  to declare the required actual-measurement flag. Verifier rejects incomplete
  counters/hashes/duration/RTT and reports which check failed.
- Typecheck before binding regeneration reported stale model fields. Direct
  CLI binding generation emits JS with JSDoc, so tsconfig now validates those
  generated JS bindings too; the manifest also generates typed bindings.
- A race build attempted after `CGO_ENABLED=0` failed as expected; reran with
  the existing compiler/CGO enabled. Some shell invocations used wrong CWD/
  unsourced tool environment and were rerun correctly; no installation required.
- Saved-evidence checker initially used a missing optional JSON property under
  PowerShell strict mode; corrected to test property existence, then PASS.

These diagnostics are not changes to peer data or acceptance criteria.

## Final workspace checks

- Local target/heading-anchor scan: **160 links across 11 documents, PASS**.
- Saved synthetic JSON duration/byte/hash checks and independent verifier:
  **PASS**; saved desktop JSONL contains one passing result and graceful
  two-peer cleanup, **PASS**.
- `gofmt -l` on changed Go code and `git -c safe.directory=D:/CrocStream
  -c core.safecrlf=false diff --check`: **PASS**. New-code conflict/whitespace
  scan passed. Remote helper PowerShell AST and Node harness syntax passed.
- Upstream-only tracked diff and untracked status: **pristine `croc/`**.
- Independent process/listener inspection found no named G4 proof/desktop test
  processes and no TCP 9009/9245 listeners left after tests. Each desktop
  harness also checked its actual ephemeral bridge listener ports.
- G3 local-preview regression on the final G4 binary: **PASS**, 3.108s measured,
  capture/JPEG/render 24/24/24 FPS, 92 captures, no dropped images, Stop/checks
  113ms, canvas clear, GDI/goroutines back to baseline, both window sizes and
  five rapid cycles, graceful exit and five observed bridge ports closed.
  The G3 helper now selects Local Preview from G4's new Home before Start.
- Final G4 harness startup/failure cleanup refinement (relay readiness now
  inside `try/finally`, private spawn/pipe errors suppressed): Node syntax
  **PASS**, followed by another 5s actual two-app proof **PASS**: 126 emitted/
  received, 125 decoded, one PLI recovery, 1,742,042 media bytes, Stop/checks
  179ms, validated END, both canvases clear, zero page errors and two closed
  bridge ports. No application runtime code changed for this harness refinement.
- Remote follow-up completion: **170 links/anchors across 11 current handoff
  documents PASS**, both remote JSONL results and separate cleanup/source
  fingerprints cross-checked, current diff check passed and `croc/` remains
  pristine. Remote report records the fresh full unit/race/vet/module
  verification commands and measured results.

## Files changed / boundaries

G0 transport façade/callback and password lifetime; G4 transport/session,
desktop service, proof harness, tests, frontend Share/Watch flow, bindings and
docs. `croc/` remains pristine. No commit or push was performed during the
implementation phase. Next gate: **G5**, not started.

## Pre-push verification (2026-11-17)

The owner requested a G4 check and GitHub push if PASS. Reviewed the current
specification, implementation and original local/remote evidence. Environment:
existing portable Go 1.27.4 windows/amd64, Node 24.22.1 and npm 12.2.1.
Go commands used `. .\app\g0\Use-Toolchain.ps1` from repository root.

| Command / check | Observed result |
| --- | --- |
| `go -C app test -count=1 -timeout=180s ./...` | PASS: proof 2.767s, bridge 3.851s, capture 0.731s, delta 9.573s, frame 0.699s, preview 2.541s, protocol 1.775s, session 12.252s, telemetry 0.700s |
| `go -C app test -race -count=1 -timeout=600s ./...` | PASS on final state: proof 2.577s, bridge 4.462s, capture 1.627s, delta 104.877s, frame 1.652s, preview 2.747s, protocol 1.852s, session 22.013s, telemetry 1.532s; no detected race |
| `go -C app vet ./...`; `go -C app mod verify` | PASS; all modules verified |
| `go -C app/g0/poc test -count=1 -timeout=120s ./...` | PASS: harness 2.851s, duplex 2.380s, transport 5.393s, PAKE 1.966s |
| `go -C app/g0/poc test -race -count=1 -timeout=180s ./...` | PASS: harness 2.586s, duplex 2.977s, transport 11.278s, PAKE 1.590s |
| `go -C app/g0/poc vet ./...`; `go -C app/g0/poc mod verify` | PASS; all modules verified |
| `npm --prefix app/frontend run typecheck`; `npm --prefix app/frontend run lint`; `npm --prefix app/frontend run test` | PASS; 2/2 frontend tests |
| `node --check app/g4/ui-proof.mjs`; `node --check app/g3/ui-proof.mjs` | PASS |
| From `app/`: `. .\g0\Use-Toolchain.ps1`; `$env:WAILS_EXP_USE_WAKE='1'`; `wails3 build` | PASS, all 7 steps ran in 6.6s, including locked dependency install, bindings, frontend build and Windows executable |
| SHA-256 comparisons against recorded values | PASS: rebuilt ordinary executable, 6 evidence/manifest files and 2 notice artifacts; all 21 recorded runtime-source hashes also match |
| Recorded hash-pinned `g4proof-remote-20261117.exe -mode verify` on saved `results` arrays | `RESULTS_VERIFIED` for local synthetic 600s, remote 30s smoke and remote 600s acceptance |
| Inline saved desktop/cleanup assertions | PASS: desktop 738 packets / 7,254,398 B, Stop 590ms, PLI recovery, validated END, zero page errors, graceful peers and closed bridge ports; saved independent remote cleanup records have zero owned processes/rules/listeners |
| PowerShell AST parse of `app/g4/Invoke-SshProof.ps1` | PASS |
| Local Markdown target/anchor check | PASS: 172 targets/anchors across 12 documents |
| `gofmt -l` on application/G4/G0 changed sources | PASS, empty output |
| `git -c safe.directory=D:/CrocStream -c core.safecrlf=false diff --check` before staging | PASS |
| Staged `diff --check` excluding `app/g4/licenses/generated-20261117/THIRD_PARTY_NOTICES.txt` | PASS; the verbatim third-party notice snapshot retains original whitespace and its recorded hash |
| `git -c safe.directory=D:/CrocStream diff --exit-code -- croc/` and upstream-only tracked/untracked status | PASS, pristine |

The recorded two-process desktop and long local/remote runs were verified from
their saved evidence, not rerun during this pre-push check. No new SSH session,
real display capture or EC2 changes were made. Ordinary tests skip actual
capture without the consent variable. Bounded fuzz campaigns were reviewed;
the unit suites reran their seeds, not a new fuzz campaign. G4 is reconfirmed
**PASS** for local desktop integration plus remote synthetic media/control;
real remote-screen/render performance and G5/G6 acceptance remain outstanding.
