# Bằng chứng nghiệm thu lịch sử G0–G6

Các báo cáo closeout ghi kết quả lịch sử của prototype Windows x64, môi trường đo và giới hạn của từng cổng nghiệm thu (Gate). [Roadmap](../specs/ROADMAP.md#current-progress) theo dõi tiến độ tổng thể. G7 đang hoãn.

| Gate | Báo cáo và hướng dẫn tái lập | Dữ liệu kiểm chứng |
| --- | --- | --- |
| G0 · Transport | [Closeout](reports/G0_CLOSEOUT.md), [khảo sát](reports/G0_FEASIBILITY.md), [toolchain](reports/G0_TOOLCHAIN.md), [thiết kế adapter](reports/G0_ADAPTER_DESIGN.md), [local](reports/G0_LOCAL_PROOF.md), [remote](reports/G0_REMOTE_PROOF.md), [độ trễ](reports/G0_LATENCY_FOLLOWUP.md) | [Kết quả PoC](evidence/g0), [hướng dẫn proof](../../scripts/qa-probes/TRANSPORT_PROOF.md), [giấy phép G0](licenses/g0) |
| G1 · Framing và CONFIG | [Closeout](reports/G1_CLOSEOUT.md) | [Test giao thức](../../app/internal/protocol) |
| G2 · Codec và telemetry | [Closeout](reports/G2_CLOSEOUT.md) | [Test codec](../../app/internal/delta), [test telemetry](../../app/internal/telemetry) |
| G3 · Local preview | [Closeout](reports/G3_CLOSEOUT.md), [harness](../../scripts/qa-probes/PREVIEW_PROOF.md) | [Evidence](evidence/g3), [giấy phép G3](licenses/g3) |
| G4 · Share/Watch | [Closeout](reports/G4_CLOSEOUT.md), [remote tổng hợp](reports/G4_REMOTE_PROOF.md), [harness](../../scripts/qa-probes/SESSION_PROOF.md) | [Evidence](evidence/g4), [giấy phép G4](licenses/g4) |
| G5 · Dashboard và stress | [Closeout](reports/G5_CLOSEOUT.md), [harness](g5/README.md) | [Evidence](evidence/g5) |
| G6 · Windows portable | [Closeout](reports/G6_CLOSEOUT.md), [đóng gói](../../packaging/README.md), [demo](../../packaging/DEMO.md) | [Evidence](evidence/g6), [verifier](../../packaging/Verify-Package.ps1) |

## Bảng ánh xạ đường dẫn sau khi tái cấu trúc dự án

Các báo cáo lịch sử trong `reports/` giữ nguyên nội dung thời điểm nghiệm thu. Khi tái lập lệnh từ các báo cáo cũ, đối chiếu đường dẫn cũ sang vị trí hiện hành như sau:

| Đường dẫn lịch sử (trong báo cáo G0–G6) | Vị trí hiện hành |
| --- | --- |
| `app/g0/Install-Toolchain.ps1`, `app/g0/Use-Toolchain.ps1`, `app/g0/Inspect-Environment.ps1` | `scripts/toolchain/` |
| `app/g0/Audit-Licenses.ps1`, `app/g0/Test-LicenseInventory.ps1`, `app/g3/Collect-Notices.ps1` | `scripts/notices/` |
| `app/g0/licenses/`, `app/g3/licenses/`, `app/g4/licenses/` | `docs/archive-gates/licenses/{g0,g3,g4}/` |
| `app/g0/poc/duplex`, `app/g0/poc/internal/{transport,upstreampake}` | `app/internal/crocduplex/` |
| `app/g0/poc/cmd/g0poc`, `app/g3/originprobe`, `app/g4/proof` | `app/cmd/qa-probes/{transportproof,originprobe,sessionproof}` |
| `app/g0/buildprobe` và các script UI/SSH proof của `g0, g3, g4` | `scripts/qa-probes/` |
| `app/g6/*` (đóng gói Portable ZIP, bật relay, DEMO) | `packaging/` |
| `docs/{PRD,ARCHITECTURE,TEST_PLAN,ROADMAP}.md` | `docs/specs/` |
| `docs/gates/*` | `docs/archive-gates/` |

[Chỉ mục giấy phép](notices.md) ghi phạm vi và fingerprint SHA-256 của từng snapshot.
