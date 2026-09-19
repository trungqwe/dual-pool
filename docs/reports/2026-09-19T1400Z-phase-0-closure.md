# Phase 0 — closure and candidate upstream lock

## Header

- Run ID: `20260919T1400Z-phase-0-closure`
- Date/time UTC: `2026-09-19T14:00:00Z`
- Agent/tool version: Codex desktop
- Roadmap phase: Phase 0
- Repository: `https://github.com/trungqwe/dual-pool`
- Branch: `phase-0/reversible-compatibility`
- Start HEAD: `d760e8fee4b2f316cfbca8871b8820280f01ec58`
- End HEAD: PENDING
- Remote push result: PENDING

## Assigned objective

Complete the remaining Phase 0 deliverables: a deterministic candidate `upstream.lock`, machine-checked lock semantics, and an evidence-backed go/no-go decision.

## Non-goals

- No Phase 1 implementation.
- No credentialed provider probe, OAuth, IDE launch, or user-config mutation.
- No claim that the candidate upstream release is production-supported.

## Starting state

- Supply-chain, binary identity, config fields, loopback bind, key enforcement, and credential-free lifecycle were already probed for CLIProxyAPI `v7.3.7`.
- U-001..004 and U-006 remained blocked. U-008 remained partial because credentialed provider shapes and dedicated health behavior were unproven.
- The prior live shadow session was cleaned and its blocked result delivered.

## Investigation and evidence

| Claim | Status | Evidence/source | Consequence |
|---|---|---|---|
| CLIProxyAPI `v7.3.7` Windows amd64 archive is independently hash-verified. | VERIFIED | `cliproxy-schema.json`, P0A-SUP-001 | May be recorded as a candidate pin. |
| Credential-free loopback lifecycle and key enforcement passed. | VERIFIED | `cliproxy-schema.json`, P0A-CPA-001 | Foundation may model the pinned boundary. |
| Credential-specific models, provider response shapes, and dedicated health endpoint are unproven. | UNKNOWN | U-008 | Lock must not claim provider support. |
| Antigravity redirect/envelope and Codex picker routing remain blocked. | BLOCKED | U-001..004, U-006 | Live integration and release remain NO-GO. |

## Decisions

- Phase 0 closes as `PASS_WITH_BLOCKERS`, using the roadmap rule that explicit blockers may satisfy the audit exit.
- Phase 1 fixture-only foundation work is GO because it does not depend on the blocked IDE/provider behavior.
- Live Antigravity/Codex integration and release promotion remain NO-GO.
- `upstream.lock` status is `candidate`; `config_adapter_version` is `UNIMPLEMENTED` until Phase 2 produces and verifies an adapter.

## Changes

| Path | Change | Reason |
|---|---|---|
| `upstream.lock` | Pin candidate tag, commit, URLs, hashes, verified and unverified capabilities. | Complete Phase 0 candidate lock deliverable. |
| `scripts/phase0-upstream-lock.cjs` | Fail-closed schema and claim validator. | Prevent accidental promotion or moving-release URLs. |
| `scripts/phase0-upstream-lock.test.cjs` | Candidate identity and negative-claim tests. | Make the lock reproducible. |
| `evidence/phase-0-closure.json` | Machine-readable go/no-go result. | Close Phase 0 without weakening blockers. |

## Verification

| Test/command | Result |
|---|---|
| Test before implementation | FAIL: validator module absent |
| `node --check scripts/phase0-upstream-lock.cjs` | PASS |
| `node scripts/phase0-upstream-lock.cjs` | PASS: `UPSTREAM_LOCK_VALID` |
| `node --test scripts/phase0-upstream-lock.test.cjs` | PASS: 2/2 |
| `node --test scripts/*.test.cjs` | PASS: 50/50 |

Final JSON, docs-link, privacy, manifest, PowerShell parser, and diff gates run on the staged set.

## Security/privacy review

- No secrets, auth data, prompts, user paths, or account identity were read or written.
- The lock contains public release metadata and cryptographic hashes only.
- No dependency or binary was added to the repository.
- Real user config mutation remained disabled.

## Acceptance evaluation

- Repository/current-state inventory: PASS.
- Sanitized compatibility evidence: PASS with documented limits.
- U-001..U-008 resolution: PASS for Phase 0 exit because every unresolved item has an explicit blocker or `PARTIAL_UNKNOWN` classification and required future proof.
- Go/no-go recommendation: PASS.
- Candidate `upstream.lock`: PASS.
- Product/live integration: NO-GO.

## Risks and unresolved items

| Risk/unknown | Severity | Required next action |
|---|---|---|
| U-001..004 Antigravity compatibility | High | Re-probe only when a supported reversible setting path exists. |
| U-006 picker/provider retention | High | Do not retry real config mutation; require a supported authenticated isolation path. |
| U-008 provider contracts | High | Credentialed contract probes in the account phase before production promotion. |

## Git delivery

- Files committed: PENDING
- Commit(s): PENDING
- Push result: PENDING
- PR: not requested

## Next run

- Exact next objective: Phase 1 — create the Go CLI skeleton, version output, stable error envelope, and initial unit-test/CI foundation.
- Entry constraints: fixture-only config work; no live IDE/provider integration; preserve all Phase 0 blockers.
