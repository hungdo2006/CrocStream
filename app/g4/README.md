# G4 verification

G4 uses the verified G0 secure duplex and protocol-v1 LZ4 media. It is not
compatible with stock croc send/receive. Overall gate status:
[G4 closeout](https://github.com/hungdo2006/CrocStream/blob/8e10002f7997bb1a12a8f2996d883610e1072b4d/app/G4_CLOSEOUT.md).

## Local synthetic proof — two OS processes

From `app/`:

```powershell
. .\g0\Use-Toolchain.ps1
go build -race -buildvcs=false -o "$env:GOBIN\g4proof.exe" ./g4/proof
& "$env:GOBIN\g4proof.exe" -duration 30s
```

The parent owns an ephemeral 127.0.0.1 upstream croc relay. Separate host and
viewer child processes exchange CONFIG/KEY/DELTA, Ping/Pong, PLI and END.
Every synthetic canonical image is checked against independently generated
expected pixels. The parent verifies both peers' final hash, IDs, total
media bytes, counter alignment, duration and exit status.

Invitations/passwords travel only through private pipes. **Do not** launch
`-mode host` directly with stdout redirected to a public log: its first IPC
line contains an invitation. Normal parent output has numeric counters and
hashes only. No images or keys are written to disk.

## Local desktop proof — explicit capture consent required

The owner authorized local actual display capture for this task. Keep sensitive
content closed before running. From `app/`:

```powershell
. .\g0\Use-Toolchain.ps1
$env:WAILS_EXP_USE_WAKE='1'
wails3 build
go build -tags production,g3test,g4test -buildvcs=false -o "$env:GOBIN\crocstream-g4-ui.exe" .
go build -race -buildvcs=false -o "$env:GOBIN\g4proof.exe" ./g4/proof
node g4/ui-proof.mjs "$env:GOBIN\crocstream-g4-ui.exe" "$env:GOBIN\g4proof.exe" 30
```

The harness launches its own relay and two instrumented Windows apps with
distinct temporary WebView profiles. It presses Share Start, privately forwards
the invitation into Watch, verifies rendered non-black canvases and PLI recovery,
then Stop/disconnect/canvas clearing, process exit and both closed bridge ports.
The only capture/stream target is loopback. No screenshot, tracing or recording.
The ordinary production executable does not enable test CDP or decoder fault
injection; these require the explicit `g3test,g4test` tags.

## Remote synthetic proof — PASS on the measured EC2 route

2026-11-17 follow-up: owner restarted EC2 and supplied `47.128.220.28`.
Its key matched the old trusted record selected with
`-PinnedHostKeyAlias 54.151.152.78`. Remote smoke, >=600s proof and independent
cleanup all passed; see [G4_REMOTE_PROOF](https://github.com/hungdo2006/CrocStream/blob/8e10002f7997bb1a12a8f2996d883610e1072b4d/app/G4_REMOTE_PROOF.md). Do not change
the historical known-host record or accept a new key just because the IP changed.

From repository root, after the owner confirms EC2 address/host key/ingress:

```powershell
. .\app\g0\Use-Toolchain.ps1
go -C app build -race -buildvcs=false -o "$env:GOBIN\g4proof.exe" ./g4/proof
powershell -NoProfile -File app/g4/Invoke-SshProof.ps1 `
  -Server '<confirmed EC2 public IPv4>' `
  -IdentityFile 'C:\path\to\dedicated_private_key' `
  -KnownHostsFile 'C:\path\to\verified_known_hosts' `
  -Executable "$env:GOBIN\g4proof.exe" `
  -ClientAddress '<current laptop public IPv4>' `
  -DurationSeconds 600
```

Strict host-key checking is mandatory; do not accept a changed key based solely
on the current network. The owner must check a changed fingerprint through
trusted RDP. TCP 22/9009 ingress must allow the current laptop public IPv4.
If the owner confirms the same EC2 instance has a new IPv4, pass
`-PinnedHostKeyAlias '<previously verified known_hosts entry>'`. SSH will still
require that exact previously trusted host key at the new address; it will not
trust a key learned from the new network.

The script refuses an occupied port 9009, creates a unique directory, verifies
binary SHA-256 and temporarily allows only the supplied source address in
Windows Firewall. Media uses public TCP 9009, not SSH port forwarding. SSH is
only management/transfer/setup. On exit the script kills only the executable
in its own unique directory and removes only its named temporary rule.
Unique remote binary directories remain for inspection; EC2/SSH access remains
owner-managed and can still incur charges.

Remote content is **synthetic only**, never capture. The G4 proof binary imports
the capture data types but never instantiates or invokes a real capture source.
Both `REMOTE_MEDIA_PASS` and `remote_cleanup_complete` are required.
G0's historical remote transport PASS does not count as G4 media proof.
