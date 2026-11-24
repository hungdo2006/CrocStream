# G5 closeout — UX, telemetry and resilient behavior

**Status: G5 PASS for the measured Windows MVP UX/telemetry/resilience gate.**
User authorized G5 implementation on 2026-11-17.
Starting HEAD: `d31dfb75440b3994f900332493b167869ac3b510` (G4), clean tree.
Actual 600s stress, final-binary UI follow-up and final checks below passed.

## Implementation

- `frontend/src/stream.tsx`: waiting-image/recovery UX; keep last valid image
  during recovery; clear generations on Stop/rejoin; rate-limited manual
  local-display reconnect; Copy feedback, display refresh and retry countdown.
- Dashboard: real waiting-capture FPS, uptime, empty baseline states,
  accessible descriptions of rolling bytes/savings/overhead, incomplete-window
  warning and honest N/A for optional viewer decode→render.
- `internal/session/peer.go`: last successfully emitted/decoded KEY/DELTA kind
  exposed as numeric-snapshot metadata, without changing packet bytes.
- `frontend/src/main.tsx`: Back awaits pending Start and session Stop before
  leaving; timeout stays actionable. Local preview hides prior-session metrics.
- `frontend/src/style.css`: status colors, recovery overlay, keyboard-focusable
  metric explanations and responsive overflow/letterboxing.
- `internal/session/controller.go`: numeric retry budget, capture/queue/drop
  counters, codec/baseline/JPEG timings; sanitized errors distinguish observed
  pre-authentication vs authentication stages without guessing the cause.
- `internal/desktop/diagnostics_windows.go`: native process CPU time and logical
  CPU count for reproducible numeric benchmark interpretation.
- `main_windows.go`: Share/Watch application title.
- `internal/session/g5_test.go`, `frontend/src/latest.test.ts`: retry-window
  expiry, waiting telemetry/token rotation, truthful shutdown timeout, stale
  decode rejection after rejoin and image-failure isolation.
- `g5/ui-proof.mjs`, `g5/README.md`, `g5/evidence/`: actual Windows two-app
  acceptance, reproducible commands and sanitized numeric evidence.

No dependency, media-wire, crypto or upstream source change. Existing approved
G0 transport is reused; no new service/tool installed.

## Environment and measured scope

Windows 11 Home build 26300; Intel i7-13650HX, 20 logical processors,
24,883,064 KiB visible RAM. Go 1.27.4, Wails 3.1.0, Node 24.22.1,
npm 12.2.1, TypeScript 6.0.3. Native app process pair + real croc relay on
ephemeral loopback. Actual primary display capture is owner-authorized locally.
No EC2 traffic or frame recording. Dataset is the actual desktop with Share/
Watch windows, not a standardized full-screen video benchmark.
See [environment](../evidence/g5/environment-20261117.json).

Test binary SHA-256:
`031D88BC07A43E1F2443EA5C1013CE7CC1F1635840C16B991EFCE28D55E2D7EA`.
Relay proof binary SHA-256:
`A125FDA976FF0F1C89EB51D00ED82013C4B01A5C67A52AD9716B780A60E1DB56`.

## Commands and results so far

From `app/`, after `. .\g0\Use-Toolchain.ps1`:

