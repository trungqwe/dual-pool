# Phase 2 updater/Manager lifecycle authority repair

## Header

- Run ID: `phase-2-updater-manager-lifecycle-authority-repair`
- Date/time UTC: 2026-09-21T20:03Z
- Roadmap phase: Phase 2
- Repository: `trungqwe/dual-pool`
- Branch: `phase-2/updater-manager-composition`
- Reviewed start HEAD: `473a71c372b8cb86b46e8997f67e2875424923be`
- Authoritative upstream: `a3060848f7b23edfbfb6c9b780841e7d5d6879eb` (unchanged)

## Objective and scope

Repair the updater composition authority boundary without starting the installed-slot registry or modifying `phase-2/upstream-lifecycle`. The repair must prevent external construction of a raw locked lifecycle, validate the required Manager layout even when dependencies are injected, preserve exact shared object identity, and record bounded component evidence only.

## Repair

- Replaced exported `instance.UpdaterLifecycle` and `Manager.UpdaterLifecycle()` with an unexported adapter. `instance.ComposeUpdater` is the only exported composition boundary; it accepts a Manager and returns a complete `update.Updater`.
- Moved Manager layout validation ahead of dependency-option handling. The required Root, Bin, Instances, State and Locks paths must be canonical absolute protected descendants of Root before supplied objects can be accepted.
- Added test-only dependency factories. Defaults are constructed only for absent options; valid supplied locks, Registry and State reader cause zero default-factory calls. Explicit nil supplied dependencies still fail closed.
- Added Registry lock-manager injection so Runtime, Manager, State Store and Registry use the exact same `lockfile.Manager`.

## Evidence and verification

| Claim | Status | Proof |
|---|---|---|
| External callers cannot obtain the raw locked lifecycle | PASS_COMPONENT | `TestRuntimeCannotExposeLockedUpdaterLifecycle` reflects the exported `instance.Manager` method set. |
| Updater owns a private adapter for the composed Manager | PASS_COMPONENT | `TestRuntimePrivateLifecycleUsesExactManager`. |
| Invalid layout cannot be bypassed with injected dependencies | PASS_COMPONENT | `TestInjectedDependenciesDoNotBypassLayoutValidation`. |
| Valid injected dependencies avoid default construction | PASS_COMPONENT | `TestInjectedDependenciesSuppressDefaultConstruction` counts all default factories. |
| Defaults remain available when dependencies are absent | PASS_COMPONENT | `TestDefaultDependenciesStillConstruct`. |
| One exact lock manager is shared by Runtime, Manager, Store, Registry and Updater | PASS_COMPONENT | `TestRuntimeCompositionSharesExactObjects`. |

Targeted tests, 50-run stress tests for `instance`, `runtimeupdate` and `update`, and ten-run race tests for the same packages passed. Full repository, repository race, module, vet, build, Node, upstream-lock, JSON, link, privacy, secret, scanner-control and diff gates are recorded in the delivery evidence after their execution.

## Claim boundary

All proof is `PASS_COMPONENT`. Fixtures use protected TEMP-only files, synthetic process records and deterministic private hooks. This run did not start a real updater, installed-slot registry, child process, listener, provider request or product/user configuration mutation. It does not demonstrate subprocess crash or power-loss durability, retained executable handles, a second trusted production slot, production Smoke, or live multi-version promotion.

## Delivery and next step

The source repair and its documentation/evidence are committed as separate scoped commits and pushed normally. The exact final Source CI receipt is the delivery gate. `phase-2/upstream-lifecycle` remains unchanged. A separate independent read-only audit may review the delivered SHA; it must not begin a registry or live-mutation slice.
