# Requirements Traceability Matrix

This initial matrix defines minimum test identities. Implementation must add exact test file/evidence links.

| Requirement/invariant | Tests | Evidence level | Status |
|---|---|---|---|
| FR-001/002, INV-POOL-01/02 | ISO-001..004, CPA-INT-001 | E3 | Open |
| FR-003, INV-NET-01/02 | NET-001, NET-INT-001 | E3 | Open |
| FR-004..009 | AUTH-001..004, AUTH-LIVE-001 | E3/E4 | Open |
| FR-010, INV-ROUTE-05 | AG-DIFF-001 | E4 | Open |
| FR-011/012, INV-ROUTE-01..04 | ROUTE-001..003, AG-E2E-001 | E3/E4 | Open |
| FR-013/014, INV-ROUTE-06 | AG-CONTRACT-001, AG-DIFF-002 | E3/E4 | Open |
| FR-015..017, INV-ROUTE-07/08 | CX-001..003, CX-E2E-001 | E4 | Open |
| FR-018/019 | CX-CATALOG-001, CX-PICKER-001 | E3/E4 | Open |
| FR-020 | AFF-001/002, AFF-LIVE-001 | E4 | Open |
| FR-021..023, INV-CFG-01..07 | CFG-001..004, CFG-PROP-001 | E3 | Open |
| FR-024 | DOC-001, DOC-BUNDLE-001 | E3 | Open |
| FR-025, INV-PROC-03..05 | UPD-001..003, SUP-001 | E3 | Open |
| NFR-001, INV-LOG-01..05 | LOG-001, DEL-001, SECRET-001 | E3 | Open |
| NFR-002 | ISO-001..004, ISO-LIVE-001 | E4 | Open |
| NFR-003 | CFG-ROLL-001, AG-ROLL-001 | E4 | Open |
| NFR-004 | AG-LONG-001, CX-LONG-001 | E4 | Open |
| NFR-005 | PROC-001..003 | E3 | Open |
| NFR-006 | PERF-AG-001, PERF-CX-001 | E4 | Open |
| NFR-007/008 | BUILD-001, ARCH-001 | E3/review | Open |
| NFR-009 | EVIDENCE-SCHEMA-001 | E3 | Open |
| NFR-010 | PRIV-001 | E3/review | Open |
| Phase 0A repository/environment/port discovery | P0A-REPO-001, P0A-ENV-001, P0A-PORT-001 | E2 | Probed — `evidence/2026-09-18T1930Z-phase-0a-compatibility-inventory/probe-results.json` |
| ADR-006 installed Astra identity and safe export | P0A-CODEX-001 | E2 | Probed — `evidence/2026-09-18T1930Z-phase-0a-compatibility-inventory/codex-catalog.json` |
| INV-NET-01/02/03 candidate behavior | P0A-CPA-001 | E2 | Candidate probed; production E3 gate remains open |
| INV-PROC-04 candidate supply chain | P0A-SUP-001 | E2 | Candidate probed; production pin/promotion gate remains open |
| INV-LOG-06 evidence boundary | P0A-SEC-001, P0A-CLEANUP-001 | E2/review | Probed — `evidence/2026-09-18T1930Z-phase-0a-compatibility-inventory/probe-results.json` |

## Update rule

No requirement may be marked DONE until every mapped mandatory test passes for the pinned release/target environment. Add new rows when behavior is discovered; never remove a mapping solely because implementation is difficult.
## Phase 0B evidence-repair addendum

| Requirement | Evidence | Status |
|---|---|---|
| P0B-CX-TRANSPORT-001: CLI Responses, lifecycle, sentinel, cleanup | [v5 script](../scripts/phase0b-codex-cli-transport-v5.ps1), [v5 result](../evidence/phase-0b-v5/codex-cli-transport-v5-20260918T212522665Z.json) | PASS — Windows 10 exploratory; U-005 remains PARTIAL / UNKNOWN |
| Extension picker retention | [two-request probe](../evidence/phase-0b-codex-extension/codex-extension-two-request-20260919T031615744Z.json) | BLOCKED — baseline loopback request arrived, but payload parsing failed before model/sentinel proof; Astra request not run |
| Antigravity setting/redirect | User-assisted reload and loopback discovery | BLOCKED |
| Antigravity route/envelope | Depends on U-001 and U-002 | BLOCKED |
| Credential-specific Gemini eligibility | Separately authorized provider probe | BLOCKED |
## Phase 0B parser-repair addendum

| P0B-CX-EXTENSION-PARSER-V2 | [v2 recorder](../scripts/phase0b-extension-recorder-v2.cjs), [v2 runner](../scripts/phase0b-codex-extension-baseline-v2.ps1), [v2 result](../evidence/phase-0b-codex-extension-v2/extension-baseline-20260919T040521877Z.json) | Parser/sanitizer and ingress controls PASS; overall gate BLOCKED by `BASELINE_MODEL_MISMATCH`; U-005 PARTIAL / UNKNOWN; U-006 BLOCKED |
## Phase 0B authenticated-picker addendum

| P0B-CX-CONFIG-LAYER-001 | [config-resolution](../evidence/phase-0b-codex-extension-u006/config-resolution.json), [U-005 correction](../evidence/phase-0b-codex-extension-u006/u005-audit-correction.json) | U-005 PROBED for observed Windows 10/runtime scope |
| P0B-CX-MODEL-RESOLUTION-001 | [U-006 result](../evidence/phase-0b-codex-extension-u006/u006-result.json) | BLOCKED: AUTHENTICATED_PROBE_UNAVAILABLE; no picker or wire request run |
| P0B-CX-AUTH-REUSE-001 | [auth bootstrap](../evidence/phase-0b-codex-u006-auth-reuse/auth-bootstrap-20260919T053402170Z.json), [U-006 result](../evidence/phase-0b-codex-u006-auth-reuse/u006-20260919T053402492Z.json) | Opaque CLI auth copy authenticated, but extension did not consume it; BLOCKED: CLI_AUTH_NOT_CONSUMED_BY_EXTENSION |
| P0B-CX-PROFILE-CLONE-001 | [profile-clone report](../docs/reports/2026-09-19T0553Z-phase-0b-u006-profile-clone.md), [profile-clone evidence](../evidence/phase-0b-u006-profile-clone/) | COLD copy reached Probe; dropdown Astra was visible but authenticated account was not recognized; BLOCKED: CLONED_AUTH_STATE_NOT_RECOGNIZED |
| P0B-CX-PRIMARY-PROFILE-001 | [primary-profile report](../docs/reports/2026-09-19T0715Z-phase-0b-u006-primary-profile.md), [primary-profile evidence](../evidence/phase-0b-u006-primary-profile/) | Config restored byte-for-byte, but watchdog could not relaunch Antigravity; BLOCKED: NORMAL_RELAUNCH_FAILED |
| P0-CLOSURE-001 | [closure result](../evidence/phase-0-closure.json), [candidate lock](../upstream.lock), [validator](../scripts/phase0-upstream-lock.cjs) | PASS_WITH_BLOCKERS: Phase 1 fixture foundation GO; live IDE/provider integration and release NO-GO |
| P0-LOCK-ALLOWLIST-001 | [validator](../scripts/phase0-upstream-lock.cjs), [negative tests](../scripts/phase0-upstream-lock.test.cjs), [repair report](reports/2026-09-19T1523Z-phase-0-lock-allowlist.md) | PASS: unknown, duplicate and non-string verified capability claims fail closed; candidate lock remains valid |

## Phase 1 foundation bootstrap

| Requirement | Test and evidence | Status |
|---|---|---|
| P1-CLI-001: supported commands and usage failure | [app tests](../internal/app/app_test.go), [result](../evidence/phase-1-foundation-bootstrap/test-result.json) | PASS; Source CI `35452170609` |
| P1-VERSION-001: deterministic and safe build output | [buildinfo tests](../internal/buildinfo/buildinfo_test.go), [result](../evidence/phase-1-foundation-bootstrap/test-result.json) | PASS; Source CI `35452170609` |
| P1-ERROR-001: exact categories and stable code registry | [error tests](../internal/apperr/apperr_test.go), [result](../evidence/phase-1-foundation-bootstrap/test-result.json) | PASS; Source CI `35452170609` |
| P1-ERROR-REDACTION-001: wrapped cause excluded from human and JSON output | [error tests](../internal/apperr/apperr_test.go), [security gate](../evidence/phase-1-foundation-bootstrap/security-gate.json) | PASS; Source CI `35452170609` |
| P1-CI-001: source-only Windows workflow | [workflow](../.github/workflows/ci.yml), [result](../evidence/phase-1-foundation-bootstrap/test-result.json) | PASS; Source CI `35452170609` |

## Phase 1 data-root and logging foundation

| Requirement | Test and evidence | Status |
|---|---|---|
| P1-DATAROOT-001: deterministic local-only resolver without filesystem mutation | [resolver tests](../internal/dataroot/dataroot_test.go), [result](../evidence/phase-1-dataroot-logging/test-result.json) | PASS; Source CI `35453552820` and delivery CI `35453645973` |
| P1-LOG-ALLOWLIST-001: typed closed fields, families, levels, routes and error codes | [logger tests](../internal/safelog/safelog_test.go), [result](../evidence/phase-1-dataroot-logging/test-result.json) | PASS; Source CI `35453552820` and delivery CI `35453645973` |
| P1-LOG-REDACTION-001: unit sentinel/path/email/token rejection with zero output | [logger tests](../internal/safelog/safelog_test.go), [security gate](../evidence/phase-1-dataroot-logging/security-gate.json) | Foundation/unit PASS; end-to-end LOG-001 remains open |
| P1-SESSION-FINGERPRINT-001: keyed deterministic truncated HMAC | [fingerprint tests](../internal/safelog/safelog_test.go), [result](../evidence/phase-1-dataroot-logging/test-result.json) | PASS; Source CI `35453552820` and delivery CI `35453645973` |
| P1-APPERR-UNKNOWN-001: unknown code cannot downgrade to usage error | [error tests](../internal/apperr/apperr_test.go), [result](../evidence/phase-1-dataroot-logging/test-result.json) | PASS; Source CI `35453552820` and delivery CI `35453645973` |

## Phase 1 state schema and migration foundation

| Requirement | Test and evidence | Status |
|---|---|---|
| P1-STATE-SCHEMA-001: typed state v1, account metadata and isolated ports | [state tests](../internal/state/state_test.go), [result](../evidence/phase-1-state-schema/test-result.json) | PASS; Source CI `35455036621` |
| P1-OWNERSHIP-SCHEMA-001: complete ownership records and closed typed values | [ownership tests](../internal/state/ownership_test.go), [result](../evidence/phase-1-state-schema/test-result.json) | PASS; persistence and rollback execution closed by config transaction evidence |
| P1-STATE-CODEC-001: strict fields, duplicate keys, UTF-8, limits and deterministic round trip | [codec implementation](../internal/state/codec.go), [state tests](../internal/state/state_test.go) | PASS; Source CI `35455036621` |
| P1-MIGRATION-001: explicit sequential validated migration chain | [migration tests](../internal/state/migration_test.go), [result](../evidence/phase-1-state-schema/test-result.json) | PASS locally with synthetic 7→8→9 fixtures; product supports v1 only |
| P1-STATE-SECRET-BOUNDARY-001: closed schema and opaque secret references | [schema tests](../internal/state/state_test.go), [security gate](../evidence/phase-1-state-schema/security-gate.json) | PASS at schema/unit boundary; end-to-end secret scan remains open |

## Phase 1 atomic state store foundation

