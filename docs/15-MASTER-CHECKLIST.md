# Master Checklist

This is the authoritative progress ledger. Check a box only with an adjacent evidence/report reference in the implementing commit.

## Governance

- [x] Repository remote and default branch verified; no protection workflow is configured yet. Evidence: `docs/reports/2026-09-18T1919Z-phase-0-github-bootstrap.md`.
- [x] Existing user changes inventoried and preserved. Evidence: initial empty-remote inventory in `docs/reports/2026-09-18T1919Z-phase-0-github-bootstrap.md`.
- [x] All agents have read required documents. Evidence: `docs/reports/2026-09-18T1919Z-phase-0-github-bootstrap.md`.
- [x] Current phase and scope recorded in handoff. Evidence: `docs/18-HANDOFF.md`.
- [ ] Requirements/ADRs/tests updated for any design change.

## Phase 0

### Phase 0A — non-destructive discovery

- [x] Windows/architecture/tool and installed product version inventory captured. Evidence: `evidence/2026-09-18T1930Z-phase-0a-compatibility-inventory/environment.json`, `versions.json`.
- [x] Codex CLI/config candidate and extension inventory captured without reading or changing effective config. Evidence: `versions.json`.
- [x] Exact installed Codex bundled/effective catalog export supported; Astra descriptor captured. Evidence: `codex-catalog.json`.
- [x] Antigravity version, settings path candidate and static key candidate inventoried without mutation. Evidence: `antigravity-discovery.json`.
- [x] Candidate CLIProxyAPI release/hash/config/routes and credential-free loopback/auth behavior probed. Evidence: `cliproxy-schema.json`, `probe-results.json`.
- [x] Proposed and candidate callback port collision inventory captured. Evidence: `ports.json`.
- [x] U-001 through U-008 reviewed with partial evidence kept open. Evidence: `docs/03-DECISIONS-AND-EVIDENCE.md`.

### Phase 0B — reversible mutation-dependent compatibility

- [x] Codex CLI custom provider sends Responses to loopback; Gate A v5 PASS với natural exit, sentinel scan và cleanup. Evidence: [v5 result](../evidence/phase-0b-v5/codex-cli-transport-v5-20260918T212522665Z.json). Chưa chứng minh extension/config-layer hoặc Windows 11.
- [ ] Codex picker retains desired provider route — `BLOCKED`: the isolated two-request probe reached loopback, but the extension's JSON body was not safely parseable; no Astra request was accepted. Evidence: [two-request result](../evidence/phase-0b-codex-extension/codex-extension-two-request-20260919T031615744Z.json).
- [ ] Antigravity setting path/key and loopback behavior proven reversibly.
- [ ] Native route and schema inventory captured safely.
- [ ] Transparent passthrough proven.
- [ ] Donor native envelope round trip proven.
- [x] U-001 through U-008 resolved or explicitly classified for Phase 0 exit; U-008 remains `PARTIAL_UNKNOWN` and blocks provider promotion. Evidence: `evidence/phase-0-closure.json`.

## Foundation

- [ ] Go CLI skeleton and version metadata.
- [ ] Stable errors and exit codes.
- [ ] Structured allowlist logger.
- [ ] State/ownership schema and migrations.
- [ ] Atomic store and crash recovery.
- [ ] Global/per-file locks with PID identity.
- [ ] Windows secret-store implementation reviewed.
- [ ] Config backup/patch/rollback engine.
- [ ] Redaction and sentinel-secret tests.

## CLIProxyAPI lifecycle

- [x] Candidate `upstream.lock` populated with independently verified Windows artifact metadata; provider support remains unverified. Evidence: `evidence/phase-0-closure.json`.
- [ ] Download checksum mismatch fails closed.
- [ ] Version/config adapter mismatch fails closed.
- [ ] Codex and Google configs have disjoint paths/keys/ports.
- [ ] Both instances bind only `127.0.0.1`.
- [ ] Management requires key and disallows remote.
- [ ] Process identity prevents foreign PID termination.
- [ ] Start/stop/restart/status idempotent.
- [ ] Staged update and automatic rollback tested.

