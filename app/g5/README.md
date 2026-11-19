# G5 UX, telemetry and resilience verification

Status and executed results: [G5_CLOSEOUT](https://github.com/hungdo2006/CrocStream/blob/8e10002f7997bb1a12a8f2996d883610e1072b4d/app/G5_CLOSEOUT.md).

## Reproduce the Windows UI acceptance

Use the already approved portable toolchain and dependencies. Actual display
capture is **local-only and opt-in**; close sensitive content first. No capture
frame, screenshot, video, trace, socket header or clipboard content is saved.
The harness sends capture only between two local desktop processes through an
ephemeral upstream croc relay bound to `127.0.0.1`.

From `app/`:

```powershell
. .\g0\Use-Toolchain.ps1
wails3 generate bindings
npm --prefix frontend run typecheck
npm --prefix frontend run lint
npm --prefix frontend run test
npm --prefix frontend run build
go build -tags production,g3test,g4test -buildvcs=false -o "$env:GOBIN\crocstream-g5-ui.exe" .
go build -buildvcs=false -o "$env:GOBIN\g5-relay.exe" ./g4/proof
node g5/ui-proof.mjs "$env:GOBIN\crocstream-g5-ui.exe" "$env:GOBIN\g5-relay.exe" 600
```

For saved numeric JSONL, independently recompute summary/checks:

```powershell
node g5/summarize.mjs g5/evidence/ui-600s-20261117.jsonl
```

The reader accepts UTF-8 and PowerShell 5.1's `Tee-Object` UTF-16LE output.

Ordinary production has neither CDP nor `InvalidateViewerReference`; both
require the explicitly tagged test build. Generated bindings do not need the
test-only API; the harness calls it by name in its isolated test process.
The test automatically checks:

- No capture before Start; waiting-preview vs authenticated Live.
- Keyboard focus, accessible labels/status tree, 960×640 and 1150×760,
  16:9 canvas and no horizontal overflow.
- Unreachable relay and invalid invitation errors without exposing raw errors.
- Secret-code Copy feedback, settings locks and first KEY rendered.
- Local JPEG failure while network decode continues; manual display reconnect.
- Decoder invalidation → bounded PLI → KEY recovery.
- Dashboard sampling, exact savings calculation and bounded queue depths.
- Stop target <=3s, END/disconnect, clear canvas/code and GDI return.
- Five Start→Back cycles, fresh rejoin and native-window close while Live.
- Native process exit and every observed bridge listener closed.

The script reports only fixed check names and numeric measurements. Its
failure output reports the stage; it deliberately suppresses arbitrary
exceptions which could contain code/token values. `G5_LOCAL_UI_PASS` is
required after the stress result; a `stress-result` line alone is not success.
Temporary test WebView profiles stay in the approved external temp root.

## Metrics interpretation

Snapshot fields are numeric only. Rates/means/baseline/savings use the G2 exact
rolling one-second window; status UI polls every ~500ms. Lifetime counts,
uptime and dropped captures include waiting-for-viewer. JPEG mean is
**lifetime elapsed encoding duration**, excluding publish/network, not CPU.
Encode mean includes baseline work; baseline mean is its measured subset.

`cpuMs` is cumulative kernel+user CPU from Windows `GetProcessTimes`.
Interval CPU percent is `100*delta(cpuMs)/delta(wallMs)` (one core=100%).
Divide by reported logical CPUs only when labeling total-machine capacity.
Working set/private bytes/handles refer to the **native Go app process**, not
the WebView2 tree. Resource observations are sampled; they are not guaranteed
instantaneous peaks. Allocations/frame from cumulative counters include
capture/JPEG/UI binding work and should never be called codec-only allocations.
RTT quantiles are quantiles of sampled **latest RTT**, not a unique-Pong
distribution. Optional viewer LZ4-decode→render remains N/A because no
correlated monotonic timestamp crosses the JPEG bridge. True E2E stays hidden.

## Remaining manual/platform distinctions

Accessibility-tree and keyboard automation do not assert spoken Narrator/NVDA
output. Windows GDI does not provide a screen-permission prompt in this setup;
missing-source behavior is covered by injected source-failure tests. Multi-
monitor removal, protected/HDR content, signed installer and other OSes need
their actual environments. Packaging is G6, webcam/cross-platform G7.

No EC2 or real remote-screen performance claim is implied by this local test.
