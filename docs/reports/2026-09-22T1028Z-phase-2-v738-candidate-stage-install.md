# Phase 2 — trusted v7.3.8 candidate staging and install

## Header

- Run ID: `phase-2-v738-candidate-stage-install-2026-09-22`
- Date/time UTC: 2026-09-22
- Agent/tool version: Codex implementation agent; Go toolchain from `go.mod`
- Roadmap phase: Phase 2 — pinned upstream lifecycle and isolation
- Repository: `trungqwe/dual-pool`
- Branch: `phase-2/v7.3.8-candidate-stage-install`
- Start HEAD: `5bfb1518059a171a0f3858877585e3097d9fe099`
- End HEAD: PENDING
- Remote push result: PENDING exact-SHA Source CI

## Assigned objective

Add a closed production path from the exact reviewed v7.3.8 receipt through staging, immutable slot installation, and Registry registration. Preserve the current v7.3.7 lock and APIs, do not change active selection, and do not promote or launch product instances.

## Non-goals

Production Smoke, promotion, rollback execution, updater CLI, retained launch handles, TOCTOU redesign, OS/process/power-loss crash proof, and product-root acceptance are outside this slice. No provider, OAuth, credential, listener, or live product action is in scope. The real archive test is opt-in and uses only a caller-supplied archive with a disposable TEMP product layout.

## Starting state

- Worktree status: new clean worktree and branch created at the exact authoritative upstream SHA.
- Existing unrelated changes: the original repository checkout had unrelated dirty files; it was left untouched. This run uses a separate clean worktree.
- Relevant prior report/handoff: v7.3.8 provenance receipt repair and current `docs/18-HANDOFF.md`.
- Reproduction result: focused baseline tests passed for upstreamcatalog, upstreamstage, installedslot, instance, runtimeupdate, and update before edits.

## Investigation and evidence

| Claim | Status | Evidence/source | Consequence |
|---|---|---|---|
| The production candidate authority returns exactly v7.3.8 from the reviewed embedded receipt and a validated current lock. | PASS_COMPONENT | `evidence/phase-2-v7.3.8-stage-install/candidate-authority.json`; `TestProductionCandidateReturnsOnlyReviewedV738` | Candidate selection has no caller-selected version or provenance. |
| Existing Stage remains pinned-lock-only; the new candidate wrapper derives authority before staging. | PASS_COMPONENT | `internal/upstreamstage/stage.go`; `TestStageCandidateUsesOnlyReviewedV738BeforeVerification` | Current v7.3.7 trust validation remains in place. |
| Stage cache and manifest identity bind the candidate provenance digest. | PASS_COMPONENT | `evidence/phase-2-v7.3.8-stage-install/staging.json`; digest-bound idempotence and mutation tests | Altered cached content fails closed. |
| Install independently validates stage files, exact candidate identity, manifest, ACL, reparse state, executable hash and binary identity. | PASS_COMPONENT | `evidence/phase-2-v7.3.8-stage-install/install.json`; candidate install and cross-install tests | Fabricated stage results do not establish trust. |
| Registry registration happens after atomic publication and post-publication verification under the install GLOBAL lock. | PASS_COMPONENT | `evidence/phase-2-v7.3.8-stage-install/registry.json`; synthetic install test | Registry remains an inventory bound to trusted provenance. |
| Current and candidate marker recovery is closed to exact authorities and converges through injected states. | PASS_COMPONENT | `evidence/phase-2-v7.3.8-stage-install/recovery.json` | Unknown/cross-release metadata and tampered artifacts remain fail-closed. |
| Installed candidate trust does not bypass Disposable Smoke before lifecycle mutation. | PASS_COMPONENT | `evidence/phase-2-v7.3.8-stage-install/promotion-boundary.json` | Smoke implementation and promotion remain outside this slice. |
| Real v7.3.8 TEMP stage/install acceptance was run. | SKIPPED | `DUALPOOL_V738_ARCHIVE` is unset; `TestRealV738TemporaryStageInstallOptIn` has no network fallback. | No real-release binary was downloaded or run. |

## Decisions

`ProductionCandidate(currentLock)` is the only production candidate resolver. `StageCandidate` and `InstallCandidate` accept only their bounded input objects. The internal generic engines stay unexported for the synthetic component fixtures. Legacy `lock_sha256` and `upstream_lock_sha256` JSON fields remain schema-compatible and contain the candidate provenance digest for candidate releases; no fake `upstream.lock` digest is created.

Runtime staging uses `<Bin>\\upstream-stage`, creates and inspects it lazily under GLOBAL, releases GLOBAL before calling `StageCandidate`, and uses the exact lock manager already shared by Runtime, State, Registry, Manager, and Updater. Runtime continues to expose only `Updater` and `Manager` fields.

## Changes

