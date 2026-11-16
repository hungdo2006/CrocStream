# CrocStream

CrocStream là ứng dụng desktop **đang phát triển phần lõi**, hướng tới chia sẻ
màn hình **1 người phát ↔ 1 người xem** trên Windows bằng mã bí mật. Điểm cần
kiểm chứng là khả năng dùng transport croc cho luồng dữ liệu hai chiều liên tục
và hiệu quả gửi các vùng ảnh thay đổi thay vì toàn bộ ảnh.

**Người mới bắt đầu tại [Hướng dẫn nhập môn](docs/GETTING_STARTED.md).**

| Trạng thái hiện tại | Nội dung |
| --- | --- |
| Đã có | Toolchain, adapter/PoC G0, framing/config G1, [codec delta + telemetry G2](app/README.md) |
| Chưa có | Ứng dụng chia sẻ màn hình; G3–G7 chưa triển khai |
| Trạng thái G0 | **PASS** cho khả thi prototype: transport local/remote, native build/runtime và notice review; [quyết định cuối](app/G0_CLOSEOUT.md) |
| Trạng thái G1 | **PASS** framing/config, tests/race/vet/build/fuzz; [báo cáo](app/G1_CLOSEOUT.md); C-01/C-02 đã chốt |
| Trạng thái G2 | **PASS** codec/telemetry, tests/race/vet/build/fuzz/bench; [báo cáo](app/G2_CLOSEOUT.md) |
| Công việc phát triển kế tiếp | G3 capture màn hình + preview bảo vệ token/Origin |
| Bằng chứng | Remote >600s, [mạng mới](app/G0_LATENCY_FOLLOWUP.md) RTT mean 48–51ms; giữ giới hạn mạng cũ, không hứa latency/FPS chung |

