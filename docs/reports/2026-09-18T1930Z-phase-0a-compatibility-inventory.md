# Phase 0A Compatibility Inventory Report

## Header

- Run ID: `2026-09-18T1930Z-phase-0a-compatibility-inventory`
- Date/time UTC: `2026-09-18 19:30 UTC`
- Agent/tool version: Codex / GPT-5
- Roadmap phase: Phase 0A — non-destructive compatibility inventory
- Repository: `https://github.com/trungqwe/dual-pool`
- Branch: `phase-0/compatibility-inventory`
- Start HEAD: `5e91df5f31531452a74444d171a7edb7db405573`
- End HEAD: `PENDING`
- Remote push result: `PENDING`; final delivery metadata belongs in `docs/18-HANDOFF.md`

## Assigned objective

Reconcile Phase 0 governance contradictions, establish a safe raw-versus-sanitized evidence boundary, and capture all non-destructive target-machine compatibility evidence possible without changing effective Antigravity or Codex configuration and without connecting provider accounts.

Acceptance criteria are the Phase 0A success criteria in the assigned run prompt: sanitized repository/environment/port/product evidence; independently verified and disposable-probed CLIProxyAPI candidate; honest U-001 through U-008 status; no production integration, OAuth, secret/private data, or persistent probe process.

## Non-goals

- Phase 0B mutation-dependent Codex transport/picker and Antigravity redirect/passthrough/envelope experiments.
- Poolbridge runtime, account management, OAuth, installer, startup integration, or production configuration.
- Binary/VSIX patching, TLS interception, certificate/hosts/firewall changes, or provider account connection.

## Starting state

- Worktree status: clean before branch creation and report creation.
- Existing unrelated changes: none.
- Versions/environment fingerprint: pending sanitized Phase 0A inventory.
- Relevant prior report/handoff: `docs/reports/2026-09-18T1919Z-phase-0-github-bootstrap.md`; `docs/18-HANDOFF.md`.
- Reproduction result: repository preflight matched expected `main` HEAD `5e91df5f31531452a74444d171a7edb7db405573`, target `origin`, no submodules, and no dirty paths.

## Planned files and probes

- Governance: minimally update Phase 0A/0B boundaries, isolation wording, report immutability, checklist, risk, traceability, sources, and handoff.
- Evidence safety: add root `.gitignore` and document raw → sanitize → scan → commit.
- Sanitized bundle: `evidence/2026-09-18T1930Z-phase-0a-compatibility-inventory/` with required JSON files and summary.
- Read-only probes: repository identity, Windows/tool/product versions and known locations, proposed ports, Codex supported discovery/catalog options, Antigravity metadata/settings candidates.
- Disposable upstream probe: official Windows x64 release/checksum verification, one temporary loopback instance with synthetic keys and no auth accounts, management-auth negative/positive tests, listener/cleanup proof.

## Stop conditions

Stop rather than bypass if the remote identity changes, unknown work would be overwritten, any probe requires reading token contents or changing effective app configuration, OAuth/login or patching is required, published artifact integrity cannot be independently established, a foreign listener would need termination, evidence cannot be sanitized, or a secret/privacy scan fails.

## Investigation and evidence

| Claim | Status | Evidence/source | Consequence |
|---|---|---|---|
| Repository preflight matches the assigned baseline. | PROBED | Start-of-run commands and sanitized `repo.json`. | Phase 0A may proceed on the feature branch. |
| Observed machine is Windows 10 Pro build 19045 x64. | PROBED | `evidence/2026-09-18T1930Z-phase-0a-compatibility-inventory/environment.json` | Evidence does not satisfy the charter's Windows 11 acceptance claim. |
| Canonical Astra exists in the exact installed Codex CLI catalog. | PROBED | `codex-catalog.json`, P0A-CODEX-001 | U-007 is probed; U-005 remains partial until extension transport/config sharing is tested. |
| Installed Antigravity bundle contains a Cloud Code URL key candidate. | PARTIAL / UNKNOWN | `antigravity-discovery.json`, P0A-AG-001 | U-001 remains open because settings support and loopback behavior were not tested. |
| CLIProxyAPI v7.3.7 Windows amd64 release integrity is independently established. | PROBED | `cliproxy-schema.json`, P0A-SUP-001 | Candidate can advance to later contract probes without calling it production-supported. |
| Credential-free CLIProxyAPI loopback/auth behavior meets the Phase 0A probe contract. | PROBED | `cliproxy-schema.json`, P0A-CPA-001 | Candidate bound only IPv4 loopback, required keys, created no auth records, and stopped cleanly. |
| Provider-specific endpoint response shapes are proven. | UNKNOWN | No account/OAuth was permitted. | U-004/U-008 response-shape portions remain open. |

## Decisions

- Split Phase 0 prospectively into non-destructive Phase 0A and reversible mutation-dependent Phase 0B without changing the architecture.
- Keep raw/download/binary/log/temp-root material outside Git and commit only whitelisted, scanned derivatives.
- Preserve ADR-002 while treating opposite-provider auth material as a fail-closed integrity violation rather than claiming isolation is unconditional.
- Keep U-001/U-005/U-008 open as partial and U-002/U-003/U-004/U-006 open as UNKNOWN; only U-007 reached PROBED.
- Treat CLIProxyAPI v7.3.7 as a verified candidate, not a production pin, because credentialed provider shapes/health are still open.

## Changes

