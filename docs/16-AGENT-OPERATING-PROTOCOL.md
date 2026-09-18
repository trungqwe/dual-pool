# Agent Operating Protocol

## Goal

Every implementation run must be small, reproducible, reviewable and safely handed to the next agent. Chat history is not a source of truth; repository files and evidence are.

## Start-of-run sequence

1. Read root `AGENTS.md` and the documentation control center.
2. Read `18-HANDOFF.md`, the most recent report, current roadmap phase and relevant component docs.
3. Run repository preflight: root, branch, HEAD, remotes, status, submodules/worktrees.
4. Inspect existing changes before editing. Do not reset, discard, stash or rewrite user work.
5. Reproduce the current reported state/test result.
6. Choose one bounded task from the current phase.
7. State the task, non-goals, planned files, tests and stop conditions in the new run report.

## Implementation rules

- Investigate before coding when a version-dependent fact is involved.
- Prefer interfaces and adapters with version/capability checks.
- Add failing tests for confirmed bugs/requirements before or with the fix.
- Keep diffs phase-scoped. Avoid drive-by refactors, dependency upgrades or formatting unrelated files.
- Never relax a test, invariant or error path merely to make the suite green.
- Never use live tokens/accounts in unit/CI fixtures.
- Put exploratory artifacts outside tracked paths unless sanitized evidence is explicitly required.
- Treat upstream failures as facts to document, not authorization to patch upstream.

## Verification sequence

Run the narrowest tests while iterating, then before commit:

1. formatting and static analysis;
2. unit tests for changed packages;
3. contract/integration tests affected by the change;
4. invariant/negative tests;
5. full repository test command defined by the phase;
6. documentation link/lint checks;
7. `git diff --check`;
8. secret scan of tracked changes and generated deliverables;
9. inspect final diff, status and untracked files.

If a required test cannot run, mark it BLOCKED with exact reason and do not claim completion.

## Evidence discipline

Every claim in the report must map to:

- command and exit code;
- test ID and result file;
- sanitized artifact/hash;
- or explicit source URL/version.

Avoid pasting huge logs. Record concise excerpts and paths. Never commit raw live captures.

## Commit and push

1. Update checklist entries with evidence.
2. Update handoff completely.
3. Add a dated run report.
4. Verify staged files are exactly intended.
5. Commit with phase/type/scope summary.
6. Re-run critical fast checks against committed HEAD if practical.
7. Verify `origin` URL and upstream branch.
8. Push normally; never force.
9. Record commit SHA and push result in report/handoff. If report must include the final SHA, use a second small `docs:` commit rather than amending a pushed commit.

## Required stop conditions

Stop and hand off when:

- repository access/target is ambiguous;
- a mutation would overwrite unknown user changes;
- a required permission/credential is absent;
- a compatibility gate fails;
- an invariant or secret scan fails;
- requested work requires binary patching, TLS interception or cross-provider fallback;
- scope would cross into the next roadmap phase;
- live account action requires user interaction not currently available.

## End-of-run definition

A run is complete only when code/docs are verified, the report and handoff are current, commit/push status is explicit, and the next agent can name the single next action without consulting prior chat.
