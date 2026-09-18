# Implementation Run Report Template

Copy to `docs/reports/YYYY-MM-DDTHHMMZ-phase-N-short-title.md`.

## Header

- Run ID:
- Date/time UTC:
- Agent/tool version:
- Roadmap phase:
- Repository:
- Branch:
- Start HEAD:
- End HEAD:
- Remote push result:

## Assigned objective

State one bounded deliverable and its acceptance criteria.

## Non-goals

List what was intentionally not attempted.

## Starting state

- Worktree status:
- Existing unrelated changes:
- Versions/environment fingerprint:
- Relevant prior report/handoff:
- Reproduction result:

## Investigation and evidence

For each important discovery:

| Claim | Status | Evidence/source | Consequence |
|---|---|---|---|
| | VERIFIED/UNKNOWN/BLOCKED | path, test ID, command, URL | |

## Decisions

List decisions made. For architecture changes, reference an ADR update and alternatives considered.

## Changes

| Path | Change | Reason |
|---|---|---|
| | | |

## Verification

| Test/command | Result | Evidence |
|---|---|---|
| | PASS/FAIL/BLOCKED | |

Include exact exit codes and concise results. Explicitly list mandatory tests not run.

## Security/privacy review

- Listener/bind impact:
- Secret/token handling impact:
- Config mutation/rollback impact:
- Logging/evidence review:
- Secret scan result:
- New dependencies/supply-chain impact:

## Acceptance evaluation

Map each task criterion to PASS/FAIL/BLOCKED and evidence. Do not use a general “works” statement.

## Risks and unresolved items

| Risk/unknown | Severity | Owner | Required next action |
|---|---|---|---|
| | | | |

## Git delivery

- Files committed:
- Commit(s):
- Push command/result:
- Compare/PR URL if available:
- Dirty state after push:

## Rollback

Describe how to revert this run without deleting credentials/user work. Include tested rollback result when persistent state changed.

## Next run

- Exact next objective:
- Entry criteria:
- Files/docs to read:
- Commands/tests to run first:
- Stop conditions:
