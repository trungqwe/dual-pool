# Prompt: kiểm toán độc lập updater correction Phase 2

Kiểm toán `trungqwe/dual-pool` commit
`4eb550e731f21b6a05347cd84df2fdb3b349a6b4`. Đây là review Phase 2 có phạm vi
giới hạn, không phải implementation run.

Đọc `AGENTS.md`, `docs/00-README.md`, `03-DECISIONS-AND-EVIDENCE.md`,
`05-INVARIANTS.md`, `14-ROADMAP.md`, `15-MASTER-CHECKLIST.md`,
`16-AGENT-OPERATING-PROTOCOL.md`, `17-REPORT-TEMPLATE.md`, `18-HANDOFF.md`,
`20-VERIFICATION-CONTRACT.md`, `21-NEGATIVE-TEST-MATRIX.md`,
`22-RISK-REGISTER.md`, `23-TRACEABILITY-MATRIX.md`, and the Phase 2 updater
foundation/correction reports and evidence. Do not read auth, history, raw
captures hoặc mọi file chỉ vì chúng tồn tại.

Preflight `git rev-parse HEAD`, branch, remotes, status, worktrees và
`git ls-remote origin refs/heads/phase-2/upstream-lifecycle`. Review một snapshot
sạch, chính xác. Không pull, reset, stash, clean hoặc ghi đè checkout dirty. Nếu
remote khác, ghi cả hai SHA và không quy code mới cho `4eb550e`.

Xác minh GitHub Actions run `35567470522` có đúng head SHA và xem jobs/steps thực
thi. Sau đó review `53613e4..4eb550e`, toàn bộ `internal/update` code và state,
locks, Windows ACL, lifecycle interfaces liên quan.

Trả lời bằng bằng chứng file:line:

1. Are previous and candidate slots verified before lifecycle side effects?
2. On rollback Stop failure, does the marker remain and does recovery stop
   without reporting success or committing unsafe state?
3. Does Store-based recovery cover crash windows and assert final state, not
   merely an error message?
4. Are pending, corrupt and unsafe markers rejected before mutation with a
   bounded strict schema and closed logical-version/running-set validation?
5. Are marker creation/load/remove protected against reparse, ACL and path/TOCTOU
   failures, with correct lock ordering?
6. Do tests actually exercise faults, crashes and ACL/reparse cases without
   skips, mocks that hide the failure, stale evidence or hard-coded PASS?

Chỉ được chạy verified synthetic/TEMP tests. Không đụng product root, production
Credential Manager targets, real CLIProxyAPI processes/listeners,
Codex/Antigravity configuration, OAuth/auth/session data hay real updater gate.
Không sửa `upstream.lock`, binaries, bundles, CA, hosts hoặc firewall. Giữ
U-001..U-004/U-006 blocked và U-008 qualified; fixture test không đóng live hoặc
release gate.

Tạo Vietnamese UTF-8 report theo `docs/17-REPORT-TEMPLATE.md`, gồm SHA/branch
đã review, CI evidence, mọi command và exit code, finding theo severity có
trigger/impact/fix, E3/component limits và đúng một next objective. Chạy
repository-prescribed synthetic verification, docs-link check, `git diff --check`
và scoped privacy scan có synthetic positive control. Không ghi đè historical
evidence.

Nếu có blocking finding hoặc required check fail, không commit/push; next task
duy nhất là fixture-backed repair. Nếu không có blocker, next task duy nhất là
Phase 2 implementation run riêng cho installed-slot registry và
`instance.Manager` active selection. Production updater acceptance vẫn OPEN.
