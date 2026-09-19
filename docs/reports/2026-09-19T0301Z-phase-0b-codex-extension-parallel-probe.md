# Phase 0B Codex extension parallel two-request probe

## Header

- Run ID: `phase-0b-codex-extension-parallel-20260919T0301Z`
- Date/time UTC: `2026-09-19T03:01Z`
- Agent/tool version: Codex CLI `0.154.0`
- Roadmap phase: Phase 0B
- Repository: `https://github.com/trungqwe/dual-pool.git`
- Branch: `phase-0/reversible-compatibility`
- Start HEAD: `c55cd44bf951050b75dc6b7dd8325a9788e52499`
- End HEAD: pending
- Remote push result: pending

## Assigned objective

Prove U-005 and U-006 with a second isolated Antigravity instance and two user-assisted Codex extension requests: a baseline request at the observed non-Astra model, then a request after selecting canonical `gpt-6-astra`.

Acceptance requires loopback `/v1/responses`, the temporary provider and synthetic authorization on both requests, baseline model `gpt-5.6-sol`, Astra model on request two, and survival of every pre-existing Antigravity PID.

## Non-goals

- No Antigravity Cloud Code probe.
- No Phase 1 work.
- No real OpenAI/Codex provider account.
- No modification of real Codex configuration, Antigravity settings, or installed extension files.
- No copying auth state, sessions, cookies, or global storage.

## Starting state

- Worktree: existing uncommitted Phase 0B probe files and handoff updates from the prior user-assisted run; no unknown changes discarded.
- Branch/HEAD: expected branch and HEAD verified.
- Installed versions: Antigravity CLI `1.107.0`, Codex extension `openai.chatgpt@26.5730.61309`, Codex CLI `0.154.0`.
- Prior result: `ASTRA_NOT_VISIBLE` on the single-request picker attempt; its evidence remains immutable.
- Current IDE: 23 Antigravity-related processes observed before this run; all are protected.

## Plan and stop conditions

The probe will snapshot protected PIDs, verify supported CLI flags, run a parallel smoke test, copy only the installed Codex extension package into a temporary extension silo, start a loopback two-request recorder, and launch the probe with temporary `--user-data-dir` and `CODEX_HOME`.

Stop on missing `--user-data-dir`, single-instance reuse, isolated Codex authentication requirement, missing baseline capture, missing Astra, route/provider mismatch, a protected PID disappearing, or any cleanup/security failure.

## Official documentation recheck

OpenAI Developers documentation was rechecked for `CODEX_HOME`, user-level `config.toml`, `model_provider`, `model_providers.<id>.env_key`, and `wire_api = "responses"`. Sources: `https://developers.openai.com/es-419/docs/config-file/environment-variables`, `https://developers.openai.com/es-419/docs/config-file/config-basic`, and `https://developers.openai.com/es-419/docs/config-file/config-reference`.

## Changes and verification

The first two-request attempt reached `POST /v1/responses` with `application/json`, but the recorder could not parse the body before model observation. The sanitized blocked result is `evidence/phase-0b-codex-extension/codex-extension-two-request-20260919T031300553Z.json`; the recorder was then updated to trim BOM/whitespace and handle supported content encodings while retaining only body length/hash/prefix metadata on parse failure. The final run must include JSON validation, PowerShell/Node syntax checks, documentation-link checks, security scan, cleanup verification, before/after control-hash comparison, and `git diff --check` before commit.

The next attempt reached the same endpoint with a 58,422-byte identity encoded JSON body beginning with a JSON object, but a framed JSON parser still could not parse it. The final sanitized result is `evidence/phase-0b-codex-extension/codex-extension-two-request-20260919T031615744Z.json`. The observed limitation is recorded as `EXTENSION_PAYLOAD_NOT_PARSEABLE`; U-005 remains partial/unknown and U-006 remains blocked.

## Acceptance status

U-001..U-004 remain blocked by scope. U-005 and U-006 are open for the two-request probe. U-007 remains probed by the existing Gate A v5 evidence. U-008 remains partial/unknown.

## Rollback and next step

The probe uses disposable temporary directories and child-only environment variables. No real state restoration should be needed. After the run, update current handoff/traceability/checklist docs, finalize sanitized evidence, commit scoped changes, push normally, and record a delivery receipt.
