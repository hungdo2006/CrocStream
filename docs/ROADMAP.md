# Milestones, dependencies & Go/No-Go gates

> **Do not skip G0**. Each phase ends with code/tests/artifacts, exact observed evidence, and owner decision if a blocker remains. No blanket task “build everything” until transport has passed.

Current workspace (2026-11-15): clean upstream croc plus separate planning
documents only. Previous custom code and experiment records are not retained.
The table is a future development plan, not a completion report. Agree on a
separate implementation workspace before starting.

## Codex handoff
Use this starting task **only when the owner requests continued development**. Scope: **G0 architecture and transport feasibility**, not the whole application.

1. Read `../AGENTS.md`, then [PRD](PRD.md), [ARCHITECTURE](ARCHITECTURE.md) (decisions, blockers, protocol), [TEST_PLAN](TEST_PLAN.md) and this roadmap. Inspect Git status, source, dependencies and available tools; old experimental toolchains are no longer assumed installed. Agree on a separate implementation repository/module and keep the upstream reference pristine.
2. Verify `github.com/schollz/croc/v11` at **552f40fd2d7593316118370bec0a93478d48e715**, which reports 11.6.5 but is not the tag itself. Check proposed Wails v3 APIs/toolchain; identify actual codephrase, PAKE, mutual confirmation, E2E encryption, relay/P2P and file-lifecycle code paths. Do not assume v9/v10 APIs.
3. Build the smallest GUI-independent, two-process PoC in the approved implementation workspace, using actual supported authenticated mechanisms. Follow [transport proof](TEST_PLAN.md#d-transport-g0-proof-two-processes-no-gui): repeated deterministic duplex data, hashes/order, reverse control, clean cancellation and >=10-minute session. Plain TCP is not a substitute for croc security.
4. Report exactly what ran: localhost/LAN, public/self-hosted remote relay, security assumptions, limits, duration, commands and redacted logs. Missing public-relay permission, network access or suitable OS means NOT RUN/BLOCKED, not PASS.
5. PASS: propose a pure adapter interface, module boundaries, exact dependency pins, tests and a focused G1 task. FAIL/BLOCKED: describe feasible options, maintenance costs and scope changes; ask the owner before changing transport/relay/crypto. Do not bypass PAKE or substitute UI progress for network evidence.
6. Write the report/evidence in the approved implementation workspace using the template below. No automatic commits/pushes or deletion/overwrite of user work.

**G0 Definition of Done:** a concrete, evidence-backed answer about continuous authenticated duplex-byte integration, with reproducible PoC where the environment permits, or an honest BLOCKED result identifying needed owner decisions. G0 never means “CrocStream complete.”

## Gates
| Gate | Milestone | Deliverables | Pass criteria | If fail |
| --- | --- | --- | --- | --- |
| **G0** | Source inventory + croc transport feasibility | Existing repo audit, dependency/license inventory, pinned croc v11 adapter PoC, README with reproducible two-process commands, `G0_FEASIBILITY.md` in implementation workspace | Authenticated encrypted >=10 MiB each direction, reverse control, >=10 min session, verified chosen remote-relay path; otherwise BLOCKED for remote MVP unless owner explicitly narrows scope to LAN | Stop; propose options, ask owner; do not implement broad app on guessed API |
| **G0 adapter follow-up** (if needed, requires approval) | Secure-duplex API suitable for application use | Adapter in the approved implementation workspace, CLI harness, isolation/lifecycle/unit/integration/race tests and review report | Public Host/Accept/Join API without private test-only coupling; reviewed protocol isolation; real two-process evidence; regression compared with upstream | Stop for owner review. Local API acceptance does not waive remote/two-machine requirements |
| **G1** | Binary framing + config | `protocol` package, safe reader/writer, config validation, fuzz seeds | Exact fixtures + truncation/oversize/unknown-type tests pass | Fix protocol and test gap |
| **G2** | Delta codec + telemetry | Pure Go encoder/decoder, canonical reference, tests/bench | Multi-frame exact reference reconstruction, no drift, malformed delta fail-closed, fair baseline metrics | Fix codec before any UI |
| **G3** | Screen capture + local bridge | Windows screen capture adapter, secured local WS, React canvas local preview | 1280×720/24fps on test machine measured, Stop releases capture + WS, safe origin/token | Profile or reduce resolution/FPS; report actual |
| **G4** | Croc app integration | Host/Join service, secure duplex channels, config/PLI/Ping/EOS, lifecycle | Two peer apps complete session, first KEY renders, reverse PLI/resync, disconnect handled | Stop and isolate fault; do not fake “connected” |
| **G5** | Full UI + telemetry + resilient behavior | 3 screens and states, real metrics, error handling, privacy | UX cases + real dashboard data + 10 min stress completed | Fix before packaging |
| **G6** | Windows build & demo handoff | Windows executable/installer if feasible, reproducible demo checklist, known issues | Actual packaged build runs on target Windows machine, no orphan handles, docs updated | Report not built; not “done” |
| **G7 later** | Webcam / cross-platform | Camera adapter, OS build matrix | Separate camera and OS tests pass | Keep deferred |

## Deliverable report format for Codex after each gate
```
Gate: Gx — PASS / FAIL / BLOCKED
Source revision: <commit/tag or "no git repo">
Environment: OS, Go version, Wails version, Node version
Changes: paths + purpose
Commands run: exact command => PASS/FAIL/NOT RUN + key output
Acceptance evidence: reproducible instructions and test names
Open risks / assumptions: ...
Next action: one focused next gate or explicit owner question
```

## Dependencies and evidence
- Resolve affected [B-01–B-05 blockers](ARCHITECTURE.md#decisions-and-blockers) and [C-01–C-05 clarifications](ARCHITECTURE.md#pending-clarifications) before the corresponding implementation.
- [Architectural risks](ARCHITECTURE.md#change-control-and-risks) guide feasibility and profiling.
- [Verification coverage](TEST_PLAN.md#coverage-by-requirement-and-gate) maps product requirements to tests and gates. Gate summaries do not replace those procedures.
- Future reports, commands, logs and benchmark artifacts belong in the approved implementation workspace; this planning folder is not a claim of completed gates.
