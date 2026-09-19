# Phase 1 — foundation bootstrap

## Header

- Run ID: `20260919T1415Z-phase-1-foundation-bootstrap`
- Date/time UTC: `2026-09-19T14:15:00Z`
- Agent/tool version: Codex desktop
- Roadmap phase: Phase 1
- Repository: `https://github.com/trungqwe/dual-pool`
- Branch: `phase-1/state-foundation`
- Start HEAD: `dd448f7c2e55351eccfb8b8f2f16e8433810e05a`
- End HEAD: PENDING
- Remote push result: PENDING

## Assigned objective

Bootstrap the standard-library-only Go CLI with build metadata, stable application errors and exit categories, unit tests, and source-only CI.

## Non-goals

- No state store, migrations, locks, config mutation, secret store, upstream lifecycle, OAuth, accounts, IDE integration, listeners, health, doctor, updater, or live probes.
- The Go product does not consume `upstream.lock` in this slice.

## Starting state

- Worktree classification: clean isolated Codex worktree.
- Branch created from exact Phase 0 closure commit `dd448f7c2e55351eccfb8b8f2f16e8433810e05a`; merge-base matches.
- Installed toolchain: `go version go1.26.0 windows/amd64`.
- Phase 0 status: `PASS_WITH_BLOCKERS`; live integration remains NO-GO.

## Planned files

- `go.mod`
- `cmd/poolbridge/main.go`
- `internal/app/*`
- `internal/buildinfo/*`
- `internal/apperr/*`
- `.github/workflows/ci.yml`
- scoped Phase 1 evidence and required checklist/traceability/handoff updates

## Planned tests

- Version/help command variants and deterministic injected metadata.
- Unknown-command usage failure.
- Exact exit-category values and complete stable-code registry.
- Human and machine error envelopes exclude wrapped sensitive causes.
- Internal packages contain no `os.Exit`.
- Go format, vet, unit tests, build, optional race test, Phase 0 Node regression, upstream lock validator, workflow structure, privacy and documentation gates.

## Stop conditions

- Toolchain unavailable, source/history mismatch, conflicting existing work, specification conflict, leak-test failure, Phase 0 regression, or CI requiring credentials/live services.

## Mapping decisions to confirm

- `PROCESS_IDENTITY_MISMATCH` maps to `security` because acting on an unverified PID could affect a foreign process.
- `MANAGEMENT_UNAVAILABLE` and `AG_POOL_UNAVAILABLE` map to `service`.
- All new foundation-only invalid command/argument codes map to `usage`.

## Investigation and evidence

- Khoản nợ Phase 0 về `verified_capabilities` đã được sửa và đẩy lên nhánh Phase 0, rồi đưa vào nhánh này qua hai commit `0084e34` và `223d2e8`. Candidate lock không đổi.
- Kiểm toán mã ban đầu phát hiện lệnh nhận đối số thừa, metadata build chưa kiểm tra giá trị và test registry dùng chính hằng implementation. Cả ba đã được sửa bằng test.
- Go tạo executable test trong TEMP mặc định bị Windows từ chối mở cho package `internal/app`. Dùng `GOTMPDIR` riêng trong thư mục tạm của worktree, toàn bộ Go tests và race test đều chạy đạt. Nguyên nhân từ chối ở TEMP mặc định vẫn `UNKNOWN`; CI phải xác nhận độc lập.

## Decisions

