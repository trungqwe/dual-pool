# Phase 0B Final Assisted Gates

## Starting state

- Run ID: `2026-09-18T2054Z-phase-0b-final-assisted-gates`
- Branch: `phase-0/reversible-compatibility`
- Start HEAD: `a22e70bb913b6a547cbdf23d792c4280a33f219d`
- Remote: `https://github.com/trungqwe/dual-pool.git`
- Worktree: clean at start.

## Objective

Repair the remaining v2 evidence defects, run a trustworthy v3 Codex CLI transport gate, then proceed to user-assisted Codex picker and Antigravity compatibility checkpoints only if Gate A passes. Phase 1 is out of scope.

## Non-goals

- No Phase 1 runtime, merge, or production integration.
- No OAuth, token inspection, auth-file copying, binary/VSIX patching, TLS interception, hosts edits, or CA installation.
- No automatic desktop UI manipulation where policy prohibits it.

## Planned probes and checkpoints

- v3 loopback recorder with independent in-memory sentinel assertions and persisted-artifact scans.
- Final machine-readable manifest, status supersession, and security-gate evidence.
- User-assisted Codex extension picker checkpoint after v3 PASS.
- User-assisted Antigravity U-001 checkpoint, followed by U-002/U-003 only if their prerequisites pass.

## Expected files

- `scripts/phase0b-codex-cli-transport-v3.ps1`
- Append-only v3 evidence under `evidence/2026-09-18T1953Z-phase-0b-reversible-compatibility/`
- This report, mutable current-state docs, and delivery handoff.

## Security assertions

Only metadata, safe schema key/type paths, event names, and derived booleans may be persisted. Secret and prompt sentinels, request bodies, header values, prompts, session values, and tool arguments must be absent from committed artifacts.

## Stop conditions

Stop on any v3 assertion failure, non-loopback listener, raw artifact persistence, failed secret/privacy scan, config restoration mismatch, concurrent edit, unavailable required user action, OAuth/token requirement, or any request to begin Phase 1.

## Initial decision

The historical v2 report and evidence remain immutable. This report and the v3 evidence are the prospective correction record.

## Execution result

- `P0B-CX-TRANSPORT-001`: `FAIL` at Gate A. The recorder verified the request in memory in the successful capture attempt, but the Codex process did not terminate naturally after the synthetic SSE response; the harness was stopped and the owned temporary process/tree was cleaned manually.
- The v3 evidence therefore does not authorize user configuration mutation. Codex picker and Antigravity checkpoints were not run.
- The v2 and v3 evidence deficiencies are retained as historical/diagnostic evidence; current status is in `phase-0b-status-v3.json`.
- Phase 0 outcome: `BLOCKED` until the v3 harness has a bounded, independently recorded process exit and passes all assertions.
