# G6 closeout — Windows portable and demo handoff

**G6 PASS for the unsigned Windows x64 portable prototype.**
Date: 2026-11-17. Starting HEAD:
`886f86611462f84b97c1b8644ca5cd43a0f23d00` (G5); starting tree clean.
The owner requested G6 and a restrained, practical UI. No commit, push, release
publication, new tool installation or EC2 operation was performed.

## Deliverable

Final local ZIP:
`app/release/g6-final-20261117/CrocStream-0.0.1-windows-amd64.zip`
(8,092,277 bytes, approximately 7.72 MiB), with adjacent `.zip.sha256`.
These generated artifacts are ignored by Git; the build recipe and evidence
are retained in source. Earlier/partial build folders are not the final handoff.

| Artifact | SHA-256 |
| --- | --- |
| Final ZIP | `B5CAB7E621A64516FCF15D332032FA325F42FA63C7C31B0D9630C13426165035` |
| Shipped `crocstream.exe` | `D7CB11BDA36FB4E8BE78816B1FEB4D127750387A4E1B389AD6C694E1F62E42D0` |
| Shipped `crocstream-relay.exe` | `729CC73FC240F0D4F1F2AF1A7B2E52B581CB8060850E3AF8AE7CB6B1C2A8E0F8` |

The ZIP has one named top-level directory and **16 allowlisted files**, including
the desktop EXE, optional demo relay EXE/launcher, README, demo instructions,
known issues, notice bundle, build/dependency metadata, verifier and manifest.
No development tools, source profile, room code, password or capture image.
The production executable has only `production` tags, `CGO_ENABLED=0`, no
race runtime, no test recovery method and no test CDP support. WebView2 is an
external prerequisite, not silently installed or redistributed.

The manifest fingerprints each payload file. SHA-256 verifies integrity,
not authenticity if an attacker supplies both artifact and checksum. The
package is explicitly **NotSigned**; installer/signing tools and certificates
are absent. The roadmap permits an executable/installer **if feasible**;
G6 is closed for a tested portable ZIP, not for an unbuilt installer or a signed
public release. Installer/UAC/SmartScreen reputation tests remain NOT RUN.

## UI changes

`frontend/src/main.tsx`, `stream.tsx`, `style.css`, `frontend/index.html` and
`main_windows.go` now use a compact desktop layout: plain CrocStream heading,
two clear Home actions, local preview as a secondary action, neutral surfaces,
small corner radii, system typeface and no slogan/gradient/oversized metric tiles.
The invitation sits next to session setup; three primary measurements remain
below the canvas. Codec statistics stay available in **Chi tiết luồng**, collapsed
by default. Main labels are Vietnamese; the technical meaning and empty states
remain unchanged. The window title is simply CrocStream.

Existing state transitions, capture consent, settings locking, keyboard focus,
status announcements, lossless network/reference behavior and privacy rules
are preserved. This phase did not change crypto, media bytes or dependency pins.
Test selectors in G3–G5 harnesses follow the new Home buttons and labels.

## Demo relay and packaging implementation

- `cmd/demo-relay/{main.go,main_test.go}`: small wrapper around the pinned
  upstream `tcp.RunWithListener`. It accepts only a literal loopback/private
  IPv4 interface and a bounded 12–1024-byte password via stdin. Refuses
  wildcard/public binds. Does not log upstream diagnostics or passwords.
  Ctrl+C/context cancellation and stdin closure stop it; owned readers are
  unblocked and joined. No service registration or firewall changes.
- `g6/Start-DemoRelay.ps1`: secure-input launcher; secrets go over an inherited
  private stdin pipe, never arguments/files. Reclaims its child on exit.
- `g6/Build-Portable.ps1`, `runtime-deps.mjs`: builds with the existing Wails/Go
  toolchain, enforces production flags, collects notices/metadata, fingerprints
  an allowlisted payload and creates ZIP/checksum. Refuses existing destinations.
- `g6/Verify-Package.ps1`: rejects changed files, unknown paths and extra files.
- `g6/Smoke-Package.ps1`: tests the exact extracted EXE's native startup/normal
  close, unsigned status and packaged relay listener/cancellation.
