# Hướng dẫn chạy G0 với AWS EC2 Windows

Mục tiêu:

```text
Laptop của bạn ── Internet ── EC2 relay croc ── EC2 viewer
```

Đây là một phiên kiểm thử giữa **hai máy Windows khác nhau**. Relay chạy trên
EC2 cùng máy với viewer; vì vậy nó chứng minh đường laptop → relay từ xa →
peer khác máy. Nó không phải bằng chứng về một relay công cộng độc lập.

Nếu đã bật SSH và đối chiếu fingerprint qua RDP, nên dùng
[`Invoke-SshProof.ps1`](README.md#automated-ssh-proof) để tự điều phối, kiểm tra
hash và dọn process/firewall. Chế độ đó dùng đúng relay croc trong PoC trên một
cổng 9009, không cần relay CLI hoặc thao tác copy invitation bằng tay.
Các bước thủ công bên dưới là lựa chọn khác; không chạy cùng lúc hai relay
trên cổng 9009.

## 0. Chuẩn bị

- Tài khoản AWS có quyền tạo EC2, security group và key pair.
- Laptop Windows hiện tại.
- Khoảng 20 phút.
- Không gửi AWS private key, relay password hoặc CrocStream invitation vào chat.

Windows EC2 bị tính phí theo thời gian chạy; `t3.medium` có 4 GiB RAM và phù hợp
hơn cho Windows + PoC race-enabled. Có thể dùng `t3.small` để giảm chi phí nhưng
hãy theo dõi tài nguyên. Sau bài test, **Terminate** instance nếu không cần giữ
lại; Stop vẫn có thể còn phí EBS/public IPv4.

## 1. Tạo EC2 Windows

1. Mở <https://console.aws.amazon.com/ec2/> và chọn region gần bạn.
2. Chọn **Instances → Launch instances**.
3. Đặt tên: `crocstream-g0-remote`.
4. AMI: **Windows Server 2022 Base** x86_64.
5. Instance type: `t3.medium`.
6. Tạo key pair mới:
   - Name: `crocstream-g0-key`
   - Type: RSA
   - Format: `.pem`
   - Tải file xuống và giữ riêng; AWS chỉ cho tải một lần.
7. Network:
   - VPC/subnet mặc định là đủ.
   - Bật **Auto-assign public IP**.
   - Tạo security group mới.
8. Inbound rules:
   - TCP `3389`, source **My IP** — để RDP.
   - TCP `9009-9013`, source **My IP** — để laptop vào relay.
   - Nếu public IP laptop thay đổi, cập nhật source thành IP mới; không cần mở
     relay cho `0.0.0.0/0`.
9. Storage: root gp3 khoảng 30 GiB là đủ.
10. Launch instance và đợi trạng thái **Running**, đồng thời 2/2 status checks
    thành công.

## 2. Kết nối RDP

1. Chọn instance → **Connect → RDP client**.
2. Chọn **Get password**.
3. Chọn file `.pem` đã tải → **Decrypt password**.
4. Tải file `.rdp`.
5. Mở file `.rdp`, đăng nhập:
   - User: `Administrator`
   - Password: mật khẩu AWS vừa giải mã.
6. Khi RDP hỏi chứng chỉ, xác nhận fingerprint nếu đúng instance.

Nếu không kết nối được, kiểm tra security group TCP `3389` đang cho đúng public
IP hiện tại của laptop.

## 3. Build binary trên laptop

Chạy tại `D:\CrocStream`:

```powershell
. .\app\g0\Use-Toolchain.ps1

go -C croc build -buildvcs=false `
  -o "$env:GOBIN\croc-relay.exe" .

go -C app/g0/poc build -race -buildvcs=false `
  -o "$env:GOBIN\g0poc-verified.exe" ./cmd/g0poc

Get-FileHash "$env:GOBIN\croc-relay.exe" -Algorithm SHA256
Get-FileHash "$env:GOBIN\g0poc-verified.exe" -Algorithm SHA256
```

Hai file cần chuyển sang EC2:

- `croc-relay.exe`
- `g0poc-verified.exe`
- `app/g0/poc/Run-RemotePeer.ps1`

## 4. Chuyển file qua RDP

Trước khi mở RDP, trong cửa sổ Remote Desktop:

1. **Show Options → Local Resources → More**.
2. Chọn drive chứa repo, thường là `D:` và drive chứa binary tạm, thường là `C:`.
3. Kết nối lại RDP.

Trong PowerShell trên EC2:

```powershell
New-Item -ItemType Directory -Force C:\g0 | Out-Null

# Điều chỉnh D/C nếu drive mapping của RDP khác.
Copy-Item '\\tsclient\D\CrocStream\app\g0\poc\Run-RemotePeer.ps1' C:\g0\
Copy-Item '\\tsclient\C\Users\Admin\AppData\Local\Temp\opencode\crocstream-g0\bin\croc-relay.exe' C:\g0\
Copy-Item '\\tsclient\C\Users\Admin\AppData\Local\Temp\opencode\crocstream-g0\bin\g0poc-verified.exe' C:\g0\

Unblock-File C:\g0\*.exe
Get-ChildItem C:\g0
```

Nếu máy local không phải `D:\CrocStream` hoặc user không phải `Admin`, sửa
đường dẫn nguồn tương ứng.

## 5. Mở port relay trên Windows Firewall

PowerShell RDP phải chạy **Run as Administrator**:

```powershell
New-NetFirewallRule `
  -DisplayName 'CrocStream G0 relay 9009-9013' `
  -Direction Inbound `
  -Action Allow `
  -Protocol TCP `
  -LocalPort 9009-9013 `
  -Profile Any
```

AWS security group và Windows Firewall đều phải cho phép port. Thiếu một trong
hai thì laptop sẽ không vào được relay.

## 6. Chạy relay trên EC2

Mở một cửa sổ PowerShell riêng trên EC2. Nhập password bằng cách ẩn:

```powershell
$secure = Read-Host 'Relay password' -AsSecureString
$ptr = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($secure)
try {
    $env:CROC_PASS = [Runtime.InteropServices.Marshal]::PtrToStringBSTR($ptr)
} finally {
    [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($ptr)
}

cd C:\g0
.\croc-relay.exe relay `
  --host 0.0.0.0 `
  --ports 9009,9010,9011,9012,9013
```

Giữ cửa sổ này mở. Không dùng `--debug`, không chụp log chứa mã phiên.

## 7. Chạy Host trên laptop

Trên laptop, tìm public IPv4 của EC2 trong EC2 console. Tại
`D:\CrocStream`, mở PowerShell:

```powershell
. .\app\g0\Use-Toolchain.ps1
powershell -NoProfile -ExecutionPolicy Bypass `
  -File .\app\g0\poc\Run-RemotePeer.ps1 `
  -Mode host `
  -Executable "$env:GOBIN\g0poc-verified.exe" `
  -RelayAddress 'EC2_PUBLIC_IPV4:9009' `
  -DurationMinutes 10
```

Script sẽ hỏi relay password ở dạng ẩn. Sau khi Host vào relay, nó in một
**private invitation**. Copy invitation đó qua cửa sổ RDP trên EC2; không gửi
vào chat và không lưu vào file.

## 8. Chạy Viewer trên EC2

Trong cửa sổ PowerShell thứ hai trên EC2:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass `
  -File C:\g0\Run-RemotePeer.ps1 `
  -Mode viewer `
  -Executable C:\g0\g0poc-verified.exe `
  -DurationMinutes 10
```

Viewer sẽ hỏi:

1. Invitation — dán mã Host vừa in.
2. Relay password — phải giống password khi chạy relay.

Viewer dùng `127.0.0.1:9009` vì relay chạy cùng EC2. Host dùng public IPv4 của
EC2. Cả hai phải chạy trong cùng cửa sổ thời gian; invitation có thời gian chờ
giới hạn. Timeout hiện bắt đầu trước xác thực và bằng thời lượng bài thử cộng
45 giây; cần chuẩn bị cửa sổ Viewer trước khi chạy Host để không tốn thời gian
chép tay. Nếu mất quá lâu, dừng cả hai peer và bắt đầu phiên mới.

## 9. Kết quả cần lưu

Mỗi peer sẽ in JSON cuối phiên. Kết quả đạt cần có:

- `LOCAL_PASS` không được dùng cho remote; phiên này cần ghi route là remote
  trong báo cáo thủ công.
- `bytes_sent` và `bytes_received` đối ứng giữa Host/Viewer.
- SHA-256 nhận được khác nhau theo hướng; dùng `-mode verify` như trong
  [README](README.md#independently-verify-remote-peer-results) để kiểm tra độc lập
  hai JSON kết quả. Chạy hai peer thủ công không có parent chung tự kiểm tra.
- Phiên hoạt động khoảng 600 giây.
- `pings_sent == pongs_received`.
- `key_requests_received` đối ứng.
- Không có thông báo race/error.

Chụp lại **chỉ JSON kết quả**, che `Invite`, relay password, public IP nếu không
cần chia sẻ. Gửi kết quả JSON đã che để cập nhật `G0_LOCAL_PROOF.md`.

## 10. Dọn tài nguyên sau test

Trên EC2:

```powershell
Remove-NetFirewallRule -DisplayName 'CrocStream G0 relay 9009-9013'
```

Dừng relay bằng `Ctrl+C`. Trong AWS console:

1. Chọn instance.
2. **Instance state → Terminate instance** nếu không cần dùng lại.
3. Kiểm tra **Billing/Cost Explorer** để chắc không còn instance chạy.

Không gửi `.pem`, mật khẩu Windows, relay password hoặc invitation vào
repository/chat.
