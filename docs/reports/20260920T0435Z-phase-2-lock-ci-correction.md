# Phase 2 — sửa race canonical Lstat trên Source CI

## Header

- Run ID: phase-2-product-init-ci-correction
- Date/time UTC: 2026-09-20
- Roadmap phase: Phase 2
- Branch: `phase-2/upstream-lifecycle`
- Start HEAD: `6899d95663de12d924f313226d7ad5bf987731bd`
- End HEAD: PENDING
- Push/CI: PENDING

## Assigned objective

Khôi phục Source CI sau lỗi `TestSimultaneousStaleReclaim/file` trong `internal/lockfile`. Đây là gate bắt buộc để chấp nhận evidence commit của product initialization.

## Non-goals

Không đổi ACL/key policy, không chạy lại fresh initialization, không tạo CLIProxyAPI config/process/listener hoặc chạm provider credentials.

## Starting state and evidence

- Implementation commit `9b0aabc` Source CI PASS, run `35488118466`.
- Live product init và idempotence PASS; evidence/correction commit `6899d95` đã push.
- Source CI [run 35488802945](https://github.com/trungqwe/dual-pool/actions/runs/35488802945) FAIL tại `go test ./...`: `TestSimultaneousStaleReclaim/file` iteration 5, `claim_open: canonical_lstat: mutation lock persistence failed`. Các package khác gồm `productinit` và `winacl` PASS.
- Exact Windows error code tại `Lstat`: UNKNOWN trong CI log đã sanitize. Repro cục bộ chưa lặp lại. Không coi CI này là PASS.

## Decision and change

`openCanonical` đã có bounded 250 ms retry cho `GetFileAttributesW` và `CreateFileW` trong cùng cửa sổ stale-reclaim. Mở rộng đúng giới hạn đó cho `os.Lstat` khi Windows trả `ERROR_ACCESS_DENIED` hoặc `ERROR_SHARING_VIOLATION`. Các lỗi khác tiếp tục fail closed; hết deadline vẫn trả persistence error. Không bỏ qua test hoặc đổi điều kiện acceptance.

## Verification

| Command | Result |
|---|---|
| `go test -count=5 -run '^TestSimultaneousStaleReclaim$' ./internal/lockfile` | PASS, exit 0 |
| `go mod verify`, `go vet ./...`, `go test -count=1 ./...`, Windows build | PASS, exit 0 |
| `node --test scripts/*.test.cjs`, candidate lock validator | PASS, 54/54 và `UPSTREAM_LOCK_VALID` |
| Full `go test -race -count=1 ./...` | PASS, exit 0; lock stress 209.349 s |
| Secret scan, docs links, `git diff --check` | PENDING |
| Correction Source CI | PENDING |

## Security/privacy

Không có secret mới, dependency mới hoặc persistent product mutation trong correction. Retry chỉ áp dụng cho lỗi truy cập/chia sẻ tạm thời ở canonical lock lookup; nếu không ổn định sau 250 ms thì fail closed.

## Acceptance and next run

`P2-ENTRY-ACL-001` và `P2-ENTRY-KEYS-001` đã PASS ở real gate nhưng delivery vẫn chờ Source CI correction PASS. Phase 2 vẫn OPEN. Sau CI PASS, ghi append-only delivery receipt và kiểm tra CI của receipt. Slice kế tiếp: pinned v7.3.7 config adapter + two isolated instance config generation.
