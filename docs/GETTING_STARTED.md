# Bắt đầu với CrocStream

Một người chia sẻ màn hình, một người xem. Cả hai mở CrocStream trên Windows
10/11 x64. Máy chạy cần Microsoft Edge WebView2 Evergreen Runtime.

## Thử trong vài phút

1. Nhận ZIP và mã kiểm tra từ người tổ chức, xác minh rồi giải nén toàn bộ gói.
   [Kiểm tra ZIP](../app/g6/DEMO.md#kiểm-tra-zip-nhận-được)
2. Mở `crocstream.exe`, chọn **Xem trước trên máy này**, chọn màn hình và bấm
   **Bắt đầu preview**. Bấm **Dừng preview** khi xong.
3. Thử Share/Watch bằng hai cửa sổ trên cùng máy hoặc hai máy trong LAN.
   [Các bước demo](../app/g6/DEMO.md) · [Tạo ZIP cho buổi demo](../app/README.md#g6-windows-portable-and-demo)

## Ba thông tin khi kết nối

| Thông tin | Dùng thế nào |
| --- | --- |
| **Địa chỉ relay** | Cả hai nhập địa chỉ của chương trình chuyển tiếp. Cùng một máy dùng `127.0.0.1:9009`. |
| **Mật khẩu relay** | Cả hai nhập cùng mật khẩu do người chạy relay đặt. |
| **Mã mời** | Người phát bấm **Sao chép mã** và gửi riêng. Người xem dán vào ô **Mã phiên**. Dừng phiên sẽ thu hồi mã. |

```text
Màn hình người phát → CrocStream → relay → CrocStream của người xem
```

Hai ứng dụng xác thực và mã hóa phiên. **KEY** là ảnh đầy đủ, **DELTA** chứa
phần thay đổi. **FPS hiển thị** là số hình vẽ mỗi giây và **RTT** là thời gian
khứ hồi giữa hai ứng dụng.

[Cẩm nang toàn diện & Bảo vệ đồ án](../PROJECT_GUIDE.md) · [Tìm hiểu giới hạn demo](../app/g6/KNOWN_ISSUES.md) · [Hồ sơ nghiệm thu](gates/README.md) · [Trang chủ](../README.md)
