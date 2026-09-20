# Phase 2 — xác minh khởi tạo product root thật

## Header

- Run ID: phase-2-product-init-live-verification
- Date/time UTC: 2026-09-20
- Roadmap phase: Phase 2
- Repository: `trungqwe/dual-pool`
- Branch: `phase-2/upstream-lifecycle`
- Start HEAD: `9b0aabc1306b7e11f8c395a5a08d0cc325775bcf`
- End HEAD: PENDING
- Remote push result: PENDING

## Assigned objective

Đóng `P2-ENTRY-ACL-001` và `P2-ENTRY-KEYS-001` sau khi implementation Source CI PASS; xác minh mutation thật, idempotence, recovery fixture và non-disclosure. Correction nhỏ trước khi chốt: kiểm tra chính xác ACE inheritance flags, đọc lại toàn bộ bốn key sau `Put`, và báo lỗi khi nhả global lock thất bại.

## Non-goals

Không tạo CLIProxyAPI config, instance auth root, provider credential, listener hoặc process. Không thay đổi U-001..004, U-006 hay U-008.

## Starting state

- Worktree sạch sau implementation commit; remote HEAD khớp local.
- Source CI của implementation `9b0abc` đã PASS: [run 35488118466](https://github.com/trungqwe/dual-pool/actions/runs/35488118466).
- Product root được phân loại `root_absent` trước mutation thật. Thành công của preflight trong initializer với root absent chứng minh bốn production purposes vắng mặt; không enumerate Credential Manager.

## Investigation and evidence

| Claim | Status | Evidence/source | Consequence |
|---|---|---|---|
| Windows volume và ACL contract hỗ trợ creation-time descriptor | VERIFIED | [CreateDirectoryW](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-createdirectoryw), [ACL flags](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-getvolumeinformationw), `internal/winacl/winacl_test.go` | Tạo root/children an toàn ngay tại thời điểm tạo |
| DACL owner, principals, flags và inheritance đúng | VERIFIED | `evidence/phase-2-product-init/acl-gate.json`, gated real test | `P2-ENTRY-ACL-001` PASS |
| Bốn key độc lập, 32 byte, không đổi ở lần chạy hai | VERIFIED | `evidence/phase-2-product-init/key-gate.json`, gated real test | `P2-ENTRY-KEYS-001` PASS |
| Provider/live compatibility | UNKNOWN | U-001..004, U-006, U-008 chưa được probe thêm | Giữ nguyên blocker |

## Decisions

Giữ root và bốn credential sau thành công; không có rollback xóa tự động. Khi lỗi giữa chừng, lần chạy sau tiếp tục từ tập key hợp lệ. Không xuất SID, path, credential target, key bytes hoặc key hash ra bằng chứng.

## Changes

| Path | Change | Reason |
|---|---|---|
| `internal/winacl/winacl.go` | Xác minh ACE inheritance flags chính xác | Không chấp nhận descriptor có flags lệch hợp đồng |
| `internal/productinit/` | API inspect read-only, final key readback, release check, fault và sentinel tests | Đóng các edge gate trước acceptance |
| `docs/15-MASTER-CHECKLIST.md`, `docs/23-TRACEABILITY-MATRIX.md`, `docs/18-HANDOFF.md` | Đồng bộ gate và handoff | Giữ traceability hiện hành |
| `evidence/phase-2-product-init/` | JSON đã sanitize | Bằng chứng không tiết lộ local identity hoặc key |

## Verification

| Test/command | Result | Evidence |
|---|---|---|
| `go mod verify`, `go vet ./...`, `go build ./cmd/poolbridge` | PASS, exit 0 | Local command output |
| `go test -count=1 ./...` | PASS, exit 0; 131 Go test functions | `test-result.json` |
| `go test -race -count=1 ./...` | PASS, exit 0; lock stress 209.261 s | `test-result.json` |
| `node --test scripts/*.test.cjs` | PASS, exit 0; 54/54 | `test-result.json` |
| `node scripts/phase0-upstream-lock.cjs` | PASS, exit 0; `UPSTREAM_LOCK_VALID` | `test-result.json` |
| `DUALPOOL_RUN_REAL_PRODUCT_INIT=1` gated test | PASS, exit 0; hai lần chạy, bốn key giữ nguyên | `real-initialization.json` |
| Secret scan, docs links, `git diff --check` | PENDING final staged gate | `security-gate.json` |
| Evidence commit Source CI | PENDING | Handoff receipt sau CI |

## Security/privacy review

- Listener/bind impact: không.
- Secret handling: bốn production key 32 byte trong Windows Credential Manager; comparison buffers được zero best effort.
- Config mutation: không.
- Logging/evidence: chỉ boolean/count/classification; không có key, hash, SID hoặc absolute path.
- Secret scan: PENDING final staged gate.
- Dependencies: không thêm.

## Acceptance evaluation

- `P2-ENTRY-ACL-001`: PASS; `acl-gate.json`.
- `P2-ENTRY-KEYS-001`: PASS; `key-gate.json`.
- Redirect explicit-port hardening: PASS; regression test và implementation Source CI.
- Source CI race gate: PASS trên implementation commit.
- Phase 2 exit: OPEN; config/lifecycle/isolation chưa triển khai.

## Risks and unresolved items

| Risk/unknown | Severity | Owner | Required next action |
|---|---|---|---|
| Provider compatibility blockers | High | Phase 0B/Phase 2 | Probe sau theo từng gate, không suy diễn từ product init |
| Config adapter version/schema | High | Phase 2 | Slice kế tiếp: pinned v7.3.7 adapter |

## Git delivery

- Implementation commit: `9b0aabc1306b7e11f8c395a5a08d0cc325775bcf`.
- Implementation Source CI: PASS, run `35488118466`.
- Evidence/correction commit, push and CI: PENDING.
- PR: chưa yêu cầu.

## Rollback

Code/docs có thể revert bằng commit mới. Product root và credential đã tạo được giữ lại; không tự động xóa vì đây là persistent initialization có owner và lifecycle riêng.

## Next run

- Exact objective: pinned v7.3.7 config adapter + two isolated instance config generation.
- Entry: hai Phase 2 entry gates PASS, evidence CI PASS.
- Đọc `docs/18-HANDOFF.md`, `docs/14-ROADMAP.md`, `docs/07-CLIPROXYAPI-INTEGRATION.md` và báo cáo này.
- Dừng nếu schema/config isolation không được chứng minh; không tự khởi chạy process.
