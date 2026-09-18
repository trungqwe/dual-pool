# Master Checklist

This is the authoritative progress ledger. Check a box only with an adjacent evidence/report reference in the implementing commit.

## Governance

- [x] Repository remote and default branch verified; no protection workflow is configured yet. Evidence: `docs/reports/2026-09-18T1919Z-phase-0-github-bootstrap.md`.
- [x] Existing user changes inventoried and preserved. Evidence: initial empty-remote inventory in `docs/reports/2026-09-18T1919Z-phase-0-github-bootstrap.md`.
- [x] All agents have read required documents. Evidence: `docs/reports/2026-09-18T1919Z-phase-0-github-bootstrap.md`.
- [x] Current phase and scope recorded in handoff. Evidence: `docs/18-HANDOFF.md`.
- [ ] Requirements/ADRs/tests updated for any design change.

## Phase 0

- [ ] Windows/architecture/version inventory captured.
- [ ] Codex CLI/extension effective config path proven.
- [ ] Matching Codex catalog captured; Astra descriptor validated.
- [ ] Codex custom provider sends Responses to loopback.
- [ ] Codex picker retains desired provider route.
- [ ] Antigravity setting path/key proven reversibly.
- [ ] Native route and schema inventory captured safely.
- [ ] Transparent passthrough proven.
- [ ] Donor native envelope round trip proven.
- [ ] Candidate CLIProxyAPI version/schema/endpoints verified.
- [ ] Port and callback collision inventory complete.
- [ ] U-001 through U-008 resolved or explicitly BLOCKED.

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
- [ ] `docs/18-HANDOFF.md` updated.
- [ ] Run report added under `docs/reports/`.
- [ ] `git diff --check`, tests and docs links pass.
- [ ] Commit contains no unrelated files.
- [ ] Push succeeds without force.
- [ ] Commit SHA and compare/PR URL recorded in report.
