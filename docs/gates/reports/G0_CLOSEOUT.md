# G0 closeout — source, transport, native build and licenses

Date: 2026-11-16. **Gate status: G0 PASS — GO for the next focused G1 task.**
Scope: CrocStream experimental Windows 1-to-1 feasibility, not a released app.
The owner authorized finishing the remaining G0 work; no G1 implementation,
production dependency change, new relay provider or crypto switch is inferred.

**Decision:** the pinned croc mechanisms can support an authenticated encrypted,
continuous duplex byte stream independent of file-transfer metadata, on the
measured Windows/local/self-hosted-EC2 paths. G0 source, transport, native
dependency feasibility, notices and implementation self-review are complete
for this prototype. This is not production, media/capture, arbitrary-network
latency or independent-security-audit acceptance.

## Evidence matrix

| G0 requirement | Observed evidence |
| --- | --- |
| Pinned source | croc `552f40fd2d7593316118370bec0a93478d48e715`, recorded tree `0e401898d47eaef76e62263483943e3aa789ad3a`; pristine upstream-only status |
| B-01: continuous duplex API | Exported Host/Accept/Join, one-shot listener, `io.ReadWriteCloser` session, no file lifecycle; actual local and remote process proofs |
| B-02: chosen remote relay | Owner-provided EC2 Windows relay/viewer, local Windows host over public TCP 9009, not SSH data forwarding; >600s active traffic twice |
| Reverse control / data integrity | Both peers validate every deterministic sample/order; independent SHA-256, cross-peer byte counters, ping/pong and key-request counters |
| B-03: actual security path | Purpose/room/transcript-bound PAKE + role confirmations; directional AEAD, record validation, fail-closed cancellation; negative/unit/race/fuzz coverage; self-review below |
| B-04: Windows stack feasibility | Go 1.27.4 + Wails 3.1.0 + pinned croc + existing x/sys 0.50.0 + OS Win32 GDI compile and run together, including CGO-disabled build |
| Native runtime/resource probe | Actual Win32 exports resolved, synthetic memory-DIB copy byte-checked, 100 create/copy/free cycles and ABI/bounds tests PASS; no screen read |
| License evidence | Reviewed ordinary/race closure: 21 modules, 52 notice/source files, 135 inline notices, 21 embedded files; full notice bundle retained, regression checks PASS |
| B-05: maintenance / decision | Approved experimental adapter retained outside pristine croc; no separate full fork or new provider |

## Decisive remote results

Latest original-load proof, run `8493be781a904ec6a8731b63d4814f49`:

- Local host sent 971,931,648 B; EC2 viewer sent 983,007,232 B.
- Durations 600.0950805 / 600.0985593 seconds.
- Ping/Pong matched 9,888 / 9,173; reverse key-request counters matched.
- Application RTT mean 48.4102 / 50.9342 ms; max 190.865 / 144.7915 ms.
- Every block and both independently generated hashes matched; no detected race.
- Both peers start/end at 3 goroutines. Independent post-run SSH/local checks
  found no owned test processes, relay listener or temporary firewall rules.

See [full follow-up](G0_LATENCY_FOLLOWUP.md), [earlier remote run](G0_REMOTE_PROOF.md)
and [local proof](G0_LOCAL_PROOF.md). The earlier network's 10–12s mean / 42.8s
max RTT remains real recorded evidence, not overwritten by the new network.
Its exact root cause is unisolated; default synthetic pacing is not a guarantee
of low latency on arbitrary links.

## Native build feasibility, not G3 capture acceptance

`app/g0/buildprobe/native_windows.go` uses trusted system `user32.dll` /
`gdi32.dll` through already-resolved `golang.org/x/sys/windows`. No new capture
module, native SDK, driver or tool was installed. x/sys was already present;
only its direct/indirect annotation in this G0 probe's go.mod changed.