## Accounts

- [ ] OAuth flows use Management API only.
- [ ] OAuth URL validation and state timeout/cancel.
- [ ] Ambiguous new credential detection fails safely.
- [ ] Credential-specific model inventory.
- [ ] Tiny generation/Responses capability probe.
- [ ] Eligibility exclusion with stable reason.
- [ ] List/test/disable/enable/reauth commands.
- [ ] Poolbridge never downloads/parses auth token files.
- [ ] One or more eligible accounts per pool.

## Antigravity

- [ ] Bridge modes and fixed upstream allowlist.
- [ ] Streaming, backpressure and cancellation.
- [ ] Model/catalog requests passthrough.
- [ ] Exact donor selector and catalog fingerprint.
- [ ] Generation endpoint allowlist.
- [ ] Native envelope mapping golden fixtures.
- [ ] Google-down degraded behavior.
- [ ] Non-donor parity suite.
- [ ] Donor route reaches only Google port.
- [ ] Session affinity and failover.
- [ ] Tool/file/terminal behavior.
- [ ] 20+ turn and restart test.
- [ ] Emergency rollback tested with services down.

## Codex

- [ ] User-level custom provider patch is reversible.
- [ ] Secret reaches client without plaintext config/log.
- [ ] Canonical Astra descriptor validated.
- [ ] Picker/provider-retention test.
- [ ] Exact-version catalog fallback only if needed.
- [ ] Responses HTTP/SSE event contract.
- [ ] WebSocket separately tested or explicitly disabled.
- [ ] Tool/apply_patch/shell/multi-file behavior.
- [ ] Reasoning and compaction behavior.
- [ ] Parent/subagent affinity.
- [ ] 30+ turn and restart test.
- [ ] Traffic reaches only Codex port.

## Security and privacy

- [ ] No wildcard/LAN listeners.
- [ ] Four distinct strong secrets.
- [ ] Restricted ACLs verified.
- [ ] No open proxy/SSRF route.
- [ ] No prompt/output/tool/source persistence.
- [ ] No raw session/account identity in logs.
- [ ] Secret scan passes repo, report and ZIP.
- [ ] Supply-chain verification and SBOM.
- [ ] No binary/VSIX/CA/hosts patch.

## Release and handoff

- [ ] Unit, contract, integration and live gates pass.
- [ ] Performance threshold passes or waiver accepted.
- [ ] Doctor and diagnostic bundle validated.
- [ ] Rollback/uninstall without network tested.
- [x] `docs/18-HANDOFF.md` updated. Evidence: current handoff delivery receipt.
- [x] Run report added under `docs/reports/`. Evidence: `docs/reports/2026-09-18T1930Z-phase-0a-compatibility-inventory.md`.
- [x] Applicable Phase 0A JSON/hash/security/privacy/cleanup checks and docs links pass; runtime tests are not yet in scope. Evidence: `probe-results.json` and the run report.
- [x] Commit contains no unrelated files. Evidence: final staged name/status review in the run report.
- [x] Push succeeds without force. Evidence: commit `3f70ed84cbc52772ed1e448f3ee891c6f3343bbe` on `origin/phase-0/compatibility-inventory`.
- [x] Commit SHA and compare/PR URL recorded in handoff. Evidence: `docs/18-HANDOFF.md`.
## Phase 0B parser-repair addendum — 2026-09-19

- [x] v2 parser self-tests and privacy-safe serializer checks pass.
- [x] One isolated baseline request reached `POST /v1/responses`; route, auth, identity encoding, UTF-8, JSON, prompt, sanitizer, and response lifecycle passed.
- [ ] Baseline model gate remains open: `BASELINE_MODEL_MISMATCH`; U-005 is PARTIAL / UNKNOWN and U-006 remains BLOCKED. Evidence: [v2 result](../evidence/phase-0b-codex-extension-v2/extension-baseline-20260919T040521877Z.json).
## Phase 0B authenticated-picker addendum — 2026-09-19

