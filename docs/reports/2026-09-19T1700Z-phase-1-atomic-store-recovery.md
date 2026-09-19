# Phase 1 atomic store and recovery

## Header

- Run ID: `phase-1-atomic-store-recovery`
- Date/time UTC: 2026-09-19T17:00:00Z
- Agent/tool version: Codex / Go 1.26.0 windows/amd64
- Roadmap phase: Phase 1 — Foundation and state safety
- Repository: `https://github.com/trungqwe/dual-pool`
- Branch: `phase-1/state-foundation`
- Worktree: dedicated clean Phase 1 worktree
- Start HEAD: `31cb158cd0176dd48793048812624cec0e129771`
- End HEAD: PENDING
- Remote push result: PENDING

## Assigned objective

Implement a crash-consistent application-level store for the two fixed state documents using disposable TEMP fixtures: synced sibling candidates, immutable synced recovery intent, Windows replacement/install primitives, CAS, hash-based explicit recovery, and deterministic crash injection.

## Non-goals

No real `%LOCALAPPDATA%\DualPool` initialization, process locks, PID identity, CLI state command, external config mutation, secret store, process lifecycle, OAuth, IDE integration, listener, doctor, update flow, cross-document transaction, or complete product ACL policy.

## Starting state

- Local and remote Phase 1 HEAD: `31cb158cd0176dd48793048812624cec0e129771`.
- Authoritative Phase 0 remote HEAD: `6619034bc7edd79af3101ee7015a3de724b41542`.
- Worktree status: clean; unrelated checkouts remain untouched.
- Baseline: 34 Go test functions, 53 Phase 0 Node tests, `UPSTREAM_LOCK_VALID`.

## Decisions

- Correct ownership paths first by rejecting Windows reserved device components, including names with extensions.
- Store directory is injected; fixed document identities map only to `state.json` and `ownership.json`.
- Existing files use Win32 `ReplaceFileW`; first creation uses same-directory `MoveFileExW` with write-through. `os.Rename` is not the commit primitive.
- Pin `golang.org/x/sys` at `v0.48.0`. `NewLazySystemDLL` restricts system DLL lookup; `MoveFileEx` is exported, while the isolated wrapper resolves `ReplaceFileW` from `kernel32.dll` through that secure boundary.
- A recovery marker is immutable and contains only version, document kind, opaque transaction ID, old/new hashes, and owned sibling basenames.
- Candidate, marker, and final target are synced at their required boundaries. The claim is crash-consistent application behavior under tested Windows semantics, not a formal filesystem transaction or universal power-loss guarantee.
- `Load*` is read-only and returns recovery-required while a marker exists. `Recover` is explicit and idempotent.
- State and ownership transactions are independent.
- CAS detects drift immediately before the single replace/install call. Process serialization remains open work.

## Fault points and planned tests

Fault points: after candidate create/write/sync, after marker sync, before CAS, after CAS before replace, after replace, after target verify/sync, before backup cleanup, and before marker cleanup.

Tests cover reserved device paths; first create and replacement for both documents using production Windows primitives; sync and round trip; corrupt existing targets; CAS drift; marker guard; recovery truth table; invalid marker/candidate/backup; idempotence; all fault points for old and absent targets; and subprocess exits after marker sync and after replacement.

## Stop conditions

Stop on any gate failure, unsafe reparse handling, real product-root access, unverified replacement semantics, target corruption without a trustworthy recovery copy, path/secret leakage, remote divergence, or CI failure.

## Verification

| Command/gate | Result | Evidence |
|---|---|---|
| `go fmt`, `go vet`, tests, race, build | PASS; 47 Go test functions | `evidence/phase-1-atomic-store/test-result.json` |
| Fault matrix | PASS; 22 in-process scenarios plus 2 subprocess exits | `evidence/phase-1-atomic-store/crash-matrix.json` |
| `go mod verify`, `go list -m all` | PASS; only `golang.org/x/sys v0.48.0` | Test result |
| Phase 0 regression and lock validator | PASS; 53/53 and `UPSTREAM_LOCK_VALID` | Test result |
| JSON/docs/privacy/path/secret/diff gates | PASS | `evidence/phase-1-atomic-store/security-gate.json` |

## Security/privacy review

- Real product root touched: false; TEMP-root guards and evidence scan PASS.
- Evidence will contain classifications and OLD/NEW/ABSENT relations only.
- ACL hardening remains tied to later product-root initialization work.
- Dependency and license/source impact: one pinned low-level Go module, `golang.org/x/sys v0.48.0`; module hashes verified. No broad persistence library was added.

## Next run

Global/per-file locks with PID identity and stale-lock recovery, only after this slice passes all gates.
