# G0 — Remote EC2 proof

Date: 2026-11-16. Status: **REMOTE_RELAY_PASS for duplex correctness and longevity**.
This earlier checkpoint's native/license/review gaps were subsequently closed
in [G0_CLOSEOUT](G0_CLOSEOUT.md), **G0 PASS**. The high latency recorded below
remains an old-network limitation; no general low-latency claim is made.

> Later same-day [new-network follow-up](G0_LATENCY_FOLLOWUP.md) completed
> another ten-minute proof at the same original data rate, with mean
> application RTT 48–51 ms, max 191 ms, hashes/control/cleanup verified.
> The high RTT below belongs to the earlier route and is retained, not relabeled.

## Authorized topology and trust

The owner created EC2, enabled Windows OpenSSH and supplied its host fingerprint
through the existing chat after reading it in RDP. A dedicated local ED25519
identity was generated outside the repository with restricted directory ACLs.
No Windows password or AWS `.pem` was requested or copied into the repository.
The scanned server key matched the supplied fingerprint before the first
connection; SSH uses `StrictHostKeyChecking=yes`, `IdentitiesOnly=yes` and a
dedicated known-hosts file. No unknown-host acceptance or password fallback.

Executed data path: local Windows host → Internet/public TCP 9009 →
self-hosted actual croc relay on EC2 → viewer using EC2 loopback TCP 9009.
Both peers run the same SHA-256-checked, race-enabled Windows executable.
SSH is management only, not a tunnel for the data stream.

This proves neither stock croc client compatibility nor an independent public
relay, and it is not a media/capture benchmark. EC2 Windows Server is a separate
tested platform from a second Windows 10/11 laptop.

## Observed environment

Authenticated inventory via SSH:

- Account: local EC2 Administrator; hostname recorded by operator.
- Microsoft Windows Server 2022 Datacenter, version 10.0.20348, 64-bit.
- EC2 `m7i-flex.large`, 2 logical processors.
- TotalPhysicalMemory: 8,421,658,624 B; free OS memory at inventory: 5,819,172 KiB.
- C: size 32,210,153,472 B, available 9,682,186,240 B.
- `sshd` Running. `C:\g0` did not exist before setup.
- SSH login succeeded with the dedicated key. A preliminary TCP 9009 probe
  was not reachable before relay startup; not an established firewall diagnosis.

Local SSH client source IPv4 was independently measured before configuring the
source-restricted rules. Public addresses, private keys, invitations and relay
passwords are deliberately omitted from this report.

## Changes and verification

- `app/g0/poc/cmd/g0poc/main.go`: independent result-verification mode; explicit
  private-stdin relay mode using the same upstream relay as local proof.
  No change to peer PAKE, security records or G1 media wire bytes.
- `app/g0/poc/cmd/g0poc/main_test.go`: valid and malformed result verification.
- `app/g0/poc/Invoke-SshProof.ps1`: pinned SSH setup, verified executable
  transfer, source-restricted Windows Firewall, private automatic handoff,
  two-peer collection, independent hash/counter verification and owned cleanup.
- README/AWS guide: automated route, manual limitations, prompt order and
  source-IP rule corrections. No requirement to open relay to all Internet.

Commands/results so far:

- `ssh-keyscan -t ed25519`, followed by `ssh-keygen -lf` and exact fingerprint
  comparison: matched; host key pinned before login.
- `ssh ... powershell.exe -NoProfile -NonInteractive -EncodedCommand ...`:
  authenticated inventory succeeded. Scripts contained no secrets.
- `go -C croc build -buildvcs=false -o "$env:GOBIN\croc-relay.exe" .`: PASS.
  SHA-256 `DD01E62C689443E18C3C444318353747EB3E0252CB69F0C64871F84C130B001D`.
  The automated proof uses the single-port PoC relay instead of this CLI binary.
- Initial verifier-only change: `go test`, `go test -race`, `go vet`, race build
  and local >=10-MiB smoke PASS. Relay mode was added afterward, so those earlier
  results alone are not proof of the final executable.
- Final relay/verifier `go -C app/g0/poc test -count=1 -timeout=120s ./...`:
  PASS (harness 1.954s, transport 5.531s, PAKE 1.257s).
- Final `go -C app/g0/poc test -race -count=1 -timeout=120s ./...`:
  PASS (harness 2.978s, transport 17.519s, PAKE 1.757s).
