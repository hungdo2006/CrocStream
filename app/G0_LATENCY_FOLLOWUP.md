# G0 — Latency follow-up after network move

Date: 2026-11-16. Status: **REMOTE_RELAY_PASS, >=600s at original load on the
new network**, following paired-load diagnostics PASS. Keep the earlier remote
correctness PASS and its high RTT as separate historical evidence.
Final overall feasibility decision: **[G0 PASS](G0_CLOSEOUT.md)** after the
subsequent native-build, license and self-review closeout.

## Environment and scope

Same Windows laptop, same EC2 Windows Server 2022 `m7i-flex.large`, same
fingerprint-pinned SSH identity and self-hosted croc relay. The operator moved
location and the local public IPv4 changed. After the owner updated AWS ingress
and Windows SSH firewall scope, key-only SSH login succeeded again.
Existing port 9009 had no listener before the new test.

The sleep previously reported started after the completed 605s experiment,
as documented in [remote proof](G0_REMOTE_PROOF.md); it does not explain that
experiment's delayed control messages.

No upstream source, PAKE purpose, crypto/key schedule, transport records or G1
media wire format changed. Diagnostic harness pacing/measurements are not
production app transport settings or a replacement for capture backpressure.

## Idle TCP baseline

Twelve sequential `TcpClient.ConnectAsync` calls to EC2's existing SSH port,
each with a five-second bound and socket disposal:

```text
52.376, 46.169, 50.214, 47.674, 45.602, 45.487,
45.195, 46.738, 48.870, 47.320, 52.092, 50.590 ms
```

Mean 48.1939 ms, max 52.376 ms. This includes TCP connection setup on the idle
path; it is not pure ICMP RTT, encrypted-session RTT or a simultaneous loaded
network measurement.

## Same-binary control experiment on the new network

Ran `Invoke-SshProof.ps1` using the old, already verified `g0poc-remote.exe`,
with the new source IPv4 and `-DurationSeconds 30`. No pacing/code change:
32-KiB block per 20 ms, both directions. SHA-256 remains
`C1C304594C68125AD66041F0C5AE1F3CFDD5A60166FC5503F5EF4EC96221EE49`.
Run ID `3b2c135445704abdaefa8f3c41f8863c`.

| Metric | Local host | EC2 viewer |
| --- | ---: | ---: |
| Bytes sent | 48,562,176 | 49,184,768 |
| Bytes received | 49,184,768 | 48,562,176 |
| Duration | 30.0965219s | 30.0832343s |
| Ping / matching Pong | 492 / 492 | 457 / 457 |
| Key requests received | 457 | 492 |
| RTT mean / max | 47.6281 / 184.0531 ms | 49.2463 / 139.1567 ms |
| Goroutines start / end | 3 / 3 | 3 / 3 |
| Received SHA-256 | `e6356b41ecaab5394147bc36d91a174e712e42cc6603bf2dff78232f4b0dc103` | `91d9bfb6f2a06f1a397dcf1265e4dfd66814949466dbb01e14134082fd4f424a` |

Result **REMOTE_SMOKE_PASS**, followed by `remote_cleanup_complete`.
All byte/control counts and independently recomputed hashes matched; no detected
race. This is a 30-second diagnostic, not a new ten-minute acceptance run.

This same-binary observation shows the old multi-second latency is not
unavoidable on this croc-based path. It correlates strongly with changed
network conditions, but does not isolate old-network congestion, retransmission,
host scheduling or TCP buffering. Do not blame a particular provider/relay,
or claim physical network latency from application Ping/Pong alone.

## Bounded pacing instrumentation

`cmd/g0poc/main.go` adds optional private-setup `SendInterval`, default **20 ms**
for backward behavior, validated integer milliseconds **20..100**. Local
CLI uses `-send-interval`; SSH helper uses `-DataIntervalMilliseconds`.
At 100 ms the 320-block minimum needs about 32 seconds, still inside the
existing duration+45s peer bound even for the one-second smoke.

New measured fields: `data_interval_ms`, `data_write_mean_ms`,
`data_write_max_ms`. Write duration includes the entire data-message stream
write (encryption and socket admission), not delivery/ACK time and not queue
occupancy. A short write duration does not prove the kernel/network is empty.
The independent verifier still accepts old evidence without these optional
fields; validates new pacing bounds and finite nonnegative write timing.

