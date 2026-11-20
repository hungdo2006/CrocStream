# CrocStream application core

Module `crocstream.local/app` chứa ứng dụng Windows, codec và test. Hai module
G0 trong `g0/poc` và `g0/buildprobe` có bước kiểm tra riêng.
[Hợp đồng media v1](../docs/ARCHITECTURE.md#media-protocol-v1) · [Hồ sơ G0–G6](../docs/gates/README.md) · [Giấy phép](../docs/gates/notices.md)

## Run checks

Từ gốc repo, chạy bộ kiểm tra Go, Wails và frontend.

```powershell
powershell -NoProfile -File .\scripts\verify.ps1 -Race
```

Go test riêng và các mục tiêu fuzz có giới hạn thời gian.

```powershell
. .\app\g0\Use-Toolchain.ps1
go -C app test -count=1 -timeout=180s ./...
go -C app vet ./...
foreach ($target in @('FuzzDecodeHeader', 'FuzzDecodeConfig', 'FuzzDecodePacket', 'FuzzReadStream')) {
    go -C app test ./internal/protocol -run '^$' -fuzz "^${target}$" -fuzztime=30s -parallel=2
    if ($LASTEXITCODE -ne 0) { throw "Fuzz failed: $target" }
}
foreach ($target in @('FuzzDecodeDelta', 'FuzzCompressedFrame', 'FuzzDecoderAtomicity')) {
    go -C app test ./internal/delta -run '^$' -fuzz "^${target}$" -fuzztime=30s -fuzzminimizetime=1s -parallel=2
    if ($LASTEXITCODE -ne 0) { throw "Fuzz failed: $target" }
}
go -C app test ./internal/delta -run '^$' -bench '^BenchmarkCodec$' -benchmem -benchtime=2s -count=1
```

Benchmark trên ảnh tổng hợp gồm encode, baseline và decode. Phạm vi kiểm tra,
fixture và phương pháp đo nằm trong [kế hoạch kiểm thử](../docs/TEST_PLAN.md).

## Protocol API and ownership

- `EncodeHeader` / `DecodeHeader` xử lý header BigEndian 18 byte. `Header.Validate`
  kiểm tra ràng buộc gói. `Config.Limits` trả ngân sách ảnh và tile-record.
- `DecodePacket` mượn payload từ bộ đệm đầu vào. `ReadPacket` trả payload thuộc
  caller và xác minh gói trước khi cấp phát theo giới hạn.
- Quyết định `ResolveVideo` sau khi codec kiểm tra video. Khi nhận
  `ErrNeedKeyframe`, giữ khung hợp lệ cuối cùng và yêu cầu KEY theo giới hạn PLI.
  Kết thúc phiên khi lỗi framing/I/O hoặc cạn frame ID.
- `WritePacket` chạy đồng bộ với một writer duy nhất và mượn payload đến khi
  trả kết quả. Phiên sở hữu socket, deadline, hủy và xử lý END.

`FromBroadcaster` chỉ chiều do người phát gửi. CONFIG đứng trước video, với
END cho phép hủy sớm. Video đầu tiên là KEY1. Chiều ngược mang control.
[Framing và thứ tự gói](../docs/ARCHITECTURE.md#media-protocol-v1) · [G1 closeout](../docs/gates/reports/G1_CLOSEOUT.md)

## G2 frame and codec API

1. `frame.Normalize` nhận RGBA/BGRA thẳng theo thứ tự từ trên xuống. Hàm kiểm
   tra stride, chép hàng, bỏ phần đệm và đặt alpha 255. Caller sở hữu ảnh trả về.
2. Tạo encoder và decoder từ CONFIG cho từng phiên. `Emit` so các tile với ảnh
   tham chiếu đã gửi và chỉ cập nhật ảnh/ID sau khi ghi thành công. Bỏ mẫu,
   nếu cần, trước khi mã hóa.
3. `Decode` xác minh một LZ4 frame và toàn bộ tile record trước khi cập nhật
   ảnh. Gọi `ResolveVideo(err == nil)`. Khi reader cần KEY, gọi thêm
   `decoder.Invalidate()`. `Snapshot()` trả bản sao ảnh.
4. Encoder/decoder chạy đồng bộ với một chủ sở hữu. `PacketWriter` tiêu thụ
   payload trong lời gọi. Lỗi ghi kết thúc encoder và phiên.

[Định dạng LZ4 và tile](../docs/ARCHITECTURE.md#media-protocol-v1) · [G2 closeout](../docs/gates/reports/G2_CLOSEOUT.md)

## G2 telemetry API

Ghi sự kiện `Emitted` sau khi gửi thành công, `Received` khi nhận đủ gói trước
khi giải mã và `Decoded` khi giải mã thành công. Các loại FPS đo riêng.
`Snapshot` dùng cửa sổ `(now-1s, now]`. Tốc độ gồm header của KEY/DELTA.
Baseline dùng ảnh gốc cùng mẫu. Cờ known và complete quyết định lúc hiển thị
giá trị. Tiết kiệm có thể âm. RTT và độ trễ hiển thị cần phép đo riêng.
[Hợp đồng telemetry](../docs/ARCHITECTURE.md#telemetry-contract)

## G3 desktop preview — build and run

Từ `app/`, nạp toolchain portable và build. Wails cài dependency theo lockfile,
sinh binding, build frontend và tạo EXE Windows.

```powershell
. .\g0\Use-Toolchain.ps1
$env:WAILS_EXP_USE_WAKE='1'
wails3 build
.\bin\crocstream.exe
```

Trong cùng môi trường, chạy dev bằng `wails3 dev -host 127.0.0.1 -port 9245`.
Cổng cố định thuộc cấu hình kiểm tra Origin. Dùng Ctrl+C để dừng watcher.

Các hook phiên, canvas và danh sách màn hình dùng chung giữa Preview và
Share/Watch. Stop đợi Start đang chờ. Hàng đợi hiển thị giữ JPEG mới nhất và
giải phóng bitmap sau khi dùng. Bridge bind loopback và kiểm chính xác Host,
Origin cùng token qua WebSocket subprotocol. Origin production là
`http://wails.localhost`, dev dùng cổng 9245.
[Sơ đồ code](../PROJECT_GUIDE.md#c-kiến-trúc-dự-án) · [G3 proof](g3/README.md)

Sau khi sinh binding, chạy `npm run typecheck`, `npm run lint`, `npm run test`,
`npm run test:ui` hoặc `npm run build` từ `frontend/`. Playwright dùng Edge
và ảnh tổng hợp. Bài kiểm chứng WebView2 native có hướng dẫn riêng.

## G4 Share/Watch — local desktop and remote synthetic proof

Người phát tạo mã mời sau khi relay sẵn sàng. Hai peer xác thực trước media.
Stop/Leave đóng capture, bridge và transport của phiên. Nếu hết thời gian chờ,
UI giữ `Stopping` đến khi dọn xong. Nếu giải phóng capture thất bại, UI báo lỗi
đã được lọc.
[Test controller](internal/session/controller_test.go) · [Chạy proof loopback](g4/README.md)

Bằng chứng hai máy qua EC2 dùng ảnh tổng hợp. Các số đo phiên desktop thật được
ghi trong bài kiểm thử loopback riêng. [G4 closeout](../docs/gates/reports/G4_CLOSEOUT.md) · [Báo cáo remote](../docs/gates/reports/G4_REMOTE_PROOF.md)

## G5 UX and measured dashboard

**Chi tiết luồng** phân biệt capture, gửi, nhận, giải mã và hiển thị. Giữ dấu
thiếu dữ liệu khi bộ đếm chưa đo được. Nối lại hiển thị cục bộ giữ phiên
media đang xác thực. [Số đo và stress test](../docs/gates/reports/G5_CLOSEOUT.md) · [Tái lập](../docs/gates/g5/README.md)

## G6 Windows portable and demo

Từ `app/`, nạp toolchain và đóng ZIP vào thư mục đích mới.

```powershell
. .\g0\Use-Toolchain.ps1
.\g6\Build-Portable.ps1 -OutputDirectory release/new-build
```

ZIP Windows x64 chưa ký số cần WebView2 Evergreen Runtime trên máy đích.
Gói có manifest, checksum và thông báo giấy phép.
[Demo](g6/DEMO.md) · [Giới hạn](g6/KNOWN_ISSUES.md) · [G6 closeout](../docs/gates/reports/G6_CLOSEOUT.md)

Kiểm tra gói đã giải nén từ `app/`. Bài UI production chạy loopback, cần đồng ý
chụp màn hình thật và clipboard trống.

```powershell
$packageDirectory = 'C:\duong-dan\thu-muc-da-giai-nen'
.\g6\Smoke-Package.ps1 -Directory $packageDirectory
.\g6\Test-ProductionUI.ps1 -Directory $packageDirectory
```
