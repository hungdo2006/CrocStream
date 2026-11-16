# G0 — Khảo sát transport và quyết định khả thi

> **Kết quả mới nhất (2026-11-16):** PoC đã đạt local và hai máy qua relay EC2
> trong các phiên >600s. [Follow-up mạng mới](G0_LATENCY_FOLLOWUP.md) đạt RTT
> mean 48–51ms ở tải gốc, hash/control/cleanup đúng.
> **[G0_CLOSEOUT](G0_CLOSEOUT.md) chốt G0 PASS**, gồm native build/runtime,
> notice review và self-review. Chưa triển khai G1 hoặc ứng dụng.
> [Kết quả remote trước](G0_REMOTE_PROOF.md) vẫn giữ giới hạn của mạng cũ.
> Các mục dưới đây lưu lịch sử khảo sát ban đầu, không phải trạng thái hiện tại.

- **Ngày:** 2026-11-15
- **Gate:** G0 — **BLOCKED / No-Go để chuyển sang G1**
- **Phạm vi đã thực hiện:** khảo sát nguồn, môi trường, API và giấy phép.
- **PoC hai process:** chưa triển khai/chạy; chưa có bằng chứng transport đạt G0.

> **Cập nhật sau phê duyệt:** chủ dự án đã đồng ý cài toolchain và tiếp tục thiết
> kế/review adapter. Go 1.27.4 và Wails CLI 3.1.0 đã cài portable; build probe
> Go/Wails/croc đã chạy thành công. Kết quả mới, gồm cả lỗi kiểm tra upstream,
> nằm trong [G0_TOOLCHAIN](G0_TOOLCHAIN.md); thiết kế cần review ở
> [G0_ADAPTER_DESIGN](G0_ADAPTER_DESIGN.md). Các bảng bên dưới giữ kết quả lượt
> khảo sát ban đầu, không mô tả trạng thái toolchain hiện tại.

## 1. Kết luận

Có các thành phần có thể tái sử dụng, nhưng chưa tìm được API công khai kết hợp
**mã mời → PAKE xác thực hai bên → luồng byte bảo mật độc lập với truyền file**
đáp ứng CrocStream ngay trong snapshot hiện tại.

Ba trở ngại:

1. **Môi trường:** không tìm thấy Go hoặc `wails3` trên PATH và các đường dẫn phổ
   biến đã kiểm tra. Chưa thể compile hay chạy test Go; cần phê duyệt cài công cụ.
2. **Tích hợp/bảo mật:** `pakekey` chỉ chấp nhận purpose truyền file, local probe
   và SSH. Handshake hoàn chỉnh nằm trong vòng đời truyền file hoặc các hàm SSH
   không export. Cần quyết định thiết kế adapter và cách tách biệt giao thức.
3. **Thử nghiệm từ xa:** chưa có relay mục tiêu được xác nhận cho thử nghiệm này,
   điều kiện sử dụng/băng thông hoặc máy thứ hai để nghiệm thu.

Đây là kết luận **chưa đủ điều kiện triển khai PoC an toàn theo thiết kế hiện
tại**, không phải bằng chứng rằng croc không thể truyền hai chiều.

## 2. Provenance và môi trường

| Mục | Kết quả quan sát |
| --- | --- |
| Repository | `D:\CrocStream`, nhánh `main`; một Git ở gốc |
| CrocStream HEAD tại lúc khảo sát | `565c192c536c7fd926ba7fa940d817cee13ed070` |
| Upstream được ghim | `552f40fd2d7593316118370bec0a93478d48e715` |
| `git rev-parse HEAD:croc` | `0e401898d47eaef76e62263483943e3aa789ad3a` |
| Tree của commit upstream qua GitHub API | Cùng giá trị `0e401898d47eaef76e62263483943e3aa789ad3a` |
| Working tree `croc/` | Không có thay đổi được Git báo; không có Git lồng bên trong |
| Version trong nguồn | `src/version/version.go`: `11.6.5` |
| Go tối thiểu trong nguồn | `croc/go.mod`: `1.27.3` |
| OS quan sát qua .NET | `Microsoft Windows 10.0.26300`, X64 |
| Registry Windows | ProductName `Windows 10 Home`, DisplayVersion `26H2`, build `26300.9974`; ghi nguyên giá trị báo về, không coi đây là ma trận OS đã kiểm thử |
| PowerShell | `5.1.26100.9818` |
| Go / Wails CLI | Không tìm thấy trên PATH hoặc các đường dẫn phổ biến đã kiểm tra |
| Node / npm | `v24.22.1` / `12.2.1` |
| Git / GCC | `2.47.1.windows.2` / `13.2.0`; GCC có mặt chưa chứng minh race build Go hoạt động |

