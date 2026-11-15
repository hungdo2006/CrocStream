# CrocStream

Repository: [hungdo2006/CrocStream](https://github.com/hungdo2006/CrocStream).

Một repository duy nhất chứa mã nguồn croc gốc, tài liệu thiết kế và phần ứng dụng
CrocStream sẽ phát triển. Local `D:\CrocStream` nối với repository GitHub ở trên;
nhánh hiện dùng là `main`.

[`croc/`](croc/README.md) chứa snapshot mã nguồn upstream đầy đủ, giữ nguyên nội
dung và giấy phép. Đây là thư mục mã nguồn bình thường trong repo CrocStream,
không có repo Git lồng bên trong; xem [nguồn gốc và giấy phép](UPSTREAM.md).
Phần ứng dụng mới dự kiến nằm trong `app/`; thư mục này chưa được tạo và chưa có
mã ứng dụng chia sẻ màn hình CrocStream.

## Bố trí hiện tại

Theo lựa chọn của chủ dự án ngày 2026-11-15:

```text
D:\CrocStream\
├── .git\                 # Git của duy nhất repo CrocStream
├── croc\                 # Mã nguồn croc gốc, không có .git riêng
├── docs\                 # PRD, kiến trúc, kế hoạch kiểm thử, roadmap
├── README.md
├── AGENTS.md
└── UPSTREAM.md
```

Khi bắt đầu triển khai, `app/` sẽ nằm cùng cấp với `croc/` và `docs/`, trong
cùng repository. Bố trí này thay thế hai repo local tách rời trước đây.

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
Giữ `croc/` nguyên gốc để tham chiếu; phát triển phần mới trong `app/` của cùng
repo khi được giao triển khai, bắt đầu bằng kiểm chứng transport G0. Việc sắp xếp
repo chưa phải triển khai ứng dụng hoặc cài toolchain. Mọi đường dẫn `docs/`,
`croc/` và `app/` tính từ gốc repository; cấu trúc ứng dụng trong tài liệu kiến
trúc là đề xuất, chưa tồn tại.
