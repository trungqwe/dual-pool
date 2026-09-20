# Phase 2 — pinned empty instance lifecycle

## Header

- Roadmap phase: Phase 2, slice thứ tư.
- Branch: `phase-2/upstream-lifecycle`.
- Start HEAD: `6d6cfd28600c26a3188455df9ef3b7c258bdaab6`.
- End HEAD/push/CI: PENDING.

## Objective and non-goals

Install one exact pinned CLIProxyAPI v7.3.7 binary under the protected product root; implement PID-safe lifecycle for `codex` and `google`; then run credential-free L0/L1/L2 smoke and cleanly stop both.

No OAuth, provider credential, provider request, IDE user-config mutation, raw config/log/evidence copy, or update promotion work is in scope.

## Design

- Installation consumes only `upstreamstage.Stager` output validated against the current lock. Product install creates a protected candidate and marker before artifacts, then uses `MoveFileExW` write-through without replacement. The final version directory permits only executable and strict manifest.
- The manifest binds schema, product/version/tag/commit/platform, executable digest, lock digest, adapter version and executable basename. Reuse revalidates DACL, hash, PE AMD64 and version/commit identity.
- `internal/instance` owns the closed two-instance lifecycle. Each process record is strict, protected and non-secret. PID is accepted only with exact creation FILETIME, canonical image and installed binary identity.
- Launcher uses the exact executable, `-config <final config> -local-model`, final instance directory as cwd, and only `SystemRoot`/`WINDIR` child environment values. It rejects `.env` and all unknown instance-top-level artifacts.
- Listener inspection uses Windows TCP owner tables, requires exactly one IPv4 `127.0.0.1:<port>` listener owned by the managed PID and rejects all IPv6, wildcard, alternate or extra listeners.
- L0 is process identity, L1 is loopback listener plus unauthenticated `/healthz`, L2 is negative/positive client `/v1/models` and management `/v0/management/debug` authentication. All HTTP traffic is loopback-only, direct transport, bounded and redacted.

## Sequencing and stop conditions

1. Write synthetic fixture tests for install, process identity, record recovery, port ownership, listener classification, environment and probes.
2. Implement and run the complete local suite; commit and require Source CI PASS.
3. Only then run `DUALPOOL_RUN_REAL_LIFECYCLE=1`: preflight existing product state, install/reuse binary, execute the ordered real smoke, and require full cleanup.
4. Stop at malformed installation/record, foreign process or port, unexpected listener/artifact, failed identity/auth probe, any config/key/auth-root mutation, or CI failure.

## Known context

- The requested dirty root checkout was `phase-0/reversible-compatibility`; it was preserved. This run uses isolated worktree `phase-2-empty-lifecycle` at the requested Phase 2 start commit.
- The prior config slice established protected configs, client base64url wire keys, management bcrypt verifiers and no listener/process state.
- Upstream staging still uses `os.Rename` only in its TEMP staging component. Product binary installation will not reuse that primitive as its persistence proof.

## Verification plan

Run targeted unit/fault/subprocess tests, then `go test ./...`, `go test -race -count=1 ./...`, `go vet ./...`, `go build ./cmd/poolbridge`, `go mod verify`, pinned integration, Node lock tests, link check, secret scan and `git diff --check`. Hosted CI never sets the real lifecycle gate.

Phase 2 remains open after this slice for Management inventory/L3, provider-root integrity with credentials, OAuth, account lifecycle and update promotion/rollback.

## Local implementation result

- Added `internal/instance`: protected pinned-binary installation, strict immutable installation manifest, two-instance start/stop/restart/status APIs and protected non-secret process records.
- PID authority requires the record PID, exact Windows creation FILETIME, canonical executable image and installed binary digest to agree. A stale record from a naturally exited child is removed before restart; an identity mismatch fails closed.
- Config inspection recognizes only the optional protected `process.json` lifecycle artifact in addition to the owned configuration tree.
- `go test ./...`, `go vet ./...` and `go build ./cmd/poolbridge` passed locally. The race suite, delivery CI, and gated real lifecycle smoke remain PENDING and must pass before any lifecycle checkbox can be marked complete.
