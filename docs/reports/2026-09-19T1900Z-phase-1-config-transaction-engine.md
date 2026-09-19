# Phase 1 — config transaction engine

## Header

- Run ID: `20260919T1900Z-phase-1-config-transaction-engine`
- Date/time UTC: `2026-09-19T19:00:00Z`
- Roadmap phase: Phase 1
- Branch: `phase-1/state-foundation`
- Worktree: isolated Codex worktree
- Start HEAD: `94db0342dbb8c1401fcc87dceaae1531d01f3ca6`
- Go: `go1.26.0 windows/amd64`
- End HEAD: PENDING

## Objective

Repair fail-closed synthetic WinCred cleanup, then implement a TEMP-only transactional Codex TOML configuration engine with exact owned-key mutation, byte-preserving surgical edits, verified backup, locks, CAS, ownership intent, recovery and conflict-safe rollback.

## Non-goals

- No real Codex or Antigravity config, real product root, provider account, OAuth, CLIProxyAPI, product key generation, product startup or CLI config command.
- No Antigravity production adapter or guessed key. U-001..004 and U-006 remain BLOCKED; U-008 remains PARTIAL_UNKNOWN.
- No product ACL acceptance, catalog generation, reconfiguration of an already-owned target to different desired values or Phase 2 work.

## Owned-key scope

Production planning is closed to `model`, `model_provider`, `model_providers.dualpool_codex` and an explicitly authorized `model_catalog_json`. Ordinary plans keep catalog fallback disabled. The provider table accepts only fixed string fields `name`, `base_url`, `wire_api` and `env_key`; the latter stores an environment-variable name, never a secret.

## TOML and dependency strategy

Pin `github.com/pelletier/go-toml/v2` at an exact tagged MIT-licensed version for full-document semantic parsing and validation. Use a constrained internal line/span adapter for only canonical top-level string assignments and the exact regular provider table. The serializer is not used. Unstable parser types do not cross the adapter boundary; unsupported dotted/quoted aliases, inline/array tables, duplicates, nested or unexpected provider content fail closed. Golden tests lock byte ranges and formatting behavior.

## Transaction protocol

Acquire GLOBAL; parse and hash target; create, sync, re-read and verify an exact unique backup; build and verify a sibling candidate; create a strict synced marker; persist exact ownership records as PENDING; acquire the per-target lock; recheck target hash; call `ReplaceFileW` with flags zero; re-read semantic/hash verification and sync target; release target lock; persist ownership APPLIED; remove marker; release GLOBAL.

`ReplaceFileW` is used so Windows can preserve supported target metadata, including DACL/security and selected attributes. Unsupported write-through, ignore-merge and ignore-ACL flags are not used. Microsoft documents that the resulting target has the replacement file's identity, so the original identity is evidence for the pre-write state rather than an invariant after replacement.

## Journal and recovery protocol

A strict small marker contains operation, transaction ID, target resource ID, PRE/POST hashes, backup basename/hash and exact owned key paths, never values or absolute target paths. Recovery matches the complete marker set against ownership records. PRE aborts pending intent; POST validates semantic values and finalizes; missing/corrupt target restores a valid backup; other valid hashes conflict. Rollback uses the same PRE/POST truth derivation and keeps diagnostic state on ambiguity.

## Rollback semantics

Rollback first requires every current owned value to equal its recorded applied value. It restores original values/presence through surgical edits, preserves unrelated post-apply edits, and treats owned-value drift as `ErrRollbackConflict`. A second rollback is a safe no-op. A same-plan apply is a no-write success; a different desired plan fails as unsupported reconfiguration.

## Encoding and insertion rules

Accept UTF-8 with optional BOM and uniform LF or CRLF up to a conservative bounded size. Preserve BOM and newline exactly. Existing scalar edits replace only the TOML value token. Missing top-level scalars are inserted before the first table; a missing provider table is appended with one deterministic separator. Rollback removes only statements/tables inserted by Poolbridge.

