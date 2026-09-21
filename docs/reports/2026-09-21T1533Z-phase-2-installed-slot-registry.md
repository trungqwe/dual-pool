# Báo cáo triển khai Phase 2 — trusted installed-slot registry

## Header

- Run ID: `2026-09-21T1533Z-phase-2-installed-slot-registry`
- Date/time UTC: 2026-09-21T15:33Z
- Roadmap phase: Phase 2, component slice
- Repository: `trungqwe/dual-pool`
- Branch: `phase-2/installed-slot-registry`
- Start HEAD: `2f5dd7b134ad19b9390af9c174095ef94ed09abc`
- End HEAD at implementation push: `68f3869bc3530e538973c3e74f75f1b2b8d81b80`
- Remote push result: normal push succeeded; Source CI run `35619497269` completed successfully for exact SHA `68f3869bc3530e538973c3e74f75f1b2b8d81b80`.

## Assigned objective

Xây dựng mapping tin cậy `state.ActiveUpstreamVersion -> installed slot -> executable` bằng registry kín, immutable và bảo vệ; tích hợp active selection vào `instance.Manager`; dùng cùng `VerifyInstalled` cho updater contract; giữ nguyên pin v7.3.7 và không chạy updater/process/provider thật.

## Non-goals

Không chạy real updater, không tải/cài release mới, không sửa `upstream.lock` hoặc `cliproxyconfig`, không OAuth/provider/config/Registry/Credential Manager, không khởi chạy CLIProxyAPI thật, không fast-forward `phase-2/upstream-lifecycle`.

## Thiết kế và quyết định

Chọn **explicit protected registry document** tại `<Bin>\cliproxyapi\installed-slots.json` vì mapping cần tồn tại bền vững, phát hiện được registry thiếu/hỏng/trùng version và giữ rollback slot cũ. Tài liệu được ghi bằng file tạm đã `Sync`, kiểm tra strict JSON, rồi publication nguyên tử (`MoveFileEx` cho lần đầu, `ReplaceFileW` cho thay thế). Registry không nhận path từ caller; path được suy ra từ `layout.Bin`, component cố định `cliproxyapi` và logical version đã qua validator. Bản ghi chỉ được thêm; cùng version chỉ nhận dạng lại khi toàn bộ identity trùng khớp.

`InstallManifest` được alias sang `internal/installedslot.Manifest` để Manager và registry dùng một kiểu metadata. `state.State` không đổi schema và không có path. `ProcessRecord` lên schema 2, thêm `upstream_version` và `manifest_sha256`; record cũ thiếu binding bị từ chối.

## Thay đổi

| Path | Thay đổi | Lý do |
|---|---|---|
| `internal/installedslot/registry.go` | Registry strict schema, path/version validator, ACL/reparse/hash/manifest/provenance verifier, no-replace registration, atomic publication, fault seam | Một nguồn trust cho slot |
| `internal/installedslot/registry_test.go` | Hai slot synthetic, path/device/corruption/no-rebind/publication-fault tests | Negative matrix và immutability |
| `internal/instance/instance.go` | Dependency injection registry/state reader; active state resolution; exact executable launch; ProcessRecord v2; recorded-slot Status/Stop; Install/recovery registration | Runtime không còn cố định vào `m.lock.Version` |
| `internal/instance/instance_test.go` | Active selection, old-record fail-closed, recorded-slot identity tests | Bảo vệ selection và PID/image semantics |
| `internal/state/validate.go` | Shared closed logical-version validator | State không nhận device/traversal/reserved name |
| `internal/state/state_test.go` | Negative active-version cases | Chứng minh state fail closed |
| `internal/update/update.go` | Dùng cùng logical-version validator | Updater và registry đồng nhất syntax |
| `internal/update/update_test.go` | Compile contract: Registry implements `SlotVerifier` | Updater có thể inject cùng registry |

## Verification

