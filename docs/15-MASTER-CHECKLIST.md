# Master Checklist

This is the authoritative progress ledger. Check a box only with an adjacent evidence/report reference in the implementing commit.

## Governance

- [x] Repository remote and default branch verified; no protection workflow is configured yet. Evidence: `docs/reports/2026-09-18T1919Z-phase-0-github-bootstrap.md`.
- [x] Existing user changes inventoried and preserved. Evidence: initial empty-remote inventory in `docs/reports/2026-09-18T1919Z-phase-0-github-bootstrap.md`.
- [x] All agents have read required documents. Evidence: `docs/reports/2026-09-18T1919Z-phase-0-github-bootstrap.md`.
- [x] Current phase and scope recorded in handoff. Evidence: `docs/18-HANDOFF.md`.
- [x] Requirements/ADRs/tests updated for Phase 1 design changes. Evidence: `docs/reports/2026-09-20T0000Z-phase-1-exit-reconciliation.md`, `evidence/phase-1-exit-reconciliation/`.

## Phase 0

### Phase 0A — non-destructive discovery

- [x] Windows/architecture/tool and installed product version inventory captured. Evidence: `evidence/2026-09-18T1930Z-phase-0a-compatibility-inventory/environment.json`, `versions.json`.
- [x] Codex CLI/config candidate and extension inventory captured without reading or changing effective config. Evidence: `versions.json`.
- [x] Exact installed Codex bundled/effective catalog export supported; Astra descriptor captured. Evidence: `codex-catalog.json`.
- [x] Antigravity version, settings path candidate and static key candidate inventoried without mutation. Evidence: `antigravity-discovery.json`.
- [x] Candidate CLIProxyAPI release/hash/config/routes and credential-free loopback/auth behavior probed. Evidence: `cliproxy-schema.json`, `probe-results.json`.
- [x] Proposed and candidate callback port collision inventory captured. Evidence: `ports.json`.
- [x] Candidate lock verified-capability claims use an evidence-backed closed allowlist; unknown and duplicate names fail closed. Evidence: `scripts/phase0-upstream-lock.test.cjs`, `docs/reports/2026-09-19T1523Z-phase-0-lock-allowlist.md`.
- [x] U-001 through U-008 reviewed with partial evidence kept open. Evidence: `docs/03-DECISIONS-AND-EVIDENCE.md`.

### Phase 0B — reversible mutation-dependent compatibility

- [x] Codex CLI custom provider sends Responses to loopback; Gate A v5 PASS với natural exit, sentinel scan và cleanup. Evidence: [v5 result](../evidence/phase-0b-v5/codex-cli-transport-v5-20260918T212522665Z.json). Chưa chứng minh extension/config-layer hoặc Windows 11.
- [ ] Codex picker retains desired provider route — `BLOCKED`: the isolated two-request probe reached loopback, but the extension's JSON body was not safely parseable; no Astra request was accepted. Evidence: [two-request result](../evidence/phase-0b-codex-extension/codex-extension-two-request-20260919T031615744Z.json).
- [ ] Antigravity setting path/key and loopback behavior proven reversibly.
- [ ] Native route and schema inventory captured safely.
- [ ] Transparent passthrough proven.
- [ ] Donor native envelope round trip proven.
- [x] U-001 through U-008 resolved or explicitly classified for Phase 0 exit; U-008 remains `PARTIAL_UNKNOWN` and blocks provider promotion. Evidence: `evidence/phase-0-closure.json`.

## Foundation

