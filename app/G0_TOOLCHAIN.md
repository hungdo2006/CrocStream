# G0 — Portable toolchain and adapter-design follow-up

> Historical tooling/design checkpoint. The subsequent implementation and runtime
> evidence are recorded in [G0_LOCAL_PROOF](G0_LOCAL_PROOF.md) (2026-11-16).
> Final feasibility gate: [G0 PASS](G0_CLOSEOUT.md). Earlier failures/blocked
> statuses below are historical, not relabeled as passing upstream regressions.

Date: 2026-11-15. This is the follow-up to the initial
[BLOCKED survey](G0_FEASIBILITY.md), after the owner approved portable tooling
and the croc-based adapter design/review work.

## Approval and scope

- Approved: Go portable 1.27.4, Wails CLI 3.1.0, dependencies for G0, and
  concrete adapter design/review under option A.
- Current design: [G0_ADAPTER_DESIGN.md](G0_ADAPTER_DESIGN.md).
- New cryptographic derivative/record protocol is a proposal for review, not
  an implemented or independently audited transport.
- Remote relay target/permission and second machine remain unspecified.

## Environment and installation

Windows X64, PowerShell 5.1; repository HEAD at start:
`565c192c536c7fd926ba7fa940d817cee13ed070`.
Pre-existing onboarding/survey changes were present and retained.

Portable root:
`C:\Users\Admin\AppData\Local\Temp\opencode\crocstream-g0`.

| Item | Observed result |
| --- | --- |
| Go archive | `go1.27.4.windows-amd64.zip`, official metadata from `https://go.dev/dl/?mode=json&include=all` |
| Verified SHA-256 | `178ab98950ac69ab25069307dc6ade79f3816e414cce95b4da166a115cde1e7e` |
| `go version` | `go version go1.27.4 windows/amd64` |
| `go env GOOS GOARCH CGO_ENABLED CC` | `windows`, `amd64`, `1`, `gcc` |
| `GOROOT` | portable root + `go1.27.4\go` |
| `GOMODCACHE` | portable root + `gomodcache` |
| `GOTOOLCHAIN` | `local`; no automatic replacement toolchain |
| Wails CLI | Installed `v3.1.0`; `go version -m` confirms module version and Go 1.27.4 |
| Wails module checksum | `h1:ld0AXSicXiTTsIvmTq3qNn9FzCP8vIHRSJdkRLgaAo8=` |
| `wails3 doctor` | Exit 0, “No issues found”; WebView2 `157.0.4341.46` detected |
| Packaging tools | Doctor reports MSIX Packaging Tool, MakeAppx, NSIS and SignTool not installed; signing not configured |
| Machine reported by doctor | i7-13650HX, RTX 4060 Laptop GPU, 23 GB RAM; Windows branding 11 Home / build 26300 |

Scripts: [Install-Toolchain](g0/Install-Toolchain.ps1),
[Use-Toolchain](g0/Use-Toolchain.ps1),
[Inspect-Environment](g0/Inspect-Environment.ps1).
Exact reproduction steps: [g0/README.md](g0/README.md).

## Commands and results

Commands run from `D:\CrocStream`. For Go commands, the first line is always:

```powershell
. .\app\g0\Use-Toolchain.ps1
```