| Requirement | Test and evidence | Status |
|---|---|---|
| P1-OWNERSHIP-PATH-DEVICE-001: reserved Windows device components rejected | [ownership regression](../internal/state/ownership_test.go), [security gate](../evidence/phase-1-atomic-store/security-gate.json) | PASS; Source CI `35456545843` |
| P1-STORE-ATOMIC-001: synced candidate, immutable marker and post-commit target sync | [store tests](../internal/state/store_test.go), [result](../evidence/phase-1-atomic-store/test-result.json) | Component PASS locally; cross-document atomicity excluded |
| P1-STORE-CAS-001: concurrent target drift is not overwritten | [CAS test](../internal/state/store_test.go), [security gate](../evidence/phase-1-atomic-store/security-gate.json) | PASS; process-lock dependency closed by mutation lock evidence |
| P1-STORE-RECOVERY-001: hash-based NEW/OLD/ABSENT/backup reconciliation | [recovery tests](../internal/state/store_test.go), [crash matrix](../evidence/phase-1-atomic-store/crash-matrix.json) | PASS; Source CI `35456545843` |
| P1-STORE-CORRUPTION-001: corrupt target, marker, candidate and backup fail closed | [corruption tests](../internal/state/store_test.go), [security gate](../evidence/phase-1-atomic-store/security-gate.json) | PASS locally |
| P1-STORE-CRASH-INJECTION-001: deterministic fault matrix and abrupt subprocess exits | [crash tests](../internal/state/store_test.go), [crash matrix](../evidence/phase-1-atomic-store/crash-matrix.json) | 22 in-process scenarios and 2 subprocess boundaries PASS locally |
| P1-STORE-WINDOWS-REPLACE-001: production `ReplaceFileW` and `MoveFileExW` behavior | [Windows integration tests](../internal/state/store_test.go), [result](../evidence/phase-1-atomic-store/test-result.json) | PASS locally in disposable TEMP fixtures |
| CFG-002/CFG-003 state persistence foundation | P1-STORE-ATOMIC-001, P1-STORE-RECOVERY-001 | Component proof only; external config transaction acceptance remains open |

## Phase 1 mutation lock foundation

| Requirement | Test and evidence | Status |
|---|---|---|
| P1-LOCK-GLOBAL-001: global mutual exclusion and crash recovery | [lock tests](../internal/lockfile/manager_test.go), [result](../evidence/phase-1-locks/test-result.json) | PASS in disposable TEMP subprocess fixtures |
| P1-LOCK-FILE-001: canonical per-file exclusion, independence and alias handling | [lock tests](../internal/lockfile/manager_test.go), [security gate](../evidence/phase-1-locks/security-gate.json) | PASS; raw target paths never enter lock filenames |
| P1-LOCK-PID-IDENTITY-001: PID, exact creation FILETIME and canonical image | [Windows inspector](../internal/lockfile/process_windows.go), [lock tests](../internal/lockfile/manager_test.go) | PASS for lock-owner identity; Phase 2 child PID-file lifecycle remains open |
| P1-LOCK-PID-REUSE-001: reused PID and image mismatch are stale identities | [lock tests](../internal/lockfile/manager_test.go), [stale matrix](../evidence/phase-1-locks/stale-lock-matrix.json) | PASS without terminating the observed process |
| P1-LOCK-STALE-RECOVERY-001: exact record recheck and bounded retry | [lock manager](../internal/lockfile/manager.go), [stale matrix](../evidence/phase-1-locks/stale-lock-matrix.json) | PASS |
| P1-LOCK-UNVERIFIABLE-001: ambiguous owner fails closed | [lock tests](../internal/lockfile/manager_test.go), [security gate](../evidence/phase-1-locks/security-gate.json) | PASS; expiry cannot override owner verification |
| P1-LOCK-STORE-INTEGRATION-001: Store lock spans initial read through cleanup | [Store tests](../internal/state/store_test.go), [security gate](../evidence/phase-1-locks/security-gate.json) | PASS; former CAS/replace writer window closed |
| P1-LOCK-RECOVERY-RACE-001: recovery cannot delete a live writer candidate | [Store tests](../internal/state/store_test.go), [security gate](../evidence/phase-1-locks/security-gate.json) | PASS in-process and subprocess fixtures |
| P1-STORE-RECOVERY-HYGIENE-001: all referenced artifacts checked before cleanup | [Store regression](../internal/state/store_test.go), [security gate](../evidence/phase-1-locks/security-gate.json) | PASS; unsafe backup preserves candidate, marker and target |
| PROC-001 lock-owner identity foundation | P1-LOCK-PID-IDENTITY-001, P1-LOCK-PID-REUSE-001 | Component PASS; CLIProxyAPI child ownership and termination remain open for Phase 2 |
| P1-LOCK-HANDLE-OWNERSHIP-001: every returned Guard retains exclusive canonical handle | [handle tests](../internal/lockfile/manager_test.go), [correction gate](../evidence/phase-1-lock-handle-correction/security-gate.json) | PASS; incompatible write/delete opens are rejected while Guard lives |
| P1-LOCK-STALE-RACE-001: simultaneous stale reclaim has one owner | [subprocess stress](../internal/lockfile/manager_test.go), [race result](../evidence/phase-1-lock-handle-correction/stale-reclaim-race.json) | PASS; 50 global and 50 per-file iterations, zero multiple owners |
| P1-LOCK-HANDLE-RELEASE-001: release deletes exact owned object by handle | [lock manager](../internal/lockfile/manager.go), [correction gate](../evidence/phase-1-lock-handle-correction/security-gate.json) | PASS; no canonical pathname deletion |
| P1-LOCK-HANDLE-CONTENTION-001: share mode blocks competing mutation/delete | [handle tests](../internal/lockfile/manager_test.go), [race result](../evidence/phase-1-lock-handle-correction/stale-reclaim-race.json) | PASS on Windows TEMP fixtures |