- [x] Go CLI skeleton and version metadata. Evidence: `internal/app/app_test.go`, `internal/buildinfo/buildinfo_test.go`, `evidence/phase-1-foundation-bootstrap/test-result.json`.
- [x] Stable errors and exit codes. Evidence: `internal/apperr/apperr_test.go`, `evidence/phase-1-foundation-bootstrap/test-result.json`.
- [x] Structured allowlist logger. Evidence: `internal/safelog/safelog_test.go`, `evidence/phase-1-dataroot-logging/test-result.json`.
- [x] State/ownership schema and migrations. Evidence: `internal/state/`, `evidence/phase-1-state-schema/test-result.json`, `docs/reports/2026-09-19T1620Z-phase-1-state-schema-migrations.md`.
- [x] Atomic store and crash recovery. Evidence: `internal/state/store_test.go`, `evidence/phase-1-atomic-store/`, `docs/reports/2026-09-19T1700Z-phase-1-atomic-store-recovery.md`.
- [x] Global/per-file locks with PID identity and handle-safe stale reclamation. Evidence: `internal/lockfile/`, `internal/state/store_test.go`, `evidence/phase-1-locks/`, `evidence/phase-1-lock-handle-correction/`, `docs/reports/2026-09-19T1733Z-phase-1-lock-handle-race-correction.md`.
- [x] Windows secret-store implementation reviewed. Evidence: `internal/secretstore/`, `evidence/phase-1-secret-store/`, `docs/reports/2026-09-19T1821Z-phase-1-windows-secret-store.md`, Source CI `35462429013`.
- [x] Config backup/patch/rollback engine. Evidence: `internal/configtxn/`, `evidence/phase-1-config-transaction/`, `docs/reports/2026-09-19T1900Z-phase-1-config-transaction-engine.md`; synthetic TEMP fixtures only, product-root/backup ACL acceptance remains open.
- [x] Redaction and sentinel-secret tests. Phase 1 E3 component/end-to-end foundation PASS; future doctor bundle, release packaging and live provider gates remain open. Evidence: `internal/securitygate/sentinel_test.go`, `evidence/phase-1-exit-reconciliation/sentinel-scan.json`.
- [x] Phase 1 reusable logger redaction and sentinel unit layer. Full end-to-end `LOG-001` remains open. Evidence: `internal/safelog/safelog_test.go`, `evidence/phase-1-dataroot-logging/security-gate.json`.

## CLIProxyAPI lifecycle

- [x] Candidate `upstream.lock` populated with independently verified Windows artifact metadata; provider support remains unverified. Evidence: `evidence/phase-0-closure.json`.
- [x] Download checksum mismatch fails closed. Evidence: `internal/upstreamstage/stage_test.go` and `evidence/phase-2-upstream-stager/pinned-integration.json`.
- [x] Pinned binary version/commit mismatch fails closed after both artifact hashes pass.
- [x] Config adapter version mismatch fails closed; pinned `config_adapter_version` is `dualpool-cpa-v7.3.7-config-v1`. Evidence: [security gate](../evidence/phase-2-instance-config/security-gate.json).
- [x] Codex and Google configs have disjoint paths/keys/ports. Evidence: [isolation gate](../evidence/phase-2-instance-config/isolation-gate.json).
- [x] Both instances bind only `127.0.0.1`. Evidence: `9082104`, `evidence/phase-2-empty-lifecycle/real-lifecycle.json`, `docs/reports/2026-09-20T0932Z-phase-2-real-harness-hardening.md`.
- [x] Management requires key and disallows remote. Evidence: `9082104`, `internal/instance/real_windows_test.go`, `evidence/phase-2-empty-lifecycle/real-lifecycle.json`.
- [x] Process identity prevents foreign PID termination. Evidence: `internal/instance/instance_test.go` (`TestStopRecordUsesOneVerifiedHandle`), `9082104`.
- [x] Start/stop/restart/status idempotent. Evidence: `internal/instance/real_windows_test.go`, `evidence/phase-2-empty-lifecycle/real-lifecycle.json`.
- [ ] Staged update and automatic rollback tested.

## Accounts

- [ ] OAuth flows use Management API only.
- [ ] OAuth URL validation and state timeout/cancel.
- [ ] Ambiguous new credential detection fails safely.
- [ ] Credential-specific model inventory.
- [ ] Tiny generation/Responses capability probe.
- [ ] Eligibility exclusion with stable reason.
- [ ] List/test/disable/enable/reauth commands.
- [ ] Poolbridge never downloads/parses auth token files.
- [ ] One or more eligible accounts per pool.

## Antigravity

- [ ] Bridge modes and fixed upstream allowlist.
- [ ] Streaming, backpressure and cancellation.
- [ ] Model/catalog requests passthrough.
- [ ] Exact donor selector and catalog fingerprint.
- [ ] Generation endpoint allowlist.
- [ ] Native envelope mapping golden fixtures.
- [ ] Google-down degraded behavior.
- [ ] Non-donor parity suite.
- [ ] Donor route reaches only Google port.
- [ ] Session affinity and failover.
- [ ] Tool/file/terminal behavior.
- [ ] 20+ turn and restart test.
- [ ] Emergency rollback tested with services down.