The probe checks `GetDC`, `ReleaseDC`, `EnumDisplayMonitors`, `GetMonitorInfoW`,
`CreateCompatibleDC`, `CreateDIBSection`, `SelectObject`, `BitBlt`, `GdiFlush`,
`DeleteObject` and `DeleteDC`. Screen-reading functions are resolved **only**.
Executed operations copy a generated 2×2 bitmap between two memory DCs.
Allocation is bounded to 16×16; selected objects are restored before release,
and native-backed Go slices are invalidated before object destruction.

This demonstrates a feasible Windows-native capture dependency path at G0.
It does **not** implement or pass real display selection/capture, user Start
consent, scaling/normalization, 24 FPS, protected content, DPI/HDR/multi-monitor,
preview/UI cleanup or G3. G3 must implement and test those separately. Native
BitBlt on synthetic surfaces does not establish that every desktop is capturable.
No screen contents were read, logged, stored or sent during this check.

## Security self-review and maintenance contract

Reviewed source: `app/g0/poc/internal/{transport,upstreampake}` against
[the recorded design](G0_ADAPTER_DESIGN.md) and the pinned upstream schedule.
This is implementation self-review plus tests, **not an independent security
audit** or production certification.

- Relay admission and peer PAKE are distinct. Raw `Comm.Connection()` is never
  returned as an authenticated application stream. Accept/Join return only after
  mutual role-specific key confirmation.
- Purpose `peer-crocstream-g0-v1`, hashed room namespace and exact transcript
  prevent intended stock file/SSH/local-probe cross-protocol authentication;
  both directions are tested against real upstream purposes.
- Direction-specific HKDF labels, authenticated direction/sequence, exact
  plaintext lengths and AEAD tags reject replay, reflection and bad records.
  Nonce generation remains upstream random AEAD nonce, capped at 2^20 records
  per direction, last slot reserved for authenticated CLOSE.
- Peer handshake is bounded at 8 KiB, PAKE value 4096 B, data record 64 KiB.
  Fixed framing uses `io.ReadFull`; no resync scanning or unbounded app queue.
  One logical writer, separately owned read/write deadlines, terminal failures
  and socket closure unblock cancellation. No pooled frame ownership exists yet.
- Negative tests cover wrong code/password, role/purpose/room/transcript
  mismatch, malformed/truncated lengths, authenticated corruption, replay,
  reflection, cross-session data, short writes, concurrent writers and close/
  cancel during blocked work. Lifecycle test exercises 100 real relay cycles.
- The reusable adapter has no Wails/capture/codec dependency. The derived
  schedule is a small separately noticed file; upstream source remains pristine.
  Updating upstream requires reviewing admission, codephrase, PAKE and AEAD APIs,
  rerunning negative/lifecycle/relay evidence and updating exact provenance.

### Explicit residual risks, not hidden gate waivers

1. Upstream relay-admission receive bounds can reach 64 MiB before the custom
   peer framing takes over. The tested relay was private, password protected
   and firewall source restricted. Do not advertise hostile-public-relay DoS
   hardening; review this boundary before exposing a production app.
2. Six-word invitation uses upstream two room words/four PAKE words; about
   41 bits of password entropy, not an unguessable 128-bit secret. PAKE does
   not remove online guessing or relay/endpoint denial of service.
3. Go string/GC/AEAD copies do not permit a claim of complete key erasure.
4. Same-stream control cannot overtake DATA already buffered in TCP.
   Future capture backpressure is required; no global Internet latency/FPS claim.
5. Windows upstream `TestWrongPassword` can reject with connection reset rather
   than the expected text. Full upstream Windows baseline has additional
   documented environment failures. Those failures were not patched or relabeled
   PASS: see [baseline](G0_TOOLCHAIN.md).
6. No stock croc client compatibility, public relay authorization, Linux/macOS,
   capture/media performance, automatic rekey/reconnect or installer/signing
   success is inferred from this gate.

## Reviewed notice bundle