| Path | Change | Reason |
|---|---|---|
| This report | Created before substantive edits/probes. | Establish immutable run scope and evidence plan. |
| `.gitignore` | Protect raw downloads, binaries, archives, logs, captures, auth/secret files and local data roots. | Enforce the evidence boundary without ignoring sanitized evidence. |
| `evidence/2026-09-18T1930Z-phase-0a-compatibility-inventory/` | Add sanitized environment, versions, repository, ports, Codex, Antigravity, CLIProxyAPI and result records. | Make Phase 0A findings reproducible and reviewable. |
| Governance/roadmap/probe/checklist docs | Define Phase 0A/0B, isolation integrity, report immutability and evidence flow. | Reconcile contradictions before probing. |
| Decision/risk/traceability/source docs | Record honest unknown status, resolve R-13 and link immutable upstream evidence. | Keep claims and gates evidence-backed. |
| `docs/18-HANDOFF.md` | Replace stale bootstrap state with current findings, blockers and next task. | Enable a chat-independent next run. |

## Verification

| Test/command | Result | Evidence |
|---|---|---|
| Repository preflight | PASS | Expected remote, branch source HEAD, clean status, no submodules. |
| JSON parse for every committed evidence JSON | PASS | All nine evidence JSON files parsed with `ConvertFrom-Json`. |
| Evidence manifest SHA-256 verification | PASS | Seven immutable core evidence artifacts matched `evidence-manifest.json`. |
| Documentation relative-link validation | PASS | All relative Markdown targets exist. |
| Secret-value scan | PASS | No private key, provider-key, bearer value, or populated token/secret/state JSON pattern matched. |
| Privacy scan | PASS | No literal user-profile path or email address matched. |
| `git diff --check` | PASS | Final candidate diff contains no whitespace errors. |
| Disposable process/listener cleanup | PASS | No `cli-proxy-api` process or listener on 18317/18318; temporary root removed. |

## Security/privacy review

- Listener/bind impact: one credential-free disposable process bound `127.0.0.1:18318` and was stopped; no listener remains.
- Secret/token handling impact: independent CSPRNG 256-bit synthetic keys existed only in the removed temporary root and process memory; no values are in committed evidence.
- Config mutation/rollback impact: no effective Antigravity/Codex configuration was read or changed; no rollback was needed.
- Logging/evidence review: raw logs/full catalogs/download/source/config roots were removed; committed evidence uses explicit field allowlists and path placeholders.
- Secret scan result: PASS across the final candidate tree for private keys, provider keys, bearer values, populated token/secret/state fields, authorization values, emails and literal user-profile paths.
- New dependencies/supply-chain impact: no project dependency planned; candidate upstream artifact remains temporary and untracked.

## Acceptance evaluation

| Criterion | Status | Evidence |
|---|---|---|
| Governance contradictions reconciled prospectively | PASS | Phase 0A/0B, report immutability and isolation-integrity documentation diff |
| Safe evidence boundary and `.gitignore` exist | PASS | `.gitignore`, `docs/06-COMPATIBILITY-PROBE.md` |
| Repository/environment/versions/ports captured | PASS | `repo.json`, `environment.json`, `versions.json`, `ports.json` |
| Installed Codex/catalog state captured safely | PASS | `codex-catalog.json`, P0A-CODEX-001 |
| CLIProxyAPI candidate independently verified/probed | PASS | `cliproxy-schema.json`, P0A-SUP-001/P0A-CPA-001 |
| Antigravity candidates captured without mutation | PASS | `antigravity-discovery.json`, P0A-AG-001 |
| U-001 through U-008 updated honestly | PASS | `docs/03-DECISIONS-AND-EVIDENCE.md` |
| R-13 reconciled | PASS | `repo.json`, `docs/22-RISK-REGISTER.md` |
| No integration/OAuth/private data persists | PASS | P0A-CLEANUP-001/P0A-SEC-001 and final candidate-tree scan |
| Commit and push | PENDING | Delivery section |

## Risks and unresolved items

| Risk/unknown | Severity | Owner | Required next action |
|---|---|---|---|
| Observed Windows 10 differs from Windows 11 project target. | High | Owner/Platform | Confirm this machine is acceptable for exploratory Phase 0B or provide the Windows 11 target. |
| U-001/U-002/U-003 Antigravity behavior remains mutation-dependent. | High | Phase 0B | Run backed-up redirect, route, passthrough and envelope probes. |
| U-004 requires a provider account and live model probe. | High | Owner/Accounts | Authorize a separate minimal credentialed probe later; do not guess from registered routes. |
| U-005/U-006 Codex extension transport/picker remain open. | High | Phase 0B | Run reversible custom-provider and picker retention experiments. |
| U-008 provider shapes/health semantics remain partial. | High | Phase 0/Accounts | Run later credentialed contract probes against the candidate. |

## Git delivery

- Files committed: `PENDING`
- Commit(s): `PENDING`
- Push command/result: `PENDING`
- Compare/PR URL if available: `PENDING`
- Dirty state after push: `PENDING`

## Rollback

No persistent application mutation occurred. Repository changes can be reverted normally. The disposable upstream process was stopped, its listeners disappeared, and its external temporary root was removed.

## Next run

- Exact next objective: Phase 0B reversible Codex and Antigravity compatibility experiments, only if Phase 0A finds no blocking security/repository/upstream issue.
- Entry criteria: Phase 0A evidence and cleanup gates pass.
- Files/docs to read: this report, `docs/18-HANDOFF.md`, Phase 0 probe/integration/security documents.
- Commands/tests to run first: repository preflight and evidence integrity checks.
- Stop conditions: inherited safety constraints plus any blocker found in this run.
