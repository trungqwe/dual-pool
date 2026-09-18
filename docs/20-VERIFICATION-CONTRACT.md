# Verification Contract

## Rule

A claim is accepted only if another agent can reproduce it from a committed test or a precisely documented target-machine procedure without access to prior chat.

## Evidence quality

| Level | Description | May satisfy |
|---|---|---|
| E0 | Assertion/no artifact | Nothing |
| E1 | Manual observation without environment/version | Exploration only |
| E2 | Command/output with environment fingerprint | Compatibility probe |
| E3 | Repeatable automated test with sanitized result | Component acceptance |
| E4 | Independent end-to-end scenario plus failure injection | Release gate |

Security, isolation, rollback and protocol-preservation requirements require E3 or E4.

## Result integrity

Every result records test ID, exact version hashes, config adapter version, environment fingerprint, start time, duration, status, assertions and sanitized artifact hashes. Results are immutable once referenced by a report; reruns create new result IDs.

## Required proof patterns

### Pool isolation

- Enumerate each instance's auth inventory using authenticated management calls.
- Assert provider types are exclusive.
- Send one request to each path and correlate only opaque auth IDs belonging to that instance.
- Attempt opposite-model/provider routing and assert failure without other instance traffic.

### Loopback-only

- Enumerate listeners by PID after all services start.
- Assert local address is exactly `127.0.0.1` for every owned listener.
- Attempt a non-loopback connection from available local interface when safe, or verify OS listener table plus config.

### Native protocol

- Capture at the local controlled ingress only.
- Record method/path/content type and schema key/type tree/event names.
- Compare to expected Responses or Gemini-native contract.
- Do not store values from user payloads.

### Affinity/failover

- Send multiple requests with a synthetic stable session marker.
- Assert the same opaque credential ID.
- Disable that exact credential through supported management endpoint.
- Assert next request selects a different eligible ID and subsequent requests remain stable.
- Re-enable credential and assert the established new binding is not unexpectedly stolen.

### Configuration rollback

- Start from fixture plus randomized unrelated keys/comments where parser supports preservation.
- Apply, re-apply, simulate crash points, restore and hash/semantic-compare.
- Modify an unrelated/owned value externally after apply and assert conflict rather than overwrite.

### Secret non-disclosure

- Inject unique sentinel strings for every secret class.
- Exercise success and all error/log/report/diagnostic paths.
- Scan stdout, stderr, logs, evidence, state, reports, ZIP and Git diff.
- Any match is failure.

## Test status rules

- `PASS`: all assertions ran and passed.
- `FAIL`: one or more assertions failed.
- `BLOCKED`: prerequisite outside the run is unavailable; exact blocker recorded.
- `SKIPPED`: deliberately not run. Mandatory skipped tests block promotion.

Retries do not erase prior failures. The report must show flaky/retried behavior and the final result; repeated failure then pass triggers investigation.

## Waivers

Only the owner may accept a requirement/test waiver. Record scope, rationale, risk, expiration/review trigger and compensating control. No waiver can permit credential leakage, non-loopback management, cross-provider routing or destructive config overwrite.
