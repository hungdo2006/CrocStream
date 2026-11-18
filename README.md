# CrocStream

CrocStream là prototype desktop chia sẻ màn hình **1 người phát ↔ 1 người xem**
trên Windows bằng mã bí mật. Đã có bản ZIP portable và hướng dẫn demo;
hiệu năng/giới hạn được ghi từ kiểm thử thực tế, không bảo đảm trên mọi mạng.

**Người mới bắt đầu tại [Hướng dẫn nhập môn](docs/GETTING_STARTED.md).**

G0–G6 đã nghiệm thu trong phạm vi prototype Windows. G4 kiểm chứng desktop cục
bộ và truyền media tổng hợp qua hai máy. G6 bàn giao bản portable unsigned.
[Chỉ mục bằng chứng](docs/gates/README.md) ghi điều kiện và giới hạn từng gate.
Xem [hướng dẫn demo](app/g6/DEMO.md) để chạy bản đóng gói. Webcam và hỗ trợ hệ
điều hành khác thuộc G7 đang hoãn.

Repository: [hungdo2006/CrocStream](https://github.com/hungdo2006/CrocStream).

Một repository duy nhất chứa mã nguồn croc gốc, tài liệu dự án và ứng dụng
CrocStream. Local `D:\CrocStream` nối với repository GitHub ở trên;
nhánh hiện dùng là `main`.

[`croc/`](croc/README.md) chứa snapshot mã nguồn upstream đầy đủ, giữ nguyên nội
dung và giấy phép. Đây là thư mục mã nguồn bình thường trong repo CrocStream,
không có repo Git lồng bên trong; xem [nguồn gốc và giấy phép](UPSTREAM.md).
`app/` chứa module Go framing/config, codec LZ4 tile-delta, telemetry và các
báo cáo/tooling/PoC G0. Codec được kiểm thử bằng ảnh tổng hợp; capture màn hình
thật và local preview đã được triển khai ở G3. Lệnh build/chạy và API nằm trong
[app README](app/README.md).

## Bố trí hiện tại

Theo lựa chọn của chủ dự án ngày 2026-11-15:

```text
D:\CrocStream\
├── .git\                 # Git của duy nhất repo CrocStream
├── croc\                 # Mã nguồn croc gốc, không có .git riêng
├── docs\                 # Đặc tả, kiểm thử, roadmap và chỉ mục bằng chứng
├── app\                  # Mã desktop, transport, harness và báo cáo gate
├── scripts\              # Kiểm tra trên Windows
├── README.md
├── AGENTS.md
└── UPSTREAM.md
```

`app/` nằm cùng cấp với `croc/` và `docs/`, trong cùng repository.
Bố trí này thay thế hai repo local tách rời trước đây.

Nguồn croc: https://github.com/schollz/croc
Commit giữ lại: `552f40fd2d7593316118370bec0a93478d48e715`.
Đây là commit đã clone, không phải cam kết đồng bộ với phiên bản GitHub mới nhất.

Lịch sử hợp nhất repo và nguồn gốc của snapshot được ghi trong [UPSTREAM](UPSTREAM.md).

## Mục tiêu sản phẩm

CrocStream hỗ trợ chia sẻ màn hình **1 người phát ↔ 1 người xem**:

- Kết nối bằng mã bí mật, truyền dữ liệu được xác thực và mã hóa đầu-cuối.
- Dùng adapter duplex đã kiểm chứng tại G0 trên snapshot croc đã ghim.
- Dựng hình từ keyframe đầy đủ và các ô ảnh thay đổi (dirty tiles).
- Đo FPS, băng thông, RTT và hiệu quả delta bằng số liệu thực tế.
- Ưu tiên Windows 10/11 x64; webcam và macOS/Linux là hướng mở rộng.

Ứng dụng dùng Go, Wails v3, React, TypeScript và CSS thuần. Media v1 dùng RGBA
và LZ4. Canvas nhận ảnh hiển thị qua WebSocket loopback.
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
| [CONTRIBUTING](CONTRIBUTING.md) | Thiết lập, kiểm tra và review thay đổi |
| [G0_FEASIBILITY](app/G0_FEASIBILITY.md) | Kết quả khảo sát thực tế, blocker, lệnh đã chạy và quyết định cần xác nhận |
| [G0_TOOLCHAIN](app/G0_TOOLCHAIN.md) | Cài công cụ portable, kết quả build/test và hướng dẫn tái lập |
| [G0_ADAPTER_DESIGN](app/G0_ADAPTER_DESIGN.md) | Thiết kế transport thử nghiệm để review trước triển khai |
| [G0_LOCAL_PROOF](app/G0_LOCAL_PROOF.md) | Kết quả PoC, lệnh kiểm chứng, giới hạn và điều kiện remote còn thiếu |
| [G1_CLOSEOUT](app/G1_CLOSEOUT.md) | Framing/config, quyết định C-01/C-02, tests/fuzz và bàn giao G2 |
| [G2_CLOSEOUT](app/G2_CLOSEOUT.md) | Codec delta, canonical reference, telemetry, tests/fuzz và benchmark |
| [G3_CLOSEOUT](app/G3_CLOSEOUT.md) | Capture GDI, bridge token/Origin, React canvas, bản Windows và bằng chứng thực tế |
| [G4_CLOSEOUT](app/G4_CLOSEOUT.md) | Share/Watch, secure media/control, lifecycle và local/remote proof |
| [G4_REMOTE_PROOF](app/G4_REMOTE_PROOF.md) | Hai máy qua EC2 >=600s với ảnh tổng hợp, hash/counters và cleanup |
| [G5_CLOSEOUT](app/G5_CLOSEOUT.md) | UX/telemetry, stress local và giới hạn resource/benchmark |
| [G6_CLOSEOUT](app/G6_CLOSEOUT.md) | Portable Windows, checksum/notices, production UI proof và demo handoff |

## Cách tiếp tục

Người mới đọc `GETTING_STARTED` để có bức tranh tổng thể. Khi nhận việc, đọc
`AGENTS.md`, rồi `PRD → ARCHITECTURE → TEST_PLAN → ROADMAP`.
Giữ `croc/` nguyên gốc để tham chiếu; phát triển phần mới trong `app/` của cùng
repo khi được giao triển khai. G0 đã đóng; package protocol G1 hiện có trong
`app/internal/protocol`. Xem closeout và gate kế tiếp trong Roadmap khi nhận
task mới. Mọi đường dẫn `docs/`, `croc/` và `app/` tính từ gốc repository;
G0–G6 đã có bằng chứng; demo bản portable trước khi nhận các phần mở rộng G7.
