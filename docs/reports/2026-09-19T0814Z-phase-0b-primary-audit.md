# Audit Phase 0B U-006

## Header

- Run ID: 20260919T0814Z-primary-audit
- Agent: Codex
- Phase: 0B, U-006
- Branch: `phase-0/reversible-compatibility`
- Start HEAD local: `4f57f8a402be282cbefff802de6473c167e6735a`
- Remote HEAD đã kiểm tra: `54f9744821c12c8a801839f746edb8c8a78af6a2`
- End HEAD: không đổi; audit chưa commit/push vì safety gate fail.

## Mục tiêu và phạm vi

Audit bằng chứng primary-profile, sửa verifier báo PASS sai và xác định điều kiện chạy lại. Không thay đổi config thật, đọc auth, chuyển phase hoặc chạy lại các chiến lược auth đã thất bại.

## Trạng thái ban đầu

Index/ref local vẫn ở commit cũ; remote đã có hai commit delivery. Ba artifact lịch sử trong `evidence/phase-0b-codex-extension/` giữ nguyên. Các file primary-profile hiện trên working tree đã được push ở lượt trước.

## Phát hiện

| Mức | Phát hiện | Bằng chứng | Hệ quả |
|---|---|---|---|
| HIGH | Verifier hard-code cleanup/restore/config-content PASS; không kiểm tra exit code Node/Git | `scripts/phase0b-verify-primary-profile.ps1` bản trước audit | PASS cũ không phải gate triển khai |
| HIGH | Watchdog đợi file checkpoint; không có console reader/helper cho người dùng sau khi IDE đóng | `WaitControl` | Probe không độc lập với agent |
| HIGH | Backup/restore không có compare-and-swap; recovery helper chưa kiểm tra hash backup trước ghi | watchdog và restore helper | Có thể ghi đè chỉnh sửa mới hoặc backup sai |
| HIGH | Watchdog gán probeActive sau atomic replace; lỗi giữa replace/cleanup có thể bỏ qua restore | `AtomicReplace` và caller | Cửa sổ lỗi rollback chưa được kiểm thử |
| HIGH | Recorder tự trả catalog Astra; SSE thiếu payload hoàn chỉnh; ready xuất hiện trước bind | recorder primary | Không dùng để chứng minh entitlement/lifecycle |
| MEDIUM | Hai trường unchanged từng được sửa từ chuỗi hash thành true mà không lưu after hash | result authoritative và lịch sử lượt trước | Tính bất biến settings/extension là UNKNOWN |
| MEDIUM | Manifest hash theo working tree, không xác nhận byte chuẩn trong Git | manifest cũ | Cần kiểm tra artifact sau checkout |

## Quyết định

Giữ nguyên artifact lịch sử; ghi correction mới. Verifier phải trả nonzero khi thiếu bằng chứng, tách kiểm tra artifact khỏi U-006. Dừng chạy live theo AGENTS.md mục 10 vì gate fail. Đường chạy tiếp phải có checkpoint ngoài IDE và cho phép người dùng tự mở lại IDE; synthetic key chỉ truyền vào process probe qua launcher, không qua mở IDE thông thường.

## Thay đổi và kiểm chứng

Đã thay verifier bằng evaluator chỉ đọc, trả exit 1 khi thiếu bằng chứng. Watchdog chặn trước side effect. Giữ nguyên result JSON và report đã push.

- `node --test scripts/phase0b-primary-evidence.test.cjs`: exit 0, 10/10 PASS.
- `powershell -NoProfile -File scripts/phase0b-verify-primary-profile.ps1`: exit 1 đúng dự kiến; settings/extension, relaunch, auth, picker, wire và lifecycle chưa đủ bằng chứng.
- `git diff --check`: exit 0.
- Bốn tài liệu delivery trước audit khớp remote HEAD: `git diff --quiet` exit 0.
- PowerShell parser và Node syntax: PASS; watchdog gọi với session giả trả exit 1 tại guard trước side effect.
- Privacy scan 7 file audit: 0 matches; 21 liên kết nội bộ: 0 lỗi. Không tuyên bố đây là scan toàn bộ repository.
- `git diff <remote-sha>` hiển thị các file primary là deleted vì chúng chưa nằm trong index local; file thật vẫn tồn tại dưới dạng untracked. Không dùng diff này để stage deletion hoặc kết luận mất file.

## Security và rollback

Không mở listener, không đọc auth, không thay đổi config thật. Thay đổi chỉ nằm trong script kiểm tra và tài liệu; có thể revert commit audit mà không chạm cấu hình người dùng.

## Acceptance và bước tiếp

U-006 BLOCKED. Không chấp nhận PASS cũ làm bằng chứng sẵn sàng chạy lại. Sau sửa orchestrator/recorder và kiểm thử lỗi rollback trên fixture, chạy đúng một primary-profile probe có người dùng hỗ trợ; không tự chuyển Phase 1.
