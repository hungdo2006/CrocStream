# G2 closeout — deterministic delta codec and measured telemetry

Date: 2026-11-16. **Gate status: G2 PASS — ready for the focused G3 task.**
Source HEAD at start: `cc4856b21cc9ca51f1040e2ad343bba7921eb4b3`.
Starting working tree was clean; these G2 changes are uncommitted.

## Authorization and dependencies

The owner requested G2 after [G1 PASS](G1_CLOSEOUT.md), then explicitly
approved adding/pinning `github.com/pierrec/lz4/v4 v4.1.33` in `app/go.mod`.
The module has no external module requirements, uses Go 1.22+, and is
BSD-3-Clause. Exact source/checksums and complete notice are retained in
[THIRD_PARTY_NOTICES](THIRD_PARTY_NOTICES.md); `go.sum` pins both checksums.
Go module verification passed. No new tool installation was needed.

The G1 wire contract is preserved: header/config sizes, type values, version,
resolution pairs and key intervals are unchanged. LZ4 settings select a standard
frame representation already permitted by that contract.

## Delivered

- `internal/frame`: checked top-down RGBA/BGRA stride normalization, owned
  packed output, alpha=255. Input is straight color; no premultiplied conversion.
- `internal/delta`: separate encoder/decoder canonical images, exact RGB MAE
  comparison, strict >65% fallback, periodic/PLI KEY, independent lossless LZ4
  frames, bounded tile parser and transactional reference updates.
- Encoder commits only after the synchronous packet writer succeeds. Partial
  write errors terminate the encoder; canceled/invalid input consumes no ID.
- Decoder validates framing/config/order, one standard LZ4 envelope, bounded
  decompression, full tile records and opaque alpha before reference commit.
  Failed decode retains the last good image, invalidates delta reference and
  rejects consumed/replayed IDs until a new forward KEY.
- `internal/telemetry`: mutex-protected exact `(now-1s, now]` rolling window,
  fixed 4096-event ring, distinct captured/queued/dropped/emitted/received/
  decoded/rendered counters. Received bytes are recorded before decode,
  including failed compressed bodies; decode/render counts require success.
- Full-frame baseline uses each same normalized emitted capture, includes
  header bytes, and uses exactly the same LZ4 settings. On KEY the measured
  full-frame result is also the transmitted payload. Negative savings and
  unknown/empty states are represented explicitly.

API/ownership and reproducible commands: [application README](README.md).

## Acceptance evidence

| Requirement | Test evidence |
| --- | --- |
| Packed RGBA, padding, channel order and alpha | `frame.TestNormalizeStrideOrderAndOwnership`, `TestNormalizeRejectsBounds` |
| Canonical convergence over time | `TestCanonical201Frames`: 201 frames × 2 resolutions × tau 0/4, every frame byte-equal across peers and independent slow oracle; tau0 also equals raw capture |
| Subthreshold changes accumulate correctly | `TestSubthresholdAccumulatesAgainstTransmittedReference`: 1-step per-pixel drift accumulates against transmitted tiles, emits at MAE 5 rather than resetting comparison each capture |
| Deterministic threshold/fallback/schedule | `TestDirtyRatioAndSchedule`: exactly 65% DELTA, >65% KEY; intervals 1/30/120 and PLI do not shift scheduled IDs |
| Random reference/current comparisons | `TestRandomPairsAgainstOracle`, fixed seed 2026 |
| Edge tile packing and max record | `TestRecordFixturesAndEdges`, both sizes, last rightmost/bottom tile, all tiles, empty DELTA |
| Malformed record atomicity | `TestMalformedDeltaDoesNotMutatePixels`: duplicate/unsorted/out-of-range indices, bad counts, truncated index/tile, extra bytes, early/late bad alpha |
| LZ4 framing, bounds, corruption | `TestLZ4SingleFrameAndBombBounds`: every truncation point, trailing/concatenated/empty extra frame, legacy/skippable, checksum/descriptor errors, oversized block/content size, no-size compression bomb |
| Standard frame variants | `TestDependentBlocksAndFalseContentSize`, four standard block sizes/checksum variants, interoperability with the library's standard reader, deterministic repeated encoding |
| Ownership and commit boundaries | `TestWriterCommitFailureAndCancellation`: writer failure, no retry, successful-write cancellation race, copied input/snapshots, uint32 exhaustion |
| Recovery/replay and cancellation | `TestDecoderRecoveryAndAtomicity`, `TestInitialDecodeFailureAndInputValidation`, `TestCancellationDuringCodecWork` |
| G1 + G2 + telemetry integration | `TestProtocolCodecTelemetryIntegration`: real protocol Writer/Reader, CONFIG, five video frames, codec acceptance, exact wire/baseline bytes and counters |
| Honest rolling metrics | Exact one-second cutoff, negative/zero/unknown/missing baselines, received-but-not-decoded frames, rejected controls, explicit capacity/clock errors, concurrent recording/snapshot |
| Fuzz | `FuzzDecodeDelta`, `FuzzCompressedFrame`, `FuzzDecoderAtomicity` |