- `g6/Test-ProductionUI.ps1`, `ui-labels.json`: Windows UI Automation over the
  **exact shipped executables**, without CDP/test methods. Starts Share/Watch,
  privately copies the invitation, joins, observes viewer render, stops and
  closes native windows/relay. Requires an empty clipboard before its temporary
  Copy test and clears only its test-owned invitation afterward. Output is
  fixed labels/numbers, no arbitrary exception strings or secrets.
- `g6/{README,DEMO,KNOWN_ISSUES,NOTICE_SCOPE}.md`, `NOTICE.txt`: delivered
  operator instructions and measured limitations. `.gitignore` excludes build
  output, not handoff scripts/evidence.

The helper is intended for manual loopback/private-LAN demos. The README's
two-PC procedure is reproducible operator guidance; the actual G6 live-screen
tests here were two processes on one Windows machine. The earlier G4 two-machine
synthetic proof remains separate. No Internet screen-capture claim.

## Environment and tests actually run

Windows 11 Home x64 build **26300**; Intel Core i7-13650HX, 20 logical
processors, 24,883,064 KiB visible memory. WebView2 **157.0.4341.46**;
Go **1.27.4**, Wails **3.1.0**, Node **24.22.1**, npm **12.2.1**.
See [environment](g6/evidence/environment-20261117.json).
Commands below are from `app/` after `. .\g0\Use-Toolchain.ps1`, unless stated.

| Executed command/check | Observed result |
| --- | --- |
| `go test -count=1 -timeout=180s ./...` | PASS: demo-relay 2.481s, session 12.963s, delta 23.470s; all app packages |
| `go test -race -count=1 -timeout=600s ./...` | PASS: demo-relay 2.961s, session 34.052s, delta 379.233s; no detected race on executed paths |
| `go vet ./...`; `go mod verify` | PASS; all modules verified |
| `go -C g0/poc test -race -count=1 -timeout=180s ./...` | PASS: harness 2.880s, façade 3.170s, transport 17.357s, PAKE 1.696s |
| `go -C g0/poc vet ./...`; `go -C g0/poc mod verify` | PASS; all modules verified |
| `npm --prefix frontend run typecheck`; `run lint`; `run test`; `run build` | PASS, 4 frontend tests; final Wails build also rebuilt frontend |
| `wails3 doctor` | PASS / no issues; installer/signing tools absent, not installed |
| `g6/Build-Portable.ps1 -OutputDirectory release/g6-final-20261117` | PASS: production flags, 16-file manifest, ZIP/checksum above; last app build cached after successful final compilation |
| `Expand-Archive` to a fresh approved temp directory, `Verify-Package.ps1`, `Smoke-Package.ps1` | PASS on exact final ZIP: native startup/normal close; relay loopback bind, wildcard rejection, stdin stop and listener closure |
| `g6/Test-ProductionUI.ps1 -Directory <freshly extracted final package>` | **PRODUCTION_TWO_APP_UI_PASS**, viewer render counter observed 14 FPS during startup, Stop **535ms**, normal close; no debug protocol. The startup sample is not a performance benchmark |
| `node g5/ui-proof.mjs <g6-final-tagged-app> <local-proof> 15` | **26 checks PASS**, zero page exceptions, 11 bridge listeners closed, Stop **148ms**; p50 render 24 FPS on short loopback run |
| `node g4/ui-proof.mjs <g6-tagged-app> <local-proof> 15` | **LOCAL_DESKTOP_MEDIA_PASS**, PLI/recovery/Pong/END, both canvases and graceful exits; 2 bridge listeners closed |
| `node g3/ui-proof.mjs <g6-tagged-app> 5 packaged stop` | PASS, exact packaged Origin, Stop 119ms, cleared canvas/settings unlocked |
| `node g3/ui-proof.mjs "$env:GOBIN\wails3.exe" 5 dev-cli stop` | PASS, exact dev Origin `http://wails.localhost:9245`, Stop 118ms, graceful native close |
| `node g5/summarize.mjs g6/evidence/ui-vi-final-20261117.jsonl` | G5_EVIDENCE_VERIFIED; recomputed numeric window/quantile/cleanup evidence |
| PowerShell AST parse of new scripts; `node --check` on updated proof scripts/runtime-deps | PASS |
| Modified README / injected extra binary on an owned temp copy | Both rejected by package verifier |
| Re-run packaging into final destination | Rejected before writes; archive hash unchanged |
| Final extracted app/relay process inspection | Zero owned live processes |

