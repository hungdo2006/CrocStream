# Bằng chứng nghiệm thu G0–G6

Các closeout ghi kết quả lịch sử của prototype Windows x64, môi trường đo và giới hạn của từng gate. [Roadmap](../ROADMAP.md#current-progress) theo dõi tiến độ. G7 đang hoãn.

| Gate | Báo cáo và hướng dẫn tái lập | Dữ liệu kiểm chứng |
| --- | --- | --- |
| G0 · Transport | [Closeout](../../app/G0_CLOSEOUT.md), [khảo sát](../../app/G0_FEASIBILITY.md), [toolchain](../../app/G0_TOOLCHAIN.md), [thiết kế adapter](../../app/G0_ADAPTER_DESIGN.md), [local](../../app/G0_LOCAL_PROOF.md), [remote](../../app/G0_REMOTE_PROOF.md), [độ trễ](../../app/G0_LATENCY_FOLLOWUP.md) | [Kết quả PoC](../../app/g0/poc/evidence), [hướng dẫn PoC](../../app/g0/poc/README.md) |
| G1 · Framing và CONFIG | [Closeout](../../app/G1_CLOSEOUT.md) | [Test giao thức](../../app/internal/protocol) |
| G2 · Codec và telemetry | [Closeout](../../app/G2_CLOSEOUT.md) | [Test codec](../../app/internal/delta), [test telemetry](../../app/internal/telemetry) |
| G3 · Local preview | [Closeout](../../app/G3_CLOSEOUT.md), [harness](../../app/g3/README.md) | [Evidence](../../app/g3/evidence) |
| G4 · Share/Watch | [Closeout](../../app/G4_CLOSEOUT.md), [remote tổng hợp](../../app/G4_REMOTE_PROOF.md), [harness](../../app/g4/README.md) | [Evidence](../../app/g4/evidence) |
| G5 · Dashboard và stress | [Closeout](../../app/G5_CLOSEOUT.md), [harness](../../app/g5/README.md) | [Evidence](../../app/g5/evidence) |
| G6 · Windows portable | [Closeout](../../app/G6_CLOSEOUT.md), [đóng gói](../../app/g6/README.md), [demo](../../app/g6/DEMO.md) | [Evidence](../../app/g6/evidence), [verifier](../../app/g6/Verify-Package.ps1) |

Giữ đường dẫn hiện hành khi tái lập. `app/g0/poc` cung cấp transport qua `duplex` và local `go.mod` replacement, `app/g0/buildprobe` là module kiểm thử độc lập, còn `app/g4/proof` là lệnh Go trong module app. Script build, script proof và dữ liệu evidence có vòng đời riêng. [Chỉ mục giấy phép](notices.md) ghi phạm vi và fingerprint của từng snapshot.
