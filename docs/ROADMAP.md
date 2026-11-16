# Milestones, dependencies & Go/No-Go gates

> **Cách đọc:** tài liệu này cho biết thứ tự triển khai và điều kiện chuyển bước,
> áp dụng cho cả thành viên nhóm và coding agent. Người mới nên đọc
> [GETTING_STARTED](GETTING_STARTED.md) trước. **G0 PASS** cho khả thi prototype
> đã kiểm chứng; xem [closeout](../app/G0_CLOSEOUT.md). **G1 PASS** cho
> framing/config; [bằng chứng G1](../app/G1_CLOSEOUT.md).
> **G2 PASS** codec/telemetry; [bằng chứng G2](../app/G2_CLOSEOUT.md).

> **Do not skip G0**. Each phase ends with code/tests/artifacts, exact observed evidence, and owner decision if a blocker remains. No blanket task “build everything” until transport has passed.

Current workspace (2026-11-16): one CrocStream repository containing pristine
upstream source in `croc/`, planning documents in `docs/`, G0 artifacts,
isolated `app/g0/buildprobe` / `app/g0/poc` modules, and the pure-Go application
module with `app/internal/{protocol,frame,delta,telemetry}`. The transport PoC,
framing/config and codec/telemetry are implemented; the desktop app is not.
Previous custom code and experiment records are not retained. Source and
workspace paths are relative to the repository root.

## Current progress

Kết quả hiện hành: **G0 PASS, G1 PASS, G2 PASS**, ngày 2026-11-16.
Trạng thái nghiệm thu G1 nằm trong [G1 closeout](../app/G1_CLOSEOUT.md).
Xem [quyết định và giới hạn](../app/G0_CLOSEOUT.md),
[bằng chứng local](../app/G0_LOCAL_PROOF.md),
[toolchain](../app/G0_TOOLCHAIN.md),
[thiết kế dự thảo](../app/G0_ADAPTER_DESIGN.md) và
[khảo sát ban đầu](../app/G0_FEASIBILITY.md).
Tiếp nối cùng ngày: chủ dự án đã tạo EC2 Windows và cho phép SSH.
[Bằng chứng remote](../app/G0_REMOTE_PROOF.md) ghi nhận smoke hai máy qua relay
Internet và phiên dài 605s đã đạt kiểm tra dữ liệu/control. RTT trung bình
10–12s, tối đa 42.8s được giữ như giới hạn của điều kiện mạng cũ.
[Follow-up mạng mới](../app/G0_LATENCY_FOLLOWUP.md): cùng binary cũ đạt RTT
48–49ms trong 30s; hai mức tải và phiên 600.1s ở mức tải gốc đều đạt.
RTT phiên dài mới trung bình 48–51ms, max 191ms, hash/control/cleanup đúng.
Giữ nguyên kết quả độ trễ cao của mạng trước; chưa xác định root cause.

| Task | Trạng thái quan sát | Việc còn lại |
| --- | --- | --- |
| G0.1 | PASS: toolchain, combined native probe, 21-module/notice review; lỗi baseline Windows được giữ rõ | Rà lại khi chọn dependency production/đóng gói G6 |
| G0.2 | PASS G0: hợp đồng adapter, self-review, provenance và negative tests | Không coi là audit production độc lập |
| G0.3 | PASS: CLI PoC, Host/Accept/Join, secure stream; tests/race/vet/checksums đạt | Giữ tách transport khỏi codec/capture/UI khi tích hợp |
| G0.4 | LOCAL_PASS; remote 605s trên mạng cũ và 600.1s trên mạng mới; hash/control/cleanup đúng; RTT mạng mới 48–51ms mean | Ghi rõ giới hạn theo mạng, không suy ra FPS/latency chung |
| G0.5 | PASS: quyết định GO cho G1, bằng chứng native/license/security và giới hạn ở G0_CLOSEOUT | C-01/C-02 đã được chủ dự án chốt khi giao G1 |
| G1 | PASS: framing/config, reader/writer, tests/race/vet/Windows build và 4 fuzz targets; xem G1_CLOSEOUT | Bàn giao G2 codec + telemetry |
| G2 | PASS: codec/telemetry, canonical/oracle, malformed, tests/race/vet/build/fuzz/bench; [G2_CLOSEOUT](../app/G2_CLOSEOUT.md) | Bàn giao G3; theo dõi allocation khi tích hợp |
| G3–G7 | Chưa bắt đầu | Phụ thuộc gate trước theo bảng bên dưới |

Inventory một mình không đóng G0; lần đóng gate này dựa trên toàn bộ bằng chứng
transport local/remote, native compile/runtime, tests và notice review.
Ngày 2026-11-16, chủ dự án xác nhận **chưa có máy thứ hai/relay**. Phần remote
được ghi BLOCKED tại mốc đó; sau đó đã cung cấp EC2 như cập nhật phía trên.
Các blocker thiếu môi trường tại mốc đó đã được đóng bằng kết quả thực tế,
không phải thu hẹp tiêu chí.

### G1 handoff and implementation

Ngày 2026-11-16, sau khi xác nhận G0, chủ dự án yêu cầu triển khai G1 và chốt
C-01/C-02: key interval 1–120, default 30; chỉ 640×360 và 1280×720.
`app/internal/protocol` hiện có header 18 byte, CONFIG 11 byte, reader/writer
bounded, fixtures, malformed/truncation/replay tests và fuzz.
Chi tiết lệnh/kết quả, API codec acceptance và giới hạn:
[G1_CLOSEOUT](../app/G1_CLOSEOUT.md), [app README](../app/README.md).

