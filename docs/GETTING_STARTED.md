# Bắt đầu với CrocStream

Trang này dành cho thành viên mới, giải thích thiết kế bằng tiếng Việt trước khi
đọc đặc tả kỹ thuật. Các tài liệu được liên kết bên dưới là nguồn quy định chi
tiết; trang này không thay thế giao thức hoặc tiêu chí nghiệm thu.

## 1. Chúng ta đang làm sản phẩm gì?

CrocStream hướng tới ứng dụng Windows cho hai người:

1. **Người phát (Broadcaster)** chọn màn hình, bấm Start, chờ chuẩn bị phiên thành
   công rồi nhận mã bí mật để chia sẻ cho người xem.
2. **Người xem (Viewer)** nhập mã trong ứng dụng CrocStream.
3. Hai bên xác thực; người xem nhận cấu hình và ảnh đầy đủ đầu tiên, sau đó nhận
   các cập nhật hình ảnh.
4. Người dùng có thể Stop/Leave; ứng dụng đóng tài nguyên và xóa nội dung phiên.

Mục tiêu MVP là xem màn hình và đo hiệu quả truyền ảnh, chưa có âm thanh, điều
khiển máy từ xa, ghi hình hay nhiều người xem. Webcam và macOS/Linux làm sau.
Đây là prototype kỹ thuật; hiệu năng cần được đo trên máy và mạng cụ thể.
Chi tiết hành vi nằm trong [PRD](PRD.md).

## 2. Hiện tại đã có gì?

Theo cập nhật 2026-11-17:

| Thành phần | Trạng thái |
| --- | --- |
| `croc/` | Snapshot upstream để khảo sát; giữ nguyên mã nguồn và giấy phép |
| `docs/` | Đặc tả đề xuất, kế hoạch kiểm thử và roadmap |
| `app/` | Có G1 framing/config, G2 codec/telemetry, G3 capture/preview Wails, G4 Share/Watch và báo cáo/tooling G0 |
| Transport dùng cho CrocStream | G0 khả thi đã PASS; local/remote >600s, RTT mạng mới mean 48–51ms, không bảo đảm cho mọi mạng |
| Ứng dụng và benchmark | G0–G5 PASS; G4 remote synthetic và G5 local UI stress >=600s đã kiểm chứng; [G5 và giới hạn](../app/G5_CLOSEOUT.md) |

