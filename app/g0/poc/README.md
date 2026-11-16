# G0 secure-duplex PoC

Experimental CLI only: no UI, media codec, capture or stock croc-client support.
Source: `croc/` at `552f40fd2d7593316118370bec0a93478d48e715`.
The module uses a local `replace`; it must be built inside this repository.

Final feasibility gate: **[G0 PASS](../../G0_CLOSEOUT.md)**. Keep the reviewed
[third-party notices](../licenses/README.md) with G0 binaries; prototype PASS
does not certify production or media/capture/UI behavior.

## Run the two-process local proof

From repository root, PowerShell:

```powershell
. .\app\g0\Use-Toolchain.ps1
powershell -NoProfile -File app/g0/Inspect-Environment.ps1
go -C app/g0/poc build -race -buildvcs=false -o "$env:GOBIN\g0poc-verified.exe" ./cmd/g0poc
& "$env:GOBIN\g0poc-verified.exe" -duration 10m
```

The parent opens an OS-assigned **127.0.0.1** TCP listener using the actual croc
relay implementation and launches separate broadcaster/viewer OS processes.
The parent privately forwards the invitation between their anonymous pipes.
Passwords/codes are never command-line flags or files. Parent stdout contains
only setup status and the final JSON measurements; child stdout is private IPC.
Do not launch internal `-mode host/viewer` with stdout redirected to a public
log: host's first IPC message contains the invitation.

The two peers perform relay admission, CrocStream-purpose upstream-derived PAKE,
mutual confirmation, then directional AES-GCM records. The parent independently
checks the peers' counts and expected hashes. `LOCAL_PASS` means this local
synthetic-data scenario passed; it does not mean the remote MVP passed.

For a smoke run use `-duration 1s`. Each peer still sends at least 320 blocks of
32 KiB (10 MiB), so the minimum run takes roughly 6.4 seconds. Long runs send
one block every 20 ms throughout the duration, not a burst followed by sleep.
This artificial pacing is not a throughput capacity benchmark.

For diagnostic load comparison, local mode accepts `-send-interval 100ms`.
Private peer setup also accepts `SendInterval` (Go nanoseconds); zero/default
is 20 ms, supported values are integer milliseconds 20..100. Each sample is
still 32 KiB, so the 100-ms minimum 10-MiB smoke takes about 32 seconds.
`data_write_mean_ms`/`data_write_max_ms` measure stream-write time, not confirmed
delivery or physical network latency.

Ping/Pong and KeyframeRequest-equivalent harness packets share the same secure
stream as data. They are not G1 media packets and do not implement the media
PLI rate policy. One application writer per process prioritizes queued replies,
one reader validates every data sample; control queue capacity is 8 and only
one locally initiated ping is outstanding. Stop with Ctrl+C; the parent cancels
the relay and terminates/reaps its child processes.

## Tests and exact module scope

```powershell
go -C app/g0/poc test -count=1 -timeout=120s ./...
go -C app/g0/poc test -race -count=1 -timeout=120s ./...
go -C app/g0/poc vet ./...
go -C app/g0/poc test ./internal/transport -run '^$' -fuzz '^FuzzRecord$' -fuzztime=15s -parallel=2
go -C app/g0/poc test ./internal/transport -run '^$' -fuzz '^FuzzAuthenticatedPlaintext$' -fuzztime=15s -parallel=2
go -C app/g0/poc test ./internal/transport -run '^$' -fuzz '^FuzzHandshakeBody$' -fuzztime=15s -parallel=2
```

`go.mod`/`go.sum` are the PoC's resolved dependencies, not production pins.
Ordinary upstream tests are separate; their known Windows failures remain in
`app/G0_TOOLCHAIN.md`. The PoC's lifecycle test runs its own relay with a higher
admission limit solely to exercise 100 sessions; the executable's relay keeps
upstream default admission limits.

## Ownership / security contract

- `transport.Host` returns a listener with invitation after relay admission,
  before peer authentication. `Accept` is one-shot. `Join` and `Accept` return
  `io.ReadWriteCloser`-compatible sessions only after mutual confirmation.
- Parent and accept contexts own the session, so they must remain alive while
  streaming. Cancel or idempotent `Close` unblocks socket I/O.
- Upstream `Comm.Receive` is not used after admission; a bounded `io.ReadFull`
  parser owns read deadlines independently of write deadlines.
- 8 KiB peer-handshake envelope, <=4096-byte PAKE values; 64 KiB data records;
  sequence/direction checks; random nonce using croc AEAD helpers; per-direction
  record budget 2^20. Last slot reserved for authenticated CLOSE.
- EOF without authenticated CLOSE is a disconnect error; an invalid record
  permanently closes the session without returning its contents.
- No PAKE algorithm or relay implementation is patched in `croc/`. Derived
  schedule and provenance/license live in `internal/upstreampake/`.
