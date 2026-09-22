# Phase 2 v7.3.8 stage/install ACL seam repair

## Header

- Run ID: `phase-2-v738-stage-install-acl-seam-repair-2026-09-22`
- Date/time UTC: 2026-09-22T13:03Z
- Roadmap phase: Phase 2 — trusted v7.3.8 candidate staging/install
- Repository: `trungqwe/dual-pool`
- Branch: `phase-2/v7.3.8-candidate-stage-install`
- Start HEAD: `a4bc87bc4370a8010a8619ade147f53d3689eaed`
- Repair code/evidence HEAD: `ae9cd3350aeb5b0a6937f14b254706cedc1132b0`
- Repair commit push: PASS, normal push to `phase-2/v7.3.8-candidate-stage-install`

## Assigned objective

Repair only the real `StageCandidate` → `InstallCandidate` filesystem/ACL seam. Candidate source artifacts are untrusted data under the protected staging root; installation derives trust from `ProductionCandidate(m.lock)`, copies into protected destination objects, validates them again, and publishes the installed-slot registry under GLOBAL.

## Non-goals

No production Smoke, promotion, upstream ref movement, v7.3.8 download or execution, provider request, credential/OAuth access, product-root mutation, or real lifecycle action was performed. The real-archive opt-in test was run without `DUALPOOL_V738_ARCHIVE` and skipped without network access.

## Starting state

- Worktree: clean managed worktree on the requested branch; original dirty checkout untouched.
- Authoritative `phase-2/upstream-lifecycle`: `5bfb1518059a171a0f3858877585e3097d9fe099`.
- Reproduction: new normal-filesystem stage-child regression failed before the fix with `ErrBinaryInstallConflict`; focused test passed after the fix.

## Investigation and evidence

| Claim | Status | Evidence/source | Consequence |
|---|---|---|---|
| Stager uses normal filesystem APIs for candidate child directory and files | VERIFIED | `internal/upstreamstage/stage.go`, `TestInstallTrustedAcceptsActualStagerACLTopology` | Child protected ACL is not a valid source trust requirement |
| Stage cache naming is shared | VERIFIED | `upstreamstage.CandidateStagePath` derives closed production provenance and shares `candidateStageName` with staging | Public install binds to the exact reviewed cache directory |
| Stage root remains protected | VERIFIED | `TestInstallCandidateRejectsUnprotectedStageRoot`; `winacl.Manager.Inspect` | Public candidate install fails closed on an unsafe root |
| Source stage is revalidated without child ACL authority | VERIFIED | normal `os.MkdirTemp`/`os.OpenFile`/`os.WriteFile` regression passed through private install engine | Source validation remains path/type/reparse/topology/manifest/hash/PE/identity based |
| Installed slot and files retain protected ACLs | VERIFIED | regression checks `Inspect`/`InspectFile` on installed directory, executable, and manifest; Registry verification passes | Destination trust boundary remains intact |
| A complete synthetic trusted stage outside the expected product root is rejected by the root-bound engine; the public wrapper rejects candidate-shaped outside-root input before mutation | VERIFIED | `TestInstallCandidateRejectsValidCandidateOutsideProductStageRoot`; `TestPublicInstallCandidateRejectsCandidateShapedStageOutsideProductStageRootBeforeMutation` | Caller stage paths cannot select a production cache location; public early rejection is verified before content validation |
| Real v7.3.8 StageCandidate→InstallCandidate archive integration | UNKNOWN / NOT RUN | opt-in test skipped because archive environment variable was unset | No real archive was downloaded or executed for this repair |

## Decisions

The public wrapper computes the exact production candidate path using a helper that resolves only `ProductionCandidate(currentLock)`. It checks exact string equality and, under GLOBAL, validates the protected root. The private synthetic engine stays available to component tests. Candidate source child ACL checks were removed; `validateInstallFor` and protected destination creation/validation were not relaxed. Current v7.3.7 `Stage`/`Install` behavior is unchanged.

## Changes

| Path | Change | Reason |
|---|---|---|
| `internal/upstreamstage/stage.go` | Share candidate cache naming and expose a closed production-candidate path derivation helper | Avoid divergent format strings and caller-selected provenance |
| `internal/instance/instance.go` | Bind public candidate installation to the exact cache path; require protected stage-root ACL; stop requiring protected ACLs on untrusted source children | Align validation with actual staging filesystem topology while preserving the protected install boundary |
| `internal/instance/candidate_install_test.go` | Add RED/GREEN source topology, outside-root side-effect, and unsafe-root regressions | Prove the defect and relevant fail-closed behavior |
| Evidence/report/handoff | Added | Record exact test scope, limitations, and delivery state |

