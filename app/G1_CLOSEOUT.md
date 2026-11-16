# G1 closeout — binary framing and session configuration

Date: 2026-11-16. **Gate status: G1 PASS — ready for the focused G2 task.**
Scope: pure-Go protocol package in `app/internal/protocol`.

## G0 confirmation and authorization

Reviewed `docs/PRD.md`, `ARCHITECTURE.md`, `TEST_PLAN.md`, `ROADMAP.md`,
`G0_CLOSEOUT.md`, `G0_LOCAL_PROOF.md`, `G0_LATENCY_FOLLOWUP.md`,
`G0_TOOLCHAIN.md` and the collected
[remote JSON](g0/poc/evidence/remote-new-network-20261116.json).

The G0 closeout and underlying evidence agree: **G0 PASS** for the measured
prototype. The recorded remote run lasted 600.095/600.099s, transferred
971,931,648/983,007,232 B, matched both peers' byte/control counters and reported
48.410/50.934ms mean application RTT. Earlier network limits and Windows
upstream baseline failures remain in the original reports. Those experiments
were reviewed, not rerun in G1.

The owner explicitly requested G1 implementation and confirmed:

- **C-01:** key interval **1–120 emitted frames**, default **30**.
- **C-02:** only **640×360** and **1280×720**; reject mixed pairs.

PRD, architecture and test contract now record those decisions. Header/CONFIG
byte layout, type values, version and exact default fixtures are unchanged.

## Implementation and acceptance evidence

| G1 requirement | Implementation / exercised evidence |
| --- | --- |
| Exact 18-byte BigEndian header | `header.go`; `TestHeaderFixtures`, `TestDecodeHeaderMalformed`; independent timestamp/ID/length byte fixture |
| Exact 11-byte CONFIG | `config.go`; `TestConfigFixture`; all allowed FPS/resolution/key-interval edges, including 31 and 120 |
| Unsupported values and bounded dimensions | `TestConfigRejectsUnsupportedValues`; mixed/zero/65535×65535 dimensions rejected; arithmetic widens before multiplication |
| Type/direction/control fields/size before payload allocation | `Header.Validate`, `streamState.check`; `TestHeaderRejectedBeforePayloadRead` observes no body reads for rejected headers, including >8 MiB and uint32-max lengths |
| Bounded payload ownership | Maximum 8 MiB returned per packet; `TestMaxPayloadAndBufferOwnership`; no pool, hidden payload retention or background queue |
| CONFIG once, first video KEY1 | `TestHeaderRejectedBeforePayloadRead`, `TestWriterValidationDoesNotWrite`; reverse stream needs no CONFIG |
| Fragmented I/O and truncation | `TestFragmentedPipeRoundtrip` with 1–3 byte reads/writes in both directions; `TestEveryTruncationIsTerminal`; real `io.Pipe` sender termination halfway through payload |
| Fail closed, no magic scanning | Terminal reader errors remain sticky; tests append valid subsequent packets and verify the parser stops |
| Commit only after acceptance | `TestVideoCommitAndRecovery`; pending video blocks read-ahead; failed mock decode leaves last good ID unchanged |
| Gaps, replay and recovery | KEY can advance over gaps; DELTA must follow accepted reference; rejected records drained in bounded 4-KiB chunks; `TestReplayAfterRejectedVideo` covers replay of failed frames |
| Partial-write failure and no wraparound | `TestWriterIOFailureIsTerminal`, `TestRejectedInitialKeyAndExhaustion`, `TestNextFrameID`; state only commits after full write, failed writer cannot retry |
| END and cancellation boundary | END delivered once; no further write/read; `TestSessionCloseCancelsBlockedIO` verifies session-owned context/pipe closure unblocks I/O |
| Fuzz | `FuzzDecodeHeader`, `FuzzDecodeConfig`, `FuzzDecodePacket`, `FuzzReadStream` |

Implementation review found that using only the last accepted codec ID could
allow reuse of an ID whose payload had failed validation. The reader now also
tracks the highest consumed wire ID; rejected frames never advance canonical
progress but cannot be replayed. Regression tests exercise initial failed KEY,
later failed KEY, discarded DELTA gaps and recovery with a new forward KEY.

API integration and ownership: [application README](README.md).

## Environment and commands actually run

- Windows x64, `Microsoft Windows NT 10.0.26300.0`; PowerShell.
- Go **1.27.4 windows/amd64**, using existing `app/g0/Use-Toolchain.ps1`;
  module language/toolchain minimum **1.27.3**, consistent with G0.
- `go env GOOS GOARCH CGO_ENABLED CC`: windows / amd64 / 1 / gcc;
  existing compiler at `C:\MinGW\bin\gcc.exe`. No installation required.