Authoritative snapshot: [reviewed-20261116](../../../app/g0/licenses/reviewed-20261116/inventory.json),
with [full notice text](../../../app/g0/licenses/reviewed-20261116/THIRD_PARTY_NOTICES.txt).
Bundle SHA-256:
`0996E3672FDFE890833087181C2D2B234B2D6C7F53654EB99BE7F07BB4EB9106`.
Inventory SHA-256:
`7575E4A162D58F278EBA8BDE31FA052562DAEE4C82F37AC44BAB27AAC15AD59B`.

The root licenses already identified as MIT/BSD/Apache/zlib were checked with
their nested overrides, not treated as an all-MIT dependency set. Additional
attributions include EFF CC-BY-4.0 data/source/modification notice, WebView
loader ISC, Winc MIT, W32/clipboard/Chromium BSD, embedded nanoid/OXC/Babel/
Rolldown runtime terms, Go standard/vendor patent notices, and the LLVM
race-runtime license at the exact commit recorded by this Go distribution.
Inline fiat-crypto and go-ole notices are preserved.

All 21 module roots have license text; 52 source documents, 135 inline notices,
21 embedded-file fingerprints and 8 separately retrieved license texts are
recorded. The per-package source sets are the **union** of ordinary/race
builds, including `x/sys/windows/race0.go` as well as the race variant.
The earlier `generated-20261116` draft is retained as superseded evidence;
only `reviewed-20261116` is authoritative.

Disposition: **G0 dependency/notice review PASS**, retain the bundle with G0
binaries. No dependency license incompatibility requiring a provider/crypto
switch was identified within this reviewed scope. This is an engineering
inventory/obligation review, not legal advice or a blanket shipping opinion.
Wails CLI/Node/compiler distributions and a future React/capture/installer
package are not redistributed here; their eventual release contents need a
fresh G6 audit, not retroactive G0 approval. See [scope](../../../app/g0/licenses/README.md).

## Commands actually executed in closeout

Environment: Windows x64 build 26300 (Windows 11 Home branding), i7-13650HX,
23 GiB RAM as reported by Wails Doctor, PowerShell 5.1, Go 1.27.4,
Wails CLI/module 3.1.0. Repository HEAD remains
`565c192c536c7fd926ba7fa940d817cee13ed070` plus uncommitted G0 changes.
From repository root, after `. .\app\g0\Use-Toolchain.ps1`:

| Command | Result |
| --- | --- |
| `go version` | Go 1.27.4 windows/amd64 |
| `wails3 doctor` | Exit 0, no development issues; Wails 3.1.0 / WebView2 157.0.4341.46. Packaging/signing tools still absent |
| `gofmt -w app/g0/buildprobe` | Completed |
| `go -C app/g0/buildprobe test -v -count=1 -timeout=120s ./...` | PASS, 2.828s; 100 synthetic copies, GDI handles 0→0 |
| `go -C app/g0/buildprobe test -race -count=1 -timeout=120s ./...` | PASS, 3.334s |
| `go -C app/g0/buildprobe vet ./...` | PASS |
| `go -C app/g0/buildprobe build -buildvcs=false -o "$env:GOBIN\g0-native-probe.exe" .` and execution | PASS; combined Go/Wails/croc/Win32-GDI, synthetic bitmap exact |
| Same build with process-local `CGO_ENABLED=0`, output `g0-native-nocgo.exe`, then execution | PASS; SHA-256 `D8F54849099BBD35BFF191321F3402F5A15EEC550A9090947532FD9E4198F696` |
| First license inventory attempt | FAIL: unsupported Go template `json` function; replaced with explicit fields before output creation |
| Corrected `Audit-Licenses.ps1`, temporary output | Generated 21 modules / 49 license-source documents / 134 inline notices / 21 embedded files; manual review found 3 more nested attributions to include |
| `Audit-Licenses.ps1 -OutputDirectory app/g0/licenses/generated-20261116` | Generated 21 modules / 52 notice documents / 134 inline notices / 21 embeds |
| Same tool, final output `app/g0/licenses/reviewed-20261116`, after ordinary/race union fix | PASS generation: 21 / 52 / 135 / 21; additional notice is BSD `x/sys/windows/race0.go` |
| `Test-LicenseInventory.ps1 -Directory app/g0/licenses/reviewed-20261116` | PASS: closure, nested attributions, ordinary/race coverage, 8 downloaded hashes and refuses overwrite without changing existing bundle |
| Final `go -C app/g0/poc test -count=1 -timeout=120s ./...` | PASS: 1.944s / 5.263s / 1.244s |
| Final `go -C app/g0/poc test -race -count=1 -timeout=120s ./...` | PASS: 2.877s / 17.262s / 1.706s |
| Final `go -C app/g0/poc vet ./...` | PASS |
| `go -C app/g0/buildprobe mod verify` and `go -C app/g0/poc mod verify` | Both `all modules verified` |
| `powershell -NoProfile -File app/g0/Inspect-Environment.ps1 -VerifyUpstream` | Exit 0; fresh snapshot tree match and pristine upstream, 2026-11-16T12:00:55Z |
| AST parser for audit/test scripts, `gofmt -l app/g0/buildprobe app/g0/poc`, `git diff --check` | PASS |
| Final notice regression check and upstream lookup | PASS; GitHub tree MATCH `0e401898d47eaef76e62263483943e3aa789ad3a`, upstream pristine |
| Final local Markdown target/heading-anchor validation | PASS: 168 links/anchors across 20 documents |

