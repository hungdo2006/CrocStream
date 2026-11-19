# CrocStream

CrocStream là ứng dụng thử nghiệm chia sẻ màn hình giữa **một người phát và một
người xem** trên Windows 10/11 x64. Hai ứng dụng kết nối qua cùng một relay,
ghép đôi bằng mã mời và truyền hình ảnh qua kênh mã hóa đã xác thực. Chế độ xem
trước chạy ngay trên máy phát, không cần relay. CrocStream dùng giao thức media
riêng, không tương thích với chức năng truyền tệp của croc CLI.

## Chạy bản portable

Giải nén toàn bộ ZIP rồi mở `crocstream.exe`. Máy chạy cần Microsoft Edge
WebView2 Evergreen Runtime. Bản thử nghiệm chưa ký số và không có trình cài đặt.
Chọn **Xem trước trên máy này** để kiểm tra nguồn chụp. Để chia sẻ, chạy demo
relay, nhập cùng địa chỉ và mật khẩu relay trong hai ứng dụng, rồi gửi mã mời
cho người xem. Hướng dẫn có lệnh cho cùng một máy hoặc hai máy trong mạng LAN,
kèm cách xác minh gói tải về. [Hướng dẫn demo](app/g6/DEMO.md) · [Giới hạn](app/g6/KNOWN_ISSUES.md)

## Build trên Windows

Dùng Go 1.27.3 trở lên, Wails CLI 3.1.0, Node 24 và npm. Bản portable G6 dùng
Go 1.27.4 và Node 24.22.1. Nếu dùng bộ công cụ portable của dự án, nạp
`app/g0/Use-Toolchain.ps1` vào phiên PowerShell hiện tại trước khi chạy các lệnh
dưới đây. [Thiết lập toolchain](app/g0/README.md#portable-tools)

Chạy từ thư mục gốc:

```powershell
cd app
$env:WAILS_EXP_USE_WAKE = '1'
wails3 build
.\bin\crocstream.exe
```

`wails3 build` sinh binding từ Go, cài frontend theo lockfile, build Vite và
đóng gói tài nguyên vào EXE. Chế độ phát triển chạy bằng
`wails3 dev -host 127.0.0.1 -port 9245` trong `app/`, với cùng biến môi trường.
Giữ cổng 9245 để Origin của WebView khớp cấu hình. Nhấn Ctrl+C để dừng dev watcher.

Để tạo ZIP portable và checksum, dùng bộ công cụ portable của dự án rồi chạy
`.\g6\Build-Portable.ps1 -OutputDirectory release/new-build` từ `app/` với
thư mục đích mới. [Thành phần gói portable](app/g6/README.md) · [Giấy phép và ghi công](app/THIRD_PARTY_NOTICES.md)

## Đọc mã nguồn

| Vị trí | Trách nhiệm |
| --- | --- |
| `app/internal/desktop` | Các binding của Wails và điều phối chế độ xem trước hoặc Share/Watch. |
| `app/internal/capture`, `frame`, `preview` | Chụp màn hình Windows, chuẩn hóa RGBA, quản lý phiên xem trước. |
| `app/internal/session`, `transport` | Vòng đời Share/Watch, ghép đôi qua relay và điều khiển phiên. |
| `app/internal/protocol`, `delta`, `telemetry` | Định dạng gói, codec LZ4 tile-delta và số đo. |
| `app/internal/bridge`, `app/frontend/src` | JPEG qua WebSocket nội bộ, giao diện React/TypeScript và canvas. |
| `app/g0/poc`, `app/g3`–`app/g6` | Adapter duplex đang được tái sử dụng và các bộ kiểm chứng/build theo mốc. |
| `croc/` | Snapshot upstream được ghim để tái sử dụng relay và cơ chế xác thực. [Nguồn gốc](UPSTREAM.md) |

Máy phát chụp và chuẩn hóa ảnh RGBA, sau đó gửi ảnh đầy đủ (**KEY**) hoặc các ô
thay đổi (**DELTA**) qua relay. Máy xem dựng lại ảnh tham chiếu và yêu cầu KEY mới
nếu mất chuỗi DELTA. Hai ứng dụng dùng JPEG riêng để hiển thị ảnh lên canvas qua
loopback. [Kiến trúc và hợp đồng giao thức](docs/ARCHITECTURE.md)

## Kiểm thử

Chạy từ thư mục gốc. Các lệnh frontend cần binding do Wails sinh và các gói đã
cài theo lockfile ở bước build.

```text
go -C app test ./...
go -C app test -race ./...
go -C app vet ./...
go -C app/g0/poc test ./...
npm --prefix app/frontend run typecheck
npm --prefix app/frontend run lint
npm --prefix app/frontend run test
npm --prefix app/frontend run build
```

Chạy `-race` trên môi trường có CGO và trình biên dịch C hỗ trợ Go race detector.
Các bước thử canvas, vòng đời phiên, đóng gói và đo hiệu năng có chỉ dẫn riêng
trong tài liệu kỹ thuật. Kết quả G0–G6 đã ghi nhận gồm phiên desktop nội bộ,
truyền ảnh tổng hợp giữa hai máy và kiểm thử gói portable trên Windows. Hiệu năng
chụp rồi hiển thị màn hình thật qua Internet vẫn chưa được đo.
[Phạm vi kiểm thử](docs/ARCHITECTURE.md#kiểm-thử) · [Kết quả G0–G6](docs/VERIFICATION.md)
