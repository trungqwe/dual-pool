# Phase 0B — sửa lifecycle và finalization Gate A

## Header

- Run ID: `2026-09-18T2120Z-phase-0b-gate-a-finalization-repair`.
- Start HEAD: `aa420da331c4dadab1443c2547f645e13f8eb1b6`.
- Branch: `phase-0/reversible-compatibility`; remote: `https://github.com/trungqwe/dual-pool.git`.
- Worktree ban đầu sạch; không có thay đổi chưa rõ chủ sở hữu.
- Phase: 0B. Commit cuối được ghi trong delivery receipt sau push.

## Mục tiêu và phạm vi

Tạo v5 với lifecycle có timeout, recorder tự thoát, hai writer JSON được self-test và evidence an toàn. Không chạy picker, Antigravity, OAuth hoặc Phase 1; không sửa cấu hình người dùng.

## Điều tra trước implementation

- Diagnostic cố định của v4: `NOT_FOUND` qua `Test-Path`; không có exception gốc để kết luận nguyên nhân chính xác.
- V4 dùng PowerShell delegates cho async output; đây là nghi phạm crash ngoài luồng/runspace, chưa phải bằng chứng lỗi serializer.
- `HttpListener` dựa trên HTTP.sys không bảo đảm socket có PID bằng recorder. V5 dùng Node HTTP server loopback để kiểm chứng PID trực tiếp bằng OS listener table. Node chỉ là dependency của probe, không phải runtime sản phẩm.
- Help của Codex đã cài hỗ trợ `--ephemeral`, `--strict-config`, prompt qua stdin. V5 dùng các bề mặt này và thư mục làm việc tổng hợp.

## Kế hoạch và stop conditions

1. Self-test LF/LF, serializer, fallback và HTTP response closure trước Codex.
2. Ghi lifecycle từ Process object, đọc pipe async bằng Task; dữ liệu chỉ ở memory.
3. Scan nguyên trạng ephemeral files, dọn process/temp, sau đó compose JSON primitives và read-back.
4. Tạo status/index, kiểm tra docs/security, hash evidence và tạo manifest cuối cùng.

Gate thất bại được ghi FAIL với stage cụ thể; không chuyển sang UI hay phase tiếp theo. Không commit dữ liệu nhạy cảm hoặc tuyên bố cleanup khi chưa đo.

## Kết quả điều tra và thay đổi

- Diagnostic v4 vẫn `NOT_FOUND`; nguyên nhân exception chính xác vẫn `UNKNOWN`. Bỏ callback PowerShell và global trap; process pipe dùng `ReadToEndAsync`.
- LF/LF được kiểm tra theo cặp byte liên tiếp. HTTP self-test đọc được hai event JSON và EOF trước Codex.
- Writer chính serialize/write/read/parse; self-test fallback cố ý ghi vào directory gây exception rồi xác nhận result FAIL tối giản. Các Process/Task/Exception không đi vào finalResult.
- Node recorder đóng response và server sau request đích; natural exit được quan sát riêng với Codex.
- Codex chạy `--strict-config --ephemeral` trong thư mục tổng hợp, prompt qua stdin, key qua environment chỉ của child. Không có stdout/stderr log file.
- `.gitattributes` giữ byte evidence JSON và line ending source v5 ổn định để hash không đổi khi checkout Windows.

## Verification

| Lệnh/test | Kết quả | Evidence |
|---|---|---|
| Preflight Git | PASS; HEAD đúng, worktree sạch | `aa420da331c4dadab1443c2547f645e13f8eb1b6` |
| Lần đầu gọi script trước live test | Lỗi default param `PSScriptRoot`; đã sửa, chưa tạo process/temp | `probe-results-v5.json` |
| Gate A development | PASS, exit 0 | `codex-cli-transport-v5-20260918T212340079Z.json` |
| Gate A sau bổ sung forced-writer-failure self-test và pipe-secret assertions | PASS, exit 0 | [Result chính thức](../../evidence/phase-0b-v5/codex-cli-transport-v5-20260918T212522665Z.json) |
| JSON/docs/security/hash/cleanup cuối | Kết quả được script verifier tính sau probe | `security-gate-v5.json`, `phase-0b-v5-evidence-manifest.json` |

Lệnh tái lập: `powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts/phase0b-codex-cli-transport-v5.ps1`. Lệnh kiểm tra evidence đã giao: `powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts/phase0b-verify-v5.ps1`.

Scanner tự phát hiện tên counter `raw_body` trong chính output thống kê trùng detector field cấm; đổi tên counter thành `raw_body_fields`, giữ nguyên regex phát hiện dữ liệu. Positive controls vẫn kiểm tra thật trong memory. Không có raw body hoặc sentinel value trong output bị chặn này.

## Security, cleanup và correction lịch sử

Request `POST /v1/responses`, model `gpt-6-astra`, hai sentinel ingress đều đúng. Tree lưu key/type; memory pipe không chứa secret. Scan file gốc trước xóa: không sentinel, không scan error. Kết quả writer được parse lại và hash sau cleanup.

Codex và recorder natural exit 0, timeout false, forced kill false. Cả hai cấu hình người dùng có SHA-256 trước/sau giống nhau; thứ tự hash trong result là Codex rồi Antigravity. Không cần restore vì không có mutation. Temp v5 đã xóa, listener/process sở hữu còn 0; environment cha không bị sửa.

Cleanup audit bổ sung phát hiện **bốn recorder v4 còn chạy** từ lịch sử, không phải listener người dùng. Đã kiểm tra command line/CreationDate ngay trước khi dừng và xác minh bốn port không còn. [Correction](../../evidence/phase-0b-v5/legacy-cleanup-correction.json) supersede lời khẳng định cleanup cũ; không sửa report/evidence cũ.

## Acceptance và giới hạn

Gate A v5 PASS là prerequisite CLI trên Windows 10 build 19045, Codex 0.154.0. U-005 vẫn `PARTIAL / UNKNOWN`; U-006 cùng U-001..U-004 vẫn BLOCKED, U-007 PROBED, U-008 PARTIAL / UNKNOWN. Chưa có final acceptance Windows 11. UI và credential-specific tests không thuộc run này.

## Git delivery và rollback

Commit scoped scripts v5, evidence, report, current-state docs và quy tắc Git byte stability; push bình thường lên `phase-0/reversible-compatibility`. SHA/push/compare thực tế được ghi receipt riêng sau push để report này bất biến. Không tạo PR. Rollback thay đổi repo bằng commit đảo nếu cần; không có app integration để gỡ.

## Next run

Phase 0B — user-assisted Codex extension picker/config-layer probe. Đọc status v5, handoff và protocol; xác minh Git, backup/hash, environment an toàn trước khi mutation. Dừng nếu không restore được hoặc có concurrent edit. Phase 1 chưa bắt đầu.