| Test/command | Kết quả | Bằng chứng |
|---|---|---|
| `go test -count=1 ./...` | PASS | `evidence/phase-2-installed-slot-registry/test-result.json` |
| `go test -count=100 ./internal/installedslot` | PASS | cùng evidence |
| `go test -count=50 ./internal/instance` | PASS | cùng evidence |
| `go test -count=50 ./internal/update` | PASS | cùng evidence |
| `go test -race -count=10 ./internal/installedslot ./internal/instance ./internal/update` | PASS | cùng evidence |
| `go vet ./...` | PASS | local command exit 0 |
| `go mod verify` | PASS | local command exit 0 |
| `go build -o %TEMP%\poolbridge-installed-slot-registry.exe ./cmd/poolbridge` | PASS | disposable output outside repo |
| `node --test scripts/*.test.cjs` | PASS, 55 tests | local output |
| `node scripts/phase0-upstream-lock.cjs` | PASS (`UPSTREAM_LOCK_VALID`) | local output |
| `git diff --check` | PASS | local command exit 0 |
| Source CI `35619497269` | PASS; exact SHA `68f3869bc3530e538973c3e74f75f1b2b8d81b80` | all `source-checks` steps succeeded |

## Security/privacy review

- Listener/bind impact: none; no process or listener started.
- Secret/token handling: none; no provider/auth material read or written.
- Config mutation/rollback: none; only TEMP synthetic slot fixtures in tests.
- Path/TOCTOU: validation rejects reparse slot components and binds manifest/hash immediately before launch; process image is checked after start. Handle-retained launch is not claimed; residual pathname race remains documented.
- Lock topology: registry mutation uses GLOBAL; Manager public Start/Stop/Restart/Status use GLOBAL; Manager.Install calls `RegisterLocked` while already holding GLOBAL. No updater-to-public-Manager adapter was added, so no nested GLOBAL composition was introduced.
- Production pin: `upstreamlock` and `cliproxyconfig` validators are unchanged.
- Evidence: JSON is sanitized and contains no absolute TEMP path, fixture bytes, SID or secret.

## Acceptance evaluation

| Criterion | Status | Evidence |
|---|---|---|
| Closed logical version to immutable slot | PASS_COMPONENT | registry-contract.json |
| No raw path in State / no arbitrary resolver path | PASS | active-selection.json |
| Reparse/traversal/device/corruption fail closed | PASS_COMPONENT | registry/slot validation tests |
| No-replace and previous slot retention | PASS_COMPONENT | registry tests |
| Two synthetic slots coexist | PASS_COMPONENT | `TestRegistryResolvesTwoImmutableSlotsAndRejectsRebind` |
| Manager selects active state version | PASS_COMPONENT | instance tests |
| Status/Stop use recorded version | PASS_COMPONENT | `TestProcessRecordBindsRecordedSlotAcrossActiveStateChanges` |
| Updater common verifier contract | PASS_CONTRACT | updater-verifier.json |
| Real multi-version updater acceptance | OPEN | explicitly out of scope |
| Source CI exact SHA | PASS | run `35619497269`, exact SHA `68f3869bc3530e538973c3e74f75f1b2b8d81b80` |
| No live mutation | PASS | security-gate.json |

## Risks and unresolved items

| Risk/unknown | Severity | Required next action |
|---|---|---|
| Full production updater-to-Manager lifecycle composition could nest GLOBAL | High | Design a locked/internal lifecycle adapter in a later slice |
| Real second release/config adapter remains unverified | High | Keep production pin and run a separately authorized compatibility gate |
| Windows pathname TOCTOU between hash and process image inspection | Medium | Evaluate handle-based launch identity in a later hardening slice |

## Git delivery

- Commit: `68f3869bc3530e538973c3e74f75f1b2b8d81b80` (`feat(phase-2): add trusted installed-slot registry`)
- Push: `git push -u origin phase-2/installed-slot-registry` succeeded normally; GitHub compare: https://github.com/trungqwe/dual-pool/compare/2f5dd7b134ad19b9390af9c174095ef94ed09abc...68f3869bc3530e538973c3e74f75f1b2b8d81b80
- Authoritative upstream unchanged at `2f5dd7b134ad19b9390af9c174095ef94ed09abc`.

## Delivery receipt

Source CI run [`35619497269`](https://github.com/trungqwe/dual-pool/actions/runs/35619497269) completed with conclusion `success` for exact head `68f3869bc3530e538973c3e74f75f1b2b8d81b80`. Job `source-checks` and every named gate passed: format, module verification, vet, Go tests, race tests, pinned-upstream integration, Windows build, Phase 0 regression tests and candidate-lock validation.

## Next run

One objective: after Source CI and independent review pass, prepare a separate delivery-only run for controlled fast-forward consideration; do not start trusted registry production integration in that delivery run.
