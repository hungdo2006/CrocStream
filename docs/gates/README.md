# Bằng chứng nghiệm thu G0–G6

Các closeout ghi kết quả lịch sử của prototype Windows x64, môi trường đo và giới hạn của từng gate. [Roadmap](../ROADMAP.md#current-progress) theo dõi tiến độ. G7 đang hoãn.

| Gate | Báo cáo và hướng dẫn tái lập | Dữ liệu kiểm chứng |
| --- | --- | --- |
| G0 · Transport | [Closeout](reports/G0_CLOSEOUT.md), [khảo sát](reports/G0_FEASIBILITY.md), [toolchain](reports/G0_TOOLCHAIN.md), [thiết kế adapter](reports/G0_ADAPTER_DESIGN.md), [local](reports/G0_LOCAL_PROOF.md), [remote](reports/G0_REMOTE_PROOF.md), [độ trễ](reports/G0_LATENCY_FOLLOWUP.md) | [Kết quả PoC](evidence/g0), [hướng dẫn PoC](../../app/g0/poc/README.md) |
| G1 · Framing và CONFIG | [Closeout](reports/G1_CLOSEOUT.md) | [Test giao thức](../../app/internal/protocol) |
| G2 · Codec và telemetry | [Closeout](reports/G2_CLOSEOUT.md) | [Test codec](../../app/internal/delta), [test telemetry](../../app/internal/telemetry) |
| G3 · Local preview | [Closeout](reports/G3_CLOSEOUT.md), [harness](../../app/g3/README.md) | [Evidence](evidence/g3) |
| G4 · Share/Watch | [Closeout](reports/G4_CLOSEOUT.md), [remote tổng hợp](reports/G4_REMOTE_PROOF.md), [harness](../../app/g4/README.md) | [Evidence](evidence/g4) |
| G5 · Dashboard và stress | [Closeout](reports/G5_CLOSEOUT.md), [harness](g5/README.md) | [Evidence](evidence/g5) |
| G6 · Windows portable | [Closeout](reports/G6_CLOSEOUT.md), [đóng gói](../../app/g6/README.md), [demo](../../app/g6/DEMO.md) | [Evidence](evidence/g6), [verifier](../../app/g6/Verify-Package.ps1) |

Giữ đường dẫn hiện hành khi tái lập. `app/g0/poc` cung cấp transport qua `duplex` và local `go.mod` replacement, `app/g0/buildprobe` là module kiểm thử độc lập, còn `app/g4/proof` là lệnh Go trong module app. Các báo cáo closeout và dữ liệu bằng chứng lịch sử được gom gọn tại `docs/gates/reports/`, `docs/gates/evidence/` và `docs/gates/g5/`. [Chỉ mục giấy phép](notices.md) ghi phạm vi và fingerprint của từng snapshot.
