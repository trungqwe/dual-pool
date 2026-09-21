# Bàn giao kiểm toán độc lập — trạng thái repository hiện tại

## Mục đích

Tài liệu này thay thế báo cáo đính kèm vốn chỉ đúng tại bootstrap commit
`2479fee`. Nó giúp kiểm toán viên độc lập xác định snapshot source hiện hành và
kiểm toán thay đổi kế tiếp mà không cần dựa vào lịch sử chat.

## Điểm báo cáo đính kèm nhận định đúng

- Traffic Gemini và Responses phải giữ native; Google/Codex pools phải tách biệt
  vật lý.
- Version/schema upstream và entitlement model cần bằng chứng ở đúng version và
  môi trường mục tiêu.
- Không chọn tùy tiện `MANAGEMENT_PASSWORD`: tài liệu CLIProxyAPI hiện nói biến
  này ép remote management; management key plaintext trong config cũng có thể bị
  bcrypt-hash và ghi lại lúc khởi động.
- Loopback không xác thực caller; bridge threat của Antigravity vẫn là nghĩa vụ
  review ở phase sau.

## Điều chỉnh dựa trên bằng chứng repository

| Nhận định cũ | Bằng chứng hiện tại | Hệ quả |
|---|---|---|
| Repository chỉ là governance package 31 file | Chỉ đúng tại bootstrap. Các branch hiện có Go source cùng Phase 1/2 evidence. | Không giao chỉ dẫn bao quát “chỉ Phase 0”. |
| Thiếu policy ignore evidence | Đã có `.gitignore`, `INV-LOG-06` và ranh giới Phase 0A/0B. | Vẫn cần scanner và promotion check; ignore không đủ. |
| Không được làm gì khi U-items blocked | Phase 0 closure cho phép fixture-only foundation và cấm live IDE/provider work. | Giữ U-001..U-004/U-006 blocked khi audit foundation scoped. |
| Checkout hiện tại là authoritative | Checkout local `4f57f8a` dirty và đã cũ. | Preflight remote/worktree; không pull/reset/stage checkout dirty. |

## Delivery map đã xác minh tại thời điểm audit

- Phase 0 remote: `6619034bc7edd79af3101ee7015a3de724b41542`.
- Phase 1 remote: `64a4d86947af96a5ee71baeb4671fa6932614511`.
- Phase 2 review target: `4eb550e731f21b6a05347cd84df2fdb3b349a6b4`.
- Source CI for the exact Phase 2 target: GitHub Actions run
  [35567470522](https://github.com/trungqwe/dual-pool/actions/runs/35567470522),
  completed with `success` and the matching head SHA.

Report correction Phase 2 yêu cầu independent audit trước installed-slot registry
và `instance.Manager` active-selection integration. CI success chỉ chứng minh
suite đã liệt kê PASS; không chứng minh production updater hoặc cho phép update
product root thật.

## Bối cảnh an toàn đã biết

Primary-profile watchdog cũ của Phase 0B từng để lại config mutation bị gián đoạn.
Recovery report lịch sử mâu thuẫn về settings và companion binaries; nhận định
toàn vẹn chat-history “100%” vượt quá bằng chứng file-presence đã dẫn. History
Phase 0 mới hơn đã cấm real primary config mutation và giữ U-006 blocked. Không
chạy lại primary/shadow probe, đọc auth/session data hay dùng việc đó để mở khóa
updater work.

## Mục tiêu tiếp theo duy nhất

Chỉ kiểm toán độc lập updater correction `4eb550e`. Đọc Phase 2 reports liên
quan, diff `53613e4..4eb550e`, `internal/update`, state/lock/ACL và lifecycle
interfaces. Chỉ chạy lại synthetic/TEMP checks sau khi kiểm tra side effect.
Không triển khai installed-slot registry trong lượt audit này.

Dùng [prompt thực hiện](../NEXT-INDEPENDENT-UPDATER-AUDIT-PROMPT.md).