## Verification

| Test/command | Result | Evidence |
|---|---|---|
| `go test ./internal/instance -run '^TestInstallTrustedAcceptsActualStagerACLTopology$' -count=1` before fix | EXPECTED FAIL | Reproduced `pinned binary installation conflicts with existing artifact` |
| `go test ./internal/instance -run 'InstallCandidate|InstallTrusted|StageRoot|ACL|CrossRelease|Marker|Recovery' -count=50` | PASS | final test set: 62.851s |
| `go test -race ./internal/instance -run 'InstallCandidate|InstallTrusted|StageRoot|ACL|CrossRelease|Marker|Recovery' -count=10` | PASS | final test set: 60.198s |
| Runtime targeted ×20 / race ×10 | PASS | 10.303s / 99.370s |
| Four affected package stress suites ×50 | PASS | upstreamstage 147.622s; installedslot 20.657s; instance 140.008s; runtimeupdate 28.523s |
| Four affected package race suites ×10 | PASS | upstreamstage 575.391s; installedslot 5.734s; instance 283.206s; runtimeupdate 110.424s |
| `go fmt ./...`; `git diff --check`; `go mod verify`; `go vet ./...` | PASS | `go mod verify` reported all modules verified; unrelated formatter line-ending rewrites were restored |
| `go test -count=1 ./...`; `go test -race -count=1 ./...` | PASS | All packages passed; lockfile race suite 212.139s; upstreamstage race suite 59.051s |
| Windows build of `./cmd/poolbridge` | PASS | Temporary output created outside repository and removed |
| `node --test scripts/*.test.cjs` | PASS | 55 passed, 0 failed |
| `node scripts/phase0-upstream-lock.cjs` | PASS | `UPSTREAM_LOCK_VALID` |
| JSON validation | PASS | 215 JSON files parsed successfully |
| Documentation links, privacy/secret scans, and scanner positive control | PASS | Handoff/report links resolved; 0 privacy/secret matches; in-memory positive control detected |
| Exact-SHA Source CI for repair commit | PASS | run `35733252908`; push event, exact branch and SHA `ae9cd3350aeb5b0a6937f14b254706cedc1132b0`; all mandatory steps succeeded
| Real archive opt-in test | SKIPPED | Environment variable unset; no network request |

## Security/privacy review

- Listener/bind impact: none.
- Secret/token handling: no credential/OAuth access; no real prompt or provider data used.
- Config mutation/rollback: none; candidate active-state remains unchanged in the regression.
- Production filesystem: no product root was modified; mutations were confined to test-owned temporary fixtures.
- Evidence: records synthetic fixture ACL outcomes, not a claim that real candidate child ACLs are protected.
- Dependencies: none added.

## Acceptance evaluation

The ACL seam repair, local acceptance tests, normal repair-branch push, and exact-SHA Source CI run `35733252908` are PASS for `ae9cd3350aeb5b0a6937f14b254706cedc1132b0`. This report/evidence receipt update is documentation-only and will receive its own Source CI run. Real v7.3.8 archive integration remains open and was deliberately not run.

## Risks and unresolved items

| Risk/unknown | Severity | Owner | Required next action |
|---|---|---|---|
| Documentation/evidence receipt commit requires exact-SHA Source CI | Medium | Implementation run | Push the receipt-only update and verify the resulting exact branch/SHA Source CI |
| Real v7.3.8 archive path not exercised in this run | Medium | Owner/auditor | Review current regression and later decide whether controlled offline archive integration is required |
| Smoke/promotion/rollback/launch-handle/OS durability gates | Open | Future Phase 2 slices | Do not start in this repair run |

## Git delivery

- Files committed: seven scoped files in repair commit `ae9cd3350aeb5b0a6937f14b254706cedc1132b0`; documentation/evidence receipt update follows.
- Commit: `ae9cd3350aeb5b0a6937f14b254706cedc1132b0` (`fix(phase-2): align candidate stage install ACL boundary`).
- Push command/result: `git push origin HEAD:refs/heads/phase-2/v7.3.8-candidate-stage-install` succeeded.
- `phase-2/upstream-lifecycle`: must remain `5bfb1518059a171a0f3858877585e3097d9fe099`.

## Rollback

Revert only this repair commit on its branch. No product or user state was changed; the authoritative upstream branch is unchanged.

## Next run

- Exact next objective: owner/auditor review and controlled delivery of the repaired v7.3.8 stage/install lineage into `phase-2/upstream-lifecycle`.
- Entry criteria: pushed exact repair SHA, Source CI completed/success, and owner/auditor acceptance.
- Stop conditions: any path/ACL/source-validation regression, failed check, or unexpected upstream ref movement.
