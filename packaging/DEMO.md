# Chạy demo CrocStream trên Windows

Dùng ZIP từ người tổ chức, Windows x64 và WebView2 Evergreen Runtime. Có thể
demo bằng hai cửa sổ trên một máy hoặc hai máy trong mạng LAN tin cậy.
[Gói portable](README.md) · [Giới hạn](KNOWN_ISSUES.md)

## Mở ứng dụng và xem trước

1. [Kiểm tra ZIP](#kiểm-tra-zip-nhận-được), giải nén toàn bộ gói vào thư mục có quyền ghi.
2. Mở `crocstream.exe` rồi chọn **Xem trước trên máy này** và màn hình cần chụp.
3. Bấm **Bắt đầu preview**. Hình hiển thị cục bộ. Bấm **Dừng preview** khi xong.

## Chuẩn bị relay

Chạy một relay cho cả hai ứng dụng. Trong File Explorer, mở thư mục chứa
`crocstream.exe`, gõ `powershell` vào thanh địa chỉ rồi nhấn Enter.

```powershell
.\Start-DemoRelay.ps1
```

Cùng một máy dùng `127.0.0.1:9009`. Nếu chạy trên hai máy, dùng lệnh dưới đây
thay cho lệnh trên và thay địa chỉ mẫu bằng IPv4 riêng của máy chạy relay.

```powershell
.\Start-DemoRelay.ps1 -ListenAddress '192.168.1.20:9009'
```

Qua LAN, cả hai ứng dụng dùng địa chỉ riêng đã chọn. Đặt mật khẩu relay ít nhất 12 ký tự khi
được hỏi và giữ cửa sổ relay mở. Nếu cần mở TCP 9009, quản trị viên chỉ cấp
quyền cho máy tham gia trên cấu hình **Private**. Giữ tường lửa bật và relay
trong mạng riêng, không chuyển tiếp cổng ra Internet. Nếu PowerShell chặn script, làm theo chính sách của đơn vị.

## Chia sẻ, tham gia và dừng

1. **Người phát** chọn **Chia sẻ màn hình**, nhập địa chỉ và mật khẩu relay, chọn
   màn hình rồi bấm **Bắt đầu chia sẻ**. Khi thấy **Đang chờ người xem**, bấm
   **Sao chép mã** và gửi mã riêng.
2. **Người xem** mở cửa sổ CrocStream thứ hai và chọn **Tham gia bằng mã**. Nhập
   cùng địa chỉ và mật khẩu relay. Dán mã vào **Mã phiên** rồi bấm **Tham gia
   phiên**. Khi thấy **Đang xem**, ảnh đã nhận qua phiên xác thực.
3. **Kết thúc** bằng **Dừng chia sẻ** hoặc **Rời phiên**. Về Home trước khi đổi
   vai trò. Đóng hai cửa sổ app rồi nhấn Enter trong cửa sổ relay.

Khi kẹt ở bước kết nối, đối chiếu địa chỉ, mật khẩu relay và mã mời của phiên
đang mở. Dừng phiên thu hồi mã. **Chi tiết luồng** hiển thị số đo của phiên hiện tại.

## Kiểm tra ZIP nhận được

Mở PowerShell trong thư mục chứa ZIP và chạy lệnh sau.

```powershell
Get-FileHash .\CrocStream-0.0.1-windows-amd64.zip -Algorithm SHA256
```

So cột `Hash` với mã do người tổ chức cung cấp qua kênh tin cậy. Khi mã khớp,
giải nén toàn bộ ZIP rồi mở PowerShell tại thư mục chứa `crocstream.exe`.

```powershell
.\Verify-Package.ps1 -Directory .
```

Dừng nếu nguồn hoặc mã kiểm tra khác dự kiến. Giữ Defender và SmartScreen bật.
