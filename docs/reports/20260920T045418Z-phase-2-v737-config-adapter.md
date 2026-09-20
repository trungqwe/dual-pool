# Phase 2 — adapter cấu hình CLIProxyAPI v7.3.7

## Header

- Roadmap phase: Phase 2, slice thứ ba.
- Branch: `phase-2/upstream-lifecycle`.
- Start HEAD: `ab03ba3d72e2ae254a0df01c5d6091d044547d80`.
- End HEAD/push/CI: PENDING.
- Go version: xem `go.mod` và local verification output; không thay đổi toolchain.

## Objective

Triển khai adapter cấu hình đúng CLIProxyAPI v7.3.7, một dạng wire key chuẩn cho bốn raw key hiện có, và hai cây instance/config bảo vệ ACL. Sau Source CI PASS mới chạy real generation có gate, chứng minh idempotence và non-disclosure.

## Non-goals

Không chạy CLIProxyAPI, mở listener, làm OAuth, đọc/tạo provider auth file, sửa credential hiện có, sửa user config hoặc bắt đầu lifecycle.

## Pinned source and fields

- Repository: `router-for-me/CLIProxyAPI`.
- Commit: `b773607e3e7756dc6020a291825e4eb08899595a`.
- Các path cần audit: `config.example.yaml`, `internal/config/{config.go,sdk_config.go,config_types.go,config_load.go}`, `internal/access/config_access/provider.go`, `internal/api/handlers/management/handler.go`, `cmd/server/main.go`.
- `internal/config/config.go`: host, port, tls, remote-management, auth-dir, debug, pprof, discovery, logging-to-file, usage-statistics-enabled, save-cooldown-status, routing, ws-auth.
- `internal/config/sdk_config.go`: request-log, api-keys, passthrough-headers.
- `internal/config/config_types.go`: TLS/pprof/discovery, management flags, routing affinity/TTL/subagents.
- `internal/access/config_access/provider.go`: chuẩn hóa và so khớp chính xác client key dạng chuỗi.
- `internal/config/config_validation.go` và `config_load.go`: prefix bcrypt được chấp nhận và plaintext secret được hash khi load; adapter xuất bcrypt verifier trực tiếp.
- `internal/api/handlers/management/handler.go`: `MANAGEMENT_PASSWORD` kích hoạt `allowRemoteOverride`; management handler dùng bcrypt compare.
- `config.example.yaml` và `cmd/server/main.go`: tên trường và đường nạp config. Tất cả từ đúng commit pinned, không dùng `main`.

## Decisions before implementation

- Bốn key sản phẩm giữ nguyên 32 raw bytes trong Windows Credential Manager; `internal/keymaterial` chuyển duy nhất sang base64.RawURLEncoding dài 43 ký tự ASCII. Mọi future consumer phải dùng cùng package. Wipe buffer chỉ là best-effort bounded plaintext lifetime.
- Pinned v7.3.7 yêu cầu `api-keys` là plaintext exact string; mỗi config protected sẽ chứa đúng một encoded client key. Đây là residual risk có chủ đích. Không có plaintext secret ngoài hai config instance do upstream yêu cầu.
- Management wire key sẽ được bcrypt ở DefaultCost, YAML chỉ chứa verifier. `MANAGEMENT_PASSWORD` bị loại khỏi runtime contract vì pinned source bật `allowRemoteOverride` khi biến này có giá trị.
- Adapter identity dự kiến: `dualpool-cpa-v7.3.7-config-v1`; chỉ kích hoạt `upstream.lock` sau khi source contract và implementation được chứng minh. Historical lock digest không sửa trong evidence cũ.
- Layout cuối: `instances/{codex,google}/{config.yaml,auth,logs}`; port 8317/8318, host `127.0.0.1`. Dùng protected creation-time DACL cho config/candidate, marker an toàn trước candidate, same-volume no-replace install và recovery có kiểm tra.
- Existing final hợp lệ được giữ byte-for-byte, gồm bcrypt salt. Partial pair được hoàn tất chỉ cho bên thiếu.

## Real mutation plan and stop conditions

1. Audit source exact commit, implement và test TEMP/fault/subprocess.
2. Chạy local gates, commit/push, chờ Source CI PASS.
3. Preflight read-only product root, bốn key, instance roots. Nếu mismatch hoặc CI fail: STOP.
4. Chạy gated real generator; lần hai xác minh byte/file identity/key giữ nguyên, auth dirs rỗng, không process/listener.
5. Ghi evidence đã sanitize, commit/push/CI, rồi delivery receipt và CI.

STOP khi nguồn pinned không chứng minh field, schema/ACL/recovery fail, thiếu key, instance state lạ, secret scan fail, Source CI fail hoặc sẽ cần chạy process/OAuth ngoài phạm vi.

## Verification and evidence

`go.mod` thêm direct dependency `gopkg.in/yaml.v3 v3.0.1` (MIT/Apache-2.0) và `golang.org/x/crypto v0.54.0` (BSD-style Go license), khớp pinned upstream. `go mod tidy`, `go mod verify` và `go list -m all` đã chạy. Không import module upstream.

Fixture adapter, fault/recovery và subprocess crash tests PASS. Lần chạy full Go đầu phát hiện fixture `upstreamstage` còn lock cũ `UNIMPLEMENTED`; fixture đã chuyển sang phiên bản/commit/adapter pinned và package test PASS. Full Go suite, full race suite, pinned upstream integration, build, vet, module verification, Node 55/55 và `UPSTREAM_LOCK_VALID` PASS. Kiểm tra link tài liệu PASS, scan pattern secret trên các file thay đổi có 0 match, `git diff --check` PASS. Source CI và real generation còn PENDING tại thời điểm cập nhật này.

`upstream.lock` giữ `candidate`, tag/commit/artifact cũ, chỉ kích hoạt adapter version. Digest mới là `db57fef18b915105e7a2fac6e0505d967945152ceeb614e638f7a8821b0e50ea`; evidence stager lịch sử giữ nguyên digest trước đó. Bcrypt salt mới là ngẫu nhiên; các lần chạy lại giữ nguyên verifier và toàn bộ config byte-for-byte.

Recovery dùng marker trước candidate chứa secret, creation-time protected DACL cho file, `MoveFileEx` không replace trên cùng volume. Marker/candidate lạ hoặc final sai schema/ACL/key đều fail closed. Không chạy binary/listener.

Implementation `1204f0ab5007f3493266002da24ce108bb1b7ed1` được push và Source CI [35491189796](https://github.com/trungqwe/dual-pool/actions/runs/35491189796) PASS: format, modules, vet, test, race, pinned integration, build, Node và lock validation. Sau gate này, `TestRealInstanceConfigGeneration` PASS: hai instance tạo/xác minh, lần chạy hai có 0 rewrite và 0 key rotation, auth file = 0. Test quét raw/wire key trong product file và repository theo contract mà không ghi giá trị. Kiểm tra OS sau test: 0 `cli-proxy-api` process và 0 listener ở port 8317/8318. Evidence đã sanitize nằm ở `evidence/phase-2-instance-config/`; không chứa YAML, key/hash, SID hoặc absolute path. Phase 2 vẫn OPEN cho runtime bind/authentication, lifecycle, health, auth inventory và update rollback.
