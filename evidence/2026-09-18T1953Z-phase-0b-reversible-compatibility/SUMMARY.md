# Phase 0B — Reversible Compatibility

## Kết quả

- `U-005-cli-transport`: `PASS` — Codex CLI đã gửi `POST /v1/responses` tới recorder loopback, dùng model `gpt-6-astra` và wire API `responses`.
- `U-006-picker-retention`: `UNKNOWN` — chưa tự động hóa Codex extension/picker.
- `U-004-antigravity-account-compatibility`: `UNKNOWN` — chưa thực hiện OAuth, tài khoản provider hoặc generation thật.
- Reversible config mutation: `NOT_RUN` — không thay đổi cấu hình người dùng.

Recorder chỉ lưu metadata (tên header, model, đường dẫn khóa JSON); không lưu body, prompt, token hoặc credential. Sau probe không còn process Codex thử nghiệm và không còn listener loopback do probe sở hữu.

## Phạm vi chưa chứng minh

Picker desktop/extension, persistence sau restart, Antigravity redirect/passthrough/envelope và compatibility với tài khoản provider vẫn cần một quy trình UI được người dùng thực hiện hoặc một harness được cho phép riêng.
