# Phase 2 — ghim byte identity receipt CLIProxyAPI v7.3.8

## Header

- Run ID: `phase-2-v738-receipt-byte-identity-repair`
- Date/time UTC: `2026-09-22T09:04Z`
- Agent/tool version: Codex; Go toolchain and GitHub Actions Source CI
- Roadmap phase: Phase 2 — provenance verification
- Repository: `trungqwe/dual-pool`
- Branch: `phase-2/verify-v7.3.8-production-provenance`
- Repair start HEAD: `50555a43f370e77f34dbcd134e367d4a7b427c11`
- Repair code/evidence HEAD: `df106aad7c823d23fbc7cae8d88455157e4646cb`
- Remote push result: normal push succeeded; exact-SHA Source CI passed

## Assigned objective

Make the embedded v7.3.8 receipt byte identity independent of Windows checkout settings and cryptographically pin the reviewed raw receipt bytes. Preserve official release identity and keep the authoritative upstream ref unchanged.

## Non-goals

- No release archive redownload or v7.3.8 candidate re-execution.
- No listener, staging, installation, promotion, rollback or production mutation.
- No `upstream.lock`, installed-slots schema, Stage policy or bcrypt policy change.
- No move of `phase-2/upstream-lifecycle`.

## Starting state

- Worktree: clean on `phase-2/verify-v7.3.8-production-provenance` at `50555a43f370e77f34dbcd134e367d4a7b427c11`.
- Existing unrelated changes: none.
- Prior delivery report: [v7.3.8 provenance delivery receipt](2026-09-22T0825Z-phase-2-v738-delivery-receipt.md).
- Authoritative upstream start: `a0b1573346007ad08a9ebe7012167bffa79fbd99`.

## Investigation and evidence

| Claim | Status | Evidence/source | Consequence |
|---|---|---|---|
| `go:embed` feeds raw receipt bytes into `parseVerifiedV738Receipt`; before this repair the digest was calculated from whichever bytes the checkout supplied | VERIFIED | `internal/upstreamcatalog/production.go`; owner review finding | Windows EOL conversion could silently change provenance identity |
| The canonical receipt bytes are LF and their SHA-256 is `0e653e4f01e00c05a44c662e7a7b7321916e7705c901db370aec3cb1116a9776` | VERIFIED | `Get-FileHash` on the main Windows worktree and a clean detached Windows Git worktree; evidence JSON | Preserve exact official release identity and reject byte drift |
| Git attributes resolve receipt as `text: set`, `eol: lf` | VERIFIED | `git check-attr text eol -- internal/upstreamcatalog/trust/v7.3.8.json` in both worktrees | Normal checkouts reproduce reviewed LF bytes |
| CRLF line-ending variant has identical JSON semantics but is rejected as `ErrInvalidCatalog` | VERIFIED | `TestVerifiedV738ReceiptRejectsCRLFByteVariant` | Provenance trusts exact reviewed bytes, not semantic JSON equivalence |
| Different valid lowercase 64-hex archive and executable hashes are rejected | VERIFIED | `TestVerifiedV738ReceiptRejectsValidLookingHashTampering` | Pin protects valid-looking identity replacement as well as malformed input |
| Production Registry shares the same catalog containing the exact pinned v7.3.8 digest | VERIFIED | `TestProductionCompositionBindsPinnedV738DigestToRegistry` | Registry receives no checkout-dependent digest |

## Decisions

Both protections are required: a narrowly scoped `.gitattributes` rule makes checkout bytes deterministic, and a production SHA-256 pin fails closed on any whitespace, timestamp or semantic byte edit. The receipt semantic content and release identity remain unchanged.

## Changes

| Path | Change | Reason |
|---|---|---|
| `.gitattributes` | Pin only `/internal/upstreamcatalog/trust/v7.3.8.json` to `text eol=lf` | Stabilize `go:embed` input on Windows |
| `internal/upstreamcatalog/production.go` | Require raw SHA-256 equal to the canonical constant; return that constant as provenance digest | Prevent silent checkout-dependent trust identity |
| `internal/upstreamcatalog/catalog_test.go` | Pin stable digest assertion; reject CRLF and valid-looking archive/executable hash mutations | Cover byte identity and digest-tampering regressions |
| `internal/runtimeupdate/runtime_test.go` | Prove Registry uses the production catalog carrying the pinned v7.3.8 digest | Protect composition binding |
| `evidence/phase-2-v7.3.8-verification/test-result.json` | Supersede stale pre-delivery fields and record LF/canonical/worktree/clean-checkout identity plus repair test gates | Keep evidence consistent with delivered state |

Repair commit: `df106aad7c823d23fbc7cae8d88455157e4646cb` — `fix(phase-2): pin v7.3.8 receipt byte identity`.

## Verification

