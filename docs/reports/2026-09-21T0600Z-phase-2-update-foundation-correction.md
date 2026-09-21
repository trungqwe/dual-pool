# Phase 2 updater foundation correction

## Header

- Run ID: `phase-2-update-foundation-correction-20260921T0600Z`
- Date/time UTC: 2026-09-21T05:59:28Z
- Roadmap phase: Phase 2
- Repository: `trungqwe/dual-pool`
- Branch: `phase-2/upstream-lifecycle`
- Start HEAD: `53613e4d69eeeafa22e71699a6fddea6859a385a`
- End HEAD: PENDING
- Remote push result: PENDING

## Assigned objective

Correct the fixture-only updater transaction foundation: fail closed during rollback, validate both logical slots and running-set intent, compose with the real state Store, reject pending/unsafe markers, and protect marker files with the existing Windows ACL implementation.

## Non-goals

No installed-slot registry, `instance.Manager` selection, `upstream.lock` change, product-root mutation, provider/OAuth/config work, CLI updater, or real updater gate.

## Investigation and evidence

Historical evidence at `evidence/phase-2-update-foundation/` was insufficient for rollback Stop failure, exact marker schema, logical-version constraints, protected marker ACL, and real state Store composition. It remains immutable and is superseded for those claims by `evidence/phase-2-update-foundation-correction/`.

## Changes

- `internal/update/update.go`: previous-slot preflight, pending-marker refusal, closed version/running-set validation, rollback Stop and Store recovery ordering.
- `internal/update/marker_windows.go`: exact schema and injected marker security boundary.
- `internal/update/security_windows.go`: adapter to `winacl.Manager`.
- `internal/update/correction_test.go`: mandatory correction and real TEMP composition tests.

## Verification

Focused updater tests, count=10, updater race, subprocess crash and ACL/reparse tests PASS. `go mod verify`, `go vet ./...`, `go test -count=1 ./...`, `go test -race -count=1 ./...`, `go build ./cmd/poolbridge`, Node 55/55, upstream-lock validation, docs links, JSON parsing, diff check and secret/control scan PASS.

## Security/privacy review

Only synthetic logical versions and TEMP roots were used. No secret, user config, product root, CPA process, listener, provider traffic, OAuth, or real updater operation was involved.

## Acceptance evaluation

UPD-002 and UPD-003 are E3 component PASS only. Production multi-version updater acceptance, FR-025, INV-PROC-05 production acceptance, R-11, installed-slot registry, Manager runtime integration and real updater gate remain OPEN.

## Rollback

Revert the correction commit. Historical evidence and runtime/product state were not changed.

## Next run

Independent audit of this correction after exact-sha Source CI PASS. Do not begin installed-slot registry work in this run.
