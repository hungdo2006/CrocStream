# CrocStream 0.0.4 · Windows portable

Bản thử nghiệm này chạy trên Windows x64 với Microsoft Edge WebView2 Evergreen
Runtime. Giải nén toàn bộ ZIP vào thư mục có quyền ghi, rồi mở `crocstream.exe`.
Gói chưa ký số. Kiểm tra nguồn và checksum trước khi mở.
[Hướng dẫn demo](DEMO.md) · [Giới hạn](KNOWN_ISSUES.md) · [WebView2 của Microsoft](https://developer.microsoft.com/microsoft-edge/webview2/)

## Trong gói

- `crocstream.exe` dùng cho Preview và Share/Watch.
- `crocstream-relay.exe` và `Start-DemoRelay.ps1` dùng cho relay loopback hoặc LAN riêng.
- `relay.json` *(tùy chọn, nếu đi kèm gói)* tự điền sẵn địa chỉ và mật khẩu relay khi mở ứng dụng.
- `Verify-Package.ps1` và `manifest.json` kiểm tra nội dung gói.
- `dependencies.json`, `build-info.txt`, `licenses/` và `NOTICE.txt` ghi thông tin build và giấy phép.

Ứng dụng bắt đầu chụp sau thao tác Start. WebView2 sử dụng dữ liệu hồ sơ và
cache thông thường. Để gỡ bản portable, dừng phiên và relay, đóng ứng dụng rồi
xóa thư mục đã giải nén. Giữ thông báo giấy phép khi sao chép EXE.

Người bảo trì có thể chạy `packaging/Test-ProductionUI.ps1 -Directory <thư-mục>`
từ repo. Bài này cần đồng ý chụp màn hình cục bộ và clipboard trống để sao chép
mã mời trong phiên loopback.