| Executed command | Observed result |
| --- | --- |
| `go test -count=1 -timeout=120s ./internal/session ./internal/desktop` | PASS, session 14.403s; desktop has no unit tests |
| `wails3 generate bindings` | PASS, 333 packages / 10 methods |
| `npm --prefix frontend run typecheck` | First run caught optional-stats narrowing; fixed, next run PASS |
| `npm --prefix frontend run lint` | PASS |
| `npm --prefix frontend run test` | Final PASS, all 4 lifecycle tests |
| `npm --prefix frontend run build` | PASS |
| `go build -tags production,g3test,g4test -buildvcs=false -o "$env:GOBIN\crocstream-g5-ui.exe" .` | PASS |
| `go build -buildvcs=false -o "$env:GOBIN\g5-relay.exe" ./g4/proof` | PASS |
| `node g5/ui-proof.mjs "$env:GOBIN\crocstream-g5-ui.exe" "$env:GOBIN\g5-relay.exe" 10` | `G5_LOCAL_UI_PASS`, 23 checks, 11 closed listeners, zero page exceptions |
| Same UI command with `600` | PASS, 600.331s including Stop; 1,166 samples; 23 checks |
| `wails3 doctor` | PASS / no issues; WebView2 157.0.4341.46. Optional packaging/signing tools absent, not installed |
| `$env:WAILS_EXP_USE_WAKE='1'; wails3 build` | PASS, actual Windows production executable, 13.2 MiB |
| `go build -tags production,g3test,g4test -buildvcs=false -o "$env:GOBIN\crocstream-g5-final.exe" .` | PASS, final frame-kind dashboard build |
| `node g5/ui-proof.mjs "$env:GOBIN\crocstream-g5-final.exe" "$env:GOBIN\g5-relay.exe" 15` | PASS, 26 checks, 11 listeners closed; added preserved inline input/cooldown/slow display and frame-kind assertions |
| `node g5/summarize.mjs g5/evidence/ui-600s-20261117.jsonl` and final JSONL | Both `G5_EVIDENCE_VERIFIED`; independently recomputed quantiles, savings, duration/cleanup |
| `node g3/ui-proof.mjs "$env:GOBIN\crocstream-g5-final.exe" 5 packaged stop` | PASS, local-preview regression, 119ms Stop, 6 bridge listeners closed, zero page errors |
| `node g3/ui-proof.mjs "$env:GOBIN\wails3.exe" 5 dev-cli stop` | PASS, actual `wails3 dev`, exact Origin `http://wails.localhost:9245`, 133ms Stop, 5 observed listeners closed, zero page errors |
| Process inspection for `crocstream-g5-ui`, `crocstream-g5-final`, `g5-relay` | Zero owned processes after tests |
| `go test -count=1 -timeout=180s ./...` | PASS, session 14.416s, delta 22.908s; all application packages |
| `go test -race -count=1 -timeout=600s ./...` | PASS, session 47.121s, delta 373.319s; no detected race on executed paths |
| `go vet ./...`; `go mod verify` | PASS; all modules verified |
| `go -C g0/poc test -race -count=1 -timeout=180s ./...` | PASS: harness 2.867s, façade 3.056s, transport 17.196s, PAKE 1.667s |
| `go -C g0/poc vet ./...`; `go -C g0/poc mod verify` | PASS; all modules verified |

Smoke: 1280×720, target 24 FPS; capture/emitted/decoded/rendered p50=24 FPS;
Stop 129ms. GDI 12→15 active→12 stopped (host), viewer 12; both return to
12 Go goroutines. This short smoke alone does **not** close the stress gate.

### Actual >=600s stress

Evidence: [raw numeric JSONL](../evidence/g5/ui-600s-20261117.jsonl),
[independently computed summary](../evidence/g5/summary-600s-20261117.json).
Warmup 3s before measurement; stress starts **2026-11-17T13:50:03.757Z**.
The stress loop runs >=600s; **600.331s including Stop**, which took **141ms**.
All 23 initial UX checks, recovery, rejoin, live native close and cleanup pass.
No full Go/race workload ran concurrently with this performance run.

| Metric | p50 | p95 |
| --- | ---: | ---: |
| Capture FPS, rolling 1s sampled ~500ms | 24 | 24 |
| Emitted / decoded / viewer-rendered FPS | 24 / 24 / 24 | 25 / 25 / 25 |
| Sampled latest host RTT, ms | 3.0702 | 6.5555 |
| Sampled latest viewer RTT, ms | 10.1835 | 20.2998 |
| Sampled mean encode incl. baseline, ms | 12.4047 | 14.1052 |
| Sampled mean baseline subset, ms | 3.4475 | 4.6963 |
| Sampled mean decode, ms | 1.8409 | 2.2544 |

The first 120s post-warmup window has 232 snapshots: rendered mean **24.0905**,
p50 **24**, p95 **25** FPS. These are overlapping-window samples, not a claim
that frame pacing always exceeds the configured 24 FPS. Waiting samples are
not incorrectly counted as emitted video.