`Invoke-SshProof.ps1` forwards the interval privately and requires results to
report the requested value. This avoids claiming a reduced-load experiment
while accidentally running an older binary that ignored the interval.

Tests added: default/edge/invalid/sub-millisecond pacing, invalid result pacing
and invalid write timing.

### Commands and measured build results

After `. .\app\g0\Use-Toolchain.ps1`:

| Command | Observed result |
| --- | --- |
| `gofmt -w app/g0/poc/cmd/g0poc` | Completed |
| `go -C app/g0/poc test -count=1 -timeout=120s ./...` | PASS: harness 1.892s, transport 5.106s, PAKE 0.570s |
| `go -C app/g0/poc test -race -count=1 -timeout=120s ./...` | PASS: harness 2.869s, transport 17.022s, PAKE 1.673s |
| `go -C app/g0/poc vet ./...` | PASS |
| `go -C app/g0/poc build -race -buildvcs=false -o "$env:GOBIN\g0poc-paced.exe" ./cmd/g0poc` | PASS |
| `g0poc-paced.exe -duration 1s -send-interval 20ms` | LOCAL_PASS, 10,485,760 B each direction, 6.4219727s, hashes match, 319/319 pings each |
| PowerShell AST parser on `Invoke-SshProof.ps1` | PASS |
| `git -c safe.directory=D:/CrocStream diff --check` | PASS |
| Upstream-only status, including untracked files | No changes |

Paced executable SHA-256:
`910E291A2BAD00548C852E03FD10044F1D94F6EA870850F5898A4EDE50BC9A27`.
`cmd/g0poc/main.go` SHA-256:
`59B727B6C18ADA483DC0347ACA584669DE9F68D164F87B044E9701C1D840D8AB`.
Transport and derived PAKE runtime source hashes remain exactly as recorded in
the prior remote report. The previous binary was not overwritten.

Paired remote measurements completed sequentially with this same new binary:
`Invoke-SshProof.ps1 ... -DurationSeconds 30 -DataIntervalMilliseconds 20`,
then the same command with `-DataIntervalMilliseconds 100`.

## Paired-load results on the same current route

[Sanitized collected JSON](g0/poc/evidence/latency-pair-20261116.json).
Both runs returned **REMOTE_SMOKE_PASS**, independently verified hashes and
cross-peer counts, correct requested pacing, no detected race, then
`remote_cleanup_complete`.

| Metric | 20-ms host | 20-ms viewer | 100-ms host | 100-ms viewer |
| --- | ---: | ---: | ---: | ---: |
| Bytes sent | 48,791,552 | 49,152,000 | 10,485,760 | 10,485,760 |
| Bytes received | 49,152,000 | 48,791,552 | 10,485,760 | 10,485,760 |
| Duration (s) | 30.0777962 | 30.0633237 | 32.14998 | 32.1488668 |
| Ping / Pong count | 494 / 494 | 445 / 445 | 296 / 296 | 299 / 299 |
| Reverse key requests received | 445 | 494 | 299 | 296 |
| RTT mean (ms) | 47.9780 | 49.0196 | 58.8052 | 60.8007 |
| RTT max (ms) | 199.8271 | 153.0790 | 263.2644 | 263.7089 |
| DATA write mean (ms) | 1.0331 | 0.0937 | 1.2629 | 0.2550 |
| DATA write max (ms) | 62.7334 | 0.8545 | 30.9585 | 1.0622 |
| Goroutines start / end | 3 / 3 | 3 / 3 | 3 / 3 | 3 / 3 |

Run IDs: 20 ms `47403d4347e547c4bae93f23620e31f3`;
100 ms `e5f0a2b0cd4a49b98fdb1583bd6d003e`.
The 100-ms run extends past 30s to meet the unchanged 10-MiB minimum.
Both directions used the same interval in each run.

Reducing the rate did **not** reduce observed mean RTT in these samples. There
is no reproduced multi-second backlog on this route at either rate. This is
not statistically controlled proof that lower rate increases RTT, and it does
not isolate the old network's root cause.

The instrumented binary's >=600s repeat completed at the original **20-ms**
interval (`-DurationSeconds 600 -DataIntervalMilliseconds 20`).

## Completed ten-minute repeat on the new network

Run ID `8493be781a904ec6a8731b63d4814f49`, same source/executable hashes above.
[Collected sanitized result](g0/poc/evidence/remote-new-network-20261116.json).

