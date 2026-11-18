# Đóng góp vào CrocStream

Đọc [AGENTS](AGENTS.md) và chuỗi [PRD](docs/PRD.md), [kiến trúc](docs/ARCHITECTURE.md), [kế hoạch kiểm thử](docs/TEST_PLAN.md), [roadmap](docs/ROADMAP.md). [Chỉ mục gate](docs/gates/README.md) tập hợp bằng chứng nghiệm thu và hướng dẫn tái lập. Giữ nguyên snapshot upstream `croc/` và làm việc trong `app/`, `docs/` hoặc tooling của repo gốc.

## Môi trường Windows

Toolchain kiểm chứng ngày 2026-11-18 dùng Windows x64, Go 1.27.4, Wails CLI 3.1.0, Node 24.22.1, npm 12.2.1 và Microsoft Edge cho Playwright. Race checks cần GCC hỗ trợ Windows/amd64 và `CGO_ENABLED=1`. Hướng dẫn dùng toolchain portable, tạo bindings và chạy desktop nằm trong [app README](app/README.md#g3-desktop-preview--build-and-run).

```powershell
powershell -NoProfile -File .\scripts\verify.ps1 -Race
```

Script chạy Go test, vet và mod verify trên ba module, build Wails và kiểm tra frontend. Dùng `-ToolRoot` nếu portable toolchain ở vị trí khác. Dùng `-Offline` khi Go và npm cache đã đầy đủ. Wails sinh `bindings/` và `dist/` từ mã nguồn. Giữ `app/frontend/dist-bootstrap.txt` để `go:embed all:dist*` có đầu vào trước lần build đầu tiên.

[CI Windows](.github/workflows/windows-ci.yml) dùng cùng lệnh kiểm tra, xác minh Git tree của `croc/` và chạy `npm audit` trên toàn bộ lockfile. Mọi advisory, kể cả dependency phát triển, làm job thất bại. CI giữ log và bản build thử unsigned tối đa 7 ngày. Bản build thử chỉ được tải lên khi checks và audit đều đạt.

## Thay đổi và review

- Dùng `gofmt` trên các file Go đã sửa. Chạy typecheck, ESLint, Vitest, Playwright và build cho thay đổi frontend.
- Kiểm tra baseline trước khi sửa. Bổ sung test cho hành vi đã duyệt thay đổi, hoặc ghi rõ cách xác minh nếu thay đổi chỉ liên quan tài liệu hoặc mã không có caller.
- Ghi mục đích, đường dẫn, kết quả lệnh, điều kiện thử nghiệm và các kiểm tra còn thiếu trong commit hoặc phần mô tả review. Đặt tên commit theo kết quả của thay đổi.
- Xin duyệt trước khi đổi hành vi, wire format, PAKE, relay, dependency production hay phạm vi phát hành. Lọc mã mời, mật khẩu, token và ảnh chụp khỏi log và artifact. Giữ các snapshot evidence, LICENSE và NOTICE tại đường dẫn hiện hành.
- Kiểm tra synthetic chạy mặc định. Chạy capture màn hình thật, chứng minh relay từ xa, kiểm tra WebView2 native và screen reader trong lượt kiểm thử có điều kiện và sự đồng ý riêng. Xem [hướng dẫn nghiệm thu](docs/TEST_PLAN.md#e-guisystem-manual-acceptance).
