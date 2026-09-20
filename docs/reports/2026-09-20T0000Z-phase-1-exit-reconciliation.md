# Phase 1 — exit reconciliation and repair

## Header

- Run ID: `20260920T0000Z-phase-1-exit-reconciliation`
- Date/time UTC: `2026-09-20T00:00:00Z`
- Roadmap phase: Phase 1
- Branch: `phase-1/state-foundation`
- Worktree: isolated Codex worktree
- Starting HEAD: `b437ff888ae0432c716627312992bb584067f7b1`
- Result: PENDING

## Phase 1 deliverable inventory

- Go CLI/version, stable errors, local data-root contract and allowlist logger.
- State/ownership schema, migrations, atomic Store recovery and mutation locks.
- Windows Credential Manager decision and exact four-purpose synthetic proof.
- Fixture-only Codex TOML backup/patch/recovery/rollback engine.
- Windows source/fixture CI and Phase 0 regression gates.

## Open checklist rows at start

- Phase 1 E3 sentinel non-disclosure gate.
- Requirements/ADR/test reconciliation for the exit repair.
- Top-level restricted ACL and four product-key rows remain unclassified between Phase 1 and Phase 2 entry.

## Audit findings

- `CFG-PATH-001` HIGH: config path validation does not inspect every ancestor component for reparse points.
- `CFG-RECOVERY-ARTIFACT-001` MEDIUM: recovery reads and best-effort removes marker/backup/candidate paths without a complete prevalidated artifact set.
- `CFG-ROLLBACK-STATUS-001` MEDIUM: owned-value rollback drift returns conflict without persisting `RollbackConflict`.
- TOML edge debt: no-table/no-terminal-newline and equal-offset insertion ordering lack explicit proof.
- Root README/bootstrap status and prospective traceability are stale.

## Planned repairs

1. Add component-aware Windows ancestry validation while retaining legitimate case/short-name aliases.
2. Validate the complete recovery artifact set before reads/removals and return cleanup failures for retry.
3. Recheck rollback drift under the target lock, persist conflict status, and keep repeated rollback idempotently conflicted.
4. Make equal-offset TOML insertions deterministic and add exact edge round trips.
5. Add a runtime-generated Phase 1 sentinel non-disclosure test across current foundation surfaces.
6. Reconcile README, architecture/security/test docs, checklist, traceability, deferred Phase 2 entry gates and exit evidence.

## Phase 1 exit criteria

- Atomicity: PENDING audit.
- Concurrent-edit protection: PENDING audit.
- Idempotence: PENDING audit.
- Redaction: PENDING E3 gate.
- Recovery tests: PENDING repair and regression.

## Deferred later-phase gates

- Candidate classification: `P2-ENTRY-ACL-001` before any real secret-containing product root/config creation.
- Candidate classification: `P2-ENTRY-KEYS-001` before two real CLIProxyAPI configs/processes are created.
- U-001..004 and U-006 remain BLOCKED; U-008 remains PARTIAL_UNKNOWN for later live/provider work.

## Stop conditions

Stop with Phase 1 `BLOCKED` on any remaining HIGH/MEDIUM foundation defect, failed exit criterion, failed regression/CI, unsafe real-config/product-root access, or ambiguous recovery mutation. Do not start Phase 2.

## Investigation, changes, verification and delivery

- Implemented component-by-component Windows ancestry inspection, consistent device-name/trailing-dot/trailing-space rejection, and alias-compatible lexical handling.
- Added handle-backed no-reparse reads for recovery artifacts, complete cleanup-set validation, reported/retryable cleanup failures, and unsafe substitution tests.
- Added target-lock recheck plus persisted/idempotent `RollbackConflict` classification.
- Combined no-table tail insertions deterministically and added parser/engine no-terminal-newline exact rollback coverage.
- Added runtime-generated Phase 1 sentinel coverage and sensitive state/ownership value rejection. No sentinel value/hash is persisted.
- Local focused tests pass. Full regression, CI and Phase 1 exit decision remain PENDING.