Nguồn kiểm tra provenance:
<https://api.github.com/repos/schollz/croc/git/commits/552f40fd2d7593316118370bec0a93478d48e715>.
Tree hash xác nhận nội dung snapshot được lưu trong Git; trạng thái working tree
được kiểm tra riêng. Các chỉnh sửa onboarding trước G0 vẫn được giữ lại.

### Công cụ đề xuất, chưa cài

- Metadata chính thức <https://go.dev/dl/?mode=json> trả về Go **1.27.4** stable,
  có archive Windows amd64, đáp ứng minimum 1.27.3 của croc.
- Có thể đề nghị cài Go portable và cache trong
  `C:\Users\Admin\AppData\Local\Temp\opencode\crocstream-g0\`, kiểm tra SHA-256 từ
  metadata chính thức trước khi giải nén. Chưa tải binary hoặc cài công cụ.
- `wailsapp/wails` tag `v3.1.0`, file `v3/go.mod`, ghi `go 1.26.0` và
  `toolchain go1.26.10`. Go minimum của croc cao hơn; chưa có build kết hợp.
- Đã đọc ví dụ `v3/examples/services/main.go` và `v3/examples/events/main.go` ở
  tag đó: `application.New`, `application.NewService`,
  `app.Window.NewWithOptions`, `app.Event.On/Emit`, `app.Run` là các API có trong
  nguồn v3. Đây là kiểm tra source, không phải `wails3 doctor` hoặc native build.
- Chưa chọn dependency capture hoặc pin production dependencies.

## 3. Bản đồ API và giới hạn tích hợp

Các đường dẫn sau trỏ vào snapshot `croc/` đã xác minh; số dòng là tại revision
đã ghim, không phải cam kết API ổn định của upstream tương lai.

| Thành phần | API / vị trí thật | Nhận định |
| --- | --- | --- |
| Mã mời | [`codephrase.go`](../croc/src/codephrase/codephrase.go), `Generate`, `Parse`, `GenerateSSH`, `ParseSSH` | Phân tách room và PAKE passphrase; định dạng mã khác nhau theo chế độ, không dùng regex tự đoán |
| Relay admission | [`tcp.go`](../croc/src/tcp/tcp.go):871, `ConnectToTCPServerControlContext(ctx, address, password, room, ...time.Duration)` | Trả `*comm.Comm`, banner, IP, capability, error; chỉ là vào relay room, chưa xác thực peer |
| Chuyển tiếp hai chiều | `tcp.go`:649–664, 779–801 | Hai hướng copy giữa socket sau khi ghép room; bằng chứng source về duplex, chưa phải runtime evidence |
| Framing upstream | [`comm.go`](../croc/src/comm/comm.go):186–235, 326–331 | `Connection() net.Conn`, `Read() ([]byte,int,[]byte,error)`, `Write([]byte)(int,error)`, `Close()` không trả error; không implement trực tiếp `io.ReadWriteCloser` |
| PAKE/key schedule | [`pakekey.go`](../croc/src/pakekey/pakekey.go):51–125 | `Init`, `Derive`, `Confirm` export; purpose allowlist không có CrocStream |
| File-transfer API | [`croc.go`](../croc/src/croc/croc.go):1805, 2040 | `Send(filesInfo, emptyFolders, totalFolders) error` và `Receive() error`; không trả authenticated stream |
| Handshake truyền file | `croc.go`:2773–2931 | PAKE → derive transcript-bound keys → kiểm tra xác nhận cả hai vai → kích hoạt data transport |
| Vòng đời file | `croc.go`:3105–3112, 3233–3284, 3870 | `TypeFinished` kết thúc; sau secured channel là file metadata; `sendData` nhận `*os.File` |
| Primitive AEAD | [`crypt.go`](../croc/src/crypt/crypt.go):45–109 | AES-GCM encrypt/decrypt bản ghi, không phải session/duplex adapter hoàn chỉnh |
| Tailcat data plane | [`endpoint_supported.go`](../croc/src/tailcattransport/endpoint_supported.go):37, 166, 197 | `Listen(ctx,key,config,events)`, `Accept(ctx)`, `Dial(ctx,offer,key,config,events)` export; phải có session key và offer được xác thực trước |
| Stream/ownership Tailcat | [`bundle_supported.go`](../croc/src/tailcattransport/bundle_supported.go):122, 192 | `Connections() []net.Conn`; phải giữ và đóng `Bundle` để dọn node/ticker, không chỉ đóng socket |
| SSH handshake | [`protocol.go`](../croc/src/sshshare/protocol.go):109, 175 | `guestPAKE` / `hostPAKE` không export; gắn với purpose SSH và authorization/host-key/terminal |
| SSH public API | [`host.go`](../croc/src/sshshare/host.go):83, [`client.go`](../croc/src/sshshare/client.go):105 | `StartHost(ctx,HostConfig)` và `Join(ctx,ClientConfig)` phục vụ terminal, không trả stream chung |
| SSH trên Windows | [`terminal_unsupported.go`](../croc/src/sshshare/terminal_unsupported.go):10–13 | Hosting terminal trả lỗi unsupported trên Windows; không phù hợp làm đường tắt cho MVP Windows |

### Phân biệt hai lớp xác thực

```text
Mã mời
  → room + PAKE passphrase
  → kết nối relay, admission bằng relay password
  → relay ghép hai socket
  → PAKE GIỮA HAI PEER, gắn purpose/room/vai/transcript
  → xác nhận khóa hai chiều
  → dữ liệu được bảo vệ
