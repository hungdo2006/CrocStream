# G4 remote media proof — owner-restarted EC2

Date: 2026-11-17. **REMOTE_MEDIA_PASS, >=600s, independent verification and cleanup PASS.**
Together with the local desktop/codec/lifecycle evidence, this closes G4 PASS
for the measured prototype scope. Real remote-screen/UI performance is not
inferred from synthetic media.
This report follows the earlier EC2-unreachable checkpoint in
[G4_CLOSEOUT](G4_CLOSEOUT.md); it does not overwrite those observed timeouts.

## Authorization and trusted identity

The owner restarted the same EC2 instance and supplied its new public IPv4
`47.128.220.28`. Remote proof is authorized for generated images only:
**no display, camera, screenshot, JPEG recording or real screen capture**.

SSH to the new address succeeded with `StrictHostKeyChecking=yes`,
the previously verified dedicated `UserKnownHostsFile`, and
`HostKeyAlias=54.151.152.78` selecting the G0 trust record. The ED25519 key
remains **SHA256:HUNOkgkjbFw0ZupdPSk04KpxjXynFJ+STZz+Geyd+yE**.
No network-learned key was accepted, host-key check disabled, AWS credential
used, or broad persistent firewall rule created. The known-host file and
private identity stay in the previously approved external tool root.

The initial authenticated inspection found no listener on TCP 9009 and
reported the SSH source IPv4 as `42.115.146.175`. Each proof creates only a
unique run-owned source-restricted Windows Firewall rule for that address.
AWS Security Group and existing SSH/RDP access remain owner-managed.

## Route, build and environment

```text
Local Windows host process
    -> public EC2 TCP 9009 / real upstream croc relay
    -> EC2-local viewer process on 127.0.0.1:9009
```

Media is not tunneled through SSH. SSH/SCP carry binary transfer, private
setup/invitation and sanitized results; application media uses public TCP 9009.
The relay password is random per run. All traffic uses the existing G0 PAKE,
mutual confirmation and directional AEAD; inside that duplex are actual
G1 CONFIG/KEY/DELTA/PLI/PING/PONG/END packets and the G2 lossless LZ4 codec.

Local: Windows x64, Go **1.27.4**, existing compiler for `-race`.
EC2: **Windows Server 2022 Datacenter**, build **20348**, x64,
Intel Xeon Platinum **8488C**, **2 logical processors**, visible memory
**8,224,276 KiB**. Authenticated environment sample:
[remote-environment-20261117.json](../evidence/g4/remote-environment-20261117.json).
These are the observed specs, not a claim about a particular EC2 instance SKU.

Synthetic dataset: 640×360 opaque RGBA, black background with a changing
64×64 colored region, deterministic RGB functions of emitted frame ID.
CONFIG requests 24 FPS and KEY interval 30, tau=4. Each decoded reference is
compared against an independently generated expected frame. The proof's
bounded producer may block on admission; this is **not a capture/render or
24-FPS Internet performance benchmark**.

Executable `g4proof-remote-20261117.exe`:

- Built with `go -C app build -race -buildvcs=false -o <portable-bin>/g4proof-remote-20261117.exe ./g4/proof`.
- SHA-256 **2F2D4A90A4A020B5EB04A8D02F6A93E4EFE68F6D1CDF333594DF8CDDE6162A56**.
- Same executable used by local host, remote viewer and remote relay; SCP
  content is checked with `Get-FileHash` before execution.
- Runtime/source fingerprints, repository HEAD and build command:
  [remote-source-20261117.json](../evidence/g4/remote-source-20261117.json).
- Application/runtime/codec/crypto code was not changed during these runs.
  Only the SSH helper gained an optional pinned alias and an extra cleanup
  rule check; no wire bytes, primitive or relay provider changed.

## Commands actually executed

From the repository root after `. .\app\g0\Use-Toolchain.ps1`:

| Command/check | Observed result |
| --- | --- |
| Strict SSH probe to owner-supplied new IP, selecting the old pinned host key via `HostKeyAlias` | PASS, `G4_SSH_READY`, source IP and Windows Server 2022 verified |
| PowerShell AST parse of `app/g4/Invoke-SshProof.ps1` | PASS |
| `go -C app test -race -count=1 -timeout=60s ./g4/proof` | PASS, 2.580s |
| `go -C app build -race -buildvcs=false -o "$env:GOBIN\g4proof-remote-20261117.exe" ./g4/proof` | PASS, executable hash above |
| `go version -m` on the earlier verified binary | Go 1.27.4, race=true, windows/amd64, local pinned croc and G0 replacements; rebuilt remote binary is separately hash-pinned |
| `ssh-keygen -lf <verified-known-hosts>` | ED25519 fingerprint matches G0 trust record above |
| Runtime-source fingerprint and upstream diff check | PASS: 21 non-test Go files recorded, `croc/` unchanged |
| `go -C app test -count=1 -timeout=180s ./...` during this follow-up | PASS: proof 3.311s, bridge 5.384s, capture 1.794s, delta 25.386s, frame 1.751s, preview 4.015s, protocol 2.270s, session 14.824s, telemetry 1.773s |
| `go -C app test -race -count=1 -timeout=600s ./...` | PASS: proof 2.827s, bridge 4.677s, capture 1.769s, delta 410.503s, frame 1.804s, preview 2.981s, protocol 2.056s, session 45.919s, telemetry 1.506s; no detected race on executed paths |
| `go -C app vet ./...`; `go -C app mod verify` | PASS; all modules verified |
| `go -C app/g0/poc test -race -count=1 -timeout=180s ./...` | PASS: harness 4.108s, façade 3.820s, transport 18.791s, PAKE 2.174s |
| G0 `vet ./...`; `mod verify` | PASS; all modules verified |
| Invalid `PinnedHostKeyAlias` with spaces | Rejected before SSH by parameter validation; no network side effect |
| Separate verifier on saved 30s and 600s results | Both `RESULTS_VERIFIED` using hash-pinned remote binary |

Smoke invocation (private key path intentionally represented symbolically):

```powershell
powershell -NoProfile -File app/g4/Invoke-SshProof.ps1 `
  -Server 47.128.220.28 `
  -IdentityFile <existing-dedicated-key> `
  -KnownHostsFile <G0-verified-known-hosts> `
  -PinnedHostKeyAlias 54.151.152.78 `
  -Executable "$env:GOBIN\g4proof-remote-20261117.exe" `
  -ClientAddress 42.115.146.175 `
  -DurationSeconds 30
```

The long run uses exactly this command with `-DurationSeconds 600`.
No passwords, invitation or token appear in arguments or evidence files.

## 30s remote smoke — PASS

Run ID **fb21f2b869fd47a4988b993e6ddc8de0**.
Saved sanitized output:
[remote-smoke-20261117.jsonl](../evidence/g4/remote-smoke-20261117.jsonl).

| Observation | Local host | EC2 viewer |
| --- | ---: | ---: |
| Active duration | 30.232713s | 30.235270s |
| Emitted / received video packets | 371 | 371 |
| Decoded successfully | — | 369 |
| KEY / DELTA | 14 / 357 | 14 / 355 |
| Application video bytes | 115,922 sent | 115,922 received |
| PLI accepted / requested | 1 | 1 |
| Successful reference recovery | — | 1 |
| Matching Pong observations | 29 | 30 |
| Latest application RTT | 46.7249ms | 57.4643ms |
| Last video ID | 371 | 371 |
| Goroutines start → end | 2 → 1 | 2 → 1 |

Final expected/received reference SHA-256:
`4d1b3664e14867fdf43f1d736d348383e5f34b149f3e6a16fee376dfecd5aa00`.
Both peers match. The two undecoded packets were consumed while reference was
deliberately invalid for the reverse-PLI test; forward KEY recovery succeeded.
No frame was silently skipped and then used as a delta reference.

The helper's independent verifier returned `RESULTS_VERIFIED` internally,
output `REMOTE_MEDIA_PASS`, then `remote_cleanup_complete`. Remote test
processes and the run-owned firewall rule were removed by the helper.
This 30s smoke alone does not close the required >=600s remote gate.

## 600s remote acceptance — PASS

Run ID **3f434ebe75a24f6183f2e298faed3c0f**; exactly the same hash-pinned binary,
source, role configuration and route as the smoke. Saved sanitized output:
[remote-600s-20261117.jsonl](../evidence/g4/remote-600s-20261117.jsonl).

| Observation | Local host | EC2 viewer |
| --- | ---: | ---: |
| Active duration | **600.277028s** | **600.282305s** |
| Emitted / received video packets | **6,839** | **6,839** |
| Decoded successfully | — | 6,837 |
| KEY / DELTA | 229 / 6,610 | 229 / 6,608 |
| Application video bytes | **2,021,033 sent** | **2,021,033 received** |
| PLI accepted / requested | 1 | 1 |
| Successful reference recovery | — | 1 |
| Matching Pong observations | 600 | 600 |
| Latest application RTT, not mean/p95 | 41.9350ms | 106.7834ms |
| Last video ID | 6,839 | 6,839 |
| Goroutines start → end | 2 → 1 | 2 → 1 |
| Validated application END | sent on normal host completion | received |

Final expected/sender/receiver canonical SHA-256:
**`037853fe872c219997c08231bc66d4bf42d61a83e18047f2f820341e65fb89f7`**.
Every image decoded outside deliberate recovery was checked against the
independently generated image. The two consumed-but-not-decoded DELTAs were
the intentional invalidation/PLI recovery interval, not unaccounted loss.
All 6,839 complete media packets/bytes were counted at the viewer.

The script checked both child exits and race diagnostics, independently
verified results, then output `REMOTE_MEDIA_PASS` and `remote_cleanup_complete`.
Afterward the saved result was reloaded and piped into:

```powershell
# $record is parsed from saved sanitized result, not live secrets:
($record.results | ConvertTo-Json -Depth 10 -Compress) |
  & "$env:GOBIN\g4proof-remote-20261117.exe" -mode verify -duration 600s
