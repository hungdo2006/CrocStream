# Kết quả kiểm chứng G0–G6

Các kết quả dưới đây thuộc bản thử nghiệm G6 ngày 17/11/2026 và các bước dẫn
đến bản đó. Dùng [README](../README.md#kiểm-thử) để chạy kiểm thử trên mã hiện tại.

| Mốc | Kết quả đã ghi nhận | Hồ sơ |
| --- | --- | --- |
| G0 | Duplex đã kiểm chứng ở hai tiến trình và hai máy qua relay. Ở mạng kiểm tra lại, phiên 600,1 giây có RTT trung bình 48–51 ms. | [Kết luận](https://github.com/hungdo2006/CrocStream/blob/8e10002f7997bb1a12a8f2996d883610e1072b4d/app/G0_CLOSEOUT.md), [log số liệu](../app/g0/poc/evidence/) |
| G1 | Framing/config, phân mảnh dữ liệu và thứ tự gói đã qua test và fuzz. | [Kết luận](https://github.com/hungdo2006/CrocStream/blob/8e10002f7997bb1a12a8f2996d883610e1072b4d/app/G1_CLOSEOUT.md) |
| G2 | Codec và telemetry đã qua test và fuzz bằng ảnh tổng hợp. Ảnh tham chiếu của máy phát và máy xem khớp theo byte. | [Kết luận](https://github.com/hungdo2006/CrocStream/blob/8e10002f7997bb1a12a8f2996d883610e1072b4d/app/G2_CLOSEOUT.md) |
| G3 | Xem trước màn hình thật trên Windows ở 720p, khoảng 24 FPS trong 10 phút. Stop đo được 118 ms. | [Kết luận](https://github.com/hungdo2006/CrocStream/blob/8e10002f7997bb1a12a8f2996d883610e1072b4d/app/G3_CLOSEOUT.md), [kiểm chứng](../app/g3/README.md) |
| G4 | Hai app chạy Share/Watch qua loopback. Hai máy qua EC2 truyền ảnh tổng hợp ít nhất 600 giây, khớp hash ảnh tham chiếu và số đếm giao thức. Đã kiểm tra dọn tài nguyên từ xa. | [Kết luận](https://github.com/hungdo2006/CrocStream/blob/8e10002f7997bb1a12a8f2996d883610e1072b4d/app/G4_CLOSEOUT.md), [kiểm chứng](../app/g4/README.md) |
| G5 | Phiên desktop loopback 600 giây ở 720p: canvas bên xem đạt p50 24 FPS, p95 25 FPS. Stop đo được 141 ms. | [Kết luận](https://github.com/hungdo2006/CrocStream/blob/8e10002f7997bb1a12a8f2996d883610e1072b4d/app/G5_CLOSEOUT.md), [kiểm chứng](../app/g5/README.md) |
| G6 | Hai bản EXE production chạy từ ZIP đã giải nén trên Windows, được kiểm tra qua Windows UI Automation. Stop đo được 535 ms. | [Kết luận](https://github.com/hungdo2006/CrocStream/blob/8e10002f7997bb1a12a8f2996d883610e1072b4d/app/G6_CLOSEOUT.md), [log số liệu](../app/g6/evidence/) |

Các kết quả FPS và RTT gắn với máy, dữ liệu và đường truyền trong từng hồ sơ.
Các hạng mục còn mở gồm demo do chủ dự án thực hiện, chụp/hiển thị màn hình thật
qua Internet, kiểm tra trên Windows mới cài, installer và ký số. Webcam và hỗ
trợ macOS/Linux vẫn thuộc G7. [Giới hạn bản portable](../app/g6/KNOWN_ISSUES.md)

Các tài liệu G0–G6 bản đầy đủ, PRD, lộ trình và kế hoạch kiểm thử ban đầu có tại
commit chuẩn trước khi gộp tài liệu, `8e10002f7997bb1a12a8f2996d883610e1072b4d`.
Trong repo local, ví dụ `git show 8e10002:app/G4_REMOTE_PROOF.md` hoặc
`git show 8e10002:docs/TEST_PLAN.md`.
[Snapshot Git](https://github.com/hungdo2006/CrocStream/tree/8e10002f7997bb1a12a8f2996d883610e1072b4d)
