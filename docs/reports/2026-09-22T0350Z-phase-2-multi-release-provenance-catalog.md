# Phase 2 trusted multi-release provenance catalog

## Header

- Date/time UTC: 2026-09-22T03:50Z
- Roadmap phase: Phase 2
- Repository: `trungqwe/dual-pool`
- Branch: `phase-2/multi-release-provenance-catalog`
- Start HEAD: `7ce7405548f10f06aa266c8d01f8c49957aae1bd`
- End HEAD: PENDING
- Remote push result: PENDING

## Assigned objective

Implement an immutable trusted release-provenance catalog and bind every
installed logical slot to provenance resolved for that exact version.

## Non-goals

No second production release, `upstream.lock` change, download, staging,
Updater promotion/recovery, CLIProxyAPI process, listener, provider traffic,
OAuth, credential access or product-root mutation was performed.

## Decisions

- `upstreamcatalog.FromPinnedLock` is the production trust adapter and requires
  the existing exact `upstreamlock.Lock.Validate` gate.
- `NewVerified` is structural-only for already independently verified internal
  sources and component fixtures; runtime composition does not accept it from
  caller configuration.
- `installed-slots.json` remains inventory. It cannot establish a catalog entry
  or authorize an absent version.
- The existing schema already records the metadata necessary to bind a slot;
  no registry-document migration is needed.

## Changes

| Path | Change | Reason |
|---|---|---|
| `internal/upstreamcatalog/` | Immutable provenance model and exact lookup | Separate trusted release identity from slot inventory |
| `internal/installedslot/` | Per-version catalog binding before hash and verifier | Prevent cross-release metadata borrowing |
| `internal/upstreamstage/` | `VerifyExpected` identity primitive | Verify binary version/commit against resolved provenance |
| `internal/runtimeupdate/` | Private current one-entry catalog composition | Preserve the production `v7.3.7` trust anchor |

## Verification

Focused commands passed:

- `go test ./internal/upstreamcatalog -count=20`
- `go test ./internal/installedslot -run 'Catalog|Release|Provenance|Registry|Rebind|Tamper' -count=20`
- `go test ./internal/upstreamstage -run 'Verifier|Identity|Pinned' -count=20`
- `go test ./internal/runtimeupdate -run 'Catalog|Composition|Production' -count=20`
- `go vet ./...`
- `git diff --check`

The final stress, race, repository, Node, lock and Source CI gates are recorded
in the delivery receipt after the final commit. No production claim is made from
synthetic fixtures.

## Acceptance evaluation

| Criterion | Status | Evidence |
|---|---|---|
| Immutable multi-release component catalog | PASS_COMPONENT | `catalog_test.go`, `catalog.json` |
| Per-version Registry metadata/hash/verifier binding | PASS_COMPONENT | `catalog_registry_test.go`, `registry-binding.json` |
| Current production one-pin behavior unchanged | PASS_COMPONENT | runtime production tests, `production-regression.json` |
| Real second production release | OPEN | Not authorized or evidenced |

## Security/privacy review

The catalog is in-memory only and has no network, persisted catalog, secret or
credential input. Synthetic entries are deterministic TEMP test fixtures.

## Risks and unresolved items

- A real second release still requires independent archive/binary identity,
  capability and config-adapter compatibility evidence.
- Candidate staging/install, production Smoke, live promotion/rollback,
  retained-handle launch identity, subprocess crash, OS crash and power-loss
  proof remain open.

## Next run

Independently verify one real second CLIProxyAPI release and its exact config
compatibility, then add it as a production trusted provenance entry without
weakening the current `v7.3.7` trust anchor.