| Path | Change | Reason |
|---|---|---|
| `internal/upstreamcatalog/production.go`, `catalog_test.go` | Added exact reviewed-candidate resolver and identity regression. | Seal the v7.3.8 production authority. |
| `internal/upstreamstage/stage.go`, `stage_test.go` | Added candidate wrapper, private provenance stage engine, digest-bound cache, and tests for authority, ordering, idempotence and mutation conflicts. | Stage only trusted candidate bytes while preserving current pin behavior. |
| `internal/instance/instance.go` | Added candidate install wrapper and private common install/recovery engine; validated candidate stage and installed manifests against exact provenance. | Publish a verified immutable slot and register under shared GLOBAL. |
| `internal/instance/candidate_install_test.go`, `updater_composition_test.go`, `real_v738_windows_test.go` | Added synthetic transaction/attack/recovery tests, Smoke promotion-boundary sentinel, and opt-in offline real archive test. | Exercise failure states without routine network or product mutation. |
| `internal/runtimeupdate/runtime.go`, `runtime_test.go`, `runtime_api_test.go` | Added safe candidate stage/install operations with lazy protected root and shared lock; asserted exported API constraints. | Compose candidate operations without exposing mutable release authority. |
| `internal/installedslot/registry.go` | Documented legacy digest field semantics. | Clarify candidate provenance digest without changing registry schema. |
| `evidence/phase-2-v7.3.8-stage-install/` | Added sanitized authority, staging, install, recovery, registry and promotion-boundary JSON. | Make component claims reviewable without local paths or binaries. |
| `docs/15-MASTER-CHECKLIST.md`, `docs/18-HANDOFF.md` | Record evidence-backed slice status and exact next objective. | Keep project handoff aligned with the pushed head. |

## Verification

Exact focused, stress, race, full-repository, build, Node, upstream-lock, JSON, links, privacy, secret-scan and scanner-positive-control outcomes are in `evidence/phase-2-v7.3.8-stage-install/test-result.json` and `security-gate.json`. Commands required by the slice will be listed individually with their terminal status there. The opt-in real archive test is intentionally SKIPPED unless `DUALPOOL_V738_ARCHIVE` is provided.

## Security/privacy review

- Listener/bind impact: none; the optional real test invokes only the binary's bounded `-h` identity probe.
- Secret/token handling impact: no credential read or test credential created.
- Config mutation/rollback impact: none; only disposable TEMP product-layout files are used in component tests.
- Logging/evidence review: evidence contains reviewed public release hashes and test identifiers only; no archive, executable, local absolute path, config, key, or auth state is committed.
- Secret scan result: PENDING.
- New dependencies/supply-chain impact: none.

## Acceptance evaluation

The component acceptance matrix and remaining gates are recorded in the evidence directory. Claims are limited to the Windows source/component tests and the code's exact production wrappers. Product-root installation, production Smoke, promotion/rollback, retained handle, TOCTOU, and process/OS/power-loss durability remain OPEN.

## Risks and unresolved items

| Risk/unknown | Severity | Owner | Required next action |
|---|---|---|---|
| Real v7.3.8 archive stage/install has not run in a disposable TEMP layout. | Medium | Owner | Optionally set `DUALPOOL_V738_ARCHIVE` to an independently verified archive and run the opt-in test. |
| Production Smoke, promotion/rollback, retained launch handle and stronger crash/durability claims are not implemented or proven. | High | Next implementation run | Keep promotion unavailable until the next bounded Smoke objective passes. |
| Exact-SHA Source CI receipt is not yet available when this report is authored. | Delivery gate | Implementation agent | Push normally, verify exact final SHA Source CI, then append a delivery receipt to the handoff. |

## Git delivery

- Files committed: PENDING final scoped diff review.
- Commit(s): PENDING.
- Push command/result: PENDING normal push to `origin/phase-2/v7.3.8-candidate-stage-install`.
- Compare/PR URL if available: not created.
- Dirty state after push: PENDING.

## Rollback

Revert the scoped feature/evidence commit(s) on the feature branch. No user configuration, credentials, production install, active state, or upstream lifecycle ref is changed by this slice. The authoritative upstream branch remains at the start SHA.

## Next run

- Exact next objective: implement production compatibility Smoke for the already installed trusted v7.3.8 candidate, preserving INV-PROC-05 so promotion cannot stop active pools or publish active state until candidate Smoke passes.
- Entry criteria: owner/auditor review of the exact pushed branch SHA and Source CI success.
- Files/docs to read: `docs/00-README.md`, `docs/03-DECISIONS-AND-EVIDENCE.md`, `docs/05-INVARIANTS.md`, `docs/14-ROADMAP.md`, `docs/15-MASTER-CHECKLIST.md`, `docs/16-AGENT-OPERATING-PROTOCOL.md`, `docs/18-HANDOFF.md`, plus updater/instance/runtimeupdate tests.
- Commands/tests to run first: verify exact upstream/branch/head and Source CI receipt; run focused updater/runtimeupdate/instance tests.
- Stop conditions: any candidate identity mismatch, current-pin regression, shared-lock divergence, failed evidence/secret scan, or non-success exact-SHA Source CI.