## Phase 1 Windows secret store

| Requirement | Test and evidence | Status |
|---|---|---|
| P1-SECRET-STORE-API-001: closed four-purpose registry, validation and safe errors | [unit tests](../internal/secretstore/store_test.go), [result](../evidence/phase-1-secret-store/test-result.json) | PASS; Source CI `35462429013` |
| P1-SECRET-CREDMAN-001: generic local-machine exact-target CRUD and replacement | [WinCred integration](../internal/secretstore/windows_test.go), [decision](../evidence/phase-1-secret-store/decision.json) | PASS on local and GitHub-hosted Windows; Source CI `35462429013` |
| P1-SECRET-CROSS-PROCESS-001: child process reads exact credential without secret transport | [WinCred integration](../internal/secretstore/windows_test.go), [result](../evidence/phase-1-secret-store/test-result.json) | PASS; Source CI `35462429013` |
| P1-SECRET-ISOLATION-001: four synthetic values remain purpose-isolated | [WinCred integration](../internal/secretstore/windows_test.go), [security gate](../evidence/phase-1-secret-store/security-gate.json) | PASS; Source CI `35462429013`; does not complete product-key generation |
| P1-SECRET-CLEANUP-001: exact pre/post cleanup and idempotent delete | [WinCred integration](../internal/secretstore/windows_test.go), [security gate](../evidence/phase-1-secret-store/security-gate.json) | PASS; Source CI `35462429013`; no enumeration |
| P1-SECRET-NO-ENUMERATION-001: production has no broad credential API | [source gate](../internal/secretstore/store_test.go), [security gate](../evidence/phase-1-secret-store/security-gate.json) | PASS |
| P1-SECRET-NO-PROVIDER-TOKEN-001: store owns no provider credential | [source gate](../internal/secretstore/store_test.go), [security gate](../evidence/phase-1-secret-store/security-gate.json) | PASS; FR-005 preserved |

## Phase 1 config transaction engine