| Metric | Local host | EC2 viewer |
| --- | ---: | ---: |
| Bytes sent | 971,931,648 | 983,007,232 |
| Bytes received | 983,007,232 | 971,931,648 |
| Duration (s) | 600.0950805 | 600.0985593 |
| Ping / matching Pong | 9,888 / 9,888 | 9,173 / 9,173 |
| Reverse key requests received | 9,173 | 9,888 |
| RTT mean / max (ms) | 48.4102 / 190.8650 | 50.9342 / 144.7915 |
| DATA write mean / max (ms) | 1.0267 / 51.3284 | 0.1559 / 1.2350 |
| Go heap start / end (B) | 2,228,224 / 3,772,376 | 1,878,208 / 2,879,064 |
| Goroutines start / end | 3 / 3 | 3 / 3 |
| Received SHA-256 | `0f1def808b3936505254b30f268b13049255cf7789ce60ed693a104e79f4af45` | `91b0c3738ae5645ecef5e8b3a375102e555ce64b0bda3d32c79c0178018d1c6a` |

Result **REMOTE_RELAY_PASS**, then `remote_cleanup_complete`. Peers sent paced
synthetic traffic throughout >600s; every sample was checked in order, both
hashes were recomputed independently, all cross-direction byte/control counts
matched. Both race-enabled peers exited successfully, no detected race.
Heap endpoints do not measure maximum RSS or prove absence of every leak.

A separate authenticated SSH cleanup verification found **0 owned remote
processes and 0 temporary firewall rules** for all four new-network runs.
No TCP 9009 listener remained; no local proof processes remained. SSH access,
RDP, existing AWS security groups and EC2 instance were deliberately retained.

Run output file timestamps bound orchestration to 17:41:40–17:52:27 local time
(UTC+07). A readable Windows System log query found **0 recorded Kernel-Power
event-42 sleep entries** in that window. This corroborates the owner's
continuity report; it is not a claim that absence of an OS log proves every
network condition. Active peer durations are measured above, not derived from
file timestamps.

## Source review and limits of inference

Pinned upstream `croc/src/tcp/tcp.go` `pipeWithProtocol`/`copyWithProtocolObservation`
forwards bidirectionally through `io.Copy` after the initial observer. Source
inspection found no custom per-record sleep or explicit `SetNoDelay(false)`.
This source reading alone is not proof of optimal Windows forwarding.

The harness prioritizes queued control before issuing another DATA write,
but TCP cannot move a later control byte ahead of data already queued in its
ordered byte stream. The harness continues sending paced DATA while one ping
is outstanding. Therefore congestion/backlog is a plausible mechanism for
high application control RTT; the old network has not been reproduced or
isolated sufficiently to call that a measured root cause.

## Next decision

**Transport feasibility: PASS on the tested new-network topology and workload.**
The same original synthetic load now completes a ten-minute secure duplex proof
with application RTT in the reported millisecond range. The earlier path's
multi-second control delay was not reproduced; it remains a documented
network-dependent limitation with no isolated root cause.

The remote/two-machine evidence gap is closed for this chosen EC2 topology.
Do not guarantee the same latency on all networks, stock croc compatibility,
screen-sharing FPS or public-relay policy. The subsequent closeout completed
G0's native feasibility/license/self-review items; real capture stays in G3.
This follow-up itself did not waive those items or change the crypto/provider.

Evidence/document checks: paired JSON parse/completeness/cross-peer counters
PASS; 140 local Markdown link targets PASS; `git diff --check` and `gofmt -l`
PASS. The first standalone JSON-check command miscounted a nested array produced
by Windows PowerShell's `ConvertFrom-Json`; correcting the checker assignment
passed without changing any collected peer data.

Completion checks: saved new-network `.results` were piped again to the
hash-matched `g0poc-paced.exe -mode verify -duration 10m`: **RESULTS_VERIFIED**.
Final `git diff --check` PASS, 143 local Markdown link targets PASS, upstream
status pristine, no stale active-run status in this report. Runtime code did
not change after the previously recorded package/race/vet/build checks.

Completion artifacts: this report, sanitized
`app/g0/poc/evidence/remote-new-network-20261116.json`, updated
`app/G0_REMOTE_PROOF.md`, `app/G0_FEASIBILITY.md`, `app/g0/poc/README.md`,
root `README.md`, and `docs/{PRD,ARCHITECTURE,GETTING_STARTED,ROADMAP}.md`.
No commits, pushes, EC2 termination or removal of existing SSH access.