```

Admission chỉ xác nhận quyền vào relay. Bỏ bước peer PAKE rồi lấy
`Comm.Connection()` để gửi media không đáp ứng yêu cầu E2E của CrocStream.

### Khoảng trống cần giải quyết trước PoC

- `pakekey.Identities` từ chối purpose ngoài `peer-transfer`, `local-ip-probe`,
  `peer-ssh`. Việc tái dùng purpose file/SSH cho media không tự chứng minh protocol
  isolation; thêm purpose mới là thay đổi hợp đồng bảo mật cần duyệt.
- `pakekey.Derive` gắn version, purpose, room, curve, vai, hai PAKE wire values
  và salt; tạo một traffic key cùng hai confirmation tags. Không được bỏ các bước
  này chỉ để có demo chạy.
- `crypt.EncryptAEADTo` dùng random nonce và nil additional data. Primitive này
  tự nó không cung cấp thứ tự bản ghi, chống replay/reflection hay tách hướng cho
  một byte stream mới. Cần thiết kế/review record envelope, không chỉ bọc
  `Encrypt`/`Decrypt` rồi tuyên bố transport đã an toàn.
- `Comm.Write` trả số byte gồm header framing; một stream adapter phải xử lý
  đúng hợp đồng `io.Writer`. `Read` của Comm là message API, không phải đọc vào
  buffer của caller.
- Receive của Comm đặt lại deadline của connection. Quyền sở hữu deadline và
  cancellation khi read/write chạy đồng thời phải được kiểm thử. Hook hủy trong
  `ConnectToTCPServerControlContext` chỉ bao quanh dial/admission; caller phải
  quản lý lifetime sau khi hàm trả về.
- Tailcat có `net.Conn` phù hợp ở data plane nhưng không giải quyết khoảng trống
  peer-authentication/control-plane ở trên. Thay session key bằng khóa hardcode
  hoặc trao đổi offer thủ công không đạt bài thử G0.

## 4. Relay: điều đã biết và chưa biết

Source relay có hai hướng copy, không tự yêu cầu file-completion metadata ở bước
pipe. Đây là dấu hiệu tích cực cho adapter riêng.

Defaults tại [`src/tcp/defaults.go`](../croc/src/tcp/defaults.go):
64 pending handshakes tổng, 32/source; first message 1 phút, handshake 5 phút;
128 waiting rooms; room TTL 3 giờ; admission 30/source và 30/room mỗi phút.
Các giá trị này **không phải** thông lượng hoặc chính sách cam kết của relay công
cộng. `comm` cũng có deadline riêng, cần tính đến khi thiết kế phiên dài.

Chưa đo RTT, throughput, reconnect, idle timeout hoặc phiên 10 phút. Chưa mở
phiên tới public relay, chưa chọn DERP thay cho croc TCP relay. Chủ dự án cần
xác nhận relay mục tiêu và môi trường hai máy; LAN-only chỉ khi được duyệt đổi
phạm vi. Khi thử relay phải chứng minh đường thực tế, tránh direct fallback làm
sai kết luận.

## 5. Dependency và giấy phép

Inventory nguồn, chưa có module ứng dụng hoặc dependency graph đã resolve:

| Thành phần trong snapshot | Phiên bản | Bằng chứng giấy phép |
| --- | --- | --- |
| croc | Commit ghim ở trên | [`croc/LICENSE`](../croc/LICENSE): MIT, giữ copyright và permission notice |
| `github.com/schollz/pake/v3` | `v3.2.1` | LICENSE ở tag: MIT |
| `golang.org/x/crypto` | `v0.59.0` | LICENSE ở tag: BSD-3-Clause |
| `github.com/magisterquis/connectproxy` | `v0.0.0-20200725203833-3582e84f0c9b` | LICENSE ở revision: zlib-style, phải đánh dấu bản sửa và giữ notice |
| `tailscale.com` | `v1.104.6` | LICENSE ở tag: BSD-3-Clause |
| Tailcat in-tree | `c04c5afee401df40e620db8ae108e957ae07bcd9` + croc-local patches | [`UPSTREAM.md`](../croc/internal/tailcat/UPSTREAM.md), [`LICENSE`](../croc/internal/tailcat/LICENSE) |
| `gvisor.dev/gvisor` | `v0.0.0-20261112101947-136968485ee1` | Notice upstream ghi Apache-2.0 cho revision cũ; cần đối chiếu revision thực tế trước phân phối |
| `github.com/fxamacker/cbor/v2` | `v2.10.0` | Notice upstream ghi phiên bản cũ `v2.9.0`; cần đối chiếu phiên bản thực tế |

Phát hiện cần lưu ý: `THIRD_PARTY_NOTICES.md` ghi Tailscale `v1.104.1`, gVisor
revision tháng 2 và CBOR `v2.9.0`; `go.mod` đã ghim các phiên bản mới hơn ở bảng
trên. `internal/tailcat/UPSTREAM.md` còn ghi gVisor revision tháng 9. Giữ snapshot
nguyên vẹn; khi có dependency graph ứng dụng phải tạo inventory/notices chính
xác trong `app/`. Chưa chứng nhận đầy đủ giấy phép transitive dependencies.

## 6. Kiểm tra đã thực hiện và phần chưa chạy

### Lệnh local đã chạy từ repository root

```powershell
git -c safe.directory=D:/CrocStream status --short --branch
git -c safe.directory=D:/CrocStream rev-parse HEAD HEAD:croc
git -c safe.directory=D:/CrocStream status --porcelain --untracked-files=all -- croc/
git -c safe.directory=D:/CrocStream diff HEAD --exit-code -- croc/
node --version
npm --version
git --version
gcc --version
```

Kết quả: Git/provenance và các lệnh version thành công; `status`/`diff` giới hạn
trong `croc/` không có thay đổi. Lookup `Get-Command` cho go/wails3/node/npm/git/
gcc/clang và `Test-Path` các vị trí phổ biến cho kết quả ở mục 2.
`safe.directory` chỉ dùng cho từng lệnh, không sửa cấu hình Git toàn máy.

Script inventory tái lập được tại [`g0/Inspect-Environment.ps1`](g0/Inspect-Environment.ps1):

```powershell
powershell -NoProfile -File app/g0/Inspect-Environment.ps1
powershell -NoProfile -File app/g0/Inspect-Environment.ps1 -VerifyUpstream
```

Script chỉ đọc môi trường/Git, tùy chọn GET metadata GitHub; kết quả inventory
không phải PASS của transport. Kết quả chạy script được ghi ở mục 9 sau kiểm tra.

### Đối chiếu HTTP/source đã thực hiện

- GitHub commit API ở mục 2: thành công, tree khớp.
- `https://go.dev/dl/?mode=json`: thành công, metadata Go 1.27.4.
- `https://raw.githubusercontent.com/wailsapp/wails/v3.1.0/v3/go.mod`: thành công.
- Lookup ví dụ Wails `v3/examples/vanilla/main.go`: **404**; sau đó liệt kê thư
  mục examples qua GitHub API, đọc đúng `services/main.go` và `events/main.go`:
  thành công. Không dùng ví dụ tưởng tượng để scaffold.