- Starting repository HEAD: `ebe18fa3c4633d4c3daa3cf6d1f7bf546f53b9f8`;
  starting working tree clean. G1 changes are uncommitted.
- The application module has only standard-library imports and no third-party
  dependencies. Existing G0 nested modules remain isolated.
- Wails/Node/frontend checks: not applicable to this pure-Go library change.

From the repository root, Go commands used:

```powershell
. .\app\g0\Use-Toolchain.ps1
```

| Command | Observed result |
| --- | --- |
| Initial `git status --short` / `git log -4 --oneline` | FAIL: repository ownership mismatch; read-only checks retried with per-command `-c safe.directory=D:/CrocStream`, no global config change |
| `go version`; `go env GOOS GOARCH CGO_ENABLED CC` | Environment above |
| `gofmt -w app/internal/protocol` | Completed |
| Initial `go -C app test -count=1 -timeout=60s ./...` | PASS, 1.858s |
| Initial `go -C app test -race -count=1 -timeout=120s ./...` | PASS, 2.502s |
| `go -C app vet ./...` | PASS before and after replay correction |
| `$env:CGO_ENABLED='0'`; `go -C app build ./...` | PASS Windows/amd64 library build before and after replay correction |
| Final `go -C app test -count=1 -timeout=60s ./...` | PASS, 1.715s, including replay correction and pipe termination test |
| Final `go -C app test -race -count=1 -timeout=120s ./...` | PASS, 2.518s; no race reported on exercised paths |
| `git -c safe.directory=D:/CrocStream status --short --untracked-files=all -- croc/` | Empty: upstream has no changed/untracked files |
| `git -c safe.directory=D:/CrocStream diff --exit-code -- croc/` | PASS |
| `gofmt -l app/internal/protocol` | PASS: empty output |
| `go -C app list -m all` | Only `crocstream.local/app`; no external module dependency |
| `git -c safe.directory=D:/CrocStream -c core.safecrlf=false diff --check` plus new-file whitespace/conflict-marker scan | PASS |
| Inline PowerShell local Markdown target/heading-anchor check | Initial checker FAIL: ambiguous .NET `Split` overload; corrected to PowerShell `-split`, then PASS: 118 targets/anchors in 8 changed documents |

Fuzz command, one target at a time:

```powershell
go -C app test ./internal/protocol -run '^$' -fuzz '^FuzzDecodeHeader$' -fuzztime=30s -parallel=2
```

Substitute the exact target below for `FuzzDecodeHeader`:

| Target | Observed result |
| --- | --- |
| `FuzzDecodeHeader` | PASS, 1,319,796 executions / 30s |
| `FuzzDecodeConfig` | PASS, 1,216,378 executions / 30s |
| `FuzzDecodePacket` | PASS, 1,313,462 executions / 30s |
| Initial `FuzzReadStream` | PASS, 1,240,905 executions / 30s, before replay correction |
| Final `FuzzReadStream` | PASS, 1,254,796 executions / 30s, after replay correction |

Fuzz seeds are versioned in `fuzz_test.go`; generated coverage corpus resides
in the portable Go cache. These bounded runs found no panic, hang or failing
case; they do not constitute exhaustive proof for all inputs.

## Scope and next gate

G1 validates opaque media framing and configuration, not compressed content.
G2 must enforce `Config.Limits` during LZ4-frame decompression, reject trailing
compressed data/bombs/malformed tiles, and update canonical bytes atomically
before resolving a received video as valid. No dependency is selected for G2
by this closeout.

Session orchestration remains G4: one serialized writer, bounded control/video
queues, PLI <=1/s, Ping/Pong response/timing, actual transport cancellation,
whole-session END and authenticated reconnect on ID exhaustion. G1 tests the
framing and ownership boundary with in-memory I/O and mock codec acceptance.

**Next focused gate: G2 — deterministic delta codec and telemetry.**
Required evidence includes 200-frame sender/receiver canonical equality at
tau=0 and tau=4, partial edge tiles, malformed record atomicity, bounded LZ4,
keyframe schedule/fallback, and fair rolling-window baseline measurements.
C-03/C-04/C-05 remain for their later bridge/UI checkpoints.

## Files changed

- Added `app/go.mod`: standard-library-only application module.
- Added `app/internal/protocol/{doc,header,config,packet,stream}.go`.
- Added `app/internal/protocol/{header,config,stream}_test.go`, `fuzz_test.go`.
- Added `app/README.md`, `app/G1_CLOSEOUT.md`: API, commands, acceptance evidence.
- Updated root `README.md`, `docs/{PRD,ARCHITECTURE,TEST_PLAN,ROADMAP,GETTING_STARTED}.md`:
  owner decisions, current implementation status and next gate.

No commits or pushes were performed.