| Command | Status / observed output |
| --- | --- |
| `powershell -NoProfile -File app/g0/Install-Toolchain.ps1 -Component Go` | PASS: verified archive, installed Go version matches |
| Same Go installer command, second invocation | PASS: detects existing 1.27.4 without reinstalling |
| `powershell -NoProfile -File app/g0/Install-Toolchain.ps1 -Component Wails` | PASS, version-qualified install; module provenance inspected |
| `wails3 doctor` | PASS for reported development prerequisites; not installer/signing/capture acceptance |
| `go -C croc test -count=1 -timeout=120s ./src/codephrase ./src/pakekey ./src/crypt ./src/comm ./src/tcp` | PASS: all five packages; 1.599s / 1.054s / 1.507s / 2.647s / 7.814s reported |
| `go -C croc test -race -count=1 -timeout=180s ./src/codephrase ./src/pakekey ./src/crypt ./src/comm ./src/tcp` | FAIL: first four packages passed; TCP `TestWrongPassword` expected “bad password”, received Windows connection reset; no race-detector warning in this output |
| `go -C croc test -count=1 -timeout=180s ./...` | FAIL: `src/cli`, `src/croc`, `src/install`, `src/utils`; details below |
| `go -C croc vet ./...` | PASS, exit 0 |
| With `GOFLAGS=-mod=mod`: `go -C app/g0/buildprobe mod tidy`; then `GOFLAGS=-mod=readonly`: `go -C app/g0/buildprobe build -buildvcs=false -o "$env:GOBIN\g0-buildprobe.exe" .`; run resulting executable | PASS: `G0_BUILD_PROBE_OK: linked Go/Wails/croc; no GUI/network/capture tested` |
| `go -C app/g0/buildprobe vet ./...` | PASS |
| `go -C app/g0/buildprobe list -m -json all` | PASS: resolved module graph; local croc replacement and Wails 3.1.0 confirmed |
| `go -C app/g0/buildprobe list -deps -f '{{if .Module}}{{.Module.Path}}\|{{.Module.Version}}\|{{.Module.Dir}}{{end}}' .` | FAIL: VCS status exit 128 under Git ownership checks |
| Same dependency listing with `-buildvcs=false` | PASS; inspected root license files and SHA-256 for 21 Windows build-dependency modules |
| `go -C croc test -race -count=1 -run '^TestWrongPassword$' -timeout=30s ./src/tcp` | FAIL in isolation with the same connection-reset/error-text mismatch (2.495s package time) |
| `powershell -NoProfile -File app/g0/Inspect-Environment.ps1 -VerifyUpstream` after test artifact cleanup | PASS: tree MATCH, upstream working tree clean, both portable executables present; `2026-11-15T15:17:10.5824530Z` |
| Re-run Wails installer with an existing executable | Expected exit 1: refuses overwrite; no reinstall attempted |
| `powershell -NoProfile -File app/g0/Use-Toolchain.ps1 -ToolRoot D:/CrocStream/docs` | Expected exit 1: missing portable Go; fails before creating environment/cache directories there |
| `gofmt -l app/g0/buildprobe/main.go` and PowerShell `Parser.ParseFile` over the three scripts | PASS: no formatting/syntax errors |
| `git -c safe.directory=D:/CrocStream diff --check` | PASS for tracked diff |
| Inline PowerShell Markdown target/heading-anchor scan over root docs, `docs/` and `app/` | PASS: 112 local links/anchors across 12 Markdown files |
| Inline whitespace/conflict-marker scan of all `app/` files | PASS |

No timing above is a media/transport benchmark.

### Dependency/license scope

`buildprobe/go.mod` and `go.sum` record the resolved G0 probe dependencies.
This is not a production module. `go list -m all` contains modules inherited
from dependencies' requirements that are not necessarily linked into this
Windows executable; do not equate that list with the runtime dependency set.

The successful `go list -buildvcs=false -deps` inventory found root license files
for all 21 non-main modules in this Windows package dependency set:

| Root license evidence | Modules / versions |
| --- | --- |
| MIT-marked text | croc pinned local tree; Wails v3.1.0; pake v3.2.1; xxhash/v2 v2.3.0; go-ole v1.3.0; imohash v1.1.1; go-colorable v0.1.16; go-isatty v0.0.24; colorstring `d06e56a500db`; uniseg v0.4.7; progressbar/v3 v3.19.1; siec `c2c6f6198406` |
| BSD-style root text | edwards25519 v1.2.0; murmur3 v1.2.2; x/crypto v0.59.0; x/net v0.62.0; x/sys v0.50.0; x/term v0.48.0; x/text v0.44.0 |
| Apache-2.0-marked text | highwayhash v1.0.4 |
| zlib-style text (also read in initial survey) | connectproxy `3582e84f0c9b` |

This is a root-file inventory, not a completed per-file redistribution-notice
audit. Wails CLI tooling has a larger dependency set. Capture/frontend and
the future adapter are not yet in this module; their license inventories
remain to be completed. No production dependency decision is implied.

### TCP baseline failure to investigate

`TestWrongPassword` at `croc/src/tcp/tcp_test.go:704–714` starts a relay on fixed
port 8385 and asserts that the returned error contains `bad password`.
The race run rejected the wrong password but returned a socket reset before
the expected error text reached the caller. The initial race run overlapped
the full test job, which also uses fixed ports. After that job finished, an
isolated run reproduced the same failure. It is therefore not explained solely
by overlapping jobs. Do not run network test jobs concurrently in future
baseline comparisons.