```

Result **RESULTS_VERIFIED**. Executable SHA-256 still matches the evidence and
all 21 recorded runtime-source hashes remain unchanged. The application and
adapter regression tests overlapped this proof; the sender produced about
11.4 frames/s under race/hash/scheduling load. Do not advertise this as a
24-FPS performance acceptance, a maximum throughput measurement, or an
isolated latency benchmark.

### Separate post-run resource verification

An additional pinned-key SSH command at **2026-11-17T12:36:00.3777266Z** checked
both run-specific executable paths and firewall rule names. It observed:

- **0 owned processes and 0 owned firewall rules for each run**.
- **0 TCP 9009 listeners**.
- Retained binaries in both unique run directories still have the exact
  expected SHA-256. These directories are left for inspection, not deleted.
- Independent local process inspection found **0 `g4proof-remote-20261117.exe`
  processes** after completion.

Saved independent result:
[remote-cleanup-20261117.json](../evidence/g4/remote-cleanup-20261117.json).
The EC2 instance, SSH/RDP and owner security groups are deliberately retained.

### Evidence hashes

| Artifact | SHA-256 |
| --- | --- |
| 30s smoke JSONL | `1F5A880A5A10684318852BD186B9D25C29A3A9E66C2DE6702B4CF2E6134516BE` |
| 600s acceptance JSONL | `76E5E12C4ED2044F5BE15D245A72AEED88EED0735F91BD21ACEC3F1751AE98D3` |
| Runtime-source manifest | `B1CB98F6601E784B428020321C98D47524B601BEA187396562859DFA935CCBA1` |
| Independent remote cleanup JSON | `1F3545ACB088E7C1CCF5F0477798A26D82C7D025FE5B60C2182BEA71BBD2A12F` |

One local test-command attempt used a PowerShell catch type unavailable in
Windows PowerShell 5.1; corrected to inspect the validation error ID, then PASS.
This was only the helper's invalid-argument test and changed no peer data.

Final completion checks: **170 local Markdown targets/anchors across 11 handoff
documents PASS**; `git diff --check` PASS; upstream-only tracked/untracked
inspection confirms pristine `croc/`. Both remote outputs, distinct run IDs,
binary hashes, cleanup records and unchanged runtime fingerprints cross-check.
All earlier uncommitted G4 implementation work was retained; this follow-up
does not commit, push, discard user work or start G5.

Files added in this follow-up: this report and `app/g4/evidence/remote-*`.
Files updated: SSH helper/usage, G4 closeout and current project-status docs.
No application/codec/crypto/dependency change, tool installation, commit,
push or EC2 termination. Next focused gate is **G5** after owner authorization.

## Limits

- Remote proof tests media engine/transport/codec and control with synthetic
  pixels, not desktop rendering on EC2. The local G4 desktop proof separately
  covers actual capture and viewer canvas. No remote real-screen claim.
- RTT is latest measured application Ping/Pong, not a mean/p95, one-way latency
  or capture-to-render time. Scheduling, codec work, queueing and race
  instrumentation affect it.
- Go goroutine endpoints do not measure peak RSS or prove every handle leak
  absent. Long UI resource/latency/UX acceptance remains G5.
- The instance, SSH/RDP and owner AWS rules are not terminated/removed by this
  task. EC2 may still incur charges. Only unique proof resources are cleaned.
- No stock croc media interoperability, arbitrary-network performance or
  independent crypto-audit claim.