- `go -C app/g0/poc vet ./...`: PASS.
- `go -C app/g0/poc build -race -buildvcs=false -o "$env:GOBIN\g0poc-remote.exe" ./cmd/g0poc`:
  PASS; binary SHA-256
  `C1C304594C68125AD66041F0C5AE1F3CFDD5A60166FC5503F5EF4EC96221EE49`.
- `g0poc-remote.exe -duration 1s`: LOCAL_PASS, 10,485,760 B in each direction,
  active duration 6.4404937s, expected received hashes match.
- Process-isolated invalid CLI input checks: relay rejects empty config and
  invalid JSON; verifier rejects wrong result count and trailing JSON. All
  exited nonzero with expected bounded errors and no stdout.
- PowerShell parser check of `Invoke-SshProof.ps1`: PASS.
- `git -c safe.directory=D:/CrocStream diff --check`: PASS; local Markdown
  link-target check PASS (132 targets); `gofmt -l` returned no changed Go files.
- Upstream-only diff/status check: pristine `croc/`, no changed or untracked files.
- Automated EC2 smoke: **REMOTE_SMOKE_PASS**; cleanup completed.
- Automated >=600s proof: **REMOTE_RELAY_PASS**, both peers >605s; cleanup
  completed, then independently rechecked by authenticated SSH.

Runtime source SHA-256 for this executable:

| File under `app/g0/poc/` | SHA-256 |
| --- | --- |
| `cmd/g0poc/main.go` | `228F7526517959CE5F254F1C793B2243D9F2471E8AB3C0F485759F29D6B81D7D` |
| `internal/transport/transport.go` | `CB0760B58A9B9B0A2D2F7F9FE166CAC2383C178ACD4C166C546BEB064D03A56E` |
| `internal/upstreampake/pake.go` | `71356651B3B7657AC3099AB04343B5802AD7B958CA036D0FCA9779F612EBD758` |

The previous ten-minute loopback results in `G0_LOCAL_PROOF.md` remain tied to
their recorded old source fingerprints. They are not retroactively relabeled
as results for this modified CLI.

## Remote smoke measurements

Command, after dot-sourcing `app/g0/Use-Toolchain.ps1`:

```powershell
.\app\g0\poc\Invoke-SshProof.ps1 `
  -Server '<EC2 public IPv4>' `
  -IdentityFile '<dedicated local private key path>' `
  -KnownHostsFile '<fingerprint-verified local known-hosts path>' `
  -Executable "$env:GOBIN\g0poc-remote.exe" `
  -ClientAddress '<actual local public IPv4>' `
  -DurationSeconds 1
```

Run ID: `0c00c7ed439a4a5a94f89ac27bdb904d`. Transfer hash matched before execution.
Both peers received and independently checked >=10 MiB in this smoke; the
minimum byte count overrides the requested one second. **It is not a ten-minute
PASS.** Final event `remote_cleanup_complete` confirms owned remote processes
were removed and its temporary Windows Firewall rule was removed.

| Metric | Local host | EC2 viewer |
| --- | ---: | ---: |
| Bytes sent | 12,976,128 | 11,763,712 |
| Bytes received | 11,763,712 | 12,976,128 |
| Duration | 22.8395099s | 22.8411993s |
| Pings / matching pongs | 9 / 9 | 9 / 9 |
| Reverse key requests received | 9 | 9 |
| RTT mean / max | 1,652.7967 / 12,501.7684 ms | 1,696.3391 / 11,469.9497 ms |
| Go heap start / end (B) | 2,165,192 / 3,783,360 | 1,859,544 / 2,830,136 |
| Goroutines start / end | 3 / 3 | 3 / 3 |
| Received SHA-256 | `7f6e19fb64159acad62a9ed87f48cd9bf96369487a9e6faf2bf32d8c59806a4e` | `11470bfb768a765216113d5f2ba1e677a65963fd1b6ee8d7f01bdaa1cc31a9f0` |

Both hashes were recomputed independently by verifier mode; cross-direction
bytes and control counts matched. Both peers exited successfully, no detected
race. The RTT outliers are substantial, preserved as observed; neither their
cause nor a low-latency performance claim is established. They may include
backpressure/scheduling, not just physical network latency.

## Completed ten-minute remote proof

Same command as above, with `-DurationSeconds 600`, completed successfully.
Run ID: `926bcb28763e4d07af6dc7ec0d7a9a41`. Executable/source hashes are unchanged.
The collected [sanitized results](g0/poc/evidence/remote-20261116.json) contain
no room code, relay password, private key or server IP.

