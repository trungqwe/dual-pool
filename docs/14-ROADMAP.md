# Implementation Roadmap

One phase per agent run by default. Each phase ends with tests, report, handoff, commit and push. Dates are intentionally absent; gates, not estimates, control progression.

## Phase 0 — Repository and compatibility audit

Entry: repository access and target Windows machine available.

Deliver:

- repository/current-state inventory;
- read-only probe command/script scaffolding;
- sanitized environment, Codex, Antigravity and candidate CLIProxyAPI evidence;
- resolved/updated unknowns U-001 through U-008;
- go/no-go recommendation and candidate `upstream.lock`.

Phase 0A is non-destructive discovery: repository/environment/port/product inventory, safe installed-catalog discovery, and credential-free disposable CLIProxyAPI probing. It does not change effective Antigravity/Codex configuration or connect accounts.

Phase 0B contains backed-up, reversible mutation-dependent compatibility experiments: Codex custom-provider transport and picker retention plus Antigravity redirect, passthrough and envelope round trip.

Exit: all Phase 0 gates in `06-COMPATIBILITY-PROBE.md` pass or explicit blockers are documented. No product integration remains enabled.

## Phase 1 — Foundation and state safety

Deliver:

- Go module/CLI skeleton and version output;
- data-root resolver, structured errors and redacted logging;
- state/ownership schemas, atomic store, locks and migration framework;
- secret-store interface and Windows implementation decision;
- config transaction/backup/rollback engine using fixtures;
- unit tests and CI basics.

Exit: atomicity, concurrent-edit protection, idempotence, redaction and recovery tests pass.

## Phase 2 — Pinned upstream lifecycle and isolation

Deliver:

- verified downloader/stager using `upstream.lock`;
- two-instance config generator for pinned version;
- process identity, start/stop/restart/status and health L0-L3;
- loopback/listener validation;
- update check/stage/promote/automatic rollback;
- isolation integration tests.

Exit: both empty instances run concurrently with disjoint roots/keys; negative security tests pass.

## Phase 3 — Account onboarding and eligibility

Deliver:

- Management API client with typed schemas/timeouts;
- serialized browser OAuth flow for Google and Codex;
- account inventory and opaque local metadata;
- list/test/disable/enable/reauth commands;
- credential-specific model and tiny live probe;
- no-token-access proof and redaction tests.

Exit: at least one eligible credential per pool; all account commands work; ineligible accounts are excluded with reasons.

## Phase 4 — Antigravity transparent bridge

Deliver:

- exact discovery adapter from Phase 0;
- fixed-origin passthrough proxy with streaming/cancel/backpressure;
- observe and passthrough modes;
- setting patch/restore plus emergency rollback;
- direct-vs-bridge differential suite.

Exit: all native models and actions pass transparently; donor override remains disabled.

## Phase 5 — Antigravity donor override

Deliver:

- model catalog capture/fingerprint and interactive donor selection;
- generation route allowlist and exact match predicate;
- native envelope adapter to Google provider endpoint;
- stable errors/degraded behavior;
- affinity/failover and 20-turn suite.

Exit: only donor traffic reaches Google instance; non-donor parity and provider isolation pass.

## Phase 6 — Codex provider transport

Deliver:

- Codex discovery/version adapter;
- user config transaction for custom provider and environment secret path;
- canonical Astra validation;
- Responses HTTP/SSE end-to-end tests and optional WebSocket evaluation;
- harness-shape evidence and per-account eligibility.

Exit: synthetic and real Codex scenarios reach only Codex instance with native Responses semantics.

## Phase 7 — Codex model selector

Deliver:

- UI selector/provider-retention automated or rigorously scripted test;
- exact-version catalog fallback if required and proven safe;
- upgrade invalidation when version/catalog changes;
- rollback of all owned Codex settings.

Exit: dropdown goal passes or an owner-approved supported alternative is recorded. No bundle patch.

## Phase 8 — Reliability and release candidate

Deliver:

- complete doctor and diagnostic bundle;
- startup integration using user scope;
- crash/restart/corruption/failure-injection suite;
- performance baseline and thresholds;
- SBOM, checksums, release packaging and uninstall/rollback guide;
- complete operator documentation.

Exit: all MUST traceability rows pass; no open critical/high risk without owner acceptance.

## Phase 9 — Target-machine acceptance

Deliver:

- four-account target or owner-accepted actual count for each pool;
- complete Google 20-turn and Codex 30-turn runs;
- restart, re-auth, failover, rollback and upgrade rehearsal;
- final sanitized evidence bundle and signed acceptance report.

Exit: Definition of Done in `01-PROJECT-CHARTER.md` is satisfied.

## Phase discipline

- A phase may split into multiple small PRs, but later phase behavior must not be smuggled into an earlier phase.
- Discovered incompatibility updates docs/tests before implementation workaround.
- Do not mark an exit criterion complete without an evidence ID/path.
- If a phase is BLOCKED, handoff states the smallest user/external action required; agents do not bypass it.