The tagged app used for fault-injection/DOM tests is **not** distributed. The
exact production two-app UI proof plus extracted smoke close that evidence gap.
G5's historical 600s stress is not relabeled as a new G6 600s test; G6 reran
the affected UI/lifecycle cases, production handoff and full automated suites.

Evidence:
[production two-app](g6/evidence/production-two-app-20261117.jsonl),
[extracted runtime](g6/evidence/portable-runtime-20261117.json),
[final 26-case suite](g6/evidence/ui-vi-final-20261117.jsonl),
[summary](g6/evidence/ui-summary-20261117.json),
[package negative checks](g6/evidence/package-verification-20261117.json),
[G4 regression](g6/evidence/g4-desktop-20261117.jsonl),
[G3 packaged](g6/evidence/g3-packaged-20261117.jsonl),
[G3 dev](g6/evidence/g3-dev-20261117.jsonl).

## Attribution review

Package includes **178 generated notice records** and the reviewed G0 nested/
inline/embedded attribution bundle, Go LICENSE and modification provenance.
Every one of **24 linked non-app Go module roots** has a notice; shipping
React/ReactDOM/scheduler/Wails runtime and LZ4/WebSocket pins were checked.
`g0/Test-LicenseInventory.ps1` PASS for the retained reviewed bundle, including
EFF CC-BY-4.0 and modified PAKE provenance. Conservative notices may mention
development/race sources not shipped; development tool binaries are excluded.
This is engineering notice review, not a legal audit.

The repository has no owner-selected project-wide license for original
CrocStream code. `NOTICE.txt` preserves that fact; this task does not invent
an MIT license or authorize public publication. Package/build metadata and
third-party attribution are ready for owner review before sharing broadly.

## Failures and limits retained

- An initial Go invocation from the repo root omitted `-C app`; it failed to
  find a module, then the corrected package tests passed.
- Windows PowerShell 5.1 cannot deserialize the lockfile's empty root key;
  native `node -e` quoting also stripped JS quotes on the first fix. A small
  checked-in `runtime-deps.mjs` now reads exact pins safely. Partial folders
  were left intact; fresh destinations were used for successful builds.
- An early post-verification command checked stale `$LASTEXITCODE` after a
  PowerShell script and returned early. The corrected smoke actually ran;
  only that observed result is claimed above.
- First Windows UIA production run failed at Copy with the sender in the
  background. Adding focus/foreground before the copy action fixed the harness;
  both the follow-up and the final ZIP run passed. No crypto/app fallback.
- No MSI/MSIX/NSIS, code signature, installer/UAC or actual downloaded-file
  SmartScreen test. WebView2 is already installed on the measured Windows
  host; absent-runtime and fresh Windows 10/second-machine installation are
  NOT RUN. No remote real capture, cross-OS, camera, HDR/multi-monitor or
  screen-reader speech certification is inferred.
- UIA confirms the production render counter advanced; it does not record or
  compare production pixel buffers. Tagged local proofs independently check
  canvases/PLI/cleanup; G4/G2 remain canonical pixel/reference evidence.
- Native handles can exceed the app's cold-start count due to lazy WebView
  initialization. GDI/workers/listeners return and native processes terminate;
  no claim of zero handle growth during a live process.

Next: owner demo/acceptance of this Windows prototype. G7 webcam and other
platforms remain unimplemented and require their own request/environment.

Final handoff checks: **191 local Markdown targets/anchors across 13 documents
PASS**; `git diff --check` PASS; final ZIP SHA-256 and extracted 16-file allowlist
reverified. [Source/package fingerprint record](g6/evidence/final-source-20261117.json)
identifies the changed runtime/relay/build/acceptance sources and packaged files.
`croc/`, G0 sources and dependency pins remain unchanged. No commit or push was performed during the G6 implementation phase.

