# Phase 2 updater audit repair — work in progress

## Header

- Run ID: `20260921T0840Z-phase-2-updater-audit-repair-wip`.
- Roadmap phase: Phase 2.
- Branch: `codex/phase2-updater-audit-repair`.
- Start HEAD: `4eb550e731f21b6a05347cd84df2fdb3b349a6b4`.
- End HEAD: WIP commit pending at time of writing.
- Delivery: NOT_READY; trạng thái WIP được commit và push theo yêu cầu của chủ sở hữu để kiểm toán độc lập, không phải bằng chứng chấp nhận sửa chữa hoặc bàn giao registry.

## Objective

Repair UPD-AUDIT-001 lifecycle crash convergence and UPD-AUDIT-002 marker
object-identity/TOCTOU safety with synthetic Windows fixtures only.

## Changes attempted

- Added a stateful lifecycle regression showing that an `AfterMarkerPublish`
  crash with the prior pools still running must not invoke duplicate `Start()`.
- Added partial-running-set recovery regression; partial state must retain the
  marker and return `ErrRecoveryUnresolved`.
- Changed recovery to observe the running set: an exact set is accepted without
  `Start`, an empty set is started, and any partial/unexpected set fails closed.
- Began replacing marker pathname read/delete with exclusive Windows handles,
  handle-based DACL inspection and handle-based rename/delete.

## Evidence and current result

The two lifecycle regressions were RED before the recovery change and PASS in
focused execution after the change.

The handle-marker implementation is not stable. Focused individual tests can
pass, but `go test -count=10 ./internal/update` repeatedly fails across marker
publish/load/remove paths. Failures include `ErrRecoveryUnresolved`, missing
marker observations after `AfterMarkerPublish`, and rollback tests that cannot
complete marker removal. This means the current handle rename/delete design is
not accepted and must not be committed.

The latest failing command was:

```text
go test -count=10 ./internal/update
exit 1
```

`go test -count=1 ./internal/update` and race/full-suite runs also exposed
intermittent marker failures after the implementation attempt. No real updater,
product root, real credential target, listener, provider credential, IDE config
or account was touched.

## Required next objective

Diagnose and replace the unstable marker handle publication/load/removal design
until focused tests are deterministic under `-count=10`, then continue the
fixture-backed repair. Do not start installed-slot registry work.