- [x] Corrected the unauthenticated Astra observation to `NON_DIAGNOSTIC_FOR_ASTRA_ENTITLEMENT`.
- [x] Closed U-005 as `PROBED` for the observed Windows 10/runtime scope using existing config-layer evidence.
- [ ] U-006 authenticated picker probe: `BLOCKED: AUTHENTICATED_PROBE_UNAVAILABLE`; the isolated custom-provider extension exposed no login UI and no real auth state was reused.
- [ ] U-006 auth-reuse follow-up: `BLOCKED: CLI_AUTH_NOT_CONSUMED_BY_EXTENSION`; opaque CLI auth status succeeded, but the isolated extension exposed no login UI and did not recognize the copied state. Evidence: `evidence/phase-0b-codex-u006-auth-reuse/`.
- [ ] U-006 opaque profile-clone follow-up: `BLOCKED: CLONED_AUTH_STATE_NOT_RECOGNIZED`; Astra dropdown visibility was observed, but no Codex Plus account was present and the Probe remained reconnecting. Evidence: `evidence/phase-0b-u006-profile-clone/`.
- [ ] U-006 primary-profile follow-up: `BLOCKED: NORMAL_RELAUNCH_FAILED`; temporary user-level config mutation was restored byte-for-byte, but no auth/Astra/wire checkpoint ran. Evidence: `evidence/phase-0b-u006-primary-profile/`.
- [x] Primary watchdog incident reconciled and real-config live mutation disabled. Evidence: `docs/reports/2026-09-19T0905Z-phase-0b-primary-watchdog-incident-audit.md`, `scripts/phase0b-primary-watchdog-disabled.test.ps1`.
- [x] Safe replacement U-006 shadow `CODEX_HOME` harness repaired with `real_config_mutation_capability=false` and `crash_requires_restore=false`; live probe remains separate and unrun. Evidence: `docs/reports/2026-09-19T1128Z-phase-0b-shadow-harness-repair.md`.
- [x] Shadow U-006 repair gate: 15/15 fixture/orchestration tests PASS; read-only current-config assessment is marker-free and TOML-valid, without claiming historical equivalence. Evidence: `evidence/phase-0b-u006-shadow-home/`.
- [x] Shadow U-006 lifecycle safety repair: SHADOW-009..013 fixed; 20/20 tests PASS; ACL, external launcher, fail-closed observation, retained-shadow timeout, and normal-reopen acceptance are covered. Evidence: `docs/reports/2026-09-19T1254Z-phase-0b-shadow-lifecycle-repair.md`.
- [x] Shadow watchdog stall repair: dynamic close-wait heartbeat and strict checkpoint input; 22/22 watchdog tests and 48/48 Node tests PASS. Evidence: `evidence/phase-0b-u006-shadow-home/stall-repair.json`.
- [ ] U-006 live shadow run: `BLOCKED: WATCHDOG_STALL_REQUIRES_FORCED_OWNED_PROCESS_CLEANUP`; cleanup PASS, real config unchanged, no accepted Astra wire evidence. Evidence: `evidence/phase-0b-u006-shadow-home/live-shadow-result-20260919T1335Z.json`.
- [ ] Audit primary-profile: PASS cũ không đủ bằng chứng; watchdog chặn trước mutation. Xem [audit](reports/2026-09-19T0814Z-phase-0b-primary-audit.md). Sửa và kiểm thử orchestration/rollback trước lần chạy live tiếp theo.
- [x] Sửa safety primary-profile: 25 Node tests, 12 assertion transaction, syntax/privacy/docs/diff PASS; chỉ là gate chuẩn bị, U-006 vẫn chờ live. [Report](reports/2026-09-19T0840Z-phase-0b-primary-repair.md).
