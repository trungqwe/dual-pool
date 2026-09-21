# Phase 2 update transaction foundation

- Roadmap phase: Phase 2
- Start HEAD: 2f5e8859002fc01d3ee5794cc91aebe5eaa2b73d
- Scope: E3 synthetic logical-slot transaction only.

## Result

`internal/update` holds GLOBAL while it calls state recovery/save operations, creates a bounded immutable transaction marker, requires disposable candidate smoke before any production stop, and restores the prior logical active selection after candidate production-smoke failure. Recovery derives the safe action from marker plus state truth.

No product root, updater command, binary selection, provider traffic, OAuth, IDE configuration, or real updater gate ran.

## Verification

- `go test ./internal/update`: PASS.
- `git diff --check`: PASS after final whitespace repair.

Evidence: `evidence/phase-2-update-foundation/`.

## Next run

Trusted installed-slot registry and `instance.Manager` active-selection integration. Keep production runtime integration and all real updater gates closed.
## Expanded foundation verification

`TestFaultBoundariesLeaveRecoverableMarker` covers a committed marker before selection and a candidate selection before finalization using fresh recovery. `TestRollbackUnresolvedRetainsMarker`, `TestGlobalBlocksConcurrentPromotion`, `TestMarkerArtifactSafety`, and `TestPromotionPreservesUnrelatedState` cover unresolved retention, GLOBAL exclusion, reparse refusal, and state preservation. These are synthetic component tests only; subprocess crash execution and installed-slot runtime remain future work.

Subprocess crash proof now exits a test helper after the committed marker boundary and after candidate state save. A fresh process reclaims the stale GLOBAL lock, recovers to the previous selection, and removes the marker.


Final local verification: `go mod verify`, `go vet ./...`, `go test -count=1 ./...`, `go test -race -count=1 ./...`, `go build ./cmd/poolbridge`, `node --test scripts/*.test.cjs`, and `node scripts/phase0-upstream-lock.cjs` all passed. No real updater gate ran.
