# G0 — Secure duplex PoC evidence

> Final gate status: [G0 PASS](G0_CLOSEOUT.md). This report preserves the earlier
> local checkpoint; its then-missing remote/native/license work is now closed
> by later evidence, not inferred from loopback success.

Date: 2026-11-16. Follow-up authorized by the owner's request to finish G0 after
the adapter draft and tooling review. Scope is the existing experimental
relay-TCP/PAKE/directional-AES-GCM design; no external relay was selected.

> Later same-day follow-up: the owner created EC2 Windows and authorized SSH.
> [Remote evidence](G0_REMOTE_PROOF.md) now records a successful two-machine
> Internet-relay smoke and completed 605s remote test, including substantial RTT.
> Local source fingerprints/results below are historical and unchanged.

## Current assessment

The PoC exists in `app/g0/poc/`. Both ordinary and race-enabled local runs
completed >=600 seconds of active traffic through two real peer processes and
a third parent process hosting the actual upstream croc relay. The final run
returned **LOCAL_PASS**, exchanging approximately 977 MB each direction with
matching hashes and control counters.

**At this local checkpoint, G0 remained BLOCKED for remote acceptance.** A local success cannot
close the two-machine/target-relay requirement. Full native capture integration
and a final redistribution-license audit are also outstanding.
The owner explicitly confirmed on 2026-11-16 that no second machine/relay is
available at that earlier checkpoint. This was an environment blocker, not approval to narrow the MVP
to LAN or to substitute a public relay.

## Environment and source

- Windows build 26300, x64, Go 1.27.4; same machine as the tooling report.
- CrocStream repository HEAD: `565c192c536c7fd926ba7fa940d817cee13ed070` plus
  uncommitted G0 artifacts listed below.
- Upstream commit: `552f40fd2d7593316118370bec0a93478d48e715`.
- Recorded upstream tree remains `0e401898d47eaef76e62263483943e3aa789ad3a`;
  `croc/` working tree clean.
- Online recheck on this date failed to fetch GitHub metadata; the inventory
  correctly reported BLOCKED for online provenance. Prior successful online
  match is retained in the 2026-11-15 report, not claimed as a fresh match.
- Route: hard-bound loopback `127.0.0.1`, OS-assigned port, croc
  `tcp.RunWithListener`. No direct fallback or Tailcat/DERP negotiation.
- Payload: synthetic 32-KiB blocks with deterministic per-direction and
  per-block xorshift pattern; exact comparison and SHA-256. No captured images.

## Implementation and design reconciliation

`app/G0_ADAPTER_DESIGN.md` describes the envelope. The implemented record
format is exactly its DATA/CLOSE framing; control packets are application
test messages **inside** the stream, not additional transport record kinds.

The isolated derived PAKE package is smaller than the draft's proposed
additive allowlist copy: it fixes `p256` and the CrocStream purpose, removes
unused old-purpose/curve selection, and retains the upstream framing and
96-byte HKDF/confirmation formula. Thus it cannot expose the old purposes
as its own public API; tests exercise isolation against the real upstream
package instead of claiming byte-compatible behavior for removed APIs.
See `internal/upstreampake/UPSTREAM.md` and its MIT notice.

The adapter now returns invitation before waiting for a viewer. It bounds
peer framing before allocation, handles short writes and tiny reads,
serializes writers, separates deadlines, rejects replay/reflection/bad tags,
caps per-key records and makes cancellation close the socket.
Secrets remain memory-only; reports contain only hashes/counters.

## Commands executed

All commands below run from the repository root after:

```powershell
. .\app\g0\Use-Toolchain.ps1
```