Clarification of the old test-plan wording: both supported widths (640 and
1280) are divisible by 32, so there is **no partial-width right tile** in v1.
Tests cover full-width rightmost tiles and partial-height bottom tiles (8/16
pixels), including the bottom-right corner. No mixed/unsupported format was
introduced merely to create a partial-width fixture.

## Resource and measurement boundaries

- Raw KEY limit: 921,600 / 3,686,400 B.
- Raw DELTA limit: 922,082 / 3,688,242 B; at most 240 / 920 tiles.
- Compressed payload <=8 MiB; decode output allocation is exactly cap+1.
  Decompression stops on overshoot. LZ4 library block buffers are bounded by
  the supported <=4-MiB block size; dependent frames may also use a bounded
  rolling dictionary. Codec work uses synchronous concurrency=1.
- Encoder/decoder own reference/staging copies; output snapshots are copies.
  There are no application buffer pools, worker goroutines or capture queues.
  Per-frame allocation is nonzero and reported below.
- Codec timings are elapsed monotonic execution time, not process CPU time.
  Encode timing includes baseline compression and excludes writer time;
  benchmark timing includes encode + baseline + decode.
- Meter overload returns `ErrCapacity` and marks the window incomplete for 1s,
  disabling exact savings. UI/session must respect known/completeness flags.
- Actual PLI rate limiting, RTT, decode-to-render timing, shutdown/reconnect,
  control priority and secure two-machine app integration remain G4/G5.
  G4 must record Received for all complete video packets, including any payload
  discarded by the framing layer, before claiming complete received-video rate.

## Commands actually run

Environment: Windows 11 Home build 26300, amd64; Intel i7-13650HX (14 cores,
20 logical processors); OS visible memory 24,883,064 KiB. Existing portable
Go 1.27.4 and compiler; module minimum Go 1.27.3. Frontend/Wails checks are
not applicable to the pure-Go codec change.

All Go commands from the repository root after:

```powershell
. .\app\g0\Use-Toolchain.ps1
```

