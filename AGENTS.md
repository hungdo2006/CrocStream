# AGENTS.md — CrocStream / Codex project instructions

## Mission
Develop a secure, measurable 1-to-1 desktop screen-streaming MVP, with Go + Wails v3 and a deterministic tile-delta codec. Use `schollz/croc` **only after a passed transport feasibility spike**. Do not imply stock croc clients understand this custom media protocol.

## Source of truth

Start with `README.md` for the current app and commands. Use
`docs/ARCHITECTURE.md` for the media, lifecycle and security contracts and
`docs/VERIFICATION.md` for existing evidence and the original decisions.
Paths are relative to the repository root. Document any conflict between code
and specification before changing wire format, security design or scope.

## Workspace and verification

Keep CrocStream changes in `app/`. Preserve the upstream source in `croc/` at
commit `552f40fd2d7593316118370bec0a93478d48e715`. The G0 duplex feasibility
gate passed before integration, and G1–G6 have verification evidence.
G7 (webcam and other platforms) remains deferred.

Work in small, verifiable changes. Report affected areas, commands actually
run, observed results and any remaining limits. If a transport change raises
questions about croc internals, licensing or security, present the evidence
and get owner approval before switching cryptography or relay providers.

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