Host pre-stop snapshot: 14,500 emitted, 506 KEY / 13,994 DELTA,
135,500,274 video bytes, 14,538 captured, 37 dropped captures (includes
waiting for a viewer). Viewer snapshot is read later during live traffic and
has 14,501 received, 14,499 decoded, last accepted ID 14,500. The one-packet
snapshot skew and intentional PLI invalidation are **not** an exact synchronized
byte/hash acceptance; use G4 for that proof. Both observe 604 Pong responses.
One deliberate recovery completed. Mean JPEG elapsed duration ~22.04ms host /
21.90ms viewer over the session, separately from codec time.

| Native Go process resources, 59 samples | Host | Viewer |
| --- | ---: | ---: |
| Sampled working set first → last, MiB | 69.40 → 73.17 | 52.80 → 66.76 |
| Maximum sampled working set, MiB | 83.66 | 72.92 |
| Maximum sampled Go heap, MiB | 28.38 | 30.33 |
| Handles during stress, min–max | 461–491 | 450–469 |
| Startup → stopped handles | 398 → 480 | 398 → 459 |
| Go goroutines startup → stopped | 12 → 12 | 12 → 12 |
| GDI startup → stopped | 12 → 12 | 12 → 12 |
| CPU, one logical core=100% | 110.22% | 64.25% |
| CPU, fraction of 20-CPU machine capacity | 5.51% | 3.21% |
| Approx. process allocations / video | 195.16 | 138.40 |
| Approx. process allocated MiB / video | 11.51 | 10.84 |

Memory fluctuates inside the measured ranges; no unbounded growth observed
in this run. Native handles do **not** return to startup count (WebView/native
runtime also initializes lazily); do not call this zero handle growth.
GDI and Go workers return, bridge listeners close, and native processes exit.
Allocations are substantial and are **not retained memory**; they include
capture, canonical copies, codec/baseline, JPEG and RPC work. They remain a
profiling opportunity, not a zero-allocation claim.

### Final-binary follow-up and retained failures

The only runtime addition after the long-run binary was the accepted frame-kind
string and dashboard label. The final short run repeated the UX suite and
added inline-input preservation, disabled retry cooldown, 150ms image-decode
lag while media decoding continues, and frame-kind checks. Result:
**26 checks PASS, 146ms Stop, zero page errors, 11 listeners closed**.
[Evidence](../evidence/g5/ui-final-20261117.jsonl).
The new counter is also covered by `TestLastFrameKindIsAcceptedMedia`.
The long-run binary and final binary are separately identified, not conflated.

Two typecheck attempts failed during development: optional-stats narrowing,
then a mock promise type in the added frontend test. Both were fixed before
the final successful typecheck/lint/test/build. Initial independent summary
parsing assumed UTF-8; PowerShell 5.1 `Tee-Object` produced UTF-16LE. The reader
now supports BOM-selected UTF-16LE/UTF-8 and both saved proofs reverify.
Raw evidence is retained in its original encoding.

| Artifact | SHA-256 |
| --- | --- |
| Long-run JSONL | `8156048B8433E601B8FAF1E056EA4086B3A5A1D52A5AB131331599CFB62FF3A3` |
| Final UI JSONL | `CB86DAE4994DE740B75C9BE231B9326887246B2503501495234178B7287313D5` |
| Final test binary | `A4FFB91132C14A7A78B2994EB4B58463154DB8701FDC06B0D93BE70690BB02FC` |
| Production `bin/crocstream.exe` | `D590A9A1B6A8296B5DB5A5549FFD6852EDD630F6F3253D2666192D018936A48D` |

## Acceptance limits

See [verification README](../g5/README.md). RTT is local sampled Ping/Pong, not
one-way latency; JPEG/codec timings are elapsed execution time. Native process
memory excludes WebView2 subprocesses. Screen-reader speech, physical monitor
removal, HDR/protected content and cross-OS tests are not claimed by automation.
An invalid code before authentication cannot always be distinguished safely
from relay admission failure; the UI says what to check, not a fabricated root
cause. No automatic reconnection or reference reuse.

The standardized scrolling-document, moving-window and full-screen-video/noise
**full-app performance matrix is NOT RUN**; G2 pattern/property tests remain
the codec evidence. No universal savings/FPS claim follows from this desktop
dataset. Optional viewer decode→render remains N/A rather than guessing from
unrelated clocks. Production installer/signing and installed-machine startup
are G6; a successful Windows executable build alone does not close G6.

