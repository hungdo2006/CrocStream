# QA Probes & Specialized Verification Harnesses

Thư mục này chứa các công cụ kiểm thử chuyên sâu (ngoài bộ test tự động tiêu chuẩn trong `scripts/verify.ps1`):

| Công cụ / Tài liệu | Mục đích |
| --- | --- |
| [`buildprobe/`](buildprobe/) | Module Go độc lập kiểm tra khả năng biên dịch kết hợp tối thiểu giữa Go 1.27, Wails v3, `croc` và Win32 GDI. |
| [`TRANSPORT_PROOF.md`](TRANSPORT_PROOF.md) | Hướng dẫn chạy kiểm thử chịu tải kênh truyền bảo mật hai chiều (`app/cmd/qa-probes/transportproof`), kèm `Invoke-TransportSshProof.ps1` và `Run-TransportRemotePeer.ps1`. |
| [`PREVIEW_PROOF.md`](PREVIEW_PROOF.md) | Hướng dẫn kiểm tra Origin của WebView2 (`app/cmd/qa-probes/originprobe`) và tự động hóa giao diện Local Preview (`preview-ui-proof.mjs`). |
| [`SESSION_PROOF.md`](SESSION_PROOF.md) | Hướng dẫn kiểm thử phiên truyền media tổng hợp/desktop giữa 2 tiến trình (`app/cmd/qa-probes/sessionproof`), kèm `session-ui-proof.mjs` và `Invoke-SessionSshProof.ps1`. |