| Requirement | Test and evidence | Status |
|---|---|---|
| P1-SECRET-CLEANUP-FAILCLOSED-001: fallback cleanup reports failure and final exact-target reads prove absence | [WinCred integration](../internal/secretstore/windows_test.go), [security gate](../evidence/phase-1-config-transaction/security-gate.json) | PASS; four synthetic targets, no enumeration |
| P1-CONFIG-TOML-SURGICAL-001: parser validation, literal-span edits, raw unrelated-byte and BOM/newline preservation | [TOML tests](../internal/configtxn/toml_test.go), [result](../evidence/phase-1-config-transaction/test-result.json) | PASS on LF, CRLF and BOM+CRLF synthetic fixtures |
| P1-CONFIG-BACKUP-001: exact verified exclusive pre-write backup | [engine tests](../internal/configtxn/engine_test.go), [security gate](../evidence/phase-1-config-transaction/security-gate.json) | PASS; product backup ACL gate remains open |
| P1-CONFIG-CAS-001 / CFG-001: final external-writer CAS | [engine tests](../internal/configtxn/engine_test.go), [fault matrix](../evidence/phase-1-config-transaction/fault-matrix.json) | PASS; external bytes preserved |
| P1-CONFIG-OWNERSHIP-001: PENDING precedes replace and APPLIED follows verified commit | [engine](../internal/configtxn/engine.go), [fault matrix](../evidence/phase-1-config-transaction/fault-matrix.json) | PASS |
| P1-CONFIG-RECOVERY-001 / CFG-003: PRE/POST/damaged/drift recovery protocol | [recovery tests](../internal/configtxn/recovery_test.go), [fault matrix](../evidence/phase-1-config-transaction/fault-matrix.json) | PASS at component level; three abrupt subprocess boundaries included |
| P1-CONFIG-ROLLBACK-001: restore original presence/value and preserve unrelated edits | [rollback tests](../internal/configtxn/engine_test.go), [rollback matrix](../evidence/phase-1-config-transaction/rollback-matrix.json) | PASS |
| P1-CONFIG-ROLLBACK-CONFLICT-001 / CFG-004: owned drift fails closed | [engine tests](../internal/configtxn/engine_test.go), [security gate](../evidence/phase-1-config-transaction/security-gate.json) | PASS |
| P1-CONFIG-IDEMPOTENCE-001: repeated Apply/Rollback/Recover | [engine and recovery tests](../internal/configtxn/), [result](../evidence/phase-1-config-transaction/test-result.json) | PASS |
| P1-CONFIG-GLOBAL-LOCK-001: process contention before transaction preparation; target lock spans CAS/replace | [subprocess test](../internal/configtxn/recovery_test.go), [security gate](../evidence/phase-1-config-transaction/security-gate.json) | PASS |
| CFG-002: pre-replace persistence/replacement failure | [fault tests](../internal/configtxn/recovery_test.go), [fault matrix](../evidence/phase-1-config-transaction/fault-matrix.json) | PASS; original intact |

## Phase 1 exit reconciliation

| Requirement | Test and evidence | Status |
|---|---|---|
| P1-CONFIG-PATH-ANCESTOR-001 / CFG-PATH-001 | [path tests](../internal/configtxn/path_test.go), [exit security gate](../evidence/phase-1-exit-reconciliation/security-gate.json) | PASS; every ancestor inspected, aliases retained, reparses/devices rejected |
| P1-CONFIG-RECOVERY-ARTIFACT-001 | [recovery tests](../internal/configtxn/recovery_test.go), [exit security gate](../evidence/phase-1-exit-reconciliation/security-gate.json) | PASS; handle-backed reads, complete prevalidation, retryable cleanup |
| P1-CONFIG-ROLLBACK-STATUS-001 | [engine tests](../internal/configtxn/engine_test.go), [exit security gate](../evidence/phase-1-exit-reconciliation/security-gate.json) | PASS; conflict persisted and repeated rollback remains non-mutating |
| P1-CONFIG-TOML-EDGE-001 | [TOML/engine tests](../internal/configtxn/), [exit security gate](../evidence/phase-1-exit-reconciliation/security-gate.json) | PASS; deterministic no-table insertion and exact no-newline rollback |
| P1-SENTINEL-NONDISCLOSURE-001 / Phase 1 NFR-001 foundation | [sentinel test](../internal/securitygate/sentinel_test.go), [sentinel result](../evidence/phase-1-exit-reconciliation/sentinel-scan.json) | E3 PASS with zero forbidden matches; doctor/release/live surfaces remain open |
| Phase 1 roadmap exit: atomicity, concurrent edits, idempotence, redaction, recovery | [exit result](../evidence/phase-1-exit-reconciliation/phase1-exit.json), [run report](reports/2026-09-20T0000Z-phase-1-exit-reconciliation.md) | PASS; no Phase 1 blockers |
| P2-ENTRY-ACL-001 / P2-ENTRY-KEYS-001 | [deferred gates](../evidence/phase-1-exit-reconciliation/deferred-gates.json) | Deferred Phase 2 entry gates before real product resources |