## Codex

- [ ] User-level custom provider patch is reversible.
- [ ] Secret reaches client without plaintext config/log.
- [ ] Canonical Astra descriptor validated.
- [ ] Picker/provider-retention test.
- [ ] Exact-version catalog fallback only if needed.
- [ ] Responses HTTP/SSE event contract.
- [ ] WebSocket separately tested or explicitly disabled.
- [ ] Tool/apply_patch/shell/multi-file behavior.
- [ ] Reasoning and compaction behavior.
- [ ] Parent/subagent affinity.
- [ ] 30+ turn and restart test.
- [ ] Traffic reaches only Codex port.

## Security and privacy

- [ ] No wildcard/LAN listeners. CPA instances PASS; future bridge/service coverage remains open.
- [x] Four distinct strong secrets. `P2-ENTRY-KEYS-001` PASS: four independent 32-byte product keys in Windows Credential Manager, pairwise distinct and unchanged on the second run. Evidence: `evidence/phase-2-product-init/key-gate.json`, `docs/reports/20260920T034857Z-phase-2-product-init.md` and `docs/reports/20260920T0420Z-phase-2-product-init-live.md`.
- [x] Restricted ACLs verified. `P2-ENTRY-ACL-001` PASS: creation-time protected DACL, current user and LocalSystem only, seven children verified. Evidence: `evidence/phase-2-product-init/acl-gate.json` and `docs/reports/20260920T0420Z-phase-2-product-init-live.md`.
- [ ] No open proxy/SSRF route.
- [ ] No prompt/output/tool/source persistence.
- [ ] No raw session/account identity in logs.
- [ ] Secret scan passes repo, report and ZIP.
- [ ] Supply-chain verification and SBOM.
- [ ] No binary/VSIX/CA/hosts patch.

## Release and handoff

- [ ] Unit, contract, integration and live gates pass.
- [ ] Performance threshold passes or waiver accepted.
- [ ] Doctor and diagnostic bundle validated.
- [ ] Rollback/uninstall without network tested.
- [x] `docs/18-HANDOFF.md` updated. Evidence: current handoff delivery receipt.
- [x] Run report added under `docs/reports/`. Evidence: `docs/reports/2026-09-18T1930Z-phase-0a-compatibility-inventory.md`.
- [x] Applicable Phase 0A JSON/hash/security/privacy/cleanup checks and docs links pass; runtime tests are not yet in scope. Evidence: `probe-results.json` and the run report.
- [x] Commit contains no unrelated files. Evidence: final staged name/status review in the run report.
- [x] Push succeeds without force. Evidence: commit `3f70ed84cbc52772ed1e448f3ee891c6f3343bbe` on `origin/phase-0/compatibility-inventory`.
- [x] Commit SHA and compare/PR URL recorded in handoff. Evidence: `docs/18-HANDOFF.md`.
## Phase 0B parser-repair addendum — 2026-09-19

- [x] v2 parser self-tests and privacy-safe serializer checks pass.
- [x] One isolated baseline request reached `POST /v1/responses`; route, auth, identity encoding, UTF-8, JSON, prompt, sanitizer, and response lifecycle passed.
- [ ] Baseline model gate remains open: `BASELINE_MODEL_MISMATCH`; U-005 is PARTIAL / UNKNOWN and U-006 remains BLOCKED. Evidence: [v2 result](../evidence/phase-0b-codex-extension-v2/extension-baseline-20260919T040521877Z.json).
## Phase 0B authenticated-picker addendum — 2026-09-19