**Có ứng dụng local preview và Share/Watch**. G4 kiểm chứng desktop capture/render
ở hai process local và media tổng hợp qua EC2 hai máy; chưa đo capture/render
màn hình thật qua Internet.
[Lệnh build/chạy G3](../app/README.md#g3-desktop-preview--build-and-run);
[PoC G0](../app/g0/poc/README.md) dùng riêng để thử transport dữ liệu tổng hợp.
Các lệnh test/build package G1 nằm trong [app README](../app/README.md).
Chạy được croc upstream cũng không chứng minh ứng dụng chia sẻ màn hình đã
hoạt động. Các lệnh desktop/frontend trong Test Plan dành cho những gate sau.

Các quyết định kiến trúc phần lớn còn đề xuất; D-12 về một repository và
C-01/C-02 về key interval/độ phân giải đã được chủ dự án xác nhận.
Xem [quyết định và blocker](ARCHITECTURE.md#decisions-and-blockers).
Khảo sát ban đầu nằm trong [G0_FEASIBILITY](../app/G0_FEASIBILITY.md);
kết quả cài toolchain/build/test tiếp theo ở [G0_TOOLCHAIN](../app/G0_TOOLCHAIN.md).
Kết quả adapter ở [G0_LOCAL_PROOF](../app/G0_LOCAL_PROOF.md) và
[G0_REMOTE_PROOF](../app/G0_REMOTE_PROOF.md). Đã kiểm chứng hai máy qua relay
EC2. [G0_CLOSEOUT](../app/G0_CLOSEOUT.md) chốt G0 PASS với native probe,
notice review và giới hạn rõ ràng; capture màn hình thật vẫn thuộc G3.
Follow-up [mạng mới](../app/G0_LATENCY_FOLLOWUP.md) đã chạy đủ 600.1s ở mức tải
gốc với RTT trung bình 48–51ms; kết quả mạng cũ vẫn được giữ riêng.
[G1 closeout](../app/G1_CLOSEOUT.md) ghi bằng chứng framing/config và bước G2.
[G2 closeout](../app/G2_CLOSEOUT.md) ghi bằng chứng codec/telemetry và benchmark
ảnh tổng hợp; chưa phải benchmark chia sẻ màn hình thật.
[G3 closeout](../app/G3_CLOSEOUT.md) ghi kiểm chứng capture/preview Windows,
Origin thực tế và vòng đời tài nguyên.
G4 tích hợp Share/Watch qua transport xác thực. Sau khi chủ dự án khởi động lại
EC2, remote synthetic proof >=600s đã PASS với hash/control/counter và cleanup.
Xem [G4 closeout](../app/G4_CLOSEOUT.md) và [remote proof](../app/G4_REMOTE_PROOF.md).

## 3. Hình ảnh đi từ máy phát tới máy xem thế nào?

Sơ đồ dưới đây là thiết kế dự kiến, chưa phải hệ thống đã chạy:

```text
MÁY PHÁT
Chụp màn hình → Chuẩn hóa RGBA → Chọn KEY hoặc DELTA → Nén LZ4
                                                        │
                      Kênh truyền xác thực, mã hóa       │
                      PoC đã đạt khả thi G0              ▼
MÁY XEM
Canvas ← Ảnh JPEG hiển thị cục bộ ← Dựng ảnh ← Kiểm tra và giải nén

Máy xem ── yêu cầu KEY / Ping ──→ Máy phát
```

- **KEY (keyframe)** chứa toàn bộ ảnh tham chiếu, giúp bắt đầu hoặc khôi phục.
- **DELTA** chứa những ô ảnh được chọn là đã thay đổi so với ảnh tham chiếu.
- Hai bên phải giữ ảnh tham chiếu giống nhau để áp dụng các DELTA tiếp theo.
- Khi mạng chậm, có thể bỏ mẫu chụp trước khi mã hóa. Khi đã phát một chuỗi DELTA,
  phía nhận phải xử lý đầy đủ; phần hiển thị được phép bỏ ảnh trung gian.
- JPEG chỉ dùng cho preview/hiển thị qua cầu nối cục bộ tới WebView. Dữ liệu
  KEY/DELTA giữa hai máy dùng LZ4 không mất dữ liệu.

**Không đồng nghĩa mọi pixel chụp được đều xuất hiện nguyên vẹn trên canvas:**
ngưỡng phát hiện thay đổi có thể bỏ qua thay đổi nhỏ, và JPEG cục bộ có mất dữ
liệu. Điều phải chứng minh là ảnh tham chiếu của bên nhận bằng ảnh tham chiếu
đã truyền của bên phát. Xem [canonical image](ARCHITECTURE.md#canonical-image-and-compression).

### Vai trò của từng công nghệ

| Thành phần đề xuất | Vai trò |
| --- | --- |
| Go | Capture, codec, giao thức, quản lý phiên và đo lường |
| croc | Adapter thử nghiệm đã đạt khả thi G0; tích hợp với ứng dụng media thuộc G4 |
| Wails v3 | Khung ứng dụng desktop, kết nối Go với WebView |
| React + TypeScript | Giao diện, trạng thái kết nối và dashboard |
| LZ4 + tile delta | Nén ảnh đầy đủ hoặc các ô ảnh thay đổi |
| WebSocket loopback + canvas | Đưa ảnh từ Go tới giao diện trên cùng máy |

WebSocket loopback **không phải** đường truyền giữa hai máy. Client croc truyền
file thông thường **không hiểu** giao thức media CrocStream. Phiên bản công cụ
được đề xuất trong Architecture phải được kiểm tra trước khi cài đặt hoặc sử dụng.

## 4. Đọc tài liệu theo thứ tự nào?

Sau trang này, đọc [AGENTS.md](../AGENTS.md) để biết quy tắc làm việc, rồi:

| Thứ tự | Tài liệu | Câu hỏi cần trả lời sau khi đọc |
| --- | --- | --- |
| 1 | [PRD](PRD.md) | Người dùng làm gì? MVP bao gồm và loại trừ gì? |
| 2 | [ARCHITECTURE](ARCHITECTURE.md) | Các module phối hợp thế nào? Điều gì đã chốt, điều gì còn chờ? |
| 3 | [TEST_PLAN](TEST_PLAN.md) | Cần bằng chứng nào để khẳng định tính đúng đắn và hiệu năng? |
| 4 | [ROADMAP](ROADMAP.md) | Việc nào làm trước, đầu ra là gì, khi nào được chuyển bước? |

Lượt đọc Architecture đầu tiên nên nắm các quyết định, blocker và pipeline.
Khi nhận task protocol/codec, đọc kỹ toàn bộ phần media protocol và các test liên
quan; bản tóm tắt nhập môn không đủ để triển khai.

## 5. Thuật ngữ thường gặp

| Thuật ngữ | Nghĩa trong dự án |
| --- | --- |
| MVP / P0 | Phạm vi sản phẩm tối thiểu ưu tiên thực hiện trước |
| Gate / G0–G7 | Cổng kiểm chứng; đạt tiêu chí của bước hiện tại mới chuyển tiếp theo roadmap |
| PoC / spike | Thử nghiệm nhỏ nhằm trả lời một câu hỏi khả thi cụ thể |
| Transport / duplex | Lớp truyền dữ liệu / truyền được cả hai chiều trên một phiên |
| Relay | Máy chủ trung chuyển dữ liệu giữa hai bên; điều kiện sử dụng và năng lực cần xác minh |
| PAKE | Cơ chế hai bên dùng bí mật chung để thiết lập khóa phiên; không phải tự gửi mật khẩu qua mạng |
| AEAD | Mã hóa có kiểm tra tính toàn vẹn/xác thực của bản ghi dữ liệu |
| RGBA | Cách biểu diễn pixel bằng các kênh đỏ, xanh lá, xanh dương và alpha |
| Tile / dirty tile | Ô ảnh / ô có thay đổi đủ điều kiện để được gửi |
| Canonical reference | Ảnh tham chiếu được cập nhật theo dữ liệu thực sự truyền, không nhất thiết là ảnh chụp mới nhất |
| Tau / MAE | Ngưỡng phát hiện thay đổi / sai khác tuyệt đối trung bình trên các kênh RGB |
| PLI | Cách tài liệu gọi yêu cầu KEY mới (`REQUEST_KEYFRAME`) của giao thức này |
| Backpressure | Bên xử lý hoặc truyền không theo kịp tốc độ tạo dữ liệu; cần hàng đợi giới hạn |
| RTT | Thời gian đo từ lúc gửi Ping đến lúc nhận Pong; không phải độ trễ một chiều |
| Baseline | Mốc so sánh: nén đầy đủ cùng các mẫu ảnh đã phát để đối chiếu với KEY/DELTA |

Tiền tố trong đặc tả: `US-` là yêu cầu sản phẩm, `D-` là quyết định,
`B-` là blocker, `C-` là điểm cần làm rõ. Trạng thái quyết định nằm trong
Architecture; không suy ra “đã duyệt” chỉ từ việc một mục có mã `D-`.

## 6. Ngày đầu của thành viên mới

- [ ] Đọc theo thứ tự trên và giải thích lại luồng Share → Watch → Stop.
- [ ] Phân biệt code upstream `croc/` với vị trí code ứng dụng tương lai `app/`.
- [ ] Xác định gate hiện tại và đọc test tương ứng; không coi kế hoạch là kết quả.
- [ ] Ghi lại những giả định ảnh hưởng tới task, đối chiếu bảng `B-` và `C-`.
- [ ] Nhận một task có đầu ra kiểm chứng được từ [gate hiện tại](ROADMAP.md#current-progress).

Task nhập môn phù hợp: đọc fixtures và reader/writer G1, giải thích CONFIG,
KEY1, DELTA tuần tự và việc chỉ xác nhận frame sau khi codec kiểm tra đầy đủ.
Đọc closeout để phân biệt các test đã chạy với phần codec/capture còn phải làm.

## 7. Bước tiếp theo và cách báo cáo

G0 đã đóng bằng PoC local/remote và native/license evidence. G1 đã triển khai
framing/config thuần Go; G2 đã triển khai codec delta và telemetry;
G3 có capture/preview; G4 Share/Watch và remote synthetic proof đã PASS.
G5 UX/telemetry và stress local đã PASS; tiếp theo G6 đóng gói. Xem [tiến độ](ROADMAP.md#current-progress)
và [bằng chứng G4](../app/G4_CLOSEOUT.md).

Sau G0: giao thức → codec → capture/preview → tích hợp hai máy → UI hoàn chỉnh
→ bản Windows bàn giao. Chi tiết và điều kiện chuyển bước ở [bảng gate](ROADMAP.md#gates).

Khi bàn giao task, ghi file đã đổi, lệnh thực sự chạy, kết quả, phần chưa kiểm
chứng và bước tiếp theo theo [mẫu báo cáo](ROADMAP.md#deliverable-report-format-for-codex-after-each-gate).
`NOT RUN` nghĩa là chưa chạy; `BLOCKED` nghĩa là chưa thể đạt gate do thiếu điều
kiện hoặc quyết định. Cả hai đều không được tính là `PASS`.