This is not evidence of password acceptance or a detected Go data race.
The adapter must not promise that every relay-auth failure is distinguishable
from network disconnect; retain the PRD's “when safely observable” qualifier.

Source hypothesis, not a proven fix: `tcp.go:504–513` sends the rejection then
returns, causing connection closure, while the client pipelines a room frame.
Unread incoming data at close may explain a Windows reset. No upstream patch
was made. Acceptance for the future adapter must require denial and cleanup
even if the platform does not deliver a specific rejection string.

### Full upstream suite failures on this Windows environment

| Package / test | Observed failure |
| --- | --- |
| `src/cli`: `TestBestPublicRelayCacheUsesAddressAndCurrentPoolOrder` | File permissions reported `666`, expected `600` |
| `src/cli`: `TestFormatUnixSSHCodeMessage` | Windows-form command does not contain the UNIX environment-variable syntax expected by the test |
| `src/croc`: `TestCrocSymlink`, symlink cases in `TestReceiveStdinNamedFileCollision` | Required Windows symlink privilege not held |
| `src/install`: installer execution tests | `/bin/bash` unavailable under the tested Windows execution path; symlink case also lacks privilege |
| `src/install`: `TestLinux32BitTargetsRemainInInstallerAndReleaseBuilds` | Literal `name: Linux-ARM\n` not found; read-only check confirms checkout has `name: Linux-ARM\r\n` |
| `src/install`: `TestStampVersion` | File permissions reported `666`, expected `640` |
| `src/utils`: `TestUnzipThroughOpenedRootResistsDestinationReplacement` | Windows rejects renaming an open destination directory |

The full run reported successful ordinary tests for `src/tcp`, `src/pakekey`,
`src/comm`, `src/crypt`, `src/sshshare`, `src/tailcattransport` and other packages.
Those package passes do not override the aggregate FAIL or establish a CrocStream
session. Full `go test -race ./...` was NOT RUN: the bounded G0-relevant race set
already found a reproducible failure, and the ordinary full baseline has known
environment failures. No privileges, OS settings, or upstream tests were changed
to force a pass.

Upstream full tests have side effects: CLI fixtures emit temporary invitation/
token text and some exercise clipboard copying. Raw test output is not copied
into project documentation; all reported findings above omit those values.
Future CrocStream harness must suppress secret-bearing diagnostics and avoid
clipboard access. These upstream fixture runs are not remote-relay acceptance.

### Workspace cleanup and inventory guard

The first post-install inventory correctly failed because the full upstream
tests had created untracked `croc/src/croc/.gitignore`. Its exact contents were
`LICENSE`, matching `TestCrocIgnoreGit` at `croc_test.go:1281–1306`; that test
opens the file without closing it before deferred removal, relevant on Windows.
It was absent in the pre-test clean inventory and still present after the test
process finished. Removed only this generated fixture, then reran provenance:
`croc/` clean and upstream tree still matches. No tracked upstream files changed.

## Gate assessment

G0 has not passed. Installing Go resolves the missing-Go obstacle in the
portable environment; it does not close B-04, which also requires native stack
and capture compatibility. Two-process 10 MiB/10-minute duplex proof, negative
security tests for the adapter, and remote acceptance have not run.

Next gate: **G0 adapter contract review**, then implementation/tests if the
specific purpose/key-schedule/record proposal is accepted. Keep the upstream
baseline failures visible when comparing adapter regressions; do not treat
them as new application failures or silently mark them passed.

## Files changed in this follow-up

- New: `app/G0_ADAPTER_DESIGN.md`, `app/G0_TOOLCHAIN.md`, `app/g0/README.md`,
  `app/g0/Install-Toolchain.ps1`, `app/g0/Use-Toolchain.ps1`,
  `app/g0/buildprobe/{main.go,go.mod,go.sum}`.
- Updated: `app/g0/Inspect-Environment.ps1`, `app/G0_FEASIBILITY.md`,
  `README.md`, `docs/{PRD,ARCHITECTURE,TEST_PLAN,ROADMAP,GETTING_STARTED}.md`.
- Tools, downloads, module/build caches and build-probe executable are in the
  approved portable root outside Git. No commits/pushes were made.