Repository: [hungdo2006/CrocStream](https://github.com/hungdo2006/CrocStream).

Một repository duy nhất chứa mã nguồn croc gốc, tài liệu thiết kế và phần ứng dụng
CrocStream sẽ phát triển. Local `D:\CrocStream` nối với repository GitHub ở trên;
nhánh hiện dùng là `main`.

[`croc/`](croc/README.md) chứa snapshot mã nguồn upstream đầy đủ, giữ nguyên nội
dung và giấy phép. Đây là thư mục mã nguồn bình thường trong repo CrocStream,
không có repo Git lồng bên trong; xem [nguồn gốc và giấy phép](UPSTREAM.md).
`app/` chứa module Go framing/config, codec LZ4 tile-delta, telemetry và các
báo cáo/tooling/PoC G0. Codec được kiểm thử bằng ảnh tổng hợp; capture màn hình
thật thuộc G3. Lệnh kiểm tra và API nằm trong [app README](app/README.md).

## Bố trí hiện tại

Theo lựa chọn của chủ dự án ngày 2026-11-15:

```text
D:\CrocStream\
├── .git\                 # Git của duy nhất repo CrocStream
├── croc\                 # Mã nguồn croc gốc, không có .git riêng
├── docs\                 # PRD, kiến trúc, kế hoạch kiểm thử, roadmap
├── app\                  # Module Go G1 và các báo cáo/tooling/PoC G0
├── README.md
├── AGENTS.md
└── UPSTREAM.md
```

`app/` nằm cùng cấp với `croc/` và `docs/`, trong cùng repository.
Bố trí này thay thế hai repo local tách rời trước đây.

Nguồn croc: https://github.com/schollz/croc
Commit giữ lại: `552f40fd2d7593316118370bec0a93478d48e715`.
Đây là commit đã clone, không phải cam kết đồng bộ với phiên bản GitHub mới nhất.

Mã transport CrocStream tùy chỉnh, chương trình thử nghiệm, toolchain/cache cục bộ,
script, log và hồ sơ thử nghiệm cũ đã được loại bỏ theo yêu cầu. Khảo sát G0 mới
được lưu tại `app/G0_FEASIBILITY.md`; bằng chứng runtime mới ở
`app/G0_LOCAL_PROOF.md`. Các giai đoạn G0–G7 là lộ trình, không phải báo cáo đã
hoàn thành toàn bộ.

## Mục tiêu sản phẩm

CrocStream dự kiến là ứng dụng desktop chia sẻ màn hình **1 người phát ↔ 1 người xem**:

- Kết nối bằng mã bí mật, truyền dữ liệu được xác thực và mã hóa đầu-cuối.
- Khảo sát khả năng tái sử dụng lõi/relay croc trước khi chọn transport.
- Dựng hình từ keyframe đầy đủ và các ô ảnh thay đổi (dirty tiles).
- Đo FPS, băng thông, RTT và hiệu quả delta bằng số liệu thực tế.
- Ưu tiên Windows 10/11 x64; webcam và macOS/Linux là hướng mở rộng.

Stack đề xuất: Go + Wails v3, React/TypeScript/Tailwind, codec RGBA + LZ4,
canvas và WebSocket loopback cho hiển thị. Đây là lựa chọn thiết kế cần kiểm chứng.
`web/` trong croc là website truyền file của upstream, không phải UI CrocStream.
Client croc thông thường không hiểu giao thức media CrocStream.

## Danh mục tài liệu

| Tài liệu | Nội dung |
| --- | --- |
| [GETTING_STARTED](docs/GETTING_STARTED.md) | Nhập môn tiếng Việt: luồng hoạt động, thuật ngữ, thứ tự đọc và checklist ngày đầu |
| [PRD](docs/PRD.md) | Mục tiêu, phạm vi MVP, giao diện, trạng thái và tiêu chí nghiệm thu |
| [ARCHITECTURE](docs/ARCHITECTURE.md) | Quyết định, blocker, module, giao thức media và nguồn tham khảo |
| [TEST_PLAN](docs/TEST_PLAN.md) | Kế hoạch kiểm thử và phương pháp đo |
| [ROADMAP](docs/ROADMAP.md) | Trình tự phát triển, cổng nghiệm thu và hướng dẫn giao việc Codex |
| [G0_FEASIBILITY](app/G0_FEASIBILITY.md) | Kết quả khảo sát thực tế, blocker, lệnh đã chạy và quyết định cần xác nhận |
| [G0_TOOLCHAIN](app/G0_TOOLCHAIN.md) | Cài công cụ portable, kết quả build/test và hướng dẫn tái lập |
| [G0_ADAPTER_DESIGN](app/G0_ADAPTER_DESIGN.md) | Thiết kế transport thử nghiệm để review trước triển khai |
| [G0_LOCAL_PROOF](app/G0_LOCAL_PROOF.md) | Kết quả PoC, lệnh kiểm chứng, giới hạn và điều kiện remote còn thiếu |
| [G1_CLOSEOUT](app/G1_CLOSEOUT.md) | Framing/config, quyết định C-01/C-02, tests/fuzz và bàn giao G2 |
| [G2_CLOSEOUT](app/G2_CLOSEOUT.md) | Codec delta, canonical reference, telemetry, tests/fuzz và benchmark |

## Cách tiếp tục

Người mới đọc `GETTING_STARTED` để có bức tranh tổng thể. Khi nhận việc, đọc
`AGENTS.md`, rồi `PRD → ARCHITECTURE → TEST_PLAN → ROADMAP`.
Giữ `croc/` nguyên gốc để tham chiếu; phát triển phần mới trong `app/` của cùng
repo khi được giao triển khai. G0 đã đóng; package protocol G1 hiện có trong
`app/internal/protocol`. Xem closeout và gate kế tiếp trong Roadmap khi nhận
task mới. Mọi đường dẫn `docs/`, `croc/` và `app/` tính từ gốc repository;
capture/UI trong kiến trúc vẫn là các bước tiếp theo.
