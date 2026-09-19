# Phase 1 state schema and migrations

## Header

- Run ID: `phase-1-state-schema-migrations`
- Date/time UTC: 2026-09-19T16:20:00Z
- Agent/tool version: Codex / Go 1.26.0 windows/amd64
- Roadmap phase: Phase 1 — Foundation and state safety
- Repository: `https://github.com/trungqwe/dual-pool`
- Branch: `phase-1/state-foundation`
- Start HEAD: `16cdad172970495dcd2d479489e504d1dfa01fb9`
- End HEAD: PENDING
- Remote push result: PENDING

## Assigned objective

Implement typed schema v1 for `state.json` and `ownership.json`, strict memory-only JSON codecs, validation, and an explicit sequential migration framework.

## Non-goals

No filesystem persistence, directory creation, atomic replacement, flush, recovery journal, locks, process identity, config mutation, backup execution, secret store, CLI state command, network listener, OAuth, or live integration.

## Starting state

- Worktree status: clean dedicated worktree.
- Existing unrelated changes: none in this worktree; another checkout remains untouched.
- Versions/environment fingerprint: Windows amd64; Go 1.26.0.
- Relevant prior handoff: `docs/18-HANDOFF.md`, Phase 1 delivery `16cdad1`.
- Reproduction result: start and remote Phase 1 heads match; authoritative Phase 0 remote head verified by `git ls-remote` as `6619034`.

## Decisions

- Product documents support schema version 1 only. No product v0 is inferred.
- Persistent instance status vocabulary is deliberately limited to `stopped` until lifecycle work defines more states.
- State input limit is 1 MiB; ownership input limit is 4 MiB.
- Encoding is deterministic compact JSON followed by LF.
- Config hashes and ownership hashes are empty only where the field represents an operation not yet populated; populated hashes are lowercase SHA-256.
- Catalog fingerprints are bounded safe identifiers without claiming adapter semantics.
- Ownership v1 supports the documented current configuration formats `toml` and `json`, UTF-8, BOM `absent|present`, newline `lf|crlf`, and rollback status `pending|applied|rolled_back|conflict`.
- Typed owned values form a closed union: `absent`, `string`, `bool`, `integer`, `number`, `string_map`, or `secret_ref`. Secret references are opaque identifiers only.
- Migration steps are pure in-memory byte transformations and must advance exactly one version with validation after every step.

## Planned changes and tests

| Path | Purpose |
|---|---|
| `internal/state/*.go` | Typed schemas, validators, strict codecs, migration engine |
| `internal/state/*_test.go` | Positive, negative, round-trip, deterministic, and secret-boundary tests |
| `evidence/phase-1-state-schema/*.json` | Sanitized verification receipts |
| `docs/15-MASTER-CHECKLIST.md` | Mark only schema/migration item after all gates pass |
| `docs/23-TRACEABILITY-MATRIX.md` | Add test mappings and correct verified earlier CI status |

Test IDs: `P1-STATE-SCHEMA-001`, `P1-OWNERSHIP-SCHEMA-001`, `P1-STATE-CODEC-001`, `P1-MIGRATION-001`, `P1-STATE-SECRET-BOUNDARY-001`.

Stop conditions: any mandatory validation or regression failure; unexpected dirty work; secret/path leakage; remote divergence; or CI failure.

## Verification

| Test/command | Result | Evidence |
|---|---|---|
| `gofmt`, `go vet ./...`, `go test -count=1 ./...` | PASS; 34 Go test functions | `evidence/phase-1-state-schema/test-result.json` |
| `go test -race -count=1 ./...`, `go build ./cmd/poolbridge` | PASS | Test result |
| `node --test scripts/*.test.cjs` | PASS; 53/53 | Test result |
| `node scripts/phase0-upstream-lock.cjs` | PASS; `UPSTREAM_LOCK_VALID` | Test result |
| Strict codec, migration, typed-value and secret-boundary gates | PASS | `evidence/phase-1-state-schema/security-gate.json` |
| JSON, documentation links, privacy/path/secret scan, `git diff --check` | PASS | Security gate |

## Security/privacy review

- Listener/bind impact: none.
- Secret/token handling impact: closed schema contains metadata and opaque secret references only.
- Config mutation/rollback impact: representation only; no I/O or mutation engine.
- Logging/evidence review: no state logging; evidence excludes local paths and raw documents.
- Secret scan result: PASS for the scoped diff and evidence artifacts.
- New dependencies/supply-chain impact: standard library only.

## Next run

- Exact next objective: Phase 1 atomic state/ownership store and crash recovery using TEMP fixtures only.
- Entry criteria: this slice passes local and remote gates.
- Stop conditions: any mutation of real `%LOCALAPPDATA%\DualPool` or need for an unproven persistence assumption.
