# Báo cáo Phase 2 — hardening harness lifecycle trước real gate

## Header

- Run ID: `2026-09-20T0932Z-phase-2-real-harness-hardening`.
- Date/time UTC: 2026-09-20T09:32Z.
- Roadmap phase: Phase 2.
- Repository: `trungqwe/dual-pool`.
- Branch: `phase-2/upstream-lifecycle` (worktree detached tại start HEAD).
- Start HEAD: `74120593f8d23d9dde1fbe0b3f0ee0fafb805d8d`.
- End HEAD và kết quả push: PENDING tại thời điểm viết report trước commit.

## Mục tiêu

Củng cố real lifecycle harness trước khi mở real gate: cleanup phải quan sát lỗi, xác nhận record/listener không còn lại, và test fixture phải chứng minh PID reuse/image mismatch không thể terminate một process chưa được xác minh.

## Không thuộc phạm vi

Không chạy `DUALPOOL_RUN_REAL_LIFECYCLE=1`, không gửi provider request, OAuth, đọc token provider, hay thay đổi config/secret ngoài phạm vi test fixture TEMP.

## Thay đổi

| Path | Change | Reason |
|---|---|---|
| `internal/instance/instance.go` | Thêm internal termination-handle seam, mặc định vẫn mở Windows handle thật. | Kiểm chứng cùng handle đã xác minh được dùng để terminate mà không tạo PID reopen race. |
| `internal/instance/instance_test.go` | Thêm fixture PID reuse, image mismatch và verified-handle lifecycle. | PID/image sai bị chặn trước `Terminate`; handle đúng phải `Terminate`, `Wait`, `Close`. |
| `internal/instance/real_windows_test.go` | Cleanup báo lỗi Stop, process record, listener; final check config bytes/file identity, auth/log rỗng, four keys constant-time và binary validation. | Real gate chỉ pass khi cleanup và bất biến sau lifecycle được chứng minh. |

## Verification

| Command | Result | Evidence |
|---|---|---|
| `go test -count=1 ./...` | PASS | Bao gồm `internal/instance`; real test skip fail-closed khi env chưa arm. |
| `go test -race -count=1 ./internal/instance` | PASS | Regression cho seam và fixture mới. |
| `go vet ./...` | PASS | Exit 0. |
| `go build ./cmd/poolbridge` | PASS | Exit 0. |
| `go mod verify` | PASS | `all modules verified`. |
| `node --test scripts/*.test.cjs` | PASS | 55/55. |
| `node scripts/phase0-upstream-lock.cjs` | PASS | `UPSTREAM_LOCK_VALID`. |
| `git diff --check` | PASS | Không whitespace error. |
| Full `go test -race -count=1 ./...` | PENDING | Source CI của commit sẽ là gate bắt buộc; giới hạn phiên lệnh cục bộ cắt chạy full trước khi hoàn tất. |
| Real lifecycle | NOT RUN | `DUALPOOL_RUN_REAL_LIFECYCLE` không được arm. |

## Security/privacy review

- Không có provider traffic, OAuth, credential inventory hoặc token logging.
- Key fixture chỉ đọc bốn product key vào memory, so sánh constant-time, rồi zero buffer; không ghi value/hash vào evidence.
- Cleanup chỉ gọi `Manager.Stop`; không `taskkill`, không xoá mù quáng.
- Config chỉ được đọc để kiểm tra byte/file identity và buffer sau đọc được zero.

## Acceptance evaluation

| Criterion | Status | Evidence |
|---|---|---|
| Cleanup không nuốt lỗi Stop | PASS | `P2-REAL-CLEANUP-001`. |
| Record/listener sau Stop phải vắng | PASS trong harness và unit path | Final assertions và cleanup assertions. |
| PID reuse/image mismatch không terminate | PASS | `TestStopRecordUsesOneVerifiedHandle`. |
| Bốn key không đổi và buffer được zero | PASS trong harness design; real execution NOT RUN | Constant-time final comparison. |
| Own Source CI pass trước real gate | PENDING | Chưa commit/push. |
| Real lifecycle authorization | CLOSED | Chưa được chạy. |

## Next run

1. Commit và push scoped thay đổi này.
2. Chỉ khi Source CI của chính commit PASS, chạy một lần `DUALPOOL_RUN_REAL_LIFECYCLE=1 go test ./internal/instance -run '^TestRealLifecycle$' -count=1 -v`.
3. Stop ngay nếu cleanup, auth/log integrity, process record, listener, key immutability hoặc binary validation fail.