## Fault points

`AFTER_BACKUP_SYNC`, `AFTER_CANDIDATE_SYNC`, `AFTER_MARKER_SYNC`, `AFTER_PENDING_OWNERSHIP`, `BEFORE_TARGET_CAS`, `AFTER_TARGET_REPLACE`, `AFTER_TARGET_VERIFY`, `AFTER_TARGET_SYNC`, `BEFORE_OWNERSHIP_FINALIZE`, `AFTER_OWNERSHIP_FINALIZE`, and `BEFORE_MARKER_CLEANUP`, with corresponding rollback coverage.

## Official sources checked

Checked 2026-09-19 UTC:

- Microsoft `ReplaceFileW`: https://learn.microsoft.com/en-us/windows/win32/api/winbase/nf-winbase-replacefilew
- `go-toml/v2/unstable` API and compatibility warning: https://pkg.go.dev/github.com/pelletier/go-toml/v2/unstable
- go-toml tagged releases: https://github.com/pelletier/go-toml/releases

## Stop conditions

Stop on cleanup leakage, unsafe/ambiguous TOML, missing recoverable ownership intent, failed PRE/POST recovery, lock-order violation, rollback conflict overwrite, real-config access, secret/path leakage, regression failure or remote CI failure. Do not weaken CAS, preservation or rollback gates.

## Investigation and evidence

- Audited `94db034` and confirmed the production secret-store contract was sound; corrected only the synthetic WinCred cleanup proof.
- Verified Microsoft `ReplaceFileW` semantics and selected flags zero. The engine records pre-write identity from volume serial/file index and does not assert identity stability across replacement.
- Evaluated and pinned `github.com/pelletier/go-toml/v2 v2.4.3` (MIT). Only its stable root decoder validates full documents; an internal constrained locator owns edit spans, and no dependency type crosses the package boundary.
- Evidence: `evidence/phase-1-config-transaction/`. It contains classifications/counts only, with no path, config body or secret.

## Changes

- Added `internal/configtxn` with a closed plan, strict errors/marker codec, surgical TOML adapter, Windows identity/replacement boundary, apply/recover/rollback engine, deterministic fault injection and TEMP-only tests.
- Added exact backup/candidate verification, marker/ownership ordering, locks, CAS, semantic verification, PRE/POST/damaged/drift recovery, safe idempotence and conflict-preserving rollback.
- Added 11 Apply fault boundaries, 8 rollback recovery boundaries, three abrupt subprocess crash cases, global contention and target-lock proof, strict marker/size/path/collision tests, and CFG-001..004 component coverage.
- Repaired `SECRET-TEST-CLEANUP-001` so fallback cleanup reports failures and final reads prove all four exact synthetic targets are absent.
- Updated state/rollback design, checklist, handoff, negative matrix and traceability. Phase 1 remains open.

## Verification

| Gate | Result |
|---|---|
| `go fmt ./...` | PASS |
| `go vet ./...` | PASS |
| `go test -count=1 ./...` | PASS; 88 Go test functions |
| `go test -race -count=1 ./...` | PASS; lock subprocess stress 215.095 s |
| `go build ./cmd/poolbridge` | PASS |
| `go mod verify` / `go list -m all` | PASS; TOML v2.4.3 exact |
| `node --test scripts/*.test.cjs` | PASS; 53/53 |
| `node scripts/phase0-upstream-lock.cjs` | `UPSTREAM_LOCK_VALID` |
| JSON parse / `git diff --check` | PASS |
| Source CI | PENDING after push |

No repository-provided documentation-link checker exists; local relative-link validation is run before commit. Real Codex/Antigravity configs and the product root were not accessed or changed.

## Git delivery

- Implementation commit: PENDING.
- Push and observable Source CI: PENDING.
- Delivery receipt commit: PENDING.

## Next run

Phase 1 exit reconciliation and remaining-foundation gate audit. Do not begin Phase 2 automatically.
