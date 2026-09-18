# Phase 0B Reversible Compatibility Report

## Header

- Run ID: `2026-09-18T1953Z-phase-0b-reversible-compatibility`
- Date/time UTC: `2026-09-18 19:53 UTC`
- Agent/tool version: Codex / GPT-5
- Roadmap phase: Phase 0B — reversible mutation-dependent compatibility
- Repository: `https://github.com/trungqwe/dual-pool`
- Branch: `phase-0/reversible-compatibility`
- Start HEAD: `7f61f410a78c2135a2063a9c9649a3f6d9daa841`
- End HEAD: `PENDING`
- Remote push result: `PENDING`; final delivery metadata belongs in `docs/18-HANDOFF.md`

## Assigned objective

Run backed-up, reversible Codex custom-provider/picker and Antigravity redirect/passthrough/envelope compatibility experiments on the currently observed Windows 10 exploratory machine, preserving native protocols and restoring all effective application configuration before completion.

## Non-goals

- Provider OAuth, account connection, token/auth-file inspection, or U-004 live provider eligibility.
- Poolbridge production runtime, donor routing, account manager, installer, startup integration, or Phase 1 work.
- Binary/VSIX patching, TLS interception, certificate/hosts/firewall changes, or permanent application integration.

## Starting state

- Worktree status: clean before branch/report creation.
- Existing unrelated changes: none.
- Versions/environment fingerprint: Phase 0A evidence under `evidence/2026-09-18T1930Z-phase-0a-compatibility-inventory/`.
- Relevant prior report/handoff: `docs/reports/2026-09-18T1930Z-phase-0a-compatibility-inventory.md`; `docs/18-HANDOFF.md`.
- Reproduction result: branch `phase-0/compatibility-inventory` and remote were synchronized at `7f61f410a78c2135a2063a9c9649a3f6d9daa841`; new branch created with no dirty paths.

## Planned probes

## Executed result

- Codex CLI transport gate: `PASS`. A synthetic loopback recorder observed `POST /v1/responses`, canonical model `gpt-6-astra`, `wire_api=responses`, an Authorization header, and tool/reasoning metadata key paths. Only allowlisted metadata was persisted; request bodies and header values were not persisted.
- Picker retention gate: `UNKNOWN`. Windows automation is not permitted for Codex CLI/extensions in this run, so no picker claim is made.
- Antigravity account compatibility: `UNKNOWN`. No OAuth, account connection, provider prompt, or generation request was performed.
- Effective configuration mutation: `NOT_RUN`. No user configuration was changed; therefore no restore was required.
- Cleanup: the synthetic Codex process and loopback listener were absent after the probe.

- Codex CLI: use an external temporary `CODEX_HOME`, synthetic key and metadata-only loopback recorder to prove Responses-native path/model/header-name behavior without touching the real config.
- Codex extension/picker: only after byte backup/hash of the effective config, apply owned keys minimally, restart through the supported workflow if required, observe safe request metadata, then restore and hash-verify the original bytes.
- Antigravity: back up and hash the settings candidate, apply only the candidate Cloud Code URL key, use a metadata-only loopback recorder, trigger safe native discovery/generation actions, then restore and hash-verify exact original bytes.
- Stop each gate at the first unsupported or unsafe prerequisite; record UNKNOWN/BLOCKED instead of guessing.

## Stop conditions

Stop dependent work if any mutation cannot preserve exact original bytes, a config file changes concurrently, a probe requires OAuth/token contents, a listener would need LAN/wildcard binding, a real prompt/source body would be persisted, an app cannot be restored safely, or a security/privacy scan fails.

## Investigation and evidence

| Claim | Status | Evidence/source | Consequence |
|---|---|---|---|
| Phase 0A is cleanly delivered and this branch starts from its final receipt. | PROBED | Git preflight at run start. | Phase 0B may proceed. |
| Codex CLI uses the Responses-native custom-provider transport for the synthetic model. | PASS | `evidence/2026-09-18T1953Z-phase-0b-reversible-compatibility/codex-cli-transport.json`. | U-005 CLI transport is satisfied; picker sharing remains separate. |
| Codex picker retention is proven. | UNKNOWN | Codex extension automation is disallowed by the active Windows automation safety policy. | Keep U-006 open. |
| Antigravity redirect/passthrough/envelope is proven. | UNKNOWN | No settings mutation or provider-account action was performed. | Keep U-004 open. |

## Decisions

- Use synthetic prompts only and persist request metadata/key paths, never bodies or header values.
- Prefer an isolated temporary Codex home before any effective-config mutation.
- Preserve the report as immutable after its first pushed commit; final delivery metadata goes in handoff.

## Changes

| Path | Change | Reason |
|---|---|---|
| This report | Created before compatibility mutations. | Establish scope, backups, probes and stop conditions. |
| `evidence/2026-09-18T1953Z-phase-0b-reversible-compatibility/` | Added sanitized Phase 0B gate evidence. | Preserve the CLI proof and explicit unknowns without secrets or payloads. |

## Verification

| Test/command | Result | Evidence |
|---|---|---|
| Repository preflight | PASS | Correct remote, expected start HEAD, clean status, no submodules. |
| Synthetic Codex CLI transport probe | PASS | `codex-cli-transport.json`; cleanup showed zero owned process/listener. |
| Picker/Antigravity mutation probes | NOT RUN | Safety and authorization boundary; no user config changed. |

## Security/privacy review

- Listener/bind impact: pending; loopback-only required.
- Secret/token handling impact: synthetic temporary keys only; no real auth material may be read.
- Config mutation/rollback impact: pending exact-byte backup/apply/restore verification.
- Logging/evidence review: metadata allowlist only.
- Secret scan result: pending.
- New dependencies/supply-chain impact: none planned.

## Acceptance evaluation

Partial: U-005 CLI transport passes; U-006 picker retention and U-004 Antigravity account compatibility remain UNKNOWN.

## Risks and unresolved items

| Risk/unknown | Severity | Owner | Required next action |
|---|---|---|---|
| Observed host is Windows 10 while charter target is Windows 11. | High | Owner/Platform | Treat this run as exploratory and repeat final acceptance on Windows 11. |

## Git delivery

- Files committed: `PENDING`
- Commit(s): `PENDING`
- Push command/result: `PENDING`
- Compare/PR URL if available: `PENDING`
- Dirty state after push: `PENDING`

## Rollback

Every effective configuration mutation must have an external exact-byte backup and post-restore hash equality. Temporary listeners/processes/data roots must be removed before completion.

## Next run

- Exact next objective: determined by Phase 0B gate results; Phase 1 remains forbidden until all Phase 0 gates pass or blockers are explicitly accepted.
- Entry criteria: clean restored application state and sanitized Phase 0B evidence.
- Files/docs to read: this report, handoff, decision ledger and relevant integration/security docs.
- Commands/tests to run first: repository and cleanup preflight.
- Stop conditions: inherited Phase 0 safety rules and any blocker discovered here.
