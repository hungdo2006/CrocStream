# Kiến trúc CrocStream

## Luồng ứng dụng

- **Share.** Sau khi người dùng bấm Start, controller chuẩn bị relay và tạo mã
  mời. Capture GDI đọc màn hình đã chọn trên worker có khóa thread, giữ tỷ lệ
  hình và thêm viền đen khi cần. Khung RGBA đi vào hàng đợi một phần tử trước
  codec và một hàng đợi độc lập cho JPEG xem trước. `Live` chỉ xuất hiện sau
  xác thực hai chiều và ghi thành công KEY đầu tiên.
- **Watch.** Xác thực bằng mã mời, đọc CONFIG và chờ KEY hợp lệ. Khi gặp DELTA
  hỏng hoặc thiếu, giữ ảnh hợp lệ cuối và chờ KEY mới. Mỗi lần Join cách nhau ít
  nhất 2 giây, tối đa 5 lần/phút và người dùng chủ động thử lại.
- **Xem trước.** Chỉ chạy capture → JPEG → bridge → canvas. Ứng dụng không chụp
  trước khi có thao tác Start. Mất màn hình hoặc thay đổi kích thước nguồn sẽ
  kết thúc capture để người dùng chọn lại.

`desktop.Service` chỉ cho phép một phiên hoạt động. Wails trả endpoint, token
và generation từ `StartPreview`, `StartBroadcast` hoặc `JoinSession`.
`GetInvitation` trả mã mời riêng. UI đọc `GetStats` hoặc `GetSessionStats`
khoảng 500 ms/lần. Stop/Leave đợi Start đang chạy trước khi đóng phiên.
Khi thoát, backend hủy worker, đóng transport/bridge và chờ giải phóng tài
nguyên capture. Mốc chờ Stop thông thường là 3 giây. Nếu vượt mốc, giữ
`Stopping` tới khi tài nguyên được giải phóng.

## Transport và hiển thị

