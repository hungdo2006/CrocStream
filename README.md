# CrocStream

**CrocStream** là ứng dụng desktop chia sẻ màn hình và cửa sổ ứng dụng **1-1** trên Windows 10/11 x64, xây dựng trên nền tảng mật mã và giao thức phòng chờ của [`schollz/croc`](https://github.com/schollz/croc) (ghim nguyên bản tại commit [`552f40fd2d75`](UPSTREAM.md)).

> **Bạn cần đọc file nào?**
> - **Thành viên nhóm & Người mới tìm hiểu (Đọc trong 3–5 phút):** Xem **[PROJECT_GUIDE.md](PROJECT_GUIDE.md)**.
> - **Hướng dẫn bấm chạy Demo từng bước:** Xem **[packaging/DEMO.md](packaging/DEMO.md)**.
> - *(Hồ sơ kỹ thuật chuyên sâu và biên bản nghiệm thu cũ nằm trong [`docs/specs/`](docs/specs/) và [`docs/archive-gates/`](docs/archive-gates/)).*

---

## 1. Tính năng nổi bật

- **Chọn toàn màn hình hoặc từng cửa sổ ứng dụng** (Chrome, Zalo, VS Code, PowerPoint…) kèm **xem trước trực tiếp** ngay trước khi bấm chia sẻ.
- **Tự ưu tiên LAN trực tiếp → tự động chuyển Relay khi khác mạng:** Khi hai máy ở cùng mạng LAN/Wi-Fi, ứng dụng tự khám phá nội bộ (UDP `9019`, chỉ gửi tag băm 16 byte, không lộ mật khẩu/mã mời) để nối thẳng độ trễ thấp; khi khác mạng hoặc bị tường lửa chặn LAN, tự động chuyển qua trạm Relay (LAN hoặc VPS) mà không cần chỉnh cấu hình.
- **Nén Tile-Delta không mất dữ liệu (LZ4):** Chia khung hình thành các ô `32×32`, chỉ gửi ô thay đổi, tự phục hồi lỗi (`PLI`) và hiển thị số đo thực tế (`FPS`, `KiB/s`, `RTT`, `Đường truyền`, `% tiết kiệm`).
- **Tự điền Relay qua `relay.json`:** Đặt file `relay.json` (bị `.gitignore` bỏ qua, không commit lên Git) cạnh `crocstream.exe` để tự điền sẵn địa chỉ & mật khẩu Relay như `croc` gốc.

---

## 2. Bố cục thư mục dự án

| Thư mục | Vai trò | Ghi chú |
| --- | --- | --- |
| **`app/`** | **Phần mềm chính CrocStream** (`frontend/`: giao diện React, `internal/`: backend Go, `cmd/demo-relay/`: trạm relay cục bộ). | Nơi chứa toàn bộ mã nguồn ứng dụng. |
| **`packaging/`** | Script đóng gói **Portable ZIP**, script bật relay cục bộ (`Start-DemoRelay.ps1`) và hướng dẫn Demo (`DEMO.md`). | Dùng khi đóng gói hoặc chạy Demo. |
| **`scripts/`** | Công cụ tự động: kiểm thử toàn diện (`verify.ps1`), dọn file tạm (`clean.ps1`), cài Go/Wails (`toolchain/`). | Dùng khi build/test từ mã nguồn. |
| **`docs/`** | Đặc tả kỹ thuật (`specs/`) và biên bản nghiệm thu `G0–G6` (`archive-gates/`). | Hồ sơ kỹ thuật để tra cứu/nộp kèm. |
| **`croc/`** | Mã nguồn `schollz/croc` gốc giữ nguyên vẹn 100% làm thư viện nền và đối chứng. | **Không chỉnh sửa.** |

---

## 3. Kết nối 2 máy & cấu hình `relay.json`

```text
Cùng LAN/Wi-Fi :  Người phát ──(LAN trực tiếp · PAKE + AES-256-GCM)──> Người xem
Khác mạng      :  Người phát ──> Trạm Relay TCP (PAKE + AES-256-GCM) ──> Người xem
```

1. **Địa chỉ & mật khẩu Relay:** Nhập trực tiếp trên giao diện, hoặc tạo file `relay.json` đặt cùng thư mục với `crocstream.exe` (ứng dụng sẽ tự nạp sẵn khi mở):
   ```json
   {
     "address": "198.51.100.10:9009",
     "password": "mat-khau-relay-tu-dat"
   }
   ```
   *(Chạy thử 1 máy dùng `127.0.0.1:9009`; chạy 2 máy cùng Wi-Fi không cần VPS có thể bật `Start-DemoRelay.ps1` và điền IP LAN của máy bật relay; chạy 2 máy khác mạng dùng Relay đặt trên VPS có IP công cộng.)*
2. **Mã mời (`cs1-...`):** Sinh ra sau khi Người phát bấm **Bắt đầu chia sẻ**. Bấm **Sao chép mã** gửi cho Người xem dán vào ô **Mã phiên**. Khi bấm dừng, mã lập tức bị thu hồi.

---

## 4. Cài đặt, Build và Chạy nhanh

**Yêu cầu:** Windows 10/11 x64 (có sẵn Microsoft Edge WebView2). Nếu build từ mã nguồn cần PowerShell 5.1+, Node.js `24.22.1`, npm `12.2.1` (Go `1.27.4` và Wails `v3.1.0` cài tự động bằng lệnh số 1).

```powershell
# 1. Cài Go & Wails portable (chỉ cần chạy lần đầu nếu máy chưa có)
powershell -NoProfile -File .\scripts\toolchain\Install-Toolchain.ps1 -Component Go
powershell -NoProfile -File .\scripts\toolchain\Install-Toolchain.ps1 -Component Wails

# 2. Nạp môi trường và build ứng dụng Windows (tạo ra app\bin\crocstream.exe)
. .\scripts\toolchain\Use-Toolchain.ps1
Push-Location app; $env:WAILS_EXP_USE_WAKE = '1'; wails3 build; Pop-Location

# 3. Chạy ứng dụng
.\app\bin\crocstream.exe
```

### Các lệnh thường dùng khác

| Mục đích | Lệnh PowerShell (chạy từ thư mục gốc) |
| --- | --- |
| **Chạy Demo nhanh không cần VPS** | `. .\scripts\toolchain\Use-Toolchain.ps1` → `go -C app build -o .\packaging\crocstream-relay.exe ./cmd/demo-relay` → `.\packaging\Start-DemoRelay.ps1` |
| **Tạo gói Portable ZIP hoàn chỉnh** | `.\packaging\Build-Portable.ps1 -OutputDirectory release/demo-build` |
| **Chạy toàn bộ kiểm thử tự động** | `powershell -NoProfile -File .\scripts\verify.ps1 -Race` |
| **Dọn sạch thư mục nháp** | `powershell -NoProfile -File .\scripts\clean.ps1` |