Next gate after verified G5 closeout: G6 packaging/demo handoff.
No commit or push was performed during the G5 implementation phase.

Final document verification: **171 local targets/anchors in 9 documents PASS**;
`git diff --check` PASS; diff against `croc/`, G0, module pins and npm lockfile
is empty. [Final changed-source fingerprints](../evidence/g5/final-source-20261117.json)
identify the implementation and verification scripts retained for handoff.
## Pre-push verification (2026-11-17)

The owner requested a G5 check and GitHub push if PASS. Reviewed the gate,
implementation, new regression tests, original stress/UI evidence and limits.
Environment: existing portable Go 1.27.4 windows/amd64, Node 24.22.1, npm 12.2.1.
Go commands used `. .\app\g0\Use-Toolchain.ps1` from repository root.

| Command / check | Observed result |
| --- | --- |
| `go -C app test -count=1 -timeout=180s ./...` | PASS: proof 2.202s, bridge 4.535s, capture 1.191s, delta 8.471s, frame 1.172s, preview 3.087s, protocol 1.489s, session 13.648s, telemetry 1.169s |
| `go -C app test -race -count=1 -timeout=600s ./...` | PASS on final state: proof 2.581s, bridge 4.410s, capture 1.567s, delta 200.139s, frame 1.602s, preview 2.443s, protocol 1.821s, session 22.766s, telemetry 1.541s; no detected race |
| `go -C app vet ./...`; `go -C app mod verify` | PASS; all modules verified |
| `npm --prefix app/frontend run typecheck`; `npm --prefix app/frontend run lint`; `npm --prefix app/frontend run test` | PASS; 4/4 lifecycle tests |
| `node --check app/g5/ui-proof.mjs`; `node --check app/g5/summarize.mjs` | PASS |
| From `app/`: `. .\g0\Use-Toolchain.ps1`; `$env:WAILS_EXP_USE_WAKE='1'`; `wails3 build` | PASS in 4.0s, 5 steps ran / 2 cached; bindings, frontend build and Windows executable succeeded |
| `Get-FileHash app/bin/crocstream.exe -Algorithm SHA256` | Matches recorded final production hash `D590A9A1B6A8296B5DB5A5549FFD6852EDD630F6F3253D2666192D018936A48D` |
| `node app/g5/summarize.mjs app/g5/evidence/ui-600s-20261117.jsonl` | `G5_EVIDENCE_VERIFIED`: 600.331s, Stop 141ms, 23 checks / 11 closed listeners; recomputed quantiles, savings, resource baselines |
| `node app/g5/summarize.mjs app/g5/evidence/ui-final-20261117.jsonl` | `G5_EVIDENCE_VERIFIED`: 15.920s, Stop 146ms, 26 checks / 11 closed listeners |
| SHA-256 comparison of final-source manifest entries and saved UI proofs | PASS: all 9 source fingerprints and both original JSONL hashes match |
| Local Markdown target/anchor check | PASS: 172 targets/anchors across 9 documents |
| `gofmt -l` on changed Go files | PASS, empty output |
| `git -c safe.directory=D:/CrocStream -c core.safecrlf=false diff --check` before staging; staged `diff --cached --check` | PASS |
| `git -c safe.directory=D:/CrocStream diff --exit-code -- croc/ app/g0/ app/go.mod app/go.sum app/frontend/package.json app/frontend/package-lock.json` and upstream/G0 untracked status | PASS: pristine upstream, unchanged G0 and dependency pins |

The actual 600s stress and final/dev/local-preview UI runs were checked from
saved numeric evidence, not repeated during this pre-push review. Ordinary Go
tests skip real display reads without explicit consent. No capture, SSH/EC2
change, new service or dependency change was made. G0 regressions above remain
prior evidence; G0 code did not change in G5. The long-run binary and final
frame-kind follow-up remain distinct as documented. G5 is reconfirmed **PASS**
for the measured Windows local UX/telemetry/resilience gate, with the existing
manual/content-matrix/remote-performance limits retained. G6 is next.
