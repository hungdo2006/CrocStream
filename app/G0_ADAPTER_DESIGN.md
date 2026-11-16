# G0 adapter — đề xuất để review

**Trạng thái:** thiết kế thử nghiệm đã được triển khai trong PoC G0 sau yêu cầu
tiếp tục hoàn thiện G0 ngày 2026-11-16. Đã kiểm chứng local/remote và self-review
ở mức G0: [closeout PASS](G0_CLOSEOUT.md). Chưa audit bảo mật độc lập.
Xem [kết quả PoC](G0_LOCAL_PROOF.md) và
[lệnh chạy](g0/poc/README.md). Các nội dung bên dưới là hợp đồng thử nghiệm,
không phải tuyên bố transport đã sẵn sàng cho production.

## 1. Phạm vi đề xuất

PoC GUI-independent dùng **một kết nối tới relay TCP croc mỗi peer**. Giữ relay
admission upstream, sau đó thực hiện PAKE peer có namespace CrocStream và truyền
secure records trên chính kết nối đó. Không chọn relay công cộng cụ thể ở bước
này; kiểm chứng đầu tiên dùng relay upstream loopback do harness sở hữu.

Đây là **transport envelope G0 phiên bản thử nghiệm 1**, độc lập với media
protocol v1 trong Architecture. PoC truyền dữ liệu xác định trước, không truyền
ảnh và không triển khai G1. Packet media sau này chỉ là byte stream bên trong
transport, không thay header 18 byte đã quy định.

### Tái sử dụng và phần phải tự duy trì

| Thành phần | Đề xuất |
| --- | --- |
| Module croc | Ghim commit `552f40fd2d7593316118370bec0a93478d48e715`; PoC có thể `replace` sang `../croc`, xác minh tree trước build |
| Mã mời | Tái dùng `codephrase.GenerateSSH/ParseSSH` làm bộ sinh/phân tích sáu từ; thêm prefix và namespace room riêng |
| Relay | Tái dùng `tcp.ConnectToTCPServerControlContext`; relay local qua `tcp.RunWithListener` + `WithCtx` |
| PAKE primitive | `github.com/schollz/pake/v3 v3.2.1`, thông qua bản dẫn xuất tối thiểu của `pakekey` |
| `pakekey` dẫn xuất | Trong PoC tại `app/g0/poc/internal/upstreampake/`, giữ MIT + nguồn; cố định purpose `peer-crocstream-g0-v1` và curve `p256`, bỏ public selection purpose/curve cũ; giữ identity/transcript/HKDF/confirmation formula |
| Key confirmation | Giữ thứ tự vai, cách đóng gói transcript, HKDF-SHA256 và confirmation tags như upstream |
| Record encryption | Dùng `crypt.NewAESGCM`, `EncryptAEADTo`, `DecryptAEAD`; thêm directional-key derivation, record sequence và kiểm tra cấu trúc |
| Adapter/framing/lifecycle | Code CrocStream mới trong `app/`, phải có test và review riêng |

`croc/` vẫn là reference snapshot. Bản dẫn xuất không dùng tên/module giả để
vượt giới hạn `internal` của Go. Không sao chép vòng đời file hoặc terminal SSH.
Đây là phương án A của báo cáo khả thi, chưa tạo fork/repository thứ hai.

## 2. Threat model và giới hạn

Relay và mạng có thể đọc metadata, trì hoãn, ngắt, chèn, thay đổi, phát lại hoặc
phản chiếu bản ghi. Peer chỉ được tin sau PAKE và xác nhận khóa hai chiều.
Nhập sai mã, room khác, vai khác hoặc giao thức khác phải thất bại trước khi trả
stream cho ứng dụng.

Mục tiêu: giữ bí mật payload, phát hiện sửa đổi/replay/reflection, giới hạn tài
nguyên, hủy được I/O đang chờ. Không cam kết chống relay DoS, che địa chỉ IP,
che kích thước/thời điểm truyền hoặc chống endpoint đã bị chiếm quyền.

Một lần host chỉ chấp nhận một peer; sai xác thực đóng phiên đó. Không tự thử
lại mật khẩu, không tiếp tục phiên cũ sau đứt kết nối. PAKE giảm khả năng đoán
offline nhưng không loại bỏ đoán online; UI/harness không có vòng reconnect vô hạn.

## 3. Mã mời và room isolation

Đề xuất mã `cs1-<six upstream EFF words>`:

1. Chỉ nhận prefix `cs1-`, giới hạn tổng input 256 byte, không tự đổi case/trim.
2. Bỏ prefix rồi dùng `codephrase.ParseSSH`; không tự tách theo số dấu `-` vì
   wordlist có từ chứa dấu gạch nối.
