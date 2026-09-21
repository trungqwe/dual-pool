# Phase 2 updater/Manager composition

## Header

- Run ID: `phase-2-updater-manager-composition`
- Date/time UTC: 2026-09-21T18:17Z
- Roadmap phase: Phase 2
- Repository: `trungqwe/dual-pool`
- Branch: `phase-2/updater-manager-composition`
- Start HEAD: `a3060848f7b23edfbfb6c9b780841e7d5d6879eb`
- End HEAD: recorded by Git history and the final delivery receipt
- Remote push result: recorded after delivery

## Assigned objective

Compose the production updater object graph with `instance.Manager` under one GLOBAL lock, then prove promotion, rollback and process-crash recovery with a shared TEMP-only vA/vB trust fixture. Acceptance requires stress, race, repository, security and exact-SHA Source CI gates.

## Non-goals

No updater CLI, real second production release, real production multi-version promotion, production Smoke implementation, retained executable handle, automatic migration, OS-crash/power-loss claim, provider request or user/product mutation was attempted.

## Starting state

The clean isolated worktree started at the authoritative `phase-2/upstream-lifecycle` SHA. The existing installed-slot registry and updater were retained. No unrelated Phase 0 changes were present.

## Investigation and evidence

| Claim | Status | Evidence/source | Consequence |
|---|---|---|---|
| Updater, Manager and Store use one lock manager | PASS_COMPONENT | [composition](../../evidence/phase-2-updater-manager-composition/composition.json), `TestRuntimeCompositionSharesExactObjects` | GLOBAL ownership has one runtime authority. |
| Updater and Manager use one Store and Registry | PASS_COMPONENT | [composition](../../evidence/phase-2-updater-manager-composition/composition.json) | State and installed-slot truth cannot diverge through duplicate production objects. |
| Adapter avoids nested GLOBAL acquisition | PASS_COMPONENT | [lock topology](../../evidence/phase-2-updater-manager-composition/lock-topology.json) | Update transactions call only locked Manager internals. |
| Synthetic promotion and rollback converge | PASS_COMPONENT | [promotion/rollback](../../evidence/phase-2-updater-manager-composition/promotion-rollback.json) | vB candidate and vA rollback identities remain record-authoritative. |
| Process-crash recovery converges or fails closed | PASS_COMPONENT | [recovery](../../evidence/phase-2-updater-manager-composition/recovery.json) | Markers are removed only after proven convergence. |
| Production one-pin registry preflight works | PASS_COMPONENT | `TestProductionCompositionResolvesPinnedSlot`, `TestProductionRegistryRejectsCandidateBeforeLifecycleMutation` | No production multi-version claim is made. |

## Decisions

`instance.UpdaterLifecycle` is the sole updater bridge and assumes the updater already owns GLOBAL. It validates and canonicalizes pool sets before mutation. `runtimeupdate.New` creates exactly one lock manager, Store and Registry and injects those same objects into Updater and Manager. Private test hooks exercise the real adapter and updater transaction engine without launching CLIProxyAPI.

## Changes

| Path | Change | Reason |
|---|---|---|
| `internal/instance/instance.go` | Added updater lifecycle adapter, locked delegates, pool validation and shared-lock injection. | Prevent nested locking and preserve Manager identity checks. |
| `internal/instance/updater_composition_test.go` | Added shared vA/vB transaction, rollback, recovery and lock-order tests. | Prove composed behavior through actual records/state. |
| `internal/instance/updater_lifecycle_test.go` | Added invalid pool and nil lock regression tests. | Fail closed before lifecycle mutation. |
| `internal/runtimeupdate/runtime.go` | Added production object-graph constructor. | Centralize shared runtime dependencies. |
| `internal/runtimeupdate/runtime_test.go` | Added object identity, one-pin production and constructor tests. | Prove graph identity and preflight behavior. |

## Verification

| Test/command | Result | Evidence |
|---|---|---|
| `go test ./internal/instance -count=50` | PASS | [test result](../../evidence/phase-2-updater-manager-composition/test-result.json) |
| `go test ./internal/runtimeupdate -count=50` | PASS | same |
| `go test ./internal/update -count=50` | PASS | same |
| Three package race commands with `-count=10` | PASS | same |
| `go test -count=1 ./...` and repository race | PASS | same |
| modules, vet, disposable build | PASS | same |
| Node suite | PASS, 55/55 | same |
| candidate lock validation | PASS | same |
| JSON, links, privacy, secrets, scanner control, diff | PASS | [security gate](../../evidence/phase-2-updater-manager-composition/security-gate.json) |

## Security/privacy review

- Listener/bind impact: no real listener was created; lifecycle state is synthetic and TEMP-only.
- Secret/token handling impact: no credentials, auth files or provider material were read.
- Config mutation/rollback impact: no real configuration was modified.
- Logging/evidence review: evidence contains no local absolute path, SID, raw identity or secret.
- New dependencies/supply-chain impact: none.

## Acceptance evaluation

All requested component-level transaction, rollback, recovery, stress, race and repository gates pass. Claims are limited to `PASS_COMPONENT`. Production multi-version behavior and live mutation gates remain open.

## Risks and unresolved items

| Risk/unknown | Severity | Owner | Required next action |
|---|---|---|---|
| No real second trusted production slot | High | next Phase 2 run | Establish independently verified second-release metadata and adapter compatibility before live promotion. |
| Production Smoke remains injected | High | next Phase 2 run | Implement and audit a credential-free production Smoke contract. |
| Launch identity is path based rather than retained-handle based | Medium | later hardening | Design exact-object launch handoff without weakening registry checks. |
| OS crash and sudden power loss | Medium | later recovery gate | Run a separate approved crash durability campaign. |

## Git delivery

Only the scoped source, tests, report, handoff, checklist, traceability and evidence files are committed. The exact commits, normal push and Source CI receipt are recorded in the final owner report and Git history. `phase-2/upstream-lifecycle` is not moved.

## Rollback

Revert the slice commits. No credential, user configuration, real process, listener or product state needs restoration because none was mutated.

## Next run

Independently audit this composition diff and its evidence at the exact delivered SHA before introducing a real second trusted release or production Smoke implementation.
