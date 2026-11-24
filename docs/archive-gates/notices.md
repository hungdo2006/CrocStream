# Chỉ mục giấy phép và attribution

Mỗi snapshot ghi lại một phạm vi phụ thuộc tại thời điểm thu thập. G6 kiểm chứng ZIP cụ thể, gồm notice của package, notice closure thu lúc đóng gói và các attribution đã review từ G0. Chi tiết phạm vi, hash ZIP và manifest nằm trong [G6 closeout](reports/G6_CLOSEOUT.md#deliverable) và [G6 notice scope](../../packaging/NOTICE_SCOPE.md). [NOTICE của G6](../../packaging/NOTICE.txt) ghi tình trạng cấp phép của mã CrocStream.

| Snapshot | Phạm vi |
| --- | --- |
| [G0 generated · 2026-11-16](licenses/g0/generated-20261116) | Bản sinh ban đầu của closure Windows cho PoC và build probe, gồm race. |
| [G0 reviewed · 2026-11-16](licenses/g0/reviewed-20261116) | Bản có thẩm quyền cho G0 và nguồn attribution G0 đi kèm G6. [Phạm vi G0](licenses/g0/README.md) |
| [G2 notice](../../app/THIRD_PARTY_NOTICES.md) | LZ4 sử dụng bởi codec. |
| [G3 reviewed · 2026-11-16](licenses/g3/reviewed-20261116) | Bản thu thập ban đầu của preview Windows và frontend. |
| [G3 final · 2026-11-16](licenses/g3/final-20261116) | Bản cuối G3 bổ sung các mục thiếu văn bản giấy phép. [Phạm vi G3](licenses/g3/README.md) |
| [G4 generated · 2026-11-17](licenses/g4/generated-20261117) | Inventory 178 mục sau tích hợp Share/Watch. [Phạm vi G4](licenses/g4/README.md) |

Giữ [provenance upstream](../../UPSTREAM.md), [giấy phép upstream](../../croc/LICENSE) và [giấy phép của adapter PAKE](../../app/internal/crocduplex/upstreampake/LICENSE) cùng mã tương ứng.

## SHA-256 trong Git

Các giá trị sau được tính trên byte của Git blob từ `git cat-file blob <rev>:<path>` với stdout nhị phân, ổn định theo revision khi Git cấu hình `core.autocrlf` khác nhau. Hash file checkout có thể khác do chuyển đổi newline. Snapshot G0 có `-text` trong [.gitattributes](../../.gitattributes), còn hash lịch sử G3/G4 được tính trên file đã tạo vào thời điểm nghiệm thu và được giữ tại các closeout hoặc README của gate.

| Snapshot | `inventory.json` | `THIRD_PARTY_NOTICES.txt` |
| --- | --- | --- |
| G0 generated | `4544F937531E58AF362E0FC1ED3E112250D0C98C47765C3028B30507A0E985F8` | `57C42C6F9BD29717E63C84FA5FED758035D0ED2EA4C281F3A8E3947F07371D73` |
| G0 reviewed | `7575E4A162D58F278EBA8BDE31FA052562DAEE4C82F37AC44BAB27AAC15AD59B` | `0996E3672FDFE890833087181C2D2B234B2D6C7F53654EB99BE7F07BB4EB9106` |
| G3 reviewed | `68E2C4AC73BD4A53C1360F52F8A6C9CD5F9DB33F2D57F8B61CCF6B3A288C0702` | `21BE7898DA6F213A3C82BCA722CAA994B1A777286296C212391BE1145FAFAB9D` |
| G3 final | `5AD5779FD7D28C2CEDFCDA2E250138EA1CF232FB1002C2E91503D5ECB718B0CC` | `F7234614B9ABE50EE25C7C8756E36A522C81249E8846686C19B8C0085B5EC14A` |
| G4 generated | `13966320BAF26C4EAAB377A3826B16AB43FA827DEE7A65EB98CF03C7B44960BE` | `39ABEBA87D70EE41B61250FA2A66F4C648857770C874554AB4F499FE8D485DF8` |