| Command | Observed result |
| --- | --- |
| `gofmt -w app/g0/poc` | Completed |
| `GOFLAGS=-mod=mod`, then `go -C app/g0/poc mod tidy`; restore `-mod=readonly` | Completed; separate PoC go.mod/go.sum |
| `go -C app/g0/poc build -buildvcs=false -o "$env:GOBIN\g0poc.exe" ./cmd/g0poc` | PASS |
| `& "$env:GOBIN\g0poc.exe" -duration 1s` | LOCAL_PASS; minimum 10 MiB each direction forces ~6.42s despite 1s requested duration |
| First `go -C app/g0/poc test -count=1 -timeout=120s ./...` | FAIL: test expected a specific unexpected-EOF error but net.Pipe returned closed-pipe; assertion corrected to require non-clean termination |
| Subsequent `go -C app/g0/poc test -count=1 -timeout=120s ./...` | PASS: harness, transport, PAKE packages |
| Subsequent `go -C app/g0/poc test -race -count=1 -timeout=120s ./...` | PASS: all three packages |
| `go -C app/g0/poc vet ./...` | PASS |
| Final `go -C app/g0/poc test -count=1 -timeout=120s ./...` | PASS: harness 1.809s, transport 5.138s, PAKE 1.099s |
| Final `go -C app/g0/poc test -race -count=1 -timeout=120s ./...` | PASS: harness 2.492s, transport 10.040s, PAKE 1.696s |
| Final `go -C app/g0/poc vet ./...` | PASS |
| `go -C app/g0/poc build -race -buildvcs=false -o "$env:GOBIN\g0poc-verified.exe" ./cmd/g0poc` | PASS |
| `& "$env:GOBIN\g0poc-verified.exe" -duration 10m` | LOCAL_PASS: both peers >=600s; parent and both children exited successfully; no detected race reported |
| `go -C app/g0/poc test ./internal/transport -run '^$' -fuzz '^FuzzRecord$' -fuzztime=15s -parallel=2` | PASS, 547,103 executions |
| Same command with `-fuzz '^FuzzAuthenticatedPlaintext$'` | PASS, 480,614 executions |
| Same command with `-fuzz '^FuzzHandshakeBody$'` | PASS, 337,101 executions |
| `go -C app/g0/poc test -race -v -count=1 -timeout=30s ./internal/upstreampake ./internal/transport -run 'TestUpstreamInitiatorIsolation\|TestMalformedPAKEAndSameRole\|TestIdleReadTimeoutAndDuplicateHandshake\|TestRelay100Lifecycles'` | PASS: opposite-direction cross-purpose tests, malformed/same-role PAKE, duplicate handshake/idle timeout, 100 lifecycle cycles; goroutines 3→4 while private relay still active |
| `go -C app/g0/poc list -buildvcs=false -deps -f '{{if .Module}}{{.Module.Path}} {{.Module.Version}}{{end}}' ./cmd/g0poc` | PASS: 19 dependency modules, all in prior root-license inventory; no new primitive/provider |
| Initial `go -C app/g0/poc mod verify` | Harness timeout at 30s; not a checksum failure or PASS |
| `go -C app/g0/poc mod verify`, rerun as background job | PASS: `all modules verified` |
| Markdown target/anchor validation | PASS: 126 local links/anchors across 15 Markdown files |
| `gofmt -l app/g0/poc`, `git -c safe.directory=D:/CrocStream diff --check` and app whitespace/conflict-marker scan | PASS |
| `git -c safe.directory=D:/CrocStream diff --exit-code -- croc/` and status limited to `croc/` | PASS: no tracked changes or untracked artifacts |

First smoke results (before later ACK/lifecycle checks):

| Metric | Broadcaster | Viewer |
| --- | ---: | ---: |
| Application data sent | 10,485,760 B | 10,485,760 B |
| Application data received | 10,485,760 B | 10,485,760 B |
| Ping sent / matching Pong received | 319 / 319 | 320 / 320 |
| Reverse key request equivalents received | 320 | 319 |
| Active duration | 6.4204734s | 6.4204734s |
| RTT mean / max | 0.5635 / 19.9126 ms | 0.4911 / 1.5069 ms |
| Received SHA-256 | `221269b1cf6431c78536bb9d5e5bb731fe5011025d1c2bbaaad38d8fa58d6c28` | `71513e1160e33e8feb2d70ad97009337f26fd965a37b46693746cdec0e65ad11` |

These are synthetic loopback observations, not guaranteed FPS, Internet latency,
or maximum throughput. The completed long-run observations follow.

### First 10-minute run (before final ACK/cleanup checks)

`& "$env:GOBIN\g0poc.exe" -duration 10m` returned **LOCAL_PASS**:

| Metric | Broadcaster | Viewer |
| --- | ---: | ---: |
| Active duration | 600.0021732s | 600.0021732s |
| Data sent | 979,009,536 B | 978,845,696 B |
| Data received | 978,845,696 B | 979,009,536 B |
| Ping/Pong matched | 29,780 | 29,779 |
| Key request equivalents received | 29,779 | 29,780 |
| RTT mean / max | 0.9494 / 395.527 ms | 0.9677 / 858.0914 ms |
| Go heap start / end | 2,065,256 / 1,083,872 B | 2,092,984 / 2,558,544 B |
| Peer goroutines start / end | 3 / 3 | 3 / 3 |

Received SHA-256: broadcaster
`9a0964d6349e86829c168f732105b1682bf783e0dcae8069453df2fb04ba3339`;
viewer `e2d674a83a7bf88e10c1f63d68147d6f73518284381e8b50f88c8d2276132ab5`.
Each matched the parent's independent expected pattern hash.

The start/end heap samples are not RSS or peak allocation measurements and
do not by themselves establish absence of every leak. Throughput is deliberately
paced, and builds/fuzz/tests on the same machine affected scheduling and RTT.

### Final race-enabled 10-minute run

