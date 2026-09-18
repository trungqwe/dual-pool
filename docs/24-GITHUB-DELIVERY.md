# GitHub Delivery and Review Workflow

## Preflight

Before edits and again before push:

```powershell
git rev-parse --show-toplevel
git remote -v
git branch --show-current
git rev-parse HEAD
git status --porcelain=v2
git log -5 --oneline --decorate
```

The intended remote is `https://github.com/trungqwe/dual-pool` or an authenticated equivalent owned by the same repository. If it differs, stop. Do not replace an existing remote without owner approval.

## Branching

Use a focused branch such as:

```text
phase-0/compatibility-probe
phase-1/state-foundation
phase-2/cpa-isolation
fix/<bounded-problem>
docs/<bounded-doc-change>
```

Respect existing repository branch policy when discovered. Do not assume direct push to default branch is allowed. Never force-push shared branches.

## Commit policy

Prefer reviewable commits where code, tests and directly related docs travel together. Suggested messages:

```text
probe(phase-0): capture Codex provider compatibility
feat(accounts): add credential eligibility checks
fix(config): preserve concurrent user edits on rollback
test(isolation): prove provider roots cannot cross
docs(handoff): record phase 2 verification
```

Do not commit:

- OAuth/auth JSON, tokens, keys, raw captures or local logs;
- user-specific paths/emails/session IDs;
- binaries/download caches unless release policy explicitly requires them;
- unrelated formatter churn;
- generated reports that fail redaction.

## Required pre-push gate

- Phase tests pass.
- `git diff --check` passes.
- Docs links/format validation passes.
- Secret scan passes staged/tracked content and deliverables.
- Final staged diff reviewed.
- Handoff and current run report are present.
- Checklist contains only evidence-backed changes.
- `git status` shows no accidental untracked sensitive files.

## Push

Use a normal upstream push for the verified current branch. If authentication, protection or network access fails:

1. do not retry with embedded credentials or change remote security;
2. preserve the local commit;
3. record exact command/error and commit SHA;
4. give the credentialed operator the minimal normal push command;
5. mark push `BLOCKED`, not complete.

## Report/commit SHA problem

The report written before a commit cannot know its own final SHA. Record `PENDING` in the report. Once that report is included in a pushed commit it is immutable. Record final commit SHA, push result and compare/PR URL in `docs/18-HANDOFF.md` or a new append-only delivery receipt. Never amend, force-push, or edit the pushed report merely to make it self-referential.

## Review prompt

The independent reviewer should:

1. verify scope against the assigned phase;
2. inspect all changed files and existing dirty state;
3. reproduce tests/evidence;
4. challenge security/isolation/rollback claims;
5. check docs/report/handoff consistency;
6. identify untested assumptions and severity-ranked findings;
7. produce the next bounded implementation prompt only after findings are resolved or accepted.

## Initial package integration

Because this specification package was authored without authenticated access to the target repository, the first credentialed agent must merge it cautiously:

1. clone/fetch target repository normally;
2. inventory existing root/docs and instructions;
3. compare filenames/content;
4. preserve superior/existing constraints and reconcile conflicts explicitly;
5. add only non-conflicting files or merge content manually;
6. run docs/secret checks;
7. commit as a docs-only bootstrap change;
8. push and record the actual commit before Phase 0 machine probes.