- CLI không có đối số in help và thoát 0. Đối số thừa trả `INVALID_ARGUMENT`, exit 2.
- Version/build output chỉ nhận `dev` hoặc phiên bản theo mẫu semantic, commit hex, thời gian RFC3339 và dirty state đã định nghĩa; giá trị build không hợp lệ thành `unknown`.
- `PROCESS_IDENTITY_MISMATCH` thuộc category security; mapping đầy đủ ở `docs/12-OBSERVABILITY-ERRORS.md`.
- CI dùng Go 1.26 theo `go.mod`, chạy trên Windows, dùng `actions/checkout@v7` và `actions/setup-go@v7` đã đối chiếu với [checkout releases](https://github.com/actions/checkout/releases) và [setup-go releases](https://github.com/actions/setup-go/releases).

## Changes

| Path | Change | Reason |
|---|---|---|
| `cmd/poolbridge`, `internal/app`, `internal/buildinfo`, `internal/apperr`, `go.mod` | CLI chuẩn thư viện Go, metadata và hợp đồng lỗi | Lát cắt Foundation đầu tiên |
| `.github/workflows/ci.yml` | Format, vet, Go tests/build, Phase 0 tests và lock validator | CI chỉ dùng source và fixture |
| `docs/12-OBSERVABILITY-ERRORS.md`, `docs/15-MASTER-CHECKLIST.md`, `docs/23-TRACEABILITY-MATRIX.md`, `docs/18-HANDOFF.md` | Mapping, tiến độ, bằng chứng và bàn giao | Giữ tài liệu đúng với triển khai |
| `evidence/phase-1-foundation-bootstrap/` | Kết quả và security gate đã khử dữ liệu riêng tư | Bằng chứng kiểm tra |

## Verification

| Test/command | Result | Evidence |
|---|---|---|
| `go version` | PASS: `go1.26.0 windows/amd64` | `test-result.json` |
| `go fmt ./...`, `go vet ./...` | PASS | Kiểm tra cục bộ |
| `go test -count=1 ./...` | PASS: 11 Go test functions | `test-result.json` |
| `go test -race -count=1 ./...` | PASS | `test-result.json` |
| `go build -o temp/poolbridge.exe ./cmd/poolbridge` | PASS; lệnh version/help và extra-argument đã chạy | `test-result.json` |
| `node --test scripts/*.test.cjs` | PASS: 53/53 sau sửa Phase 0 | `test-result.json` |
| `node scripts/phase0-upstream-lock.cjs` | PASS: `UPSTREAM_LOCK_VALID` | `test-result.json` |
| Cú pháp YAML, JSON, docs links, privacy, secret và diff | PASS cục bộ | `security-gate.json` |
| GitHub-hosted CI | PENDING: workflow chưa được push | `.github/workflows/ci.yml` |

## Security/privacy review

- Không tạo listener, không đọc credential, không thay đổi cấu hình người dùng và không chạy CLIProxyAPI.
- Test dùng nguyên nhân lỗi và metadata tổng hợp; renderer người dùng và JSON không chứa cause.
- Workflow không dùng repository secret hay live compatibility test.
- Staged privacy/secret scan: PASS; các chuỗi tổng hợp trong test được nhận diện riêng, không có dữ liệu thật.

## Acceptance evaluation

- Go module, CLI version/help, metadata injectable, stable errors/exit, machine envelope và leak tests: PASS cục bộ.
- Go vet/build/unit/race và Phase 0 regression: PASS cục bộ.
- CI foundation: định nghĩa và parse cục bộ PASS; remote run PENDING.
- Checklist, traceability, evidence và security gate: PASS cục bộ. Git delivery: PENDING.

## Risks and unresolved items

- U-001..U-004/U-006 vẫn BLOCKED, U-008 PARTIAL_UNKNOWN; lát cắt này không tiêu thụ candidate lock trong Go.
- TEMP mặc định từ chối executable test trên máy này; CI độc lập phải chạy, không được gọi remote CI là PASS khi chưa có kết quả.

## Git delivery

- Files committed: PENDING.
- Commit/push/PR: PENDING. Report này giữ nguyên sau khi push; receipt được ghi trong handoff.

## Rollback

- Revert commit Phase 1 theo phạm vi; không có state hoặc cấu hình người dùng cần khôi phục.

## Next run

- Phase 1 — data-root resolver và structured allowlist logger/redaction foundation. Không bắt đầu trong run này.
