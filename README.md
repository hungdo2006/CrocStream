# CrocStream

**CrocStream** là ứng dụng desktop chia sẻ màn hình trực tiếp **1-1** trên Windows 10/11 x64, mở rộng từ nền tảng truyền dữ liệu mã hóa của [`schollz/croc`](https://github.com/schollz/croc) (ghim nguyên bản tại commit [`552f40fd2d75`](UPSTREAM.md)).

> **Bạn cần đọc file nào?**
> - **Thành viên nhóm & Người mới tìm hiểu (Đọc trong 3–5 phút):** Xem **[PROJECT_GUIDE.md](PROJECT_GUIDE.md)**.
> - **Hướng dẫn bấm chạy Demo từng bước:** Xem **[packaging/DEMO.md](packaging/DEMO.md)**.
> - *(Hồ sơ kỹ thuật chuyên sâu và biên bản nghiệm thu cũ nằm trong [`docs/specs/`](docs/specs/) và [`docs/archive-gates/`](docs/archive-gates/) — người mới không cần đọc).*

---

## 1. Bố cục thư mục dự án

| Thư mục | Vai trò | Ghi chú cho thành viên |
| --- | --- | --- |
| **`app/`** | **Phần mềm chính CrocStream** (`frontend/`: giao diện React, `internal/`: xử lý ngầm Go, `cmd/demo-relay/`: trạm chuyển tiếp). | Nơi chứa toàn bộ mã nguồn ứng dụng. |
| **`packaging/`** | Script đóng gói bản chạy ngay (**Portable ZIP**), nút bật trạm relay và hướng dẫn chạy Demo. | Mở khi cần đóng gói hoặc chạy Demo. |
| **`scripts/`** | Công cụ tự động: chạy kiểm thử (`verify.ps1`), dọn file nháp (`clean.ps1`), cài môi trường (`toolchain/`). | Dùng khi build/test từ mã nguồn. |
| **`docs/`** | Bản vẽ thiết kế kỹ thuật (`specs/`) và kho lưu trữ biên bản nghiệm thu `G0–G6` (`archive-gates/`). | Hồ sơ hậu trường để tra cứu/nộp kèm. |
| **`croc/`** | Mã nguồn `schollz/croc` gốc giữ nguyên vẹn 100% làm thư viện nền và đối chứng trên CI. | **Không cần mở / Không chỉnh sửa.** |

---

## 2. Ba thông tin cần có khi kết nối 2 máy

```text
Màn hình Người phát ──> CrocStream ──> Trạm Relay ──> CrocStream của Người xem
```

1. **Địa chỉ relay:** Địa chỉ chương trình chuyển tiếp (`crocstream-relay.exe`). Chạy thử trên cùng 1 máy dùng `127.0.0.1:9009`; chạy 2 máy cùng Wi-Fi/LAN dùng IP của máy bật relay (ví dụ `192.168.1.20:9009`).
2. **Mật khẩu relay:** Mật khẩu tự đặt (tối thiểu 12 ký tự) khi bật relay; cả hai máy nhập giống nhau.
3. **Mã mời (`cs1-...`):** Sinh ra sau khi Người phát bấm **Bắt đầu chia sẻ**. Người phát bấm **Sao chép mã** gửi cho Người xem dán vào ô **Mã phiên**. Khi bấm dừng, mã này lập tức bị hủy.

---

## 3. Cài đặt, Build và Chạy nhanh

**Yêu cầu:** Windows 10/11 x64 (có sẵn Microsoft Edge WebView2). Nếu build từ mã nguồn cần PowerShell 5.1+, Node.js `24.22.1`, npm `12.2.1` (Go `1.27.4` và Wails `v3.1.0` có thể cài tự động bằng lệnh số 1 bên dưới).

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

| Mục đích | Lệnh PowerShell (chạy từ thư mục gốc `D:\CrocStream`) |
| --- | --- |
| **Chạy Demo nhanh trên 1 máy** | `. .\scripts\toolchain\Use-Toolchain.ps1` → `go -C app build -o .\packaging\crocstream-relay.exe ./cmd/demo-relay` → `.\packaging\Start-DemoRelay.ps1` (rồi mở 2 cửa sổ `crocstream.exe`) |
| **Tạo gói Portable ZIP hoàn chỉnh** | `.\packaging\Build-Portable.ps1 -OutputDirectory release/demo-build` |
| **Chạy toàn bộ kiểm thử tự động** | `powershell -NoProfile -File .\scripts\verify.ps1 -Race` |
| **Dọn sạch thư mục nháp sau khi build/test** | `powershell -NoProfile -File .\scripts\clean.ps1` |
