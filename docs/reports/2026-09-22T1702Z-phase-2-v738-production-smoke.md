# Phase 2 — v7.3.8 production compatibility Smoke

## Header

- Run ID: phase-2-v738-production-smoke
- Date/time UTC: 2026-09-22T1702Z
- Roadmap phase: Phase 2
- Repository: trungqwe/dual-pool
- Branch: phase-2/v7.3.8-production-smoke
- Start HEAD: 4d56626a361b10abfdef63d898a59267f1f6a84d
- End HEAD: PENDING (delivery receipt will be recorded in handoff)
- Remote push result: pending

## Assigned objective

Implement Manager-owned production compatibility Smoke for trusted v7.3.8 while preserving the existing updater transaction and INV-PROC-05 boundaries.

## Non-goals

No real archive was supplied; no product-root installation, live provider action, promotion, rollback rehearsal, retained launch handle, TOCTOU, OS/power-loss durability or updater CLI was attempted.

## Investigation and evidence

| Claim | Status | Evidence/source | Consequence |
|---|---|---|---|
| Production Smoke authority is closed and Manager-owned | VERIFIED | `evidence/phase-2-v7.3.8-production-smoke/composition.json`, runtime API tests | Caller cannot bypass pre/post Smoke with a no-op object |
| Disposable smoke is isolated | VERIFIED | `disposable.json`, `disposable-auth.json`, instance tests | Synthetic keys and protected State-relative workspace only |
| Production readiness reuses shared contract | VERIFIED | `production.json`, instance tests | No nested GLOBAL acquisition |
| Real v7.3.8 archive acceptance | UNKNOWN | `DUALPOOL_V738_ARCHIVE` absent; opt-in test skipped | Remains open for next authorized run |

## Changes

| Path | Change | Reason |
|---|---|---|
| `internal/instance/smoke.go` | Private Manager-owned Smoke implementation | Enforce trusted disposable and Production checks |
| `internal/cliproxyconfig/render.go` | Narrow compatibility renderer | Reuse production adapter semantics |
| `internal/runtimeupdate/runtime.go` | Remove public Smoke injection | Close production authority bypass |
| `internal/winacl/winacl.go` | Exclusive protected attempt creation | Prevent scratch collision/residue reuse |
| tests | Smoke, composition and rollback regressions | Prove boundaries and failure semantics |
| `evidence/phase-2-v7.3.8-production-smoke/` | Sanitized evidence | Make claims reviewable |

## Verification

| Test/command | Result | Evidence |
|---|---|---|
| Focused x20 changed suites | PASS | `test-result.json` |
| Stress x50 | PASS | `test-result.json` |
| Race x10 changed suites | PASS | `test-result.json` |
| `go test ./...` | PASS | `test-result.json` |
| `go test -race -count=1 ./...` | PASS | `test-result.json` |
| `go vet ./...`, `go mod verify` | PASS | `test-result.json` |
| Windows build | PASS | `test-result.json` |
| Node 55 tests and lock validator | PASS | `test-result.json` |
| Opt-in real archive test | SKIPPED | `DUALPOOL_V738_ARCHIVE` unset |

## Security/privacy review

- Listener/bind impact: disposable process is constrained to one ephemeral IPv4 loopback listener; production ports are not used.
- Secret/token handling impact: synthetic keys only; no production secretstore read; no key/config artifacts committed.
- Config mutation/rollback impact: disposable workspace is owned and removed exactly; updater transaction tests preserve rollback semantics.
- Logging/evidence review: sanitized JSON only; no absolute user paths, raw keys or process dumps.
- Secret scan result: PASS.
- New dependencies/supply-chain impact: none.

## Acceptance evaluation

`PASS_COMPONENT_SMOKE`: implementation and deterministic tests pass. This does not claim live promotion or product-root acceptance.

## Risks and unresolved items

| Risk/unknown | Severity | Required next action |
|---|---|---|
| Real v7.3.8 archive path was not supplied | Medium | Run separately with explicitly supplied offline archive |
| OS/process/power-loss durability and retained handle | High | Separate authorized acceptance slices |

## Git delivery

- Files committed: pending
- Commit(s): pending
- Push command/result: pending
- Dirty state after push: pending

## Rollback

Revert the implementation commit(s) on this branch; `phase-2/upstream-lifecycle` remains unchanged at the start SHA.

## Next run

Exactly one next objective: execute controlled real offline v7.3.8 acceptance through StageCandidate → InstallCandidate → Disposable Smoke in a disposable protected product layout, then prepare the first explicitly authorized live promotion/rollback acceptance run.
