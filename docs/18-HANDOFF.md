# Current Handoff

Last updated: 2026-09-18 19:19 UTC.

## Current status

Specification package prepared and initialized as a local Git repository on `main`. Runtime implementation has not started. No compatibility claim has been proven on the target Windows machine.

## Repository access status

Target remote: `https://github.com/trungqwe/dual-pool`.

The target repository was verified through GitHub CLI as `trungqwe/dual-pool`, public, empty, and writable by the authenticated owner account. The local `origin` is `https://github.com/trungqwe/dual-pool.git`; the initial documentation commit and push are pending final verification in this run.

## Current phase

Phase 0 — Repository and compatibility audit.

## Completed in this package

- Project charter, requirements and non-goals.
- Two-process provider-isolated architecture.
- Trust invariants, threat model and rollback design.
- Detailed compatibility probe and hard go/no-go gates.
- Antigravity and Codex integration contracts.
- Roadmap, master checklist, verification and negative-test matrices.
- Agent workflow, report template, GitHub delivery protocol and master prompt.

## Not completed

- No branch-protection policy exists yet; the previously empty repository has no established review workflow.
- No target-machine version/path/port evidence.
- No CLIProxyAPI release pin/checksum.
- No Antigravity setting/route/envelope proof.
- No Codex picker/provider-retention proof.
- No Go code, tests, CI, binaries or live account actions.
- No runtime implementation commit exists yet.

## Key architecture decision

Use two instances of one pinned upstream CLIProxyAPI executable:

- Codex instance: proposed `127.0.0.1:8317`, Codex-only auth root.
- Google instance: proposed `127.0.0.1:8318`, Google/Antigravity-only auth root.
- Antigravity bridge: proposed `127.0.0.1:51074`.

Ports remain probe-dependent. This structure makes cross-provider credential selection impossible at the process/auth-directory boundary.

## Next exact action

After the initial documentation push is confirmed, execute only Phase 0 from `06-COMPATIBILITY-PROBE.md`. Produce a new report under `docs/reports/`, update this handoff, and push only sanitized evidence.

## Phase 0 blockers requiring the target machine

- Effective Antigravity settings path/key and native routing schema.
- Installed Codex/extension versions, effective config layer and bundled catalog.
- Codex dropdown selection behavior with a custom provider.
- Available ports and callback behavior.
- Candidate CLIProxyAPI release artifact and exact versioned schema.

## Do not do next

- Do not implement runtime routing before probes.
- Do not connect all accounts before one-account eligibility flow is proven.
- Do not patch either third-party application.
- Do not use a Codex catalog downloaded from a mismatched branch/version.
- Do not push this standalone tree over an existing remote branch without merging/inventory.
