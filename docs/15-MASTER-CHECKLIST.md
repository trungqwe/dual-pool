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

- [ ] Codex custom provider sends Responses to loopback.
- [ ] Codex picker retains desired provider route.
- [ ] Antigravity setting path/key and loopback behavior proven reversibly.
- [ ] Native route and schema inventory captured safely.
- [ ] Transparent passthrough proven.
- [ ] Donor native envelope round trip proven.
- [ ] U-001 through U-008 resolved or explicitly BLOCKED for Phase 0 exit.

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

- [ ] `upstream.lock` populated with verified artifact.
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
