# CrocStream

CrocStream là ứng dụng thử nghiệm chia sẻ màn hình giữa **một người phát và một
người xem** trên Windows 10/11 x64. Hai ứng dụng kết nối qua cùng một relay và
ghép đôi bằng mã mời trên kênh được xác thực và mã hóa đầu-cuối. Chế độ xem trước
chạy ngay trên máy phát, không cần relay. Client croc thông thường không hiểu
giao thức media của CrocStream.

## Chạy và kiểm tra

Giải nén toàn bộ ZIP portable rồi mở `crocstream.exe`. Máy chạy cần Microsoft
Edge WebView2 Evergreen Runtime. Bản thử nghiệm chưa ký số. Dùng **Xem trước
trên máy này** để kiểm tra nguồn chụp. Hướng dẫn demo ghi cách xác minh ZIP,
chạy relay trên cùng máy hoặc LAN và kết nối hai ứng dụng.
[Hướng dẫn demo](app/g6/DEMO.md) · [Giới hạn bản portable](app/g6/KNOWN_ISSUES.md)

Hướng dẫn thiết lập toolchain và build Wails nằm trong tài liệu dành cho người
đóng góp. Sau khi đã có toolchain Windows, chạy lệnh dưới đây từ thư mục gốc.

```powershell
powershell -NoProfile -File .\scripts\verify.ps1 -Race
```

Lệnh chạy kiểm tra Go và race detector, build desktop và kiểm tra frontend bằng
dữ liệu tổng hợp. Kiểm thử chụp màn hình thật là một lượt riêng.
[Thiết lập và đóng góp](CONTRIBUTING.md) · [Kế hoạch kiểm thử](docs/TEST_PLAN.md)

## Mã nguồn

| Đường dẫn | Trách nhiệm |
| --- | --- |
| `app/internal/desktop`, `app/internal/capture` | Binding Wails và capture màn hình Windows sau thao tác Start. |
| `app/internal/preview`, `app/internal/session` | Vòng đời Preview và Share/Watch. |
| `app/internal/protocol`, `app/internal/delta`, `app/internal/telemetry` | Gói media v1, codec LZ4 tile-delta và số đo. |
| `app/internal/bridge`, `app/frontend/src` | JPEG qua WebSocket loopback và hiển thị qua React/TypeScript, canvas. |
| `app/internal/transport`, `app/g0/poc` | Adapter duplex và bằng chứng khả thi G0. |
| `croc/` | Snapshot upstream được ghim ở `552f40fd2d7593316118370bec0a93478d48e715`. [Nguồn gốc và giấy phép](UPSTREAM.md) |

Máy phát chuẩn hóa ảnh RGBA rồi gửi ảnh đầy đủ (KEY) hoặc các ô thay đổi
(DELTA) qua relay. Máy xem dựng lại ảnh tham chiếu và yêu cầu KEY khi mất chuỗi
DELTA. JPEG chỉ phục vụ hiển thị nội bộ. [Kiến trúc](docs/ARCHITECTURE.md)

## Tài liệu dự án

- Bắt đầu với [hướng dẫn nhập môn](docs/GETTING_STARTED.md).
- Khi nhận việc, đọc [AGENTS](AGENTS.md), [PRD](docs/PRD.md),
  [kiến trúc](docs/ARCHITECTURE.md), [kế hoạch kiểm thử](docs/TEST_PLAN.md) và
  [lộ trình](docs/ROADMAP.md).
- G0–G6 đã nghiệm thu trong phạm vi prototype Windows. G4 kiểm chứng Share/Watch
  cục bộ và truyền ảnh tổng hợp qua hai máy. G6 kiểm chứng bản portable.
  Webcam và các hệ điều hành khác thuộc G7 đang hoãn.
  [Chỉ mục bằng chứng và giới hạn](docs/gates/README.md)
