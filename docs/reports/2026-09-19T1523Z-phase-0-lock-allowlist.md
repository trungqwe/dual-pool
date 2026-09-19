# Phase 0 — candidate-lock allowlist repair

## Header

- Run ID: `20260919T1523Z-phase-0-lock-allowlist`
- Date/time UTC: `2026-09-19T15:23:00Z`
- Agent/tool version: Codex desktop
- Roadmap phase: Phase 0 maintenance
- Repository: `https://github.com/trungqwe/dual-pool`
- Branch: `codex/phase0-lock-allowlist`
- Start HEAD: `dd448f7c2e55351eccfb8b8f2f16e8433810e05a`
- End HEAD: PENDING
- Remote push result: PENDING

## Assigned objective

Close the recorded Phase 2 entry debt before Phase 1 work: reject unsupported `verified_capabilities` claims in the candidate upstream lock.

## Non-goals

- No change to the candidate lock, Phase 0 compatibility classifications, provider behavior, user configuration or upstream binary.

## Starting state

- Clean isolated worktree at the exact Phase 0 closure commit. The main checkout has unrelated historical dirty Phase 0 work and was not modified.
- The validator accepted arbitrary verified capability names.

## Investigation and evidence

The seven allowed names are the current candidate claims in `upstream.lock`; the candidate evidence remains linked there. New negative tests first reproduced the missing rejection for unknown, duplicate and non-string entries (2 PASS, 3 FAIL before the fix).

## Decisions and changes

- `scripts/phase0-upstream-lock.cjs`: use a closed set for verified names and reject duplicate entries.
- `scripts/phase0-upstream-lock.test.cjs`: three negative tests.
- Checklist, traceability and handoff record this bounded repair.

## Verification

| Test/command | Result |
|---|---|
| `node --test scripts/phase0-upstream-lock.test.cjs` | PASS: 5/5 |
| `node --test scripts/*.test.cjs` | PASS: 53/53 |
| `node scripts/phase0-upstream-lock.cjs` | PASS: `UPSTREAM_LOCK_VALID` |
| `node --check` on changed scripts | PASS |
| Changed-doc links | PASS |
| Staged privacy/secret pattern scan | PASS |
| `git diff --cached --check` | PASS |
| PowerShell parser | NOT APPLICABLE: no PowerShell script changed |

## Security/privacy review

No secrets, live probes, listeners, auth files or user configuration were accessed. New test data is synthetic. Candidate status and all compatibility blockers remain unchanged.

## Acceptance evaluation

- Unknown verified name: PASS, rejected.
- Duplicate verified name: PASS, rejected.
- Non-string verified name: PASS, rejected.
- Existing candidate: PASS, valid and unchanged.
- Delivery gates: PASS locally; push remains pending.

## Risks and unresolved items

Provider-specific capabilities remain unverified; the validator repair does not promote them. Phase 1 work remains fixture-only.

## Git delivery

- Commit, push and PR: PENDING. Record the final receipt in `docs/18-HANDOFF.md` after push.

## Rollback

Revert the scoped repair commit; the lock itself was not edited.

## Next run

Carry the repair into `phase-1/state-foundation`, then complete only the first Phase 1 CLI/error/unit-test/CI slice.