- [x] Corrected the unauthenticated Astra observation to `NON_DIAGNOSTIC_FOR_ASTRA_ENTITLEMENT`.
- [x] Closed U-005 as `PROBED` for the observed Windows 10/runtime scope using existing config-layer evidence.
- [ ] U-006 authenticated picker probe: `BLOCKED: AUTHENTICATED_PROBE_UNAVAILABLE`; the isolated custom-provider extension exposed no login UI and no real auth state was reused.
- [ ] U-006 auth-reuse follow-up: `BLOCKED: CLI_AUTH_NOT_CONSUMED_BY_EXTENSION`; opaque CLI auth status succeeded, but the isolated extension exposed no login UI and did not recognize the copied state. Evidence: `evidence/phase-0b-codex-u006-auth-reuse/`.
- [ ] U-006 opaque profile-clone follow-up: `BLOCKED: CLONED_AUTH_STATE_NOT_RECOGNIZED`; Astra dropdown visibility was observed, but no Codex Plus account was present and the Probe remained reconnecting. Evidence: `evidence/phase-0b-u006-profile-clone/`.
- [ ] U-006 primary-profile follow-up: `BLOCKED: NORMAL_RELAUNCH_FAILED`; temporary user-level config mutation was restored byte-for-byte, but no auth/Astra/wire checkpoint ran. Evidence: `evidence/phase-0b-u006-primary-profile/`.
- [x] Primary watchdog incident reconciled and real-config live mutation disabled. Evidence: `docs/reports/2026-09-19T0905Z-phase-0b-primary-watchdog-incident-audit.md`, `scripts/phase0b-primary-watchdog-disabled.test.ps1`.
- [x] Safe replacement U-006 shadow `CODEX_HOME` harness repaired with `real_config_mutation_capability=false` and `crash_requires_restore=false`; live probe remains separate and unrun. Evidence: `docs/reports/2026-09-19T1128Z-phase-0b-shadow-harness-repair.md`.
- [x] Shadow U-006 repair gate: 15/15 fixture/orchestration tests PASS; read-only current-config assessment is marker-free and TOML-valid, without claiming historical equivalence. Evidence: `evidence/phase-0b-u006-shadow-home/`.
- [x] Shadow U-006 lifecycle safety repair: SHADOW-009..013 fixed; 20/20 tests PASS; ACL, external launcher, fail-closed observation, retained-shadow timeout, and normal-reopen acceptance are covered. Evidence: `docs/reports/2026-09-19T1254Z-phase-0b-shadow-lifecycle-repair.md`.
- [x] Shadow watchdog stall repair: dynamic close-wait heartbeat and strict checkpoint input; 22/22 watchdog tests and 48/48 Node tests PASS. Evidence: `evidence/phase-0b-u006-shadow-home/stall-repair.json`.
- [ ] U-006 live shadow run: `BLOCKED: WATCHDOG_STALL_REQUIRES_FORCED_OWNED_PROCESS_CLEANUP`; cleanup PASS, real config unchanged, no accepted Astra wire evidence. Evidence: `evidence/phase-0b-u006-shadow-home/live-shadow-result-20260919T1335Z.json`.
- [ ] Audit primary-profile: PASS cũ không đủ bằng chứng; watchdog chặn trước mutation. Xem [audit](reports/2026-09-19T0814Z-phase-0b-primary-audit.md). Sửa và kiểm thử orchestration/rollback trước lần chạy live tiếp theo.
- [x] Sửa safety primary-profile: 25 Node tests, 12 assertion transaction, syntax/privacy/docs/diff PASS; chỉ là gate chuẩn bị, U-006 vẫn chờ live. [Report](reports/2026-09-19T0840Z-phase-0b-primary-repair.md).

## Phase 2 trusted installed-slot registry component — 2026-09-21

- [x] Closed logical version to immutable trusted slot mapping (component). Evidence: `evidence/phase-2-installed-slot-registry/registry-contract.json`.
- [x] Strict slot manifest/hash/ACL/platform/adapter/provenance validation (component). Evidence: `evidence/phase-2-installed-slot-registry/slot-validation.json`.
- [x] Manager active selection and schema-2 recorded slot identity (component). Evidence: `evidence/phase-2-installed-slot-registry/active-selection.json`.
- [x] Updater `SlotVerifier` common registry contract (component). Evidence: `evidence/phase-2-installed-slot-registry/updater-verifier.json`.
- [x] Required focused/stress/race/full local gates. Evidence: `evidence/phase-2-installed-slot-registry/test-result.json`.
- [ ] Production multi-version updater acceptance and automatic rollback. Intentionally remains open.
- [ ] Full updater-to-Manager lifecycle composition under one GLOBAL lock. Intentionally remains open.
