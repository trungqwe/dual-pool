# CLIProxyAPI Integration Contract

## Pinned downloader/stager boundary

The Phase 2 staging component consumes the exact validated bytes of `upstream.lock`. It derives the GitHub release URL, artifact name, archive digest and executable digest exclusively from that document. V1 supports only `windows_amd64`; it does not resolve a latest release or accept a caller URL override.

The download client permits HTTPS redirects only between the exact hosts observed for the pinned asset: `github.com` and `release-assets.githubusercontent.com`. It sends no authentication metadata, applies a two-minute request deadline, limits redirects to three and caps the archive at 64 MiB. Archive SHA-256 is verified before ZIP inspection.

The stager rejects unsafe ZIP paths, symlinks, device names and resource-limit violations. It selects the unique regular entry matching the locked executable SHA-256, rehashes the extracted bytes, verifies PE AMD64 and then runs the source-proven `-h` identity probe with bounded output, time and environment. No listener is started.

Completed stages are immutable identities under an injected existing root and bind to the SHA-256 of the exact lock bytes through `stage-manifest.json`. Existing matching stages are reused; incomplete or mismatched stages fail with a conflict and are not overwritten. This slice uses disposable TEMP roots only.

## Boundary

Poolbridge integrates only through:

- the pinned executable and documented command-line/config surface;
- loopback client endpoints;
- authenticated loopback Management API;
- process lifecycle and exit status;
- upstream-published release/checksum metadata.

It must not import CLIProxyAPI internal packages or depend on unexported storage layout.

## Instance specification

| Property | Codex instance | Google instance |
|---|---|---|
| Logical ID | `codex` | `google` |
| Default port | 8317 | 8318 |
| Auth root | `instances\codex\auth` | `instances\google\auth` |
| Allowed credential type | Codex | Antigravity/Google type proven by model probe |
| Client protocol | Responses | Gemini-native provider route |
| Client | Codex extension/CLI | Antigravity bridge |
| Secrets | unique | unique |
| Logs | separate | separate |

Generated config must use only fields validated against the pinned version. A template schema version is stored with `upstream.lock` and startup refuses an incompatible binary/config combination.

## Upstream lock schema

`upstream.lock` SHOULD be deterministic TOML or JSON and include:

```text
schema_version
product = CLIProxyAPI
version
commit_or_tag
download_url_by_platform
sha256_by_platform
retrieved_at
release_metadata_url
config_adapter_version
verified_capabilities[]
```

Never use `latest` at runtime. Never accept a checksum fetched from the same untrusted byte stream as the artifact unless it is independently authenticated by the release channel.

## Secret model

Generate four independent 256-bit random secrets:

- Codex client API key.
- Codex management key.
- Google client API key.
- Google management key.

At rest they must be protected with a user-bound Windows mechanism. Config generation must avoid persisting plaintext where CLIProxyAPI supports environment injection; if the upstream requires config persistence, restrict ACLs to the current user and document this residual risk. `doctor --verbose` still prints only fingerprints such as `sha256:abcd1234`, never values.

## OAuth orchestration

Pseudocode:

```text
acquire provider-oauth lock
assert expected instance identity and localhost bind
GET provider auth URL with management authentication
validate returned URL scheme/host against upstream expectations
open default browser
poll status using exact returned state with deadline/backoff
on cancel/timeout call supported cancellation endpoint if present
refresh auth inventory
identify one newly created/changed auth record
run credential-specific model and capability probes
mark eligible or ineligible with reason
release lock
```

The state is memory-only where possible. Poolbridge stores no tokens and never downloads auth files. If identifying the new credential is ambiguous, stop and ask the user instead of associating the wrong record.

## Account state

Non-secret local metadata:

```json
{
  "opaque_id": "provider runtime/file id",
  "pool": "codex|google",
  "nickname": "optional local-only label",
  "enabled": true,
  "eligibility": "unknown|eligible|ineligible",
  "reason_code": "",
  "model_ids": ["sanitized observed IDs"],
  "last_probe_at": "RFC3339",
  "last_probe_version": "pinned version"
}
```

Poolbridge must reconcile this cache with the Management API; it is not authoritative for actual credential existence or enabled state.

## Health levels

- L0 process: expected executable and instance args are running.
- L1 network: correct loopback listener and no wildcard listener.
- L2 management: authenticated endpoint responds with expected version/schema.
- L3 inventory: auth manager available and inventory parseable.
- L4 provider: eligible credential can perform a tiny real request.
- L5 scenario: streaming/tool/affinity behavior passes.

`status` may use L0-L3. `account test` uses L4. Release verification uses L5. Health checks must rate-limit real provider calls.

## Routing and affinity

When supported by the pinned schema:

```yaml
routing:
  strategy: "round-robin"
  session-affinity: true
  session-affinity-ttl: "1h"
  session-affinity-subagents: true
```

These values are starting hypotheses. Benchmarks may justify another documented strategy, but affinity remains required. Prove stable credential selection using safe auth IDs in logs, then disable the bound credential and prove controlled rebinding.

## Updates

`poolbridge update check` is read-only. `poolbridge update cliproxy --version X` performs:

1. fetch metadata and artifact to a staging directory;
2. verify allowlisted origin, size limit and SHA-256;
3. inspect executable identity/version;
4. start disposable instances with generated test configs;
5. run management, Responses and Gemini contract smoke tests without copying production auth;
6. request explicit promotion;
7. stop production instances, atomically switch version pointer, restart and smoke test;
8. automatically restore previous pointer if startup/health fails.

OAuth credentials are never migrated by the updater; both versions point to the same instance auth root only after compatibility is proven.