3. `relayRoom = lowercaseHex(SHA256("crocstream/room/g0-v1\x00" + parsed.RoomName))`.
4. PAKE password là `parsed.PAKEPassphrase`; PAKE room là `relayRoom`.

Hai từ đầu chọn room, bốn từ còn lại làm password theo upstream. Với 1296 từ,
password ngẫu nhiên có khoảng 41 bit entropy; hashing room không tăng entropy.
Không gọi room “không thể đoán”; không coi format này là chốt UX production.
Code đi qua trusted UI binding hoặc prompt không echo, không qua log/command-line
arguments. Harness hai process truyền code bằng pipe do parent sở hữu, không
lưu code xuống file.

## 4. Relay admission và quyền sở hữu connection

- `Host(ctx, opts)` sinh code, vào relay room qua API upstream, rồi trả listener
  và invitation. Trạng thái lúc đó là Waiting, chưa xác thực peer.
- `Join(ctx, invite, opts)` parse code, vào cùng room và khởi tạo PAKE vai A
  (viewer). Host là vai B (broadcaster), đúng thứ tự upstream.
- Context có hiệu lực suốt listener/session lifetime. Sau khi admission trả về,
  adapter cài hook đóng socket khi context bị hủy; không dựa vào hook tạm trong
  `ConnectToTCPServerControlContext`.
- Adapter tiếp quản `Comm.Connection()` **chỉ để chạy peer handshake và secure
  record**, chưa chuyển socket thô cho ứng dụng. Không gọi `Comm.Read` đồng thời
  với parser mới vì Receive của Comm có thể xóa write deadline.
- Một reader và một writer mỗi endpoint. Quyền đặt read/write deadline tách
  biệt; không dùng `SetDeadline` trong hot path để xóa deadline hướng còn lại.

## 5. Framing và handshake thử nghiệm

Đề xuất giữ framing ngoài của Comm để đọc được keepalive relay:

```text
4 bytes ASCII "croc" | uint32 payloadLength LittleEndian | payload
```

Parser riêng dùng `io.ReadFull`, kiểm tra magic/length trước allocation.
Handshake payload tối đa **8 KiB**; record payload có giới hạn riêng ở mục 6.
EOF giữa header/body là lỗi fatal, không scan tìm magic tiếp theo.

### Payload handshake

```text
4 bytes ASCII "CSH1" | uint8 kind | body

kind 1: INIT       body = uint16_BE pakeLength | initiatorPAKE
kind 2: RESPONSE   body = uint16_BE pakeLength | responderPAKE | salt[32]
kind 3: CONFIRM_A  body = confirmationA[32]
kind 4: CONFIRM_B  body = confirmationB[32]
```

PAKE values: 1–4096 byte; exact body length, không trailing bytes.
Không JSON/compression trong handshake mới. Giá trị version/purpose/curve là
hằng số của protocol, không negotiation/downgrade:

- identity/key schedule `ProtocolVersion=2` như croc;
- purpose `peer-crocstream-g0-v1`;
- curve cố định `p256`;
- role A=0 viewer/receiver, role B=1 broadcaster/sender.

```text
Viewer A                              Broadcaster B
Init(password, 0, p256, purpose, room)
INIT(PAKE_A) ------------------------->
                                      Init(... role 1 ...)
                                      Update(PAKE_A), random salt[32]
                  <------------------- RESPONSE(PAKE_B, salt)
Update(PAKE_B)
Derive(shared, exact transcript)        Derive(shared, exact transcript)
CONFIRM_A ---------------------------->
                                      Confirm(expectedA, receivedA)
                  <------------------- CONFIRM_B
Confirm(expectedB, receivedB)
Authenticated stream                  Authenticated stream
```

Không gửi application bytes trước bước xác nhận ở mỗi phía. Duplicate,
out-of-order, unsupported kind/version, wrong length hoặc confirmation sai đều
đóng phiên. Không gửi raw crypto error, room hoặc password ra log.

### Keepalive và timeout

Relay upstream gửi framed payload `[0x01]` cho endpoint đang chờ. Chỉ bỏ qua
keepalive trước khi nhận INIT/RESPONSE đầu tiên; nó không gia hạn deadline.
Sau khi handshake bắt đầu không chấp nhận keepalive như dữ liệu application.
Cần kiểm thử race ghép room/keepalive bằng relay thật; nếu quan sát khác source
assumption, dừng sửa thiết kế thay vì tự mở rộng parser.

Host chờ INIT tối đa 5 phút hoặc context ngắn hơn. Sau INIT hợp lệ, host có
deadline handshake tuyệt đối 30 giây; viewer có 30 giây từ lúc gửi INIT.
Admission cũng có context timeout tổng 30 giây vì timeout dial riêng upstream
không tự giới hạn mọi bước admission. Không tự retry sau timeout.