This run used the final non-test source fingerprints below. Additional test-only
cases were checked separately while it ran; no runtime source changed during
this run.

| Metric | Broadcaster | Viewer |
| --- | ---: | ---: |
| Active duration | 600.0100357s | 600.0121536s |
| Data sent | 977,305,600 B | 977,371,136 B |
| Data received | 977,371,136 B | 977,305,600 B |
| Ping sent / matching Pong received | 29,682 / 29,682 | 29,669 / 29,669 |
| Key request equivalents received | 29,669 | 29,682 |
| RTT mean / max | 2.4226 / 1,526.8474 ms | 2.4013 / 1,052.4348 ms |
| Go heap start / end | 2,214,160 / 2,487,008 B | 2,214,080 / 1,307,592 B |
| Peer goroutines start / end | 3 / 3 | 3 / 3 |
| Received SHA-256 | `60556a1485bf72175ea78a62a67ae19733c3d102b2dd39db9ee7f49d46d6c15a` | `d319b1ecf967f826d7376b7b886a7aaefc306bc582c65b5c20ca64205ea9b6a5` |

Both hashes match the parent's independently generated expected patterns.
The race-enabled binary also launches race-enabled child peers; child exit
failures are propagated by the parent. Successful execution means no race was
detected on these executed paths, not proof of race-freedom in every schedule.
High maximum RTT outliers are retained rather than hidden; this was a paced,
instrumented run on a machine also performing build/test work.

Post-run command:

```powershell
Get-CimInstance Win32_Process -Filter "Name = 'g0poc-verified.exe' OR Name = 'g0poc.exe'" |
    Select-Object Name, ProcessId, ParentProcessId
```

Result: no matching PoC processes remained. The upstream-only Git status query
also returned no changed/untracked files.

## Acceptance matrix

| Requirement | Evidence / state |
| --- | --- |
| Peer PAKE + mutual confirmation | Actual Host/Accept/Join path; wrong-password tests |
| Purpose/room/role/transcript isolation | Tests against upstream purpose variants and transcript mutations |
| Encrypted duplex, >=10 MiB each way | Two-process local smoke PASS |
| Reverse control on same stream | Ping/Pong + keyframe-request-equivalent counters and ordered IDs |
| >=10-minute active session | PASS local: ordinary run 600.002s; final race-enabled peers 600.010s / 600.012s |
| Bounded framing and decode | Unit + fuzz tests for oversize, truncation, malformed authenticated plaintext |
| Replay/reflection/session isolation | Negative record tests |
| Backpressure, tiny reads, partial/concurrent writes | Unit tests |
| Cancel and repeated ownership cleanup | Blocked read/write/Accept/Join tests, 100 relay Host/Join cycles |
| Remote LAN/two-machine/Internet relay | NOT RUN: no second-machine/authorized relay configuration supplied |
| Complete platform/capture stack | NOT RUN: prior Go/Wails/croc build probe only |
| Full upstream regression | Prior Windows failures remain documented in G0_TOOLCHAIN; upstream not modified |
| Independent security audit | NOT RUN; implementation/self-review/tests are not external audit |

## Maintenance, licenses and remaining gate

The PoC adds one derived schedule, a stream adapter, harness and tests, not a
full croc fork. Keep source SHA, notices, dependency sums and negative tests
when changing upstream. Updating purpose, room namespace or record bytes
requires a new experimental transport revision. Media protocol v1 is unchanged.

Direct dependencies: local pinned croc, pake/v3 3.2.1, x/crypto 0.59.0.
Root license evidence is recorded in `G0_TOOLCHAIN.md`; the derivative retains
MIT. The dependency closure must be audited for final redistribution before
shipping a desktop binary. No production pins have been selected.

Next action: owner confirmed no remote environment is currently available.
Keep G0 remote BLOCKED until an authorized endpoint and second Windows machine
are available, then repeat the proof on that route. Do not advance to G1 on
loopback evidence alone.

## Source fingerprints for final race-enabled run

SHA-256 of Go source (non-test files) after formatting:

| File relative to `app/g0/poc/` | SHA-256 |
| --- | --- |
| `cmd/g0poc/main.go` | `093c5f717bc053b5cdbfa4e5d30937a1396dd8cc7f937cae133f41ad5f336b89` |
| `internal/transport/transport.go` | `cb0760b58a9b9b0a2d2f7f9fe166cac2383c178acd4c166c546beb064d03a56e` |
| `internal/upstreampake/pake.go` | `71356651b3b7657ac3099ab04343b5802ad7b958ca036d0fca9779f612ebd758` |

Changes in this phase: new `app/g0/poc/` module, source, tests, README and
upstream-derived MIT/provenance notices; new `app/G0_LOCAL_PROOF.md`; updated
G0 design/history/tooling documents, root README and five planning documents.
No tracked upstream source changed. No commits or pushes.
