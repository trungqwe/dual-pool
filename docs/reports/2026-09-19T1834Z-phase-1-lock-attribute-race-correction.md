# Phase 1 — lock attribute race correction

## Header

- Run ID: `20260919T1834Z-phase-1-lock-attribute-race-correction`
- Roadmap phase: Phase 1
- Branch: `phase-1/state-foundation`
- Start HEAD: `053a87b24334d514cfdf1dcf049ad599dc52a952`
- Trigger: Source CI `35461530713`
- End HEAD: PENDING

## Finding

The Windows secret-store implementation and its real WinCred tests passed in CI, but the full Go gate stopped on the pre-existing simultaneous stale-lock regression. In global iteration 11, one contender won while the other returned `claim_open: canonical_attributes: mutation lock persistence failed` instead of the required held classification.

`openCanonical` already bounds transient `ERROR_ACCESS_DENIED` from `CreateFileW` for 250 ms while a delete-pending transition settles. The preceding `GetFileAttributesW` call did not apply the same bounded transition handling, so the same safe Windows race could escape one step earlier. This is a direct regression discovered by the required full-suite gate.

## Correction plan

- Retry only `ERROR_ACCESS_DENIED` from exact canonical attribute inspection for at most 250 ms.
- Preserve fail-closed behavior after the bound.
- Continue mapping file/path-not-found and delete-pending to the existing canonical-gone retry path.
- Preserve reparse/directory rejection, canonical handle ownership and handle-based deletion.
- Rerun focused stale-reclaim stress, full Go/race/build/module gates, Phase 0 53/53, upstream lock and repository security checks.
- Push a correction commit and require a new observable Source CI PASS before checklist movement or delivery receipt.

## Non-goals

- No lock protocol redesign, timeout widening beyond 250 ms, pathname deletion or relaxation of ownership validation.
- No change to secret-store design, WinCred behavior, provider/config/lifecycle scope or prior immutable report/evidence.

## Verification

- Focused `TestSimultaneousStaleReclaim` stress: PASS for five full runs after the correction.
- `go fmt`, `go vet`, 72-function full Go suite, WinCred integration, build and module verification: PASS.
- Full race suite: PASS, including 211-second lock stress.
- Phase 0 Node suite: PASS 53/53. Candidate lock: `UPSTREAM_LOCK_VALID`.
- JSON/docs/privacy/secret/diff checks: PASS; repeated after staging.
- Corrected remote Source CI: PENDING push.

## Git delivery

- Correction commit/push: PENDING.
- Corrected Source CI: PENDING.
- Checklist and delivery receipt remain blocked until corrected CI passes.