## 6. Secure record đề xuất

### Key separation

Sau mutual confirmation, dùng traffic key 32 byte từ upstream-derived key
schedule làm input HKDF-SHA256, salt nil, length 32, với info chính xác:

- `crocstream/record/g0-v1/viewer-to-broadcaster`
- `crocstream/record/g0-v1/broadcaster-to-viewer`

Mỗi hướng tạo AES-256-GCM riêng. Không dùng confirmation tag làm key.
Traffic key đã gắn exact PAKE transcript; tách info cố định theo hướng giúp
phản chiếu ciphertext không được xác thực ở hướng ngược lại.

**Đây là phần mở rộng key schedule cần review**, không phải API transport có
sẵn của croc.

### Plaintext và ciphertext

Mỗi framed payload sau handshake là kết quả upstream `EncryptAEADTo`:

```text
nonce[12 random bytes] | AES-GCM ciphertext | tag[16]
```

Plaintext được mã hóa:

| Offset | Bytes | Field |
| --- | ---: | --- |
| 0 | 4 | ASCII `CSR1` |
| 4 | 1 | Direction: 0 viewer→broadcaster, 1 broadcaster→viewer |
| 5 | 1 | Kind: 1 DATA, 2 CLOSE |
| 6 | 8 | Sequence uint64 BigEndian, bắt đầu 0, tăng đúng 1 mỗi record |
| 14 | 4 | DataLength uint32 BigEndian |
| 18 | DataLength | Data |

- DATA: `1 <= DataLength <= 65,536`. CLOSE: `DataLength=0`.
- Tổng framed ciphertext payload tối thiểu 46, tối đa **65,582 byte**.
- Kiểm tra outer bound trước allocation; xác thực AEAD trước parse nội dung.
  Plaintext phải đúng `18 + DataLength`, đúng hướng/sequence/kind/magic.
- Outer header không dùng AAD vì giữ primitive upstream; giá trị length phải
  khớp ciphertext và plaintext có độ dài chính xác. Truncate/extend/header
  tampering phải kết thúc phiên, không khôi phục bằng scan.
- Không trả bất kỳ data của record lỗi cho caller. DATA rỗng và unknown kind
  bị từ chối để tránh reader loop không tiến triển.
- Sequence tăng chỉ sau record gửi đủ hoặc nhận/validate thành công. Partial
  write/error làm session terminal; không retry record với nonce/state cũ.
- Transport CLOSE được xác thực; EOF socket thiếu CLOSE là unexpected
  disconnect, không phải kết thúc stream sạch. Application END của media vẫn
  là nội dung byte stream riêng, không phải transport CLOSE.

### Nonce budget

Giữ random nonce từ upstream, không tự chuyển sang counter nonce. Đề xuất
giới hạn **2^20 record mỗi hướng mỗi session**, trong đó dành record cuối cho
CLOSE; trước khi vượt giới hạn phải đóng và xác thực phiên mới. Xác suất va
chạm nonce lý tưởng ở ngân sách này xấp xỉ 2^-57 mỗi hướng. Đây không phải chứng
minh an toàn đầy đủ cho GCM; review phải xem thêm tổng dữ liệu và failed-auth
budget (một auth failure là đóng phiên).

Ngân sách này giới hạn khoảng 64 GiB payload mỗi hướng; phiên video entropy cao
có thể cần reconnect sớm. Bài PoC 10 phút dùng dữ liệu giới hạn; không coi đạt
PoC là chứng minh đủ ngân sách cho sản phẩm. Không rekey tự động trong bản G0.

## 7. API, backpressure và cleanup

Interface chức năng; PoC trả concrete `*Listener` và `*Session` tương ứng:

```go
type Listener interface {
    Invitation() string
    Accept(ctx context.Context) (io.ReadWriteCloser, error)
    Close() error
}

func Host(ctx context.Context, opts Options) (Listener, error)
func Join(ctx context.Context, invite string, opts Options) (io.ReadWriteCloser, error)
```

`Options` chứa relay address/password và timeout có giới hạn; không dùng global
proxy/logger mutable trong session. Không expose traffic keys hoặc raw net.Conn.
Chỉ một lần Accept; concurrent/second Accept trả lỗi cụ thể. Hủy context của
Host/Accept/Join hoặc Close đều đóng socket để unblock I/O.

- `Write(p)` chia thành DATA <=64 KiB, trả số **application bytes** gửi hoàn tất
  (không tính header/tag). Serial writer; lỗi sau một phần trả count đã gửi và
  lỗi, rồi đóng session. Không queue tăng vô hạn.