- Upstream relay admission remains a separate trust boundary: its client
  receive allocations are bounded by upstream Comm (up to 64 MiB), not our
  8 KiB peer-handshake limit. This inherited limit needs further review before
  a production adapter. The PoC is not an independent security audit.

## Remote proof and reproduction

There is no default public-relay experiment. The named Windows EC2 topology
has now passed two-machine >=600s proofs; see the reports below for differing
network conditions and remaining overall G0 review items. To reproduce a
remote proof, an operator must provide an authorized endpoint and second machine.
The reusable `Host`/`Join` API accepts that endpoint; private peer modes use
bounded JSON setup over stdin with fields `Address`, `Password`, `Invite`,
`Duration` (Go nanoseconds). The SSH orchestrator below passes the host's private
invitation to the viewer on the other machine and collects only final results.
See the remote evidence report for execution status; script existence is not
itself acceptance evidence.

### Automated SSH proof

`Invoke-SshProof.ps1` automates the laptop → public EC2 relay → EC2-local viewer
topology after SSH has been configured and the server host key has been checked
against a fingerprint obtained through trusted RDP. It never accepts an unknown
host key. Use a dedicated identity and known-hosts file:

```powershell
. .\app\g0\Use-Toolchain.ps1
go -C app/g0/poc build -race -buildvcs=false -o "$env:GOBIN\g0poc-remote.exe" ./cmd/g0poc
powershell -NoProfile -File app/g0/poc/Invoke-SshProof.ps1 `
  -Server 'EC2_PUBLIC_IPV4' `
  -IdentityFile 'C:\path\to\dedicated_private_key' `
  -KnownHostsFile 'C:\path\to\verified_known_hosts' `
  -Executable "$env:GOBIN\g0poc-remote.exe" `
  -ClientAddress 'LAPTOP_PUBLIC_IPV4' `
  -DurationSeconds 600
```

This creates a unique directory on EC2, verifies transferred binary SHA-256,
starts the real upstream croc `tcp.RunWithListener` with a random password,
privately relays the invitation over SSH and checks both peer results. Unlike
the stock croc relay CLI it needs only port **9009** because it does not route
file-transfer ports. The media/test payload uses public TCP 9009, **not an SSH
port forward**; only setup, binary transfer and result collection use SSH.

AWS security group must allow TCP 22 and 9009 from the actual SSH-client public
IP. The script adds/removes only its own source-restricted Windows Firewall
rule. It refuses to replace an existing listener on port 9009. On cleanup it
terminates only processes running the executable in its unique directory;
binary directories remain for inspection. The relay has a hard duration+90s
lifetime; peers retain their existing duration+45s timeout. Automatic handoff
avoids using that timeout allowance on manual copying.

Use `-DurationSeconds 1` for a >=10-MiB-each-way remote smoke; only >=600s can
return `REMOTE_RELAY_PASS`. `remote_cleanup_complete` must also be present.
`-DataIntervalMilliseconds 100` selects the slower diagnostic rate; the
helper verifies that the executable actually reports the requested pacing.
Route labels describe this explicit topology, not public-relay or general
cross-platform compatibility. See [remote evidence](../../G0_REMOTE_PROOF.md)
for what was actually executed.
Further rate/latency observations are in the
[follow-up report](../../G0_LATENCY_FOLLOWUP.md).

### Independently verify remote peer results

Save only the two final sanitized peer JSON objects in a JSON array, then run:

```powershell
Get-Content -Raw .\peer-results.json |
  & "$env:GOBIN\g0poc-remote.exe" -mode verify -duration 10m
```

`RESULTS_VERIFIED` checks distinct host/viewer roles, minimum and aligned byte
counts, duration, pings/pongs, reverse key-request counters and independently
recomputed deterministic SHA-256. Input/hash work is bounded. It does not prove
the route or that manually supplied JSON came from authentic processes.

### Windows peer helper

`Run-RemotePeer.ps1` wraps the internal child protocol without putting the relay
password or invitation in command-line arguments or PowerShell command text:

```powershell
powershell -NoProfile -File app/g0/poc/Run-RemotePeer.ps1 `
  -Mode host `
  -Executable C:\g0\g0poc-verified.exe `
  -RelayAddress 198.51.100.10:9009 `
  -DurationMinutes 10

powershell -NoProfile -File C:\g0\Run-RemotePeer.ps1 `
  -Mode viewer `
  -Executable C:\g0\g0poc-verified.exe `
  -DurationMinutes 10
```

Host prints the invitation once for manual transfer; do not save or paste it
into logs. Viewer prompts for the invitation if `-Invite` is omitted. The
script keeps setup JSON in memory and prompts for the relay password as a
hidden SecureString input, but the child process necessarily receives the
plaintext password in its private stdin during setup. The helper is for the
G0 experiment, not a production secret-management solution.

Repeat >=10 MiB each way, >=10 minutes, hash/order and reverse control on the
approved route; record network/hardware, relay policy/capacity and cancellation.
Do not count two processes on one host as two-machine or Internet-relay evidence.
