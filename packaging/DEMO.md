# Chạy demo CrocStream trên Windows

Dùng bản Portable ZIP (hoặc `app\bin\crocstream.exe`), Windows 10/11 x64 và WebView2 Evergreen Runtime. Hỗ trợ demo 1 máy, 2 máy cùng LAN/Wi-Fi (tự ưu tiên **LAN trực tiếp**) hoặc 2 máy khác mạng qua Relay VPS.
[Gói portable](README.md) · [Giới hạn](KNOWN_ISSUES.md)

## 1. Chuẩn bị Relay (chọn 1 trong 2 cách)

### Cách A — Dùng sẵn file `relay.json` (nhanh nhất, tiện cho VPS hoặc LAN cố định)
Tạo file `relay.json` đặt **cùng thư mục với `crocstream.exe`** ở cả hai máy:

```json
{
  "address": "198.51.100.10:9009",
  "password": "mat-khau-relay-tu-dat"
}
```

Khi mở ứng dụng, 2 ô Relay sẽ tự điền sẵn. Nếu hai máy ở cùng mạng LAN/Wi-Fi, ứng dụng vẫn tự ưu tiên nối thẳng **LAN trực tiếp** (cần bấm **Allow** trên Windows Firewall cho mạng **Private** ở lần chạy đầu); nếu khác mạng sẽ tự động chuyển qua Relay trong `relay.json`.

### Cách B — Tự bật Relay cục bộ không cần VPS/Internet
Trong thư mục chứa `crocstream.exe`, mở PowerShell và chạy:

```powershell
# Chạy 2 cửa sổ trên cùng 1 máy (dùng 127.0.0.1:9009)
.\Start-DemoRelay.ps1

# Hoặc chạy 2 máy cùng Wi-Fi/LAN (thay bằng IPv4 LAN của máy bật relay)
.\Start-DemoRelay.ps1 -ListenAddress '192.168.1.20:9009'
```

Đặt mật khẩu relay tối thiểu 12 ký tự khi được hỏi và giữ cửa sổ relay mở trong lúc demo.

## 2. Chia sẻ, tham gia và dừng

1. **Người phát:** Mở `crocstream.exe` → chọn **Chia sẻ màn hình** → chọn tab **Màn hình** hoặc **Cửa sổ** (khung 16:9 tự hiển thị xem trước nguồn đang chọn; lưu ý cửa sổ đang thu nhỏ sẽ không hiện trong danh sách) → bấm **Bắt đầu chia sẻ**. Khi thấy **Đang chờ người xem**, bấm **Sao chép mã** gửi cho người xem.
2. **Người xem:** Mở `crocstream.exe` → chọn **Tham gia bằng mã** → kiểm tra Relay (hoặc để tự điền từ `relay.json`) → dán mã vào **Mã phiên** → bấm **Tham gia phiên**.
3. **Quan sát số đo:** Thanh chỉ số bên dưới hiển thị trực tiếp **FPS hiển thị**, **Tốc độ video (KiB/s)**, **RTT khứ hồi (ms)** và **Đường truyền** (`LAN trực tiếp` hoặc `Relay`); bấm **Chi tiết luồng** để xem thêm `% tiết kiệm so với toàn ảnh`.
4. **Kết thúc:** Bấm **Dừng chia sẻ** hoặc **Rời phiên** để xóa khung hình và thu hồi mã mời ngay lập tức.

*(Muốn kiểm tra nguồn chụp mà không cần mạng, chọn **Xem trước trên máy này** ở màn hình chính.)*

## 3. Kiểm tra ZIP nhận được

```powershell
Get-FileHash .\CrocStream-0.0.1-windows-amd64.zip -Algorithm SHA256
```

Sau khi giải nén vào thư mục có quyền ghi, kiểm tra tính toàn vẹn gói (trước khi thêm `relay.json` riêng):

```powershell
.\Verify-Package.ps1 -Directory .
```
