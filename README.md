# CrocStream

**CrocStream** là ứng dụng desktop chia sẻ màn hình và cửa sổ ứng dụng **1-1** trên Windows 10/11 x64, xây dựng trên nền tảng mật mã và giao thức phòng chờ của [`schollz/croc`](https://github.com/schollz/croc) (ghim nguyên bản tại commit [`552f40fd2d75`](UPSTREAM.md)).

> **Bạn cần đọc file nào?**
> - **Thành viên nhóm & Người mới tìm hiểu (Đọc trong 3–5 phút):** Xem **[PROJECT_GUIDE.md](PROJECT_GUIDE.md)**.
> - **Hướng dẫn bấm chạy Demo từng bước:** Xem **[packaging/DEMO.md](packaging/DEMO.md)**.
> - *(Hồ sơ kỹ thuật chuyên sâu và biên bản nghiệm thu cũ nằm trong [`docs/specs/`](docs/specs/) và [`docs/archive-gates/`](docs/archive-gates/)).*

---

## 1. Tính năng nổi bật

- **Chọn toàn màn hình hoặc từng cửa sổ ứng dụng** (Chrome, Zalo, VS Code, PowerPoint… kể cả cửa sổ đang thu nhỏ dưới thanh tác vụ), tự động cập nhật danh sách cửa sổ và **xem trước trực tiếp** trước khi phát.
- **Tự ưu tiên LAN trực tiếp → tự động chuyển Relay khi khác mạng:** Khi hai máy ở cùng mạng LAN/Wi-Fi, ứng dụng tự khám phá nội bộ (UDP `9019`, chỉ gửi tag băm 16 byte, không lộ mật khẩu/mã mời) để nối thẳng độ trễ thấp; khi khác mạng hoặc bị tường lửa chặn LAN, tự động chuyển qua trạm Relay (LAN hoặc VPS) mà không cần chỉnh cấu hình.
- **Nén Tile-Delta không mất dữ liệu (LZ4):** Chia khung hình thành các ô `32×32`, chỉ gửi ô thay đổi, tối ưu FPS, tự phục hồi lỗi (`PLI`) và hiển thị số đo thực tế (`FPS`, `KiB/s`, `RTT`, `Đường truyền`, `% tiết kiệm`).
- **Tự điền Relay qua `relay.json`:** Đặt file `relay.json` (bị `.gitignore` bỏ qua, không commit lên Git) cạnh `crocstream.exe` để tự điền sẵn địa chỉ & mật khẩu Relay.

---

## 2. Cách tải và chạy nhanh (Windows 10/11 x64)

**Yêu cầu:** Windows 10/11 x64 có sẵn Microsoft Edge WebView2 (mặc định có trên Windows 10/11). Không cần cài Go, Node.js hay chạy lệnh terminal.

1. Tải file `CrocStream-*-windows-amd64.zip` mới nhất tại **[GitHub Releases](https://github.com/hungdo2006/CrocStream/releases)**.
2. Giải nén toàn bộ vào một thư mục có quyền ghi và mở **`crocstream.exe`** *(nếu Windows SmartScreen cảnh báo do bản thử nghiệm chưa ký số, chọn **More info → Run anyway**)*.
3. Chọn **Chia sẻ màn hình** (lấy mã mời `cs1-...`) hoặc **Tham gia bằng mã** để kết nối. Xem thêm hướng dẫn từng bước tại **[packaging/DEMO.md](packaging/DEMO.md)**.

---

## 3. Kết nối 2 máy & cấu hình `relay.json`

```text
Cùng LAN/Wi-Fi :  Người phát ──(LAN trực tiếp · PAKE + AES-256-GCM)──> Người xem
Khác mạng      :  Người phát ──> Trạm Relay TCP (PAKE + AES-256-GCM) ──> Người xem
```

1. **Địa chỉ & mật khẩu Relay:** Nếu gói tải về đã kèm sẵn `relay.json` cạnh `crocstream.exe`, ứng dụng sẽ tự điền sẵn khi mở. Khi muốn đổi sang trạm Relay khác, bạn có thể sửa trực tiếp trên giao diện hoặc sửa file `relay.json`:
   ```json
   {
     "address": "198.51.100.10:9009",
     "password": "mat-khau-relay-tu-dat"
   }
   ```
   *(Muốn demo nội bộ không cần VPS, có thể chạy `.\Start-DemoRelay.ps1` có sẵn trong gói Portable ZIP.)*
2. **Mã mời (`cs1-...`):** Sinh ra sau khi Người phát bấm **Bắt đầu chia sẻ**. Bấm **Sao chép mã** gửi cho Người xem dán vào ô **Mã phiên**. Khi bấm dừng, mã lập tức bị thu hồi.

---

## 4. Bố cục thư mục dự án

| Thư mục | Vai trò | Ghi chú |
| --- | --- | --- |
| **`app/`** | **Phần mềm chính CrocStream** (`frontend/`: giao diện React, `internal/`: backend Go, `cmd/demo-relay/`: trạm relay cục bộ). | Nơi chứa toàn bộ mã nguồn ứng dụng. |
| **`packaging/`** | Script đóng gói **Portable ZIP**, script bật relay cục bộ (`Start-DemoRelay.ps1`) và hướng dẫn Demo (`DEMO.md`). | Dùng khi đóng gói hoặc chạy Demo. |
| **`scripts/`** | Công cụ tự động: kiểm thử toàn diện (`verify.ps1`), dọn file tạm (`clean.ps1`), cài Go/Wails (`toolchain/`). | Dùng khi build/test từ mã nguồn. |
| **`docs/`** | Đặc tả kỹ thuật (`specs/`) và biên bản nghiệm thu `G0–G6` (`archive-gates/`). | Hồ sơ kỹ thuật để tra cứu/nộp kèm. |
| **`croc/`** | Mã nguồn `schollz/croc` gốc giữ nguyên vẹn 100% làm thư viện nền và đối chứng. | **Không chỉnh sửa.** |

*(Dành cho lập trình viên muốn tự biên dịch, chạy kiểm thử hoặc đóng gói từ mã nguồn: xem hướng dẫn đầy đủ tại **[app/README.md](app/README.md)**.)*
