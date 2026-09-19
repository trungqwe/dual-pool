# Phase 0B — sửa primary watchdog

## Header

- Run: 20260919T0840Z-primary-repair
- Agent: Codex
- Branch: `phase-0/reversible-compatibility`
- Start local HEAD: `4f57f8a402be282cbefff802de6473c167e6735a`
- Remote đã quan sát: `54f9744821c12c8a801839f746edb8c8a78af6a2`
- End HEAD / delivery: PENDING

## Mục tiêu

Sửa lỗi audit U-006, kiểm thử fixture và chuẩn bị watchdog độc lập. Người dùng cho phép tiếp tục tự sửa đến PASS, chỉ yêu cầu hỗ trợ khi thực sự cần. Quyền sửa không thay thế bằng chứng nghiệm thu; chỉ chạy live khi kiểm thử an toàn PASS.

## Phạm vi

Transaction config, recorder Responses, console checkpoint ngoài IDE, verifier và tài liệu Phase 0B. Không bắt đầu phase khác hoặc lặp lại auth clone. Giữ nguyên residue lịch sử và artifact đã push.

## Điều tra

- `ELECTRON_RUN_AS_NODE` tồn tại trong môi trường agent. Launcher cũ kế thừa toàn bộ môi trường. Đây là giả thuyết giải thích lỗi relaunch; chưa phải bằng chứng live.
- Config provider đặt ở user-level theo [tài liệu cấu hình](https://developers.openai.com/es-419/docs/config-file/config-reference).
- Sửa SSE theo [Responses streaming events](https://developers.openai.com/api/reference/resources/responses/streaming-events).

## Kế hoạch kiểm chứng

Fixture config PRESENT/ABSENT, crash sau replace, backup hỏng, concurrent edits, restore idempotent; recorder background/arm/auth/JSON/gzip/limits/SSE/privacy; checkpoint timeout và môi trường child. Kiểm tra links, syntax, privacy và diff trước delivery.

## Kết quả, rủi ro và delivery

Gate chuẩn bị PASS: `node scripts/phase0b-verify-primary-repair.cjs --generate` exit 0; 25 test Node và 12 assertion transaction. Tests bao gồm child process thật nhận môi trường đã bỏ Electron Node mode, nhưng chưa xác nhận relaunch IDE thật. Scanner có 6 positive control, 0 match trên 16 file; parser, Node syntax, local docs links và diff-check PASS. Gate/manifest tại `evidence/phase-0b-u006-primary-repair/`.

U-006 vẫn BLOCKED_PENDING_LIVE_PROBE, không đồng nhất safety PASS với wire PASS. Bản sửa: transaction có intent trước replace, hash backup/after, conflict refusal, TOML parse trước/sau; recorder bounded/gzip, không phát catalog, arm acknowledgement và SSE; watchdog console độc lập, chờ agent handshake và user close, manual normal reopen sau restore.

Config thật chưa bị thay đổi trong lượt sửa. Session dự kiến: `dual-pool-u006-primary-729d99527ecc4b049ae56e53944418f0` dưới thư mục TEMP. Người dùng cần console cho AUTH_OK/AUTH_LOST và ASTRA_SELECTED/ASTRA_NOT_VISIBLE; recorder tự phát hiện prompt mục tiêu. Recovery v2 sử dụng helper cùng tên với `-SessionRoot` thuộc đúng session; phải đóng IDE trước restore.

Git local không cập nhật được ref hiện hữu; một lần compare-and-swap ref vẫn thất bại. Delivery dùng checkout tạm từ remote đã xác minh, chỉ sao chép đúng file sửa; không chỉnh raw ref/index hoặc force-push. Kết quả delivery ghi trong handoff/lượt sau.

Giới hạn: kiểm thử fixture không chứng minh quyền tài khoản, picker, wire hoặc môi trường của IDE do người dùng mở thủ công. Các bằng chứng đó chờ run thật; môi trường tiến trình thủ công ghi UNKNOWN nếu không kiểm tra an toàn được. Recovery giữ backup khi có conflict hoặc restore chưa được xác nhận.