- LICENSE qua `raw.githubusercontent.com`: `schollz/pake/v3.2.1/LICENSE`,
  `golang/crypto/v0.59.0/LICENSE`, `tailscale/tailscale/v1.104.6/LICENSE`,
  `magisterquis/connectproxy/3582e84f0c9b/LICENSE`: đều HTTP 200.
- Lookup ban đầu `croc/src/relay/relay.go`: không tồn tại; đã tìm và đọc relay
  implementation thật tại `croc/src/tcp/tcp.go`.

### Acceptance matrix

| Kiểm chứng | Trạng thái | Lý do / bằng chứng |
| --- | --- | --- |
| Snapshot đúng commit | PASS (provenance) | Git tree khớp API upstream, `croc/` sạch |
| Source/API survey | DONE (source only) | Mục 3–5, xác định khoảng trống adapter |
| `go version` | NOT RUN | Không tìm thấy Go khả dụng trong các vị trí đã kiểm tra |
| `go test ./...`, `go test -race ./...`, `go vet ./...` trong `croc/` | NOT RUN | Chưa có Go; chưa chứng minh regression/race |
| Các lệnh Go trên trong `app/` | NOT RUN | Chưa có Go hoặc module ứng dụng |
| `wails3 doctor`, minimal native build | NOT RUN | Chưa có CLI/toolchain; WebView2/capture chưa kiểm chứng |
| PoC hai process, >=10 MiB mỗi chiều, hash/thứ tự | NOT RUN / BLOCKED | Thiếu toolchain và quyết định adapter bảo mật |
| Reverse control, >=10 phút, cancellation | NOT RUN / BLOCKED | Chưa có PoC |
| Hai máy và relay mục tiêu | NOT RUN / BLOCKED | Chưa xác nhận môi trường/relay |
| Wrong password, replay/reflection, protocol confusion | NOT RUN / BLOCKED | Cần adapter và harness thật |
| RTT, throughput, bộ nhớ | N/A | Chưa có số đo runtime |

