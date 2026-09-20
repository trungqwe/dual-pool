# Phase 2 — listener and cleanup safety correction

- Start HEAD: `390238bec9f5fb037f95b232743bd5f5070ece6a`.
- Prior correction CI `35496836805` remains in progress and is historical only.
- Real lifecycle execution remains prohibited.

## Findings

The prior correction needs exact ABI-safe IPv4/IPv6 TCP owner-table decoding, fail-closed inspection failures, unified owned-child cleanup, ownership/config-health separation, same-layout semantic config validation, strict installer recovery validation, and fixture coverage before any real process launch.

No OAuth, provider calls, IDE configuration changes, or real lifecycle work are in scope for this correction.

## Local verification

`go test -count=1 ./...`, `go vet ./...`, `go build ./cmd/poolbridge`, `go mod verify`, the Node suite, and `UPSTREAM_LOCK_VALID` passed. Race and a new Source CI run are pending. Real lifecycle remains NOT RUN.