Backend dùng adapter `app/g0/poc/duplex` trên relay từ snapshot `croc/`. Adapter
dùng PAKE xác thực hai chiều và cung cấp `io.ReadWriteCloser` cho peer. Một
writer tuần tự hóa CONFIG, media và control mỗi chiều. Sau khi hủy phiên, kênh
truyền có tối đa 200 ms để thử gửi END trước khi buộc đóng I/O.
[Thiết kế adapter gốc](https://github.com/hungdo2006/CrocStream/blob/8e10002f7997bb1a12a8f2996d883610e1072b4d/app/G0_ADAPTER_DESIGN.md)

Bridge mỗi phiên bind `127.0.0.1` ở cổng tạm, dùng một token 256 bit và một
đường dẫn riêng (`/preview` cho local/share hoặc `/viewer` cho Watch).
Handshake kiểm tra Host, Origin chính xác (`http://wails.localhost` ở bản
đóng gói, thêm `:9245` ở dev) và token qua subprotocol
`crocstream.preview, auth.<token>`. Chỉ `crocstream.preview` được phản hồi.
Bridge nhận một WebSocket tại một thời điểm, JPEG tối đa 4 MiB, một ảnh chờ
và thời hạn ghi một giây. Token, mã mời, khóa và frame không đi vào log.
Ứng dụng không ghi frame ra đĩa.

Canvas giải mã một JPEG tại một thời điểm, chỉ giữ ảnh chờ mới nhất và đóng
`ImageBitmap` sau khi vẽ hoặc hủy. Đóng phiên hay đổi endpoint phải dọn socket,
bỏ decode cũ và xóa canvas. Người dùng có thể nối lại kênh hiển thị của
Share/Watch mà không khởi động lại codec mạng. Cả hàng đợi trước mã hóa và hàng
đợi hiển thị đều có thể bỏ ảnh cũ. Chuỗi DELTA qua mạng phải được xử lý tuần tự.

## Media protocol v1
The following wire contract applies inside the authenticated duplex stream.
Changing its byte layout requires a protocol revision and tests.

### Stream and header
One reliable ordered bidirectional byte stream per session, one receive parser and one serialized writer per endpoint. Reverse keyframe requests share that duplex stream.

Packet = exactly **18-byte header + PayloadLength bytes**, BigEndian, no padding. Use `io.ReadFull` for both. Mid-header/payload EOF is fatal; never scan for a later magic byte. Validate type, session/configuration and type-specific lengths **before allocation**. `MAX_PAYLOAD = 8,388,608` (8 MiB); separately cap decompression.

| Offset | Bytes | Field | Validation |
| --- | ---: | --- | --- |
| 0 | 1 | Magic uint8 | `0xC5` |
| 1 | 1 | Type uint8 | One of the types below |
| 2 | 4 | FrameID uint32 | 1-based video IDs; 0 for control/END |
| 6 | 8 | TimestampMs int64 | Broadcaster Unix ms for media; 0 for control/END; not a synchronized latency clock |
| 14 | 4 | PayloadLength uint32 | Exact following byte count, <=8 MiB plus type-specific bounds |

| Type | Direction | FrameID | Payload / rules |
| --- | --- | --- | --- |
| `KEYFRAME 0x01` (KEY) | broadcaster → viewer | >=1 | Independent LZ4 frame of full packed RGBA; first video always KEY id1 |
| `DELTA 0x02` | broadcaster → viewer | >=1 | Independent LZ4 frame of tile record; valid prior reference and sequential video ID required |
| `REQUEST_KEYFRAME 0x03` (PLI) | viewer → broadcaster | 0 | Empty; max 1/s, ignore excess; next emitted video must be KEY |
| `SESSION_CONFIG 0x04` (CONFIG) | broadcaster → viewer | 0 | Exactly 11 bytes; once immediately after handshake, before video; reconnect is a new session |
| `PING 0x05` | either | 0 | 8-byte arbitrary nonce |
| `PONG 0x06` | either | 0 | Promptly echo nonce; initiator measures monotonic RTT |
| `END 0xFF` (EOS) | either | 0 | Empty; best-effort delivery; also honor EOF/cancellation |

Unknown types are fatal in v1; no silent fallback.

The G1 API enforces CONFIG as the first broadcaster packet, with END allowed
for early abort; reverse controls require no reverse CONFIG. After CONFIG,
controls may precede the first KEY.

### SESSION_CONFIG
| Offset | Bytes | Field | Value |
| --- | ---: | --- | --- |
| 0 | 1 | ProtocolVersion uint8 | 1 |
| 1 | 2 | Width uint16 | 640 or 1280 |
| 3 | 2 | Height uint16 | 360 or 720 |
| 5 | 1 | TileSize uint8 | 32 |
| 6 | 1 | PixelFormat uint8 | 1 = packed RGBA8888, alpha=255 |
| 7 | 1 | Compression uint8 | 1 = LZ4 **frame**, not block API |
| 8 | 1 | FPS uint8 | 15, 24 or 30 desired; actual rate measured |
| 9 | 2 | KeyInterval uint16 | 1–120 emitted frames, default 30 (C-01 resolved) |

Reject unsupported values; no silent downscale/rescale. Only resolution pairs
**(640,360)** and **(1280,720)** are permitted; reject mixed pairs (C-02 resolved).
Checked `Width*Height*4 <= MAX_RAW = 3,686,400` (1280×720×4). Larger formats require a new protocol revision and bounds.

### Canonical image and compression
- Normalize native capture stride into contiguous row-major `R,G,B,A`, length `Width*Height*4`, alpha forced to 255 without premultiplied alpha. Any RGB color transformation must be deterministic and identical in tests. Resolution cannot change within a session.
- KEY compresses exact canonical RGBA into one self-contained LZ4 frame. Decoded length must equal `Width*Height*4`; reject overrun, underrun and trailing unsupported data. No JPEG/WebP network reference.
- DELTA compresses the entire record below as one self-contained LZ4 frame. Compression is independent per packet; reconstruction still needs the canonical reference.
- Skipped sub-threshold tiles leave sender reference unchanged. Compare new capture against **transmitted canonical reference**, not the last capture; equality is between peers, not necessarily raw capture when tau>0.

G2 uses standard LZ4 frames with Fast compression, independent 64-KiB blocks,
content size and block/content checksums, concurrency=1. Decoder accepts all
standard block sizes and dependent blocks within a self-contained frame.
Preflight requires exactly one standard frame: no legacy, skippable,
dictionary-dependent, concatenated or trailing data. Decode is bounded to the
configured raw cap plus one overshoot-detection byte; checksums and content size
are checked before committing pixels. This selects permitted LZ4-frame options;
it does not change media v1 header/config/tile bytes.

### DELTA record before compression
```text
uint16 dirtyCount (BE)
repeat dirtyCount times, strictly increasing tileIndex:
    uint16 tileIndex (BE)
    uint8 tileRGBA[effectiveTileWidth * effectiveTileHeight * 4]
```
`tileColumns=ceil(Width/32)`, `tileRows=ceil(Height/32)`, `tileIndex=tileY*tileColumns+tileX` (zero-based).
For each tile: `x=tileX*32`, `y=tileY*32`, `w=min(32,Width-x)`, `h=min(32,Height-y)`. Pack tile rows without a per-tile length field; config/index determine size. There are 920 tiles (40×23) at 1280×720 and 240 (20×12) at 640×360. Empty DELTA is two zero count bytes before compression.

- Require `dirtyCount<=tileCount`, valid strictly ascending unique indices, exact parsed length and no trailing bytes.
- Decompressed cap: `2 + 2*tileCount + Width*Height*4` with overflow checks; stop streaming decompression on overshoot.
- Tile RGB MAE = `sum(|a-b|)/(3*pixelCount)`, excluding alpha; dirty iff `MAE > tau`, default 4.
  G2 uses the equivalent exact integer comparison `sum > tau*3*pixelCount`;
  local encoder tau is integer 0–255. Both supported widths divide by 32, so
  v1 has partial-height bottom tiles only, including the bottom-right tile.
- If `dirtyCount/tileCount > 0.65`, emit lossless KEY. Optional cost-aware fallback may use KEY when encoded DELTA >= encoded KEY, only if both candidates were actually measured.
- After DELTA, update only transmitted dirty tiles; after KEY, replace the entire canonical reference. Commit reference/FrameID only after acceptance by the session writer. A network write failure terminates the session; never continue from uncertain state.

### Ordering and recovery
1. CONFIG then KEY id1 after authentication. Receiver has no reference until a valid KEY decodes.
2. KEY with `FrameID>lastVideoID` independently resets reference, including across ID gaps. Reject non-increasing IDs/replay within a session.
3. DELTA requires a reference and `FrameID=lastVideoID+1`. Validate the full record before mutation or use a staged buffer. On error/gap retain last good image/reference bytes, mark reference invalid, issue rate-limited PLI and await KEY.
4. At most one emitted video per encoded capture tick; only emitted frames acquire IDs. Drop/replace raw captures before encoding, never intermediate DELTA packets or decode operations. UI may skip display snapshots.
5. For interval 30, scheduled KEY IDs are `1,31,61,...`; PLI/dirty fallback may force earlier KEY. Near uint32 overflow terminate and establish a fresh authenticated session; never wrap to 0.
6. On close: best-effort END, cancel workers, close sockets/readers/writers, release screen/camera, clear in-memory codephrase/token. EOF, timeout or cryptographic failure means Disconnected, with no silent reuse of delta state.
7. Keep capture queues bounded to one or two samples. The current implementation queues one. The encoder maintains one canonical sequence and the sender serializes writes.

G1 integration: `Reader.ResolveVideo` separates payload receipt from codec
acceptance. `lastVideoID`/canonical progress advances only after successful
decode; a separate consumed-wire-ID high-water mark rejects non-increasing
IDs even when earlier payloads failed validation. A recoverable sequence error
discards exactly the bounded payload, invalidates the reference and waits for
a forward KEY. Malformed framing or truncated discard is terminal. Actual
reference bytes and decompression validation are implemented in G2.
After a parser-level `ErrNeedKeyframe`, also invalidate the codec reference.
After codec Decode, pass success/failure to `Reader.ResolveVideo`; only then
read the next video.

Treat even decrypted input as untrusted. Malformed framing, directions, sizes, versions and compressed data must yield bounded errors, never panic or uncontrolled allocation. Room codes, frames, keys and tokens must not enter plaintext logs/telemetry or frame storage. Reuse verified upstream PAKE/E2E mechanisms rather than inventing a handshake.

## Số đo

- Tốc độ video tính theo tổng `18 + kích thước payload` của KEY/DELTA,
  chia 1024 để ra KiB/s trong cửa sổ `(now-1s, now]`. Control, mã hóa kênh
  và overhead mạng nằm ngoài phép đo. Kích thước trung bình KEY/DELTA gồm header.
- Baseline là byte nén LZ4 toàn ảnh trên **cùng mẫu đã gửi** và trong **cùng
  cửa sổ**, gồm header. KEY dùng lại byte đã đo. Tiết kiệm bằng
  `100 × (1 − actualBytes / baselineBytes)`. Giá trị có thể âm, để trống nếu
  thiếu baseline, mẫu số bằng 0 hoặc cửa sổ vượt giới hạn 4096 sự kiện.
- FPS chụp, gửi, nhận, giải mã và vẽ là các phép đo riêng. Gói video đầy đủ
  được tính vào số nhận trước khi giải mã. KEY/DELTA và mẫu chụp bỏ là số cộng
  dồn trong phiên. Thời gian codec/JPEG là thời gian thực thi đo được.
- RTT từ Ping/Pong dùng đồng hồ monotonic của bên khởi tạo. Chưa đo
  decode→render vì JPEG không mang timestamp liên kết với lần giải mã LZ4.
  Timestamp media là đồng hồ của máy phát. UI không suy ra độ trễ đầu-cuối từ đó.

## Kiểm thử

- **Giao thức và codec.** Dùng các test trong `internal/protocol` và
  `internal/delta` cho header/config theo byte, đọc từng phần, gói sai chiều,
  ID không tăng, frame LZ4 quá lớn hoặc bị cắt, tile lỗi và khôi phục bằng KEY.
  So sánh reference máy phát/máy xem theo byte ở cả hai độ phân giải, với
  `tau=0` và `tau=4`. Dữ liệu lỗi không được ghi một phần vào reference.
  Fixture CONFIG mặc định là `01 05 00 02 d0 20 01 01 18 00 1e`.
- **Phiên và bridge.** Kiểm tra I/O bị chặn, hủy trong lúc ghép đôi, EOF/END,
  PLI có giới hạn, hàng đợi chậm, Origin/token không hợp lệ và tài nguyên được
  thu hồi sau Stop. Frontend có test latest-only và đóng bitmap sau khi hủy.
- **Windows.** Kiểm tra cả dev Origin và bản đóng gói, Start/Stop, render, đổi
  vai trò, khôi phục ảnh, cửa sổ nhỏ, điều hướng bàn phím và trình đọc màn hình. Dùng kịch bản
  [preview](../app/g3/README.md), [Share/Watch](../app/g4/README.md) hoặc
  [stress/khôi phục](../app/g5/README.md). Với ZIP giải nén, chạy
  `app/g6/Smoke-Package.ps1 -Directory <thư-mục-giải-nén>` và
  `app/g6/Test-ProductionUI.ps1 -Directory <thư-mục-giải-nén>`.
  Bài UI chụp màn hình sau thao tác Start và cần clipboard trống.
- **Đo hiệu năng.** Ghi cấu hình máy, loại mạng, ảnh/nguồn chụp, thời lượng,
  FPS p50/p95, RSS và thời gian Stop. Benchmark codec chỉ dùng ảnh tổng hợp
  và tính cả chi phí nén baseline. Phân biệt kết quả ảnh tổng hợp, desktop loopback,
  LAN và Internet. [Kết quả đã ghi nhận](VERIFICATION.md)

Ví dụ chạy fuzz có giới hạn và benchmark từ thư mục gốc:

```text
go -C app test ./internal/protocol -run "^$" -fuzz "^FuzzReadStream$" -fuzztime=30s -parallel=2
go -C app test ./internal/delta -run "^$" -fuzz "^FuzzDecoderAtomicity$" -fuzztime=30s -parallel=2
go -C app test ./internal/delta -run "^$" -bench "^BenchmarkCodec$" -benchmem
```