Đã đọc source các test upstream `TestIdentityBoundAgreementAcrossCurves`,
`TestCrossWiredRoomsCannotConfirm`, `TestTranscriptChangesInvalidateConfirmation`,
`TestSSHPAKERejectsWrongCode`; sự tồn tại của test **không phải** kết quả test.

## 7. Các phương án để chủ dự án quyết định

| Phương án | Lợi ích | Chi phí / giới hạn |
| --- | --- | --- |
| **A. Adapter với phần mở rộng upstream tối thiểu, ghim commit** | Giữ hướng tái sử dụng croc; có thể tách khỏi file lifecycle | Cần duyệt purpose/record envelope/API và phạm vi mã dẫn xuất trong `app/`; bảo trì phần khác upstream mỗi lần nâng cấp; regression + security review bắt buộc |
| **B. Đề xuất API secure-duplex lên upstream** | Có cơ hội giảm phần fork phải tự duy trì | Chưa có cam kết upstream chấp nhận hoặc lịch phát hành; G0 vẫn chờ runtime proof |
| **C. Chọn transport xác thực khác** | Có thể có API stream phù hợp hơn | Đổi quyết định kiến trúc/security/relay; cần lựa chọn cụ thể, phê duyệt và thực hiện lại spike |

**Khuyến nghị A để khảo sát tiếp, chưa phê duyệt triển khai thay đổi bảo mật.**
Phạm vi review tối thiểu gồm bốn phần: namespace/purpose và mã mời; peer handshake
và xác nhận khóa; secure record/stream adapter; lifecycle và CLI harness. Không
fork toàn bộ UI/CLI truyền file. Mã dẫn xuất nếu được duyệt nằm trong cùng repo,
ngoài snapshot `croc/`, có ghi nguồn/license và diff bảo trì rõ ràng.

