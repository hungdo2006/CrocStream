# CrocStream — Tài liệu nội dung và hướng phát triển

Repository: [hungdo2006/CrocStream](https://github.com/hungdo2006/CrocStream).

Repository chứa tài liệu thiết kế CrocStream và bản mã nguồn upstream croc trong
[`croc/`](croc/README.md). Chưa có mã ứng dụng chia sẻ màn hình CrocStream.
Thư mục `crocstream-docs/` trong bố trí cục bộ bên dưới là gốc repository này.
Bản upstream được đưa vào dưới dạng snapshot mã nguồn đầy đủ, không phải submodule;
xem [nguồn gốc và giấy phép](UPSTREAM.md).

## Bố trí hiện tại

Theo lựa chọn của chủ dự án ngày 2026-11-15:

```text
D:\CrocStream\
├── croc\                 # Mã nguồn upstream nguyên gốc và metadata Git
└── crocstream-docs\       # Repository hungdo2006/CrocStream
    ├── README.md
    ├── AGENTS.md
    ├── UPSTREAM.md
    ├── croc\             # Snapshot upstream giữ nguyên nội dung
    └── docs\
```

Nguồn croc: https://github.com/schollz/croc
Commit giữ lại: `552f40fd2d7593316118370bec0a93478d48e715`.
Đây là commit đã clone, không phải cam kết đồng bộ với phiên bản GitHub mới nhất.

Mã transport CrocStream tùy chỉnh, chương trình thử nghiệm, toolchain/cache cục bộ,
script, log và hồ sơ thử nghiệm cũ đã được loại bỏ theo yêu cầu. Bộ tài liệu này
không kèm triển khai CrocStream hoặc bằng chứng nghiệm thu cho một bản triển khai
hiện hành. Các giai đoạn G0–G7 bên dưới là kế hoạch công việc khi tiếp tục phát triển.

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
| [PRD](docs/PRD.md) | Mục tiêu, phạm vi MVP, giao diện, trạng thái và tiêu chí nghiệm thu |
| [ARCHITECTURE](docs/ARCHITECTURE.md) | Quyết định, blocker, module, giao thức media và nguồn tham khảo |
| [TEST_PLAN](docs/TEST_PLAN.md) | Kế hoạch kiểm thử và phương pháp đo |
| [ROADMAP](docs/ROADMAP.md) | Trình tự phát triển, cổng nghiệm thu và hướng dẫn giao việc Codex |

## Cách tiếp tục

Đọc `AGENTS.md`, rồi `PRD → ARCHITECTURE → TEST_PLAN → ROADMAP`.
Giữ `croc/` và checkout riêng `../croc/` nguyên gốc để tham chiếu. Khi được giao triển khai, thống nhất vị trí
repository/module CrocStream riêng trước khi tạo code hoặc cài toolchain. Mọi đường
dẫn `docs/` trong bộ này tính từ `crocstream-docs/`; cấu trúc ứng dụng trong tài liệu
kiến trúc là đề xuất, chưa tồn tại.
