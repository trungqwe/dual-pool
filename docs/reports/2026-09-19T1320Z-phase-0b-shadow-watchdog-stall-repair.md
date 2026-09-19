# Phase 0B — shadow watchdog stall audit and repair

## Header

- Run ID: `20260919T1320Z-shadow-watchdog-stall-repair`
- Date/time UTC: `2026-09-19T13:20:00Z`
- Agent/tool version: Codex desktop
- Roadmap phase: Phase 0B
- Repository: `https://github.com/trungqwe/dual-pool`
- Branch: `phase-0/reversible-compatibility`
- Start HEAD: `b5c355777cf5568c8ec48fc237c956358ae334f8`
- End HEAD: PENDING
- Remote push result: PENDING

## Assigned objective

Audit the apparently hung shadow watchdog, repair live lifecycle defects without modifying the real Codex configuration, expand regression tests, and leave the active probe fail-closed.

## Non-goals

- No second live probe.
- No inspection of auth, sessions, chat history, prompt sentinel, or synthetic key.
- No force-close of Antigravity and no real-config mutation.

## Starting state

- The primary workspace was dirty and behind fetched remote history, so repair used an isolated worktree at `b5c3557`.
- One owned shadow session existed. Sanitized status was `WAIT_AUTH_CONFIRMATION`; heartbeat remained at `WAIT_PRIMARY_CLOSE` and stopped updating.
- The watchdog, recorder, and probe Antigravity processes were alive. The watchdog had entered its bounded probe-close wait.

## Investigation and evidence

| Claim | Status | Evidence/source | Consequence |
|---|---|---|---|
| The apparent hang was a hidden probe-close wait, not a dead watchdog. | VERIFIED | Sanitized state and process-tree inventory | Keep the shadow and do not launch a second probe. |
| Heartbeat was hard-coded to `WAIT_PRIMARY_CLOSE` and stopped before the bounded close wait. | VERIFIED | Source inspection and failing regression test | External audit could not distinguish cleanup wait from a hang. |
| Empty or unknown checkpoint input was immediately classified as failure. | VERIFIED | Source inspection and failing regression test | Accidental input could enter cleanup wait without an explicit owner choice. |

## Decisions

- Track one current lifecycle stage in status and heartbeat.
- Keep heartbeat active through probe-close wait, then stop it before cleanup.
- Accept only exact allowed checkpoint tokens and re-prompt otherwise.
- Preserve fail-closed cleanup while any matching Antigravity process is alive.

## Changes

| Path | Change | Reason |
|---|---|---|
| `scripts/phase0b-u006-shadow-home-watchdog.cjs` | Dynamic heartbeat, `CLOSE_PROBE` state, strict checkpoint loop. | Make the wait observable and prevent accidental classification. |
| `scripts/phase0b-u006-shadow-home-watchdog.test.cjs` | Two regression tests. | Reproduce and lock repaired behavior. |
| `evidence/phase-0b-u006-shadow-home/stall-repair.json` | Sanitized safety result. | Reproducible evidence without local data. |

## Verification

| Test/command | Result | Evidence |
|---|---|---|
| New tests before implementation | FAIL as expected | Missing `isAllowedAnswer`; hard-coded heartbeat detected |
| `node --check scripts/phase0b-u006-shadow-home-watchdog.cjs` | PASS, exit 0 | Local command output |
| `node --test scripts/phase0b-u006-shadow-home-watchdog.test.cjs` | PASS, exit 0 | 22/22 |
| `node --test scripts/*.test.cjs` | PASS, exit 0 | 48/48 |

Final privacy, link, parser, staged-secret, manifest, and diff checks run after the staged set is assembled.

## Security/privacy review

- Listener/bind impact: none.
- Secret/token handling impact: no values read or persisted.
- Config mutation/rollback impact: real-config mutation capability remains `false`.
- Logging/evidence review: only sanitized stage and verification metadata is recorded.
- New dependencies/supply-chain impact: none.

## Acceptance evaluation

- Observable close wait: PASS.
- Heartbeat remains active during close wait: PASS.
- Invalid checkpoint input cannot force terminal classification: PASS.
- Existing shadow retention invariant: PASS in tests; active session retained while probe is alive.
- U-006 live result: BLOCKED pending safe closure of the existing probe; no second probe was run.

## Risks and unresolved items

| Risk/unknown | Severity | Owner | Required next action |
|---|---|---|---|
| Existing probe remains open during the old watchdog's bounded close wait. | High | Owner | Close all Antigravity probe windows normally, then verify cleanup. |
| Old run cannot gain repaired heartbeat behavior in place. | Medium | Project | Record its terminal result and use committed code for later runs. |

## Git delivery

- Files committed: PENDING
- Commit(s): PENDING
- Push command/result: PENDING
- Compare/PR URL: none requested
- Dirty state after push: PENDING

## Rollback

Revert the scoped commit. No persistent user configuration is changed by this repair.

## Next run

- Exact next objective: safely close and clean the retained old shadow session; record its blocked result.
- Entry criteria: matching Antigravity process count is zero.
- Stop conditions: process-observation error, config drift, or retained probe process.