Một phương án dùng relay TCP croc với record adapter sẽ tránh phụ thuộc Tailcat
ở data plane, nhưng tạo trách nhiệm review record protocol. Phương án Tailcat
có stream sẵn nhưng thêm dependency/build và điều kiện relay/DERP. Cần chọn
trong thiết kế được duyệt, không tự chuyển relay hoặc thay cryptography.

### Việc cần xác nhận

1. Cho phép cài **Go portable 1.27.4**, Wails CLI 3.1.0 và tải dependency phục vụ
   G0 trong vùng công cụ tạm được chỉ định? Việc cài không tự duyệt production
   dependencies hoặc bảo đảm build.
2. Cho phép mở công việc **G0 adapter follow-up** theo phương án A, bắt đầu bằng
   thiết kế cụ thể và review protocol isolation/record envelope?
3. Relay thử nghiệm từ xa là public hay self-hosted có quyền sử dụng, và máy thứ
   hai nào sẽ tham gia? Có thể bổ sung trước G0.4, không cần để bắt đầu review.

## 8. Tiến độ và gate kế tiếp

| Task | Trạng thái ở lần khảo sát này |
| --- | --- |
| G0.1 | Đã kiểm kê source/môi trường; toolchain và inventory transitive còn thiếu |
| G0.2 | Đã khảo sát API/handshake; phát hiện cần quyết định adapter |
| G0.3 | BLOCKED — chưa có PoC |
| G0.4 | NOT RUN — chưa có thử nghiệm transport |
| G0.5 | Đã lập báo cáo BLOCKED này; cần cập nhật sau khi tháo blocker |

**Gate kế tiếp vẫn là G0**, với quyết định adapter/toolchain trước bước triển
khai tiếp. Chưa đủ điều kiện chuyển G1 hoặc tuyên bố transport hoàn tất.

## 9. Kiểm tra artifact của lần khảo sát

| Lệnh / kiểm tra | Kết quả thực tế |
| --- | --- |
| `powershell -NoProfile -File app/g0/Inspect-Environment.ps1` | PASS, exit 0; offline inventory: tree khớp, upstream sạch, không có Go/Wails trên PATH |
| `powershell -NoProfile -File app/g0/Inspect-Environment.ps1 -VerifyUpstream` | PASS, exit 0; `UpstreamLookup.Status = MATCH`, timestamp `2026-11-15T14:52:24.2575487Z` |
| `powershell -NoProfile -File app/g0/Inspect-Environment.ps1 -RepoRoot D:/CrocStream/docs` | PASS của negative check: exit 1 như mong đợi, từ chối sai root bằng lỗi `Expected the CrocStream repository with croc/go.mod.` |
| `git -c safe.directory=D:/CrocStream diff --check` | PASS, exit 0; Git thông báo chuẩn hóa LF/CRLF, không có lỗi whitespace ở tracked diff |
| Script PowerShell inline kiểm tra target và heading anchor của Markdown trong README/UPSTREAM/AGENTS, `docs/`, `app/` | PASS: 93 liên kết/anchor nội bộ trong 9 file |

Lần chạy đầu của cả hai chế độ inventory thất bại vì `$PSScriptRoot` rỗng khi
tính default parameter dưới PowerShell 5.1. Đã chuyển việc tính root vào thân
script; hai lần chạy sau đạt kết quả ở bảng trên. Không tính các lần thất bại
ban đầu là PASS.

Files tạo mới: `app/G0_FEASIBILITY.md`, `app/g0/Inspect-Environment.ps1`.
Files cập nhật để đồng bộ trạng thái: `README.md`, `docs/GETTING_STARTED.md`,
`docs/PRD.md`, `docs/ARCHITECTURE.md`, `docs/TEST_PLAN.md`, `docs/ROADMAP.md`.
Các chỉnh sửa onboarding có từ trước vẫn nằm trong diff chưa commit.
