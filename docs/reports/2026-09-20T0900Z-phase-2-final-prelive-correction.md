# Phase 2 — final bounded pre-live correction

- Start HEAD: `b20b3f5c7e06c384e8a7424dd07d51ff3ddc94d7`.
- Historical Source CI: `35497455654` PASS.
- Real lifecycle: NOT RUN and not authorized for this commit.

This correction adds ABI-derived TCP table decoding and tests, fail-closed listener behavior, partial installer recovery, lifecycle idempotence and stale-stop tests, then a committed gated real-lifecycle harness. No OAuth, provider request, credential inventory, IDE mutation, or real process launch is part of implementation work.

## Local verification

`go test -count=1 ./...`, `go vet ./...`, `go build ./cmd/poolbridge`, `go mod verify`, the Node suite and `UPSTREAM_LOCK_VALID` passed. The new Source CI and race suite are pending. The committed real-gate test skips unless explicitly armed and presently fails closed if armed before a complete approved harness exists.