| Command | Result |
| --- | --- |
| `go -C app mod download -json github.com/pierrec/lz4/v4@v4.1.33` | PASS, source/checksums pinned above |
| `gofmt -w app/internal/frame app/internal/delta app/internal/telemetry` | Completed |
| First `go -C app test -count=1 -timeout=180s ./...` | FAIL only in cancellation test: artificial 20-check budget exceeded the encoder's actual checkpoints, so the context never canceled. Corrected test budget to 16; no implementation failure suppressed |
| Next same full unit-test command | PASS: delta 23.003s, frame 0.814s, protocol 1.240s, telemetry 0.812s |
| `go -C app test -race -count=1 -timeout=600s ./...` | PASS: delta 403.032s, frame 1.547s, protocol 1.770s, telemetry 1.511s; overlapped fuzz, not a performance run |
| `go -C app mod verify` | PASS: all modules verified |
| `go -C app list -m all` | Exactly application + lz4/v4 v4.1.33 |
| `go -C app test -count=1 -timeout=30s ./internal/telemetry ./internal/delta -run 'TestExactRollingWindow\|TestNegativeZero\|TestCapacity\|TestRejects\|TestConcurrent\|TestProtocolCodecTelemetryIntegration'` | PASS after separating Received/Decoded: telemetry 1.257s, delta integration 1.779s |
| Final `go -C app test -count=1 -timeout=180s ./...` | PASS: delta 23.257s, frame 1.106s, protocol 1.742s, telemetry 1.158s |
| Final `go -C app test -race -count=1 -timeout=60s ./internal/telemetry` | PASS, 2.031s after measurement refinement |
| Final `go -C app test -race -count=1 -timeout=120s ./internal/delta -run 'TestProtocolCodecTelemetryIntegration\|TestWriterCommitFailureAndCancellation\|TestDirtyRatioAndSchedule'` | PASS, 5.303s after measurement refinement |
| `go -C app vet ./...` | PASS before and after final measurement refinement |
| `$env:CGO_ENABLED='0'`; `go -C app build ./...` | PASS Windows/amd64 library build before and after final refinement |
| Notice comparison against downloaded module LICENSE | PASS: complete notice text matches |
| `gofmt -l app/internal` | PASS: no unformatted files |
| `git -c safe.directory=D:/CrocStream -c core.safecrlf=false diff --check` plus new-file whitespace/conflict-marker scan | PASS |
| `git -c safe.directory=D:/CrocStream diff --exit-code -- croc/` and upstream-only status including untracked files | PASS: upstream pristine |
| Inline PowerShell Markdown target/heading-anchor check | PASS: 130 local links/anchors across 9 changed documents |

The full race suite preceded the final telemetry separation and addition of
`DirtyTilesKnown`/expanded encode timing. Affected telemetry/integration/
encoder paths were race-tested again as listed; compression, tile validation
and canonical algorithms did not change after the full race run.

Fuzz command:

```powershell
go -C app test ./internal/delta -run '^$' -fuzz '^FuzzDecodeDelta$' -fuzztime=30s -parallel=2
```

Substitute target names:

| Target | Result |
| --- | --- |
| Initial `FuzzDecodeDelta` | PASS, 173 executions / 30s; long intervals without newly reported executions |
| `FuzzCompressedFrame` | PASS, 1,166,146 executions / 30s |
| Initial `FuzzDecoderAtomicity` | PASS, 4,375 executions / 30s; long intervals without newly reported executions |
| `FuzzDecodeDelta`, adding `-fuzzminimizetime=1s` | PASS, 1,008,330 executions / 30s |
| `FuzzDecoderAtomicity`, adding `-fuzzminimizetime=1s` | PASS, 32,634 executions / 30s |

The minimized-time reruns improved useful exploration within the fixed budget.
Fuzz seeds contain only synthetic data; no captured frames were stored.

## Benchmarks

After the race and fuzz jobs completed, ran in isolation:

```powershell
go -C app test ./internal/delta -run '^$' -bench '^BenchmarkCodec$' -benchmem -benchtime=2s -count=1
```

PASS, package duration 20.801s, Go benchmark suffix `-20` (GOMAXPROCS).
Input is generated in memory before timing; black/static, alternating
pseudo-text/moving-box (ticks 10/11), and alternating RGB noise (seeds 10/11).
Default tau=4/interval=30. Operation includes encode, full baseline and decode;
excludes capture, normalization, snapshots, transport and presentation.

