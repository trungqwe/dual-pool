# Phase 1 — data-root resolver và logging an toàn

## Header

- Run ID: `20260919T1545Z-phase-1-dataroot-logging`
- Date/time UTC: `2026-09-19T15:45:00Z`
- Agent/tool version: Codex desktop
- Roadmap phase: Phase 1, lát cắt thứ hai
- Repository: `https://github.com/trungqwe/dual-pool`
- Branch: `phase-1/state-foundation`
- Start HEAD: `8da48f2c3897d57eb43c180555bfff01a0263f31`
- End HEAD: PENDING
- Remote push result: PENDING

## Assigned objective

Thêm resolver chỉ tính đường dẫn `%LOCALAPPDATA%\DualPool`, logger JSON theo allowlist, kiểm tra giá trị nhạy cảm, fingerprint HMAC cho session, và sửa `apperr.New` khi mã lỗi chưa đăng ký.

## Non-goals

Không tạo data root, state/ownership files, migration, atomic store, lock, credential, cấu hình người dùng, process, listener, CLIProxyAPI, OAuth, IDE integration, doctor hoặc live probe. Không thêm lệnh CLI.

## Starting state

- Worktree Phase 1 sạch tại đúng HEAD dự kiến. Checkout chính có thay đổi Phase 0 riêng và được giữ nguyên.
- Go: `go1.26.0 windows/amd64`.
- Remote Phase 0: `6619034bc7edd79af3101ee7015a3de724b41542`; remote Phase 1: `8da48f2c3897d57eb43c180555bfff01a0263f31`, xác minh bằng `git ls-remote`.
- `git fetch origin` tải object nhưng không cập nhật được remote-tracking ref Phase 0 cục bộ. Không reset hay sửa ref bằng tay.
- Phase 1 chứa validator Phase 0 đã sửa; hai nhánh có lịch sử delivery receipt khác nhau và không cần merge chỉ để làm thẳng ancestry.

## Planned files and tests

- `internal/dataroot/`: resolver thuần và wrapper environment; test `P1-DATAROOT-001`.
- `internal/apperr/`: checked constructor và mã `DATA_ROOT_UNAVAILABLE`; test `P1-APPERR-UNKNOWN-001`.
- `internal/safelog/`: Event/Field API kiểu hóa, allowlist, validation, redaction, fingerprint; tests `P1-LOG-ALLOWLIST-001`, `P1-LOG-REDACTION-001`, `P1-SESSION-FINGERPRINT-001`.
- Tài liệu liên quan, evidence và handoff. CI hiện có sẽ chạy thêm Go tests; chỉ sửa workflow nếu thực sự cần.

## Stop conditions

Không thể giữ resolver thuần; logger có thể ghi giá trị cấm hoặc ghi dở khi validation thất bại; fingerprint lộ input/key; test/secret scan/Phase 0 regression thất bại; phạm vi tràn sang persistence hoặc live integration.

## Investigation and evidence

- Resolver sử dụng input được inject; `ResolveCurrent` chỉ đọc `LOCALAPPDATA`. UNC/network, drive root, relative path, dot segment, NUL, Windows reserved name và thành phần malformed đều fail closed.
- `apperr.New` cũ biến code lạ thành `INVALID_ARGUMENT`. API mới trả construction error chung, không chứa code/cause; `MustNew` chỉ dành cho hằng nội bộ đã đăng ký.
- Logger không nhận map tùy ý. `Field` có state private, được dựng qua constructor theo type, và được kiểm tra lại trong `Log` để phòng caller cùng package hoặc thay đổi sau này.
- Record được validate đầy đủ, marshal trong memory, scan lần cuối, rồi thực hiện một lần gọi `Write`. Writer error và short write được trả về caller.

## Decisions

- Canonical root là `<LOCALAPPDATA>\DualPool`; resolver chỉ trả layout `bin`, `instances`, `config`, `state`, `backups`, `evidence`, `locks` và không tạo chúng.
- V1 từ chối UNC/network storage. Không fallback sang CWD, TEMP, USERPROFILE hoặc APPDATA.
- Fingerprint là 16 byte đầu của HMAC-SHA-256, encode thành 32 lowercase hex. Key do caller sở hữu và không được persist.
- Logger chấp nhận `bytes_class`: `empty`, `tiny`, `small`, `medium`, `large`, `oversize`; config hash prefix là đúng 12 lowercase hex.

## Changes

| Path | Change | Reason |
|---|---|---|
| `internal/dataroot/` | Windows-only pure resolver và test side-effect | Chuẩn hóa layout trước persistence |
| `internal/safelog/` | Typed fields, JSON-line logger, guards và fingerprint | Nền tảng metadata-only logging |
| `internal/apperr/`, `internal/app/` | Checked constructor, `DATA_ROOT_UNAVAILABLE`, caller migration | Không downgrade code lạ thành usage error |
| docs, evidence, handoff | Contract, traceability và gates | Bằng chứng có thể kiểm tra |

## Verification

| Test/command | Result | Evidence |
|---|---|---|
| `go fmt ./...`, `go vet ./...` | PASS | Local gate |
| `go test -count=1 ./...` | PASS: 22 Go test functions | `evidence/phase-1-dataroot-logging/test-result.json` |
| `go test -race -count=1 ./...` | PASS | Test result |
| `go build -o temp/poolbridge.exe ./cmd/poolbridge` | PASS | Test result |
| CLI regression variants | PASS: no args/help/version exit 0; unknown/extra args exit 2 | Test result |
| `node --test scripts/*.test.cjs` | PASS: 53/53 | Test result |
| `node scripts/phase0-upstream-lock.cjs` | PASS: `UPSTREAM_LOCK_VALID` | Test result |
| JSON, docs links, privacy/path/secret scan và diff | PASS | Security gate |
| Evidence manifest | PASS: SHA-256 của 12 artifact đã kiểm tra | Evidence manifest |
| GitHub-hosted CI | PENDING until implementation push | Existing workflow |

## Security/privacy review

- Không truy cập credential, không đọc real LOCALAPPDATA path vào evidence, không mutate filesystem ngoài disposable build/test temp.
- Sentinel, email, token-like strings, absolute local paths và raw-session marker bị từ chối trước writer; successful output được scan ngược.
- Không có third-party dependency, file log sink hoặc state persistence.

## Acceptance evaluation

- Data-root pure/deterministic/side-effect-free: PASS cục bộ.
- Logger allowlist, family/level/type validation, zero-output rejection và writer failure: PASS cục bộ.
- Session fingerprint: PASS cục bộ.
- Unknown apperr code: PASS cục bộ; không còn biến thành `INVALID_ARGUMENT`.
- Remote CI và Git delivery: PENDING.

## Risks and unresolved items

- Full end-to-end `LOG-001` vẫn mở vì chưa có product runtime log path.
- Phase 1 vẫn mở cho schema/migration, store/recovery, locks/PID identity, secret store và config transaction.
- Default Windows TEMP executable issue chưa được tuyên bố đã sửa; verification dùng worktree-local `GOTMPDIR` như run trước.

## Git delivery

- Implementation commit, push, CI, delivery receipt và PR: PENDING.

## Rollback

Revert commit Phase 1 theo phạm vi. Không có persistent product state cần khôi phục.

## Next run

Phase 1 — state/ownership schema v1 và migration framework, dùng fixture/codec thuần và chưa thêm durable persistence.
