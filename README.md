# CrocStream

CrocStream là ứng dụng desktop chia sẻ màn hình 1-1 thời gian thực trên Windows 10/11 x64, được phát triển mở rộng từ nền tảng truyền dữ liệu mã hóa của [`schollz/croc`](https://github.com/schollz/croc) (ghim tại commit [`552f40fd2d75`](UPSTREAM.md)).

> **Tài liệu dành cho thành viên nhóm & bảo vệ đồ án:** Xem ngay **[PROJECT_GUIDE.md](PROJECT_GUIDE.md)** để nắm toàn bộ luồng hoạt động, kiến trúc, bảng đối chiếu đóng góp so với repository gốc và kịch bản demo.

---

## 1. Vấn đề giải quyết

Công cụ mã nguồn mở `croc` gốc cho phép hai máy tính kết nối an toàn qua mã mời (codephrase) và trao đổi khóa mật khẩu PAKE (Password-Authenticated Key Exchange), nhưng **chỉ hỗ trợ truyền file tĩnh hoặc chia sẻ terminal dòng lệnh (SSH)**.

CrocStream mở rộng nền tảng mật mã và relay của `croc` để giải quyết bài toán **chia sẻ màn hình desktop 1-1 có xác thực và mã hóa đầu cuối (E2E)** mà không cần tạo tài khoản hay mở cổng máy người dùng, đồng thời **đo lường thực nghiệm hiệu quả tiết kiệm băng thông** của thuật toán nén phần thay đổi theo ô (**Tile-Delta**) so với nén toàn bộ khung hình (**Full-frame LZ4 baseline**).

## 2. Chức năng chính

- **Chia sẻ màn hình 1-1 (`Chia sẻ màn hình`):** Chụp màn hình thực trên Windows (1280×720 hoặc 640×360 ở 15/24/30 FPS), tạo mã mời dùng một lần (`cs1-<6 từ khóa>`) và truyền luồng ảnh đã nén/mã hóa tới người xem.
- **Xem phiên từ xa (`Tham gia bằng mã`):** Kết nối bằng mã mời và địa chỉ relay, xác thực hai chiều bằng PAKE trước khi giải mã và hiển thị hình ảnh trực tiếp lên canvas 16:9.
- **Xem trước cục bộ (`Xem trước trên máy này`):** Kiểm tra nguồn chụp màn hình và tốc độ hiển thị ngay trên máy mà không cần kết nối mạng hay relay.
- **Codec Tile-Delta xác định & tự phục hồi:** Chia khung hình thành lưới ô `32×32` pixel, chỉ truyền các ô có độ lệch màu `RGB MAE > tau` (nén không mất dữ liệu bằng LZ4 frame), tự động gửi `KEYFRAME` định kỳ hoặc khi người xem yêu cầu phục hồi (`REQUEST_KEYFRAME` / PLI).
- **Dashboard đo lường thời gian thực:** Hiển thị số đo thực tế trong cửa sổ trượt 1 giây (FPS chụp/gửi/nhận/giải mã/hiển thị, tốc độ video `KiB/s`, baseline nén toàn ảnh `KiB/s`, tỷ lệ tiết kiệm `%`, kích thước trung bình `KEY`/`DELTA`, và độ trễ khứ hồi `RTT`).
- **Relay demo đi kèm (`crocstream-relay.exe`):** Tiện ích bọc relay TCP của `croc`, nhận mật khẩu an toàn qua `stdin` để chạy demo trên một máy (`127.0.0.1:9009`) hoặc giữa hai máy trong mạng LAN riêng.

## 3. Công nghệ sử dụng

| Tầng | Công nghệ / Thư viện | Vai trò |
| --- | --- | --- |
| **Desktop & IPC** | [Wails v3.1.0](https://v3.wails.io/), WebView2 | Đóng gói ứng dụng Windows native và cầu nối RPC giữa Go và React |
| **Backend & Codec** | Go 1.27, [`pierrec/lz4/v4`](https://github.com/pierrec/lz4) | Điều phối phiên, giao thức nhị phân 18-byte header, codec Tile-Delta, nén LZ4 và telemetry |
| **Chụp màn hình & Cầu nối ảnh** | Win32 GDI (`golang.org/x/sys/windows`), [`coder/websocket`](https://github.com/coder/websocket) | Chụp màn hình DPI-aware trên luồng OS cố định; truyền ảnh JPEG cục bộ qua WebSocket loopback `127.0.0.1` có xác thực token |
| **Mật mã & Truyền vận** | [`schollz/croc/v11`](croc/), `schollz/pake/v3`, `golang.org/x/crypto` | Sinh mã mời, kết nối relay TCP, thỏa thuận khóa PAKE (`p256`) và mã hóa luồng hai chiều AES-256-GCM |
| **Frontend & Kiểm thử UI** | React 19, TypeScript 6, Vite 8, Vitest, Playwright | Giao diện người dùng, quản lý vòng đời phiên (`SessionLifecycle`), vẽ canvas bằng `ImageBitmap` |

## 4. Điều kiện tiên quyết

- **Để chạy bản đóng gói sẵn (Portable ZIP):** Windows 10/11 x64 và **Microsoft Edge WebView2 Evergreen Runtime**.
- **Để build và chạy kiểm thử từ mã nguồn:**
  - Windows 10/11 x64, PowerShell 5.1+
  - Go **1.27.4**, Wails CLI **v3.1.0** (có thể cài tự động vào thư mục tạm qua script trong `app/g0/`)
  - Node.js **24.22.1**, npm **12.2.1**
  - Trình biên dịch `gcc` (MinGW-w64) nếu chạy kiểm thử phát hiện xung đột dữ liệu (`-race`) và trình duyệt Microsoft Edge cho bài test giao diện Playwright.

## 5. Cài đặt và chạy ứng dụng

### Cách 1: Build và chạy trực tiếp từ mã nguồn

Từ thư mục gốc `D:\CrocStream`, mở PowerShell:

```powershell
# 1. (Chỉ cần lần đầu nếu chưa có Go/Wails portable)
powershell -NoProfile -File .\app\g0\Install-Toolchain.ps1 -Component Go
powershell -NoProfile -File .\app\g0\Install-Toolchain.ps1 -Component Wails

# 2. Nạp môi trường toolchain vào phiên PowerShell hiện tại
. .\app\g0\Use-Toolchain.ps1

# 3. Build ứng dụng desktop Windows
Push-Location app
$env:WAILS_EXP_USE_WAKE = '1'
wails3 build
Pop-Location

# 4. Chạy ứng dụng
.\app\bin\crocstream.exe
```

### Cách 2: Tạo gói Portable ZIP hoàn chỉnh (gồm cả ứng dụng và relay demo)

```powershell
. .\app\g0\Use-Toolchain.ps1
Push-Location app
.\g6\Build-Portable.ps1 -OutputDirectory release/demo-build
Pop-Location
```

## 6. Chạy demo và kiểm thử

### Chạy toàn bộ bộ kiểm thử tự động (Go unit/race/vet + Wails build + TypeScript/ESLint/Vitest/Playwright)

```powershell
powershell -NoProfile -File .\scripts\verify.ps1 -Race
```

*(Thêm cờ `-Offline` nếu bộ nhớ đệm Go và npm trên máy đã đầy đủ).*

### Chạy demo chia sẻ màn hình nhanh trên cùng một máy (2 cửa sổ)

1. Khởi chạy relay cục bộ tại `127.0.0.1:9009` (nhập mật khẩu từ 12 ký tự trở lên khi được hỏi):
   ```powershell
   . .\app\g0\Use-Toolchain.ps1
   go -C app build -o .\app\bin\crocstream-relay.exe ./cmd/demo-relay
   .\app\g6\Start-DemoRelay.ps1
   ```
   *(Hoặc chạy trực tiếp `.\Start-DemoRelay.ps1` trong thư mục giải nén của gói Portable ZIP).*
2. Mở cửa sổ **`crocstream.exe` thứ nhất** → chọn **Chia sẻ màn hình** → nhập địa chỉ relay `127.0.0.1:9009` và mật khẩu relay → chọn màn hình → bấm **Bắt đầu chia sẻ** → bấm **Sao chép mã**.
3. Mở cửa sổ **`crocstream.exe` thứ hai** → chọn **Tham gia bằng mã** → nhập cùng địa chỉ và mật khẩu relay → dán mã mời vào ô **Mã phiên** → bấm **Tham gia phiên**.
4. Xem hướng dẫn từng bước chi tiết cho cả mạng LAN tại [app/g6/DEMO.md](app/g6/DEMO.md).

## 7. Cấu trúc tài liệu trong repository

- **[PROJECT_GUIDE.md](PROJECT_GUIDE.md):** Tài liệu tổng hợp toàn diện bằng tiếng Việt (Tổng quan, Luồng dữ liệu, Kiến trúc, So sánh với `croc` gốc, Giải thích kỹ thuật và Kịch bản bảo vệ đồ án).
- **[docs/GETTING_STARTED.md](docs/GETTING_STARTED.md):** Hướng dẫn làm quen nhanh cho người mới.
- **[docs/PRD.md](docs/PRD.md) · [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) · [docs/TEST_PLAN.md](docs/TEST_PLAN.md) · [docs/ROADMAP.md](docs/ROADMAP.md):** Đặc tả yêu cầu sản phẩm, hợp đồng kiến trúc/giao thức nhị phân v1, kế hoạch kiểm thử và lộ trình các cổng nghiệm thu G0–G6.
- **[docs/gates/README.md](docs/gates/README.md) · [docs/gates/notices.md](docs/gates/notices.md):** Chỉ mục báo cáo nghiệm thu thực nghiệm G0–G6 (`docs/gates/reports/`, `docs/gates/evidence/`) và thông báo bản quyền phần mềm bên thứ ba.
- **[UPSTREAM.md](UPSTREAM.md) · [croc/LICENSE](croc/LICENSE):** Thông tin xuất xứ và giấy phép MIT của snapshot `schollz/croc` gốc trong thư mục `croc/`.