| Resolution / scenario | ns/op | B/op | allocs/op | Video B/frame | Baseline B/frame | Savings |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| 640×360 static | 1,130,888 | 3,730,760 | 32 | 187.0 | 4,120 | 95.46% |
| 640×360 moving | 6,017,660 | 3,725,365 | 35 | 543.5 | 6,121 | 91.12% |
| 640×360 noise | 8,155,396 | 4,188,901 | 22 | 921,761 | 921,761 | 0% |
| 1280×720 static | 20,023,431 | 14,586,839 | 34 | 699.6 | 16,265 | 95.70% |
| 1280×720 moving | 19,457,864 | 14,606,953 | 37 | 1,059 | 18,325 | 94.22% |
| 1280×720 noise | 26,950,201 | 16,844,388 | 26 | 3,686,897 | 3,686,897 | 0% |

These are single-run synthetic microbenchmarks, not universal savings, capture/
UI/relay FPS, process CPU, p50/p95 presentation latency or an RSS plateau.
They include periodic keys with different calibrated iteration counts; short
runs should not be compared as equal-length application sessions. Allocation
traffic is material (about 14.6–16.8 MB/op at 720p); profile during G3/G4 before
adding pools or claiming the full application performance target.
The two-minute application benchmark and ten-minute leak acceptance remain
later integration tests; neither was run in G2.

## Files changed and next gate

- `app/go.mod`, `app/go.sum`, `app/THIRD_PARTY_NOTICES.md`: approved dependency.
- `app/internal/frame/{normalize.go,normalize_test.go}`.
- `app/internal/delta/{codec,compression,record}.go`,
  `{codec,malformed,fuzz}_test.go`.
- `app/internal/telemetry/{meter.go,meter_test.go}`.
- `app/README.md`, `app/G2_CLOSEOUT.md`, root `README.md` and
  `docs/{PRD,ARCHITECTURE,TEST_PLAN,ROADMAP,GETTING_STARTED}.md`: implementation,
  API, gate evidence and handoff.

Next gate: **G3 — Windows capture and secured local preview bridge**.
Resolve C-03 (listener lifetime), C-04 (observed dev/packaged Origin policy)
and C-05 (basic accessibility scope) before their affected implementation.

No commits or pushes were performed during the implementation phase.

## Pre-push verification (2026-11-16)

The owner requested verification of G2 and a GitHub push if it passed.
Reviewed the implementation and test coverage against the G2 gate, then ran
these checks on the final working tree using the existing portable toolchain
(`. .\app\g0\Use-Toolchain.ps1`), Go 1.27.4 windows/amd64:

| Command / check | Observed result |
| --- | --- |
| `go -C app test -count=1 -timeout=180s ./...` | PASS: delta 8.394s, frame 1.113s, protocol 1.772s, telemetry 1.111s |
| `go -C app test -race -count=1 -timeout=600s ./...` | PASS on final state: delta 103.477s, frame 1.554s, protocol 2.305s, telemetry 1.514s; no race reported |
| `go -C app vet ./...` | PASS |
| `go -C app mod verify` | PASS: all modules verified |
| `$env:CGO_ENABLED='0'`; `go -C app build ./...` | PASS: Windows/amd64 library build |
| `gofmt -l app/internal` | PASS: empty output |
| `git -c safe.directory=D:/CrocStream -c core.safecrlf=false diff --check` | PASS |
| Inline PowerShell local Markdown target/anchor check | PASS: 130 targets/anchors across 9 documents |
| New source whitespace/conflict-marker scan | PASS |
| `git -c safe.directory=D:/CrocStream diff --exit-code -- croc/` and upstream-only status including untracked files | PASS: upstream pristine |

The earlier bounded fuzz runs and synthetic benchmark results above were
reviewed; fuzz seeds ran with the unit suite, but fuzz campaigns and benchmarks
were not rerun during this pre-push check. Frontend/desktop checks do not apply
to this library-only gate. G2 is reconfirmed **PASS**; G3 remains the next gate,
with real capture/UI and application performance acceptance still outstanding.