| Test/command | Result | Evidence |
|---|---|---|
| RED regressions before parser fix | Expected FAIL: CRLF and valid-looking archive/executable hash tampering were accepted | Terminal output during repair |
| `go test ./internal/upstreamcatalog -run 'Production|VerifiedV738|Receipt|Digest|Tamper' -count=50` | PASS, 0.070s | Local Windows terminal |
| `go test -race ./internal/upstreamcatalog -run 'Production|VerifiedV738|Receipt|Digest|Tamper' -count=10` | PASS, 1.075s | Local Windows terminal |
| `go test ./internal/runtimeupdate -run 'Production|CandidateAbsent|Composition' -count=20` | PASS, 7.553s | Local Windows terminal |
| `go test -race ./internal/runtimeupdate -run 'Production|CandidateAbsent|Composition' -count=10` | PASS, 77.144s | Local Windows terminal |
| `go fmt ./...`; `go mod verify`; `go vet ./...`; `go test -count=1 ./...`; `go test -race -count=1 ./...` | PASS | Local Windows terminal; unrelated formatter churn was restored |
| Windows executable build | PASS | Disposable build output under system TEMP; no binary committed |
| `node --test scripts/*.test.cjs` | PASS, 55/55 | Local terminal |
| `node scripts/phase0-upstream-lock.cjs` | PASS, `UPSTREAM_LOCK_VALID` | Local terminal |
| JSON validation | PASS, 205 files | PowerShell JSON parser |
| `git check-attr text eol -- internal/upstreamcatalog/trust/v7.3.8.json` | PASS: `text: set`, `eol: lf` | Main and clean detached Windows worktrees |
| Windows receipt SHA-256 | PASS: `0e653e4f01e00c05a44c662e7a7b7321916e7705c901db370aec3cb1116a9776` | `Get-FileHash` in main worktree |
| Clean checkout proof | PASS: same receipt SHA in a detached clean Windows Git worktree at final repair commit; focused embedded receipt tests passed there | Detached worktree outside the repository |
| Secret/privacy scan and positive control | PASS, zero findings; positive control detected | Staged repair files |
| `git diff --check` | PASS | Before commit and `git diff HEAD^ HEAD --check` |
| Source CI | PASS, completed | [Run 35707709709](https://github.com/trungqwe/dual-pool/actions/runs/35707709709), push event, branch `phase-2/verify-v7.3.8-production-provenance`, exact SHA `df106aad7c823d23fbc7cae8d88455157e4646cb`; every source-checks step passed |

Source CI steps: format, module verification, vet, Go tests, Go race tests, pinned upstream integration, Windows executable build, Phase 0 regression tests, candidate-lock validation.

## Security/privacy review

- Listener/process impact: no candidate process or listener was started; no real harness rerun.
- Secret/token handling: no real credential, OAuth or provider traffic.
- Config/product mutation: none; `upstream.lock` unchanged.
- Release identity unchanged: version `7.3.8`, tag `v7.3.8`, commit `c93978c4ea2e908255a2a06c37599fda3651554a`, archive SHA `5e3278ac9b57d16df503fd845827a6fdb57ec241f102b35899788287eb431351`, executable SHA `479da2fb56eb3db11a76e19adeb2e10c2a4069a512ab5e3933ac4c50628360fd`, adapter `dualpool-cpa-v7.3.7-config-v1`, receipt SHA `0e653e4f01e00c05a44c662e7a7b7321916e7705c901db370aec3cb1116a9776`.
- bcrypt semantics unchanged: production `bcrypt.DefaultCost`; package-local fixtures use private per-Generator `bcrypt.MinCost`; default validation rejects MinCost.
- TEMP: classification remains `POLICY_BLOCKED_NONREPO_TEMP_RESIDUE`; no alternate cleanup mechanism was used.

## Acceptance evaluation

| Criterion | Result | Evidence |
|---|---|---|
| Repository checkout canonicalizes the receipt to LF | PASS | `.gitattributes`, `git check-attr`, Windows worktree and clean checkout hashes |
| Production rejects any receipt bytes other than the canonical SHA | PASS | Production digest pin and CRLF/valid-hash tamper tests |
| Production catalog and Registry bind v7.3.8 to the pinned digest | PASS | Catalog identity and runtime composition regressions |
| Authoritative upstream and current Stage pin remain unchanged | PASS | Upstream ref `a0b1573346007ad08a9ebe7012167bffa79fbd99`; lock and tests |
| Exact-SHA Source CI completed successfully | PASS | Run `35707709709` for `df106aad…` |
| Independent owner audit accepted repair | NOT CLAIMED | Owner/auditor review remains the next gate |

## Risks and unresolved items

| Risk/unknown | Severity | Owner | Required next action |
|---|---|---|---|
| Candidate staging/install, Smoke, promotion/rollback and OS durability remain open | High | Owner/auditor and next implementation slice | Keep production updater acceptance open |
| Independent review of receipt-byte repair and complete v7.3.8 provenance lineage remains outstanding | High | Owner/auditor | Review exact pushed lineage before controlled upstream delivery |
| TEMP residue remains policy-blocked and contained outside repository/product/config roots | Low | Environment owner | Do not bypass cleanup policy |

## Git delivery

- Files committed: the five paths listed above.
- Commit: `df106aad7c823d23fbc7cae8d88455157e4646cb`.
- Push: `git push origin phase-2/verify-v7.3.8-production-provenance` succeeded.
- Authoritative upstream: unchanged at `a0b1573346007ad08a9ebe7012167bffa79fbd99`.
- Worktree: clean after push.

## Rollback

This repair is isolated on the provenance review branch. If rejected, revert only `df106aad7c823d23fbc7cae8d88455157e4646cb`; do not move or rewrite prior commits and do not touch upstream.

## Next run

- Exact next objective: owner/auditor review and controlled delivery of the complete v7.3.8 provenance lineage into `phase-2/upstream-lifecycle`.
- Entry criteria: audit the pushed repair head and confirm upstream remains `a0b1573346007ad08a9ebe7012167bffa79fbd99`.
- Files/docs to read: this report, `docs/18-HANDOFF.md`, `evidence/phase-2-v7.3.8-verification/`, and the complete branch diff.
- Commands/tests to run first: inspect ancestry and exact CI receipt, verify the receipt digest and attributes, then review the code/evidence diff independently.
- Stop conditions: any identity mismatch, stale/contradictory evidence, CI failure, upstream movement or observed live mutation.