| Metric | Local host | EC2 viewer |
| --- | ---: | ---: |
| Bytes sent | 726,401,024 | 305,004,544 |
| Bytes received | 305,004,544 | 726,401,024 |
| Duration | 605.0188295s | 605.0140157s |
| Pings / matching pongs | 51 / 51 | 51 / 51 |
| Reverse key requests received | 51 | 51 |
| RTT mean / max | 11,773.1240 / 42,812.2507 ms | 10,646.9006 / 37,624.1409 ms |
| Go heap start / end (B) | 2,238,208 / 802,392 | 1,864,624 / 2,691,512 |
| Goroutines start / end | 3 / 3 | 3 / 3 |
| Received SHA-256 | `d580d8c97c2871e957e2fff8b11cb6812c9f44cde87ef206a55bd386f3347acd` | `dbac8b08fcda1fe0d041eb328d1d02e2a831ba78d10a88dc1d28f64f55db5621` |

Every data block was checked in order while receiving. Independent verification
recomputed both hashes and checked aligned/minimum bytes, cross-peer counts,
duration and reverse control. Both race-enabled peers exited successfully;
no race diagnostic was detected. Traffic continued throughout the session.
This is a paced synthetic workload, not a maximum throughput or FPS benchmark.
Heap endpoints are not peak RSS or a complete leak proof.

After the script emitted `remote_cleanup_complete`, a separate pinned SSH
command queried `Win32_Process`, `Get-NetFirewallRule` and `Get-NetTCPConnection`:
both run directories had **0 remaining processes and 0 temporary firewall
rules**; **0 port 9009 listeners** remained. Local `Get-Process` also found
**0 remaining proof processes**. Existing SSH/RDP access, AWS security groups
and the instance itself were deliberately left unchanged.

### Decision and latency limitation

The remote data-integrity, encryption/authentication code path, reverse-control
delivery and >=10-minute continuous-session experiment passed on this named
topology. There is no longer a missing two-machine remote test.

However, application-level control RTT averaged **10.6–11.8 seconds**, peaked at
**42.8 seconds**, and only 51 pings per direction completed in 605 seconds.
This is not acceptable evidence of low-latency screen viewing. No specific
G0 RTT threshold was defined, so the correctness result remains PASS, but the
real-time suitability question needs diagnosis before claiming readiness.
Do not blame AWS, PAKE, the ISP or relay alone without measurements isolating
network RTT, TCP queues, socket scheduling and workload rate/backpressure.

## Remaining gate

Next focused work: explain/control the observed delay under bounded workload
and reconcile remaining B-04 native/capture and license checks. Do not silently
change crypto, relay provider or media framing to hide the latency.
G0 was not declared PASS solely because this remote correctness experiment
passed; the later native/license/self-review closeout is linked above.

## Post-move continuity and access check

The owner later reported moving location and accidentally sleeping the laptop.
Local Windows System event-log inspection found:

- Sleep entry: 2026-11-16 15:28:56–15:28:57 local time (UTC+07),
  Kernel-Power event 42; reason Button or Lid.
- Resume: 15:40:14–15:40:15 local time, Power-Troubleshooter event 1.
- The completed proof runner had already exited when checked at **15:26:57**.
  Thus this recorded sleep was **after**, not during, the accepted 605s proof.
  It does not explain the RTT outliers from that run. No rerun is required
  solely because of this later sleep.

At 15:43:31 the local public IPv4 had changed. A pinned, key-only SSH
`whoami` attempt to the existing EC2 address timed out after 8 seconds.
This is consistent with the old source-restricted firewall/security-group
rules, but timeout alone does not establish which layer blocked it.
Owner action needed: authorize the current client IPv4 in AWS SSH ingress
and EC2's Windows SSH firewall rule; keep the same verified host key.
No old key, AWS rule or existing instance was deleted or replaced.

The owner subsequently updated the rules; SSH access was restored.
[Latency follow-up](G0_LATENCY_FOLLOWUP.md) records the same-binary comparison
on the new network and the status of bounded-rate diagnostics.

The saved JSON results were independently reverified again after the move:
its `.results` array was piped to the hash-matched executable with
`-mode verify -duration 10m`; result **RESULTS_VERIFIED**. Documentation checks:
`git diff --check` PASS, 135 local Markdown link targets PASS and no tracked
upstream diff. Only evidence/documentation changed in this follow-up; package
tests were not rerun because runtime source was unchanged.

AWS resources and SSH access remain owner-managed; the automation must not
terminate the EC2 instance, delete existing keys or modify AWS security groups.