- `Read(p)` giữ tối đa một record đã xác thực đang đọc dở; không giữ cả 10 MiB.
  Zero-length `Read`/`Write` tuân hợp đồng Go, không phát DATA rỗng.
- PoC scheduler có một writer mỗi endpoint; control ưu tiên ở ranh giới packet,
  không xen byte vào packet đang gửi. Có thể dùng bounded queue 2 data + 8
  control; nhận đủ byte của mọi packet đã phát theo thứ tự.
- `Close` idempotent: thử CLOSE dưới lock/deadline ngắn nếu writer sẵn; nếu I/O
  đang kẹt thì đóng socket ngay. Không chờ write mutex vô hạn để gửi CLOSE.
  Callback hủy ưu tiên unblock ngay, không cần graceful CLOSE.
- Sau lỗi/hủy: đợi workers kết thúc có timeout, clear owned secret buffers,
  giải phóng reference. Go không bảo đảm xóa mọi bản sao string/GC/AEAD internals;
  không tuyên bố cryptographic memory erasure.

## 8. CLI harness và bằng chứng phải thu

Sau khi duyệt thiết kế:

1. `g0-relay` chỉ loopback theo mặc định, port OS-assigned, tắt debug log.
2. Parent harness chạy **hai peer child processes**, cung cấp invite bí mật qua
   pipe, thu log JSON chỉ có status/counter/hash/duration.
3. Mỗi chiều ít nhất 10 MiB pattern có seed cố định khác nhau theo role; kiểm
   tra nội dung, offset/order và SHA-256. Chia block 32 KiB; payload không lưu disk.
4. Ping/Pong + KeyframeRequest-equivalent xen giữa data qua cùng stream; kiểm
   tra nonce/control sequence. Đây là harness message, không phải G1 media.
5. Phiên giữ hoạt động >=10 phút bằng dữ liệu/control có nhịp, đo thời gian
   monotonic; không chỉ truyền xong rồi ngủ. Test idle là case riêng.
6. Wrong code, cross-purpose/file/SSH, wrong room, wrong role, bad tag, replay,
   reflection, truncated/oversize records, tiny reads, partial writes, blocked
   peer, cancel khi handshake/Accept/Read/Write và 100 Host/Join cycles.
7. Chạy LAN và hai máy qua relay được duyệt; chứng minh route và chính sách,
   giữ riêng kết quả local và remote. Không tự chọn public relay.

PoC cố định namespace, không cung cấp API cho purpose cũ. Thay cho bài byte-equal
trên API đã bỏ, test đối chiếu chéo với `pakekey` upstream thật trên cả ba purpose
cũ, cả hai hướng initiator/responder, yêu cầu không thể xác nhận khóa chéo.
Test record độc lập với relay bằng bounded simulated streams, sau đó kiểm tra
hai process thật. Lịch sử upstream baseline nằm trong báo cáo toolchain.

## 9. Self-review và quyết định trước code

| Câu hỏi review | Kết quả thiết kế / bằng chứng còn thiếu |
| --- | --- |
| Có giả vờ raw socket đã E2E không? | Không: stream chỉ trả sau mutual confirmation; actual local/remote proofs đã đạt |
| Có sửa snapshot hoặc bỏ PAKE không? | Derivative nhỏ tách riêng, primitive và snapshot giữ nguyên; provenance/notices có trong bundle |
| Giao thức có được tách biệt bằng khóa không? | Purpose trong identity/transcript; cross-purpose upstream tests cả hai chiều đã đạt |
| Chống reflection/replay thế nào? | Directional keys + authenticated direction/sequence; negative record/fault tests đã đạt |
| Tài nguyên có giới hạn không? | 8 KiB peer handshake, 64 KiB data, queue cap, timeout, nonce budget; tests/lifecycle và heap/goroutine samples có trong báo cáo |
| Còn rủi ro gì? | Record protocol mới, relay keepalive race, upstream admission resource bounds, cleanup khi write bị chặn, nonce/session budget |

Baseline Windows đã tái hiện `TestWrongPassword` dưới `-race` nhận connection
reset thay vì chuỗi lỗi `bad password`, cả khi chạy độc lập.
API adapter phải cho phép lỗi relay-auth/network chưa phân biệt được,
không hứa luôn báo chính xác sai mật khẩu. Xem [bằng chứng](G0_TOOLCHAIN.md).

Đây là self-review thiết kế, **không phải audit bảo mật độc lập**.
Derivative/adapter đã có trong module PoC; kết quả test thực tế được ghi riêng
trong `G0_LOCAL_PROOF.md` và các báo cáo remote/follow-up.
Relay từ xa/hai máy đã kiểm chứng; giới hạn và G0 PASS nằm trong `G0_CLOSEOUT.md`.