### G2 implementation and verification

Chủ dự án đã yêu cầu G2 và phê duyệt pin `pierrec/lz4/v4 v4.1.33`.
Đã triển khai canonical RGBA, LZ4-frame KEY/DELTA, tile records, bounded decode,
tau=0/tau=4 và baseline cùng mẫu phát; telemetry cửa sổ 1s có giới hạn.
Lệnh/kết quả thực tế nằm trong [G2_CLOSEOUT](../app/G2_CLOSEOUT.md).

### Next focused gate: G3 — not started

Capture màn hình Windows và preview qua WebSocket loopback bảo vệ token/Origin.
Chốt C-03/C-04/C-05 ở checkpoint tương ứng; kiểm chứng capture thật và tốc độ
1280×720/24fps trên máy chỉ định. G2 chưa phải bằng chứng capture/UI.

## Next work: G0

Checklist lịch sử/tái lập của G0 đã hoàn tất; trạng thái hiện tại ở trên.

### Mục tiêu và cách nhận việc

Trả lời bằng thực nghiệm: **có thể tái sử dụng cơ chế croc đã ghim để tạo một
luồng byte hai chiều liên tục, có xác thực và mã hóa, phù hợp cho CrocStream hay
không?** Đây là bước giảm rủi ro trước khi xây dựng các module ứng dụng.

Danh sách dưới đây mô tả đầu ra của từng task; trạng thái thực tế nằm ở bảng
Current progress phía trên. Chủ dự án đã yêu cầu bắt đầu G0; việc triển khai
tiếp đang tập trung vào các điều kiện nghiệm thu còn thiếu trong báo cáo.

| Task | Công việc | Đầu ra kiểm chứng được | Phụ thuộc |
| --- | --- | --- | --- |
| G0.1 | Khảo sát workspace, revision upstream, công cụ có sẵn, dependency và giấy phép | Inventory có lệnh/kết quả thực tế; ghi rõ công cụ còn thiếu, chưa coi phiên bản đề xuất là đã cài | Bắt đầu tại đây |
| G0.2 | Lần theo codephrase, PAKE, mutual confirmation, mã hóa, relay và vòng đời file | Sơ đồ luồng kết nối, đường dẫn/hàm/API thật, giả định bảo mật và chi phí tích hợp | G0.1 |
| G0.3 | Xây PoC CLI nhỏ nhất trong `app/` bằng cơ chế xác thực được hỗ trợ | Hai process chạy độc lập, có lệnh tái lập và hủy/đóng phiên; nếu API không phù hợp thì báo blocker | G0.2; dừng xin quyết định nếu cần đổi transport/crypto |
| G0.4 | Kiểm chứng truyền hai chiều và điều khiển ngược | Ít nhất 10 MiB mỗi chiều, hash/thứ tự đúng, phiên >=10 phút, kết quả local/LAN và đường relay từ xa mục tiêu | G0.3 và môi trường mạng thử nghiệm |
| G0.5 | Tổng hợp quyết định Go/No-Go | `app/G0_FEASIBILITY.md`, lệnh tái lập, log đã che bí mật, giới hạn, PASS/FAIL/BLOCKED và bước kế tiếp | G0.1–G0.4 |

Chuẩn bị máy thử và điều kiện sử dụng relay có thể tiến hành trong lúc khảo sát
mã nguồn. Việc cài công cụ hoặc chọn dịch vụ mới cần tuân thủ quy tắc workspace
và phê duyệt hiện hành; bảng task không tự cấp phép các thay đổi đó.

### Điều kiện kết thúc

- Tiêu chí chi tiết nằm ở [Test Plan D](TEST_PLAN.md#d-transport-g0-proof-two-processes-no-gui);
  kiểm tra build/toolchain và bảo mật còn phải đối chiếu [B-01–B-05](ARCHITECTURE.md#g0-blockers).
- Chỉ có thử nghiệm localhost không đủ nghiệm thu MVP từ xa. Thiếu môi trường
  relay/hai máy phải ghi rõ NOT RUN/BLOCKED, hoặc xin chủ dự án thu hẹp phạm vi.
- Nếu cần adapter bổ sung, dùng gate **G0 adapter follow-up** bên dưới sau phê
  duyệt. Nếu không thể tích hợp an toàn, trình phương án và chi phí để chủ dự án
  quyết định.
- Sau G0 đạt, chốt dependency/interface, ước lượng phần còn lại và giao G1.
  Roadmap hiện là thứ tự phụ thuộc, chưa phải cam kết lịch bàn giao.

## Codex handoff
The checklist below is the **historical initial G0 handoff**, now completed.
For current development use the latest closeout and next focused gate above;
do not repeat G0 or treat this checklist as authorization for the whole app.

1. Read [AGENTS.md](../AGENTS.md), then [PRD](PRD.md), [ARCHITECTURE](ARCHITECTURE.md) (decisions, blockers, protocol), [TEST_PLAN](TEST_PLAN.md) and this roadmap. Inspect Git status, source, dependencies and available tools; old experimental toolchains are no longer assumed installed. Use `app/` for the CrocStream module within this same repository and keep `croc/` pristine; do not create another Git repository.
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
- Future reports, commands, logs and benchmark artifacts belong in `app/` within this repository; the planning documents are not a claim of completed gates.