Earlier helpers had two command-only quoting/parser failures while inspecting
embedded files/npm lock metadata. Corrected inspection used Go range output
and Node's JSON parser. No dependencies or collected peer results were edited
to obtain a passing result.

## Proposed next gate — no implementation here

G0 is now closed. Next focused work is **G1: binary framing/configuration**:
pure Go package, exact 18-byte header/11-byte CONFIG fixtures, checked lengths,
unknown-type/direction/version rejection, truncated-input tests and bounded
fuzzing. No UI, capture, codec or new transport should be bundled into that task.

Existing C-01 (key interval maximum vs default) and C-02 (permitted resolution
pairs) still need owner clarification before affected G1 validation. This
closeout does not choose new wire values by assumption.

For later application transport integration, preserve `io.ReadWriteCloser`,
Host preparation/invitation before one-shot Accept, authenticated Join, bounded
options and context-owned lifetime. Keep the experimental transport version and
media protocol v1 distinct. Existing Go module sums pin the G0 experiment;
no production application go.mod or frontend package pins are created here.

## Files and operational status

Changes in this closeout:

- `app/g0/buildprobe/{main.go,go.mod,native_windows.go,native_windows_test.go}`:
  combined native dependency/runtime probe, ABI/bounds/cleanup tests.
- `app/g0/{Audit-Licenses.ps1,Test-LicenseInventory.ps1}`: notice inventory,
  ordinary/race union and reproducible regression/overwrite guards.
- `app/g0/licenses/`: reviewed snapshot, retained generated draft, scope README.
- `app/G0_CLOSEOUT.md`: final Go/No-Go, commands, evidence and G1 handoff.
- `app/G0_{FEASIBILITY,ADAPTER_DESIGN,TOOLCHAIN,LOCAL_PROOF,REMOTE_PROOF,LATENCY_FOLLOWUP}.md`:
  authoritative closeout links while preserving historical measurements/failures.
- Root `README.md`, `app/g0/{README.md,poc/README.md}`,
  `docs/{PRD,ARCHITECTURE,TEST_PLAN,ROADMAP,GETTING_STARTED}.md`: current gate and
  reproduction instructions; no media wire-format changes.

No upstream source changes, commits, pushes, new service,
credentials, tool installation or EC2 termination.
Remote test processes/listeners were already cleaned; EC2/SSH access remains
owner-managed and may still incur AWS charges.
