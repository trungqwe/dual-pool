# Observability and Error Contract

## Principles

Observability exists to prove routing, lifecycle and reliability without recording user content. Logs are structured, local, bounded and off-by-default at verbose levels.

## Allowed event fields

```text
timestamp, level, component, event, operation_id, instance_id,
provider, route_template, method, model_id, credential_opaque_id,
session_fingerprint, status_code, duration_ms, retry_count,
stream_started, bytes_class, error_code, version, config_hash_prefix
```

`route_template` must not include arbitrary query values. `bytes_class` is a coarse bucket, not payload content. Account IDs and session values use keyed fingerprints so committed evidence cannot be correlated externally.

## Forbidden fields

OAuth tokens, refresh tokens, API/management keys, Authorization/Cookie headers, raw OAuth state, email addresses, prompt bodies, response bodies, tool arguments/results, file content/path from user prompts, source code, raw session/conversation IDs and full command lines containing secrets.

## Event families

- `process.*`: start, ready, exit, stale_pid, wrong_identity.
- `health.*`: level checks and transitions.
- `oauth.*`: begin, browser_opened, wait, complete, cancel, timeout; state omitted.
- `account.*`: discovered, eligible, ineligible, enabled, disabled.
- `route.*`: passthrough, donor, rejected, provider_instance.
- `affinity.*`: bound, reused, failed_over; opaque IDs only.
- `config.*`: backup, conflict, applied, restored; path may be product-relative.
- `update.*`: checked, staged, verified, promoted, rolled_back.
- `test.*`: start, pass, fail, skip with evidence ID.

## Stable errors

| Code | Meaning | User action |
|---|---|---|
| `PORT_IN_USE` | Required port belongs to another process | Choose port or stop it manually |
| `PROCESS_IDENTITY_MISMATCH` | PID/listener is not expected instance | Run doctor; no process is killed |
| `UPSTREAM_VERSION_MISMATCH` | Binary differs from lock | Install/pin supported version |
| `MANAGEMENT_UNAVAILABLE` | Authenticated management health failed | Inspect instance log/status |
| `OAUTH_TIMEOUT` | Login did not finish by deadline | Retry/re-auth |
| `ACCOUNT_INELIGIBLE` | Required model/capability probe failed | Use another entitled account |
| `CONFIG_CONFLICT` | User/external edit occurred after snapshot | Review diff and re-apply |
| `AG_SCHEMA_UNSUPPORTED` | Donor request schema is not recognized | Disable override/reprobe |
| `AG_POOL_UNAVAILABLE` | Google instance has no usable route | Test/re-auth accounts |
| `CODEX_PICKER_ROUTE_MISMATCH` | Picker selection did not use custom provider | Use supported workaround or block |
| `CATALOG_VERSION_MISMATCH` | Catalog is not from installed Codex | Regenerate exact-version catalog |
| `POOL_ISOLATION_VIOLATION` | Opposite provider appeared/was selected | Stop all services; release blocker |
| `ROLLBACK_CONFLICT` | Owned key changed after apply | Manual review; do not overwrite |
| `SECRET_LEAK_DETECTED` | Scanner found forbidden material | Quarantine artifact and rotate exposed secret |

Errors return non-zero exit codes by category: usage 2, compatibility 10, config 20, auth 30, service 40, security 50, test 60. Exact mapping is versioned in code and docs.

## Doctor output

`poolbridge doctor --json` is stable machine-readable output. It reports:

- product/build and state schema;
- detected application/upstream versions and hashes;
- process identity, ports and bind addresses;
- config paths and ownership state;
- pool counts by eligible/ineligible/disabled;
- last successful probes and evidence IDs;
- catalog/donor fingerprints;
- warnings and stable error codes.

It never performs a real model request unless `--active` is supplied. `doctor bundle` creates an allowlisted diagnostic archive and runs secret scanning before writing it.

## Retention

Use size/time rotation with conservative defaults. Logs are deleted independently for each instance. Evidence required for a release is summarized/sanitized and committed; verbose local logs are not committed.