## Pre-push verification (2026-11-18)

The owner requested a G6 check and GitHub push if PASS. Reviewed the portable
recipe, relay implementation/tests, verifier, operator documents and recorded
production-runtime/UI evidence. Environment: existing Go 1.27.4 windows/amd64,
Node 24.22.1 and npm 12.2.1. Go commands used the existing
`. .\app\g0\Use-Toolchain.ps1` from repository root.

| Command / check | Observed result |
| --- | --- |
| `go -C app test -count=1 -timeout=180s ./...` | PASS all application packages; demo-relay 0.824s, delta 12.106s, session 11.475s |
| `go -C app test -race -count=1 -timeout=600s ./...` | PASS all application packages; demo-relay 2.744s, delta 217.851s, session 28.264s; no detected race |
| `go -C app vet ./...`; `go -C app mod verify` | PASS; all modules verified |
| `npm --prefix app/frontend run typecheck`; `npm --prefix app/frontend run lint`; `npm --prefix app/frontend run test` | PASS, 4/4 frontend tests |
| `node --check` on G6 runtime-deps and changed G3/G4/G5 proof scripts | PASS |
| From `app/`: `. .\g0\Use-Toolchain.ps1`; `$env:WAILS_EXP_USE_WAKE='1'`; `wails3 build` | PASS, all 7 steps ran in 9.5s, including frontend build and production Windows executable |
| Rebuilt production SHA-256 | Exactly matches the shipped app hash `D7CB11BDA36FB4E8BE78816B1FEB4D127750387A4E1B389AD6C694E1F62E42D0` |
| Final ZIP hash/size and adjacent checksum | PASS, original ZIP remains 8,092,277 B with the recorded `B5CAB7E6...65035` SHA-256 above |
| `Expand-Archive` into a fresh `app/release/prepush-20261118-*` directory; `app/g6/Verify-Package.ps1 -Directory <fresh-package>` | `PACKAGE_VERIFIED`; all 16 allowlisted files and recorded payload fingerprints match |
| `go version -m <extracted-crocstream.exe>`; `Get-AuthenticodeSignature` | Production-only tags, CGO_ENABLED=0, no test/race tags; NotSigned as documented |
| Final source fingerprint comparison | PASS, all 14 recorded G6 source hashes; shipped operator documents/launchers/verifier also match current source |
| Verifier on isolated copies with a modified README or an added file | Both rejected with the expected checksum/extra-file errors; original ZIP and clean extracted package untouched |
| Saved production-two-app and package-negative evidence assertions | PASS: exact shipped binary, rendered FPS >0, Stop 535ms, no debug protocol; recorded modification/extra-file/overwrite checks passed |
| `node app/g5/summarize.mjs app/g6/evidence/ui-vi-final-20261117.jsonl` | `G5_EVIDENCE_VERIFIED`, 26 checks, 11 closed listeners, Stop 148ms |
| PowerShell AST parse of all 5 G6 scripts | PASS |
| Local Markdown target/anchor check | PASS, 192 across 13 documents |
| `gofmt -l` on changed Go files; key/embedded-image marker scan | PASS, no findings |
| `git -c safe.directory=D:/CrocStream -c core.safecrlf=false diff --check` and staged `diff --cached --check` | PASS |
| Diff/status against `croc/`, G0, Go module pins and npm lockfile | Empty; upstream/G0/dependencies unchanged |

The recorded exact-production startup/two-app capture proofs and tagged UI
regressions were reviewed, not repeated during this pre-push check. No new
capture, clipboard use, SSH/EC2 operation or runtime/tool installation occurred.
The fresh extraction and two negative-test copies remain under ignored
`app/release/` for inspection. Ordinary tests skip real capture without consent.
The final portable ZIP is retained locally; this source push does not publish a
binary release. G6 is reconfirmed **PASS for the unsigned Windows x64 portable
prototype**, with installer/signing/fresh-Windows and remote-screen performance
limits retained. Next action is owner demo/acceptance; G7 remains deferred.
