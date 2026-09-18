# Test Strategy

## Layers

### Unit

- Exact route/model predicate tables.
- Envelope adapters with golden synthetic fixtures.
- TOML/JSON/YAML owned-key patching and rollback.
- State schema migration and crash journal recovery.
- Redaction against sentinel secrets.
- Process identity/PID reuse checks.
- URL/origin allowlist validation.
- Version/capability adapter selection.

Target: high coverage on security/transaction/routing packages; project-wide percentage is secondary to branch coverage of invariants.

### Contract

Run mock servers for:

- CLIProxyAPI Management API schemas and error cases.
- Responses SSE/WebSocket event sequences.
- Gemini/Cloud Code request/stream/error envelopes.
- OAuth wait/success/error/cancel state machine.
- Native passthrough including chunking, backpressure and cancellation.

Every external response parser must have malformed, oversized, missing-field, unknown-field and timeout tests.

### Integration

- Launch two disposable fake/upstream CLIProxyAPI instances with distinct roots.
- Verify ports, keys, config and account inventories never cross.
- Patch temporary Antigravity/Codex config fixtures and prove unrelated content preservation.
- Kill/restart child processes and recover stale journals/PIDs.
- Stage update and simulate checksum/startup failure.

### Live compatibility

Requires explicit target machine and accounts. It must be opt-in, minimize real calls and produce sanitized evidence. Live tests never run in generic CI.

## Core scenario suites

### Google/Antigravity full suite

1. Direct baseline for every native model.
2. Passthrough baseline for every native model.
3. Catalog refresh parity.
4. Donor simple/streaming generation.
5. Non-donor route isolation.
6. Tool call and tool result loop.
7. File read/edit and terminal actions as driven by Antigravity.
8. Cancellation before and after first byte.
9. 20+ turn session and resume.
10. Credential stickiness, disable-bound-account failover.
11. Google instance restart and supervisor recovery.
12. Bridge crash plus emergency rollback.

### Codex full suite

1. Picker visibility and provider-retention proof.
2. Responses HTTP/SSE; WebSocket separately when enabled.
3. Reasoning level supported cases.
4. `apply_patch`, shell, multi-file edit and test loop.
5. Parallel tools if descriptor supports them.
6. Compaction at a practical threshold.
7. Parent/subagent affinity.
8. Cancellation/resume.
9. 30+ turn session.
10. Disable/quota failure of bound credential and rebind.
11. Extension host restart, Windows user restart and state recovery.
12. Confirm zero traffic to Google port.

## Baseline/differential tests

For Antigravity passthrough, compare direct and bridged runs by behavior, safe headers/status, event type sequence and normalized payload hash from synthetic prompts. Timing is evaluated statistically, not byte-identically.

For Codex, compare direct supported transport versus CPA path for success, event/tool semantics, TTFT, total latency, retry count, cache indicators when safely available and error behavior.

## Performance protocol

- Warm-up runs are excluded.
- At least 10 matched samples per path for smoke benchmark; 30 for release candidate.
- Use the same synthetic task/model/reasoning and stable network window.
- Report median, p90 and failure count for TTFT and total latency.
- Do not claim model quality equivalence from latency tests.
- Gate NFR-006 using median added local overhead, with outliers documented.

## Failure injection

- Occupied ports and stale/wrong PIDs.
- Child crash during startup and mid-stream.
- Management key/client key swapped.
- Credential disabled while affinity-bound.
- 401, 403, 429, 5xx, timeouts and malformed event frames.
- Disk full/permission denied during backup/temp/replace.
- Concurrent user config edit.
- Corrupt state/ownership file and interrupted migration.
- Catalog mismatch after Codex upgrade.
- Donor catalog fingerprint change after Antigravity upgrade.
- Checksum mismatch and staged binary health failure.

## Evidence record

Each test result contains:

```json
{
  "test_id": "CX-E2E-001",
  "status": "pass|fail|blocked|skipped",
  "started_at": "RFC3339",
  "duration_ms": 0,
  "environment_id": "hash",
  "versions": {},
  "inputs_fixture": "fixture id/hash",
  "assertions": [],
  "artifacts": [],
  "error_code": null
}
```

Skipped mandatory tests fail the release gate unless the owner explicitly accepts and records a waiver.
