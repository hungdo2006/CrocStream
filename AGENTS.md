# AGENTS.md — CrocStream / Codex project instructions

## Mission
Develop a secure, measurable 1-to-1 desktop screen-streaming MVP, with Go + Wails v3 and a deterministic tile-delta codec. Use `schollz/croc` **only after a passed transport feasibility spike**. Do not imply stock croc clients understand this custom media protocol.

## Source of truth
Read, in order: `docs/PRD.md`, `docs/ARCHITECTURE.md` (including decisions, blockers and media protocol), `docs/TEST_PLAN.md`, `docs/ROADMAP.md`. Paths are relative to this documentation folder. If implementation conflicts with the spec, document the conflict and propose a patch; don't silently alter wire format, security design or scope.

## Workspace boundary
The owner chose a clean upstream checkout in `../croc/` at commit `552f40fd2d7593316118370bec0a93478d48e715` and separate planning documents on 2026-11-15. The owner subsequently requested publication of the upstream code as well: this repository now includes its pristine source snapshot in `croc/`, with provenance in `UPSTREAM.md`. Source paths in the design documents are relative to the repository root. Previous custom implementation and experiment artifacts are no longer retained. Keep both the reference snapshot and separate checkout pristine. Agree on a separate implementation repository/module before creating new code; publishing the source and documents does not authorize implementation or tool installation.

## Execution gate
- **First task = G0 only** as described in `docs/ROADMAP.md#codex-handoff`. Inspect the repository and pinned upstream croc code; produce a small duplex-stream feasibility PoC and a written decision. Do NOT scaffold full UI or claim transport is done without two-process evidence.
- Work in small, verifiable changes. After each phase: list files changed, precise commands/tests actually run, observed results, unresolved issues, and next gate. Never claim tests ran if not executed.
- If croc transport internals, licensing, or security are uncertain: stop at the gate, present evidence and viable options, ask for owner decision before switching cryptography or relay provider.

## Coding constraints
- Go: bounded resources, `context.Context` cancellation, explicit ownership of pooled buffers, no use-after-Put, no unbounded queue, one logical writer per connection; use `io.ReadFull` for fixed-size framing.
- Decode **only after** validating type, payload length, dimensions and maximum decompressed size; reject malformed lengths, duplicate/out-of-range tile indices, unsupported versions, truncated frames, overflow and compression bombs.
- Never use lossy network Keyframe encoding with a byte-exact delta reference unless both peers derive precisely the same reference. MVP uses **lossless LZ4-frame RGBA** on the network as specified.
- Keep transport (`io.ReadWriteCloser`) separate from codec/capture/desktop UI. Do not edit vendored/upstream croc crypto or weaken PAKE without explicit approval.
- Local WebSocket must bind to `127.0.0.1`, require an unpredictable per-launch token, validate Origin where available, and close with the session. Never stream images through Base64 UI events.
- Do not log room codes, key material, full frames or tokens. Avoid storing frames on disk.
- Frontend: React + TypeScript, explicit connection states, canvas cleanup, close `ImageBitmap`, show measured metrics only; no fake latency or saving percentages.

## Definition of done per change
- Add/update tests for changed behavior (including malformed inputs for network code).
- Run applicable checks where tools permit: `go test ./...`, `go test -race ./...` (supported targets), `go vet ./...`, frontend lint/test/build and a platform build. Record environment and failures honestly.
- No commits, remote pushes, deletions of user work, new external services, credentials, or production dependency changes without approval.
- No speculative compatibility with croc CLI, public relay or other operating systems without successful tests.
