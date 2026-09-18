# Requirements

Keywords MUST, MUST NOT, SHOULD, SHOULD NOT, and MAY are normative.

## Functional requirements

| ID | Requirement | Priority |
|---|---|---|
| FR-001 | The system MUST run a Google-only CLIProxyAPI instance with its own auth directory and keys. | MUST |
| FR-002 | The system MUST run a Codex-only CLIProxyAPI instance with its own auth directory and keys. | MUST |
| FR-003 | Each service MUST bind to `127.0.0.1`; wildcard, LAN and IPv6-any bindings MUST be rejected. | MUST |
| FR-004 | Account login MUST be initiated through the pinned CLIProxyAPI Management API OAuth endpoints. | MUST |
| FR-005 | Poolbridge MUST NOT store or parse access/refresh tokens; upstream auth files remain owned by CLIProxyAPI. | MUST |
| FR-006 | OAuth flows MUST be serialized per provider instance and have timeout/cancel behavior. | MUST |
| FR-007 | OAuth completion MUST be followed by a credential-specific model listing and tiny capability probe. | MUST |
| FR-008 | A credential MUST be excluded from a pool if the required target model/capability is unavailable. | MUST |
| FR-009 | Users MUST be able to list, test, disable, enable and re-auth credentials by opaque identifier. | MUST |
| FR-010 | Antigravity integration MUST first pass a transparent passthrough gate. | MUST |
| FR-011 | The donor model MUST be selected from a captured native catalog by exact internal identifier. | MUST |
| FR-012 | Only generation requests whose exact model equals the configured donor ID MAY be rerouted. | MUST |
| FR-013 | Catalog/model-list requests and all non-donor traffic MUST pass through to the discovered native upstream. | MUST |
| FR-014 | Donor requests MUST preserve the native Gemini/Cloud Code inner schema; OpenAI Chat Completions translation is forbidden. | MUST |
| FR-015 | Codex MUST use a user-level custom provider with loopback `base_url`, `env_key`, and Responses wire protocol. | MUST |
| FR-016 | Codex MUST use canonical model slug `gpt-6-astra`; aliases are forbidden in V1. | MUST |
| FR-017 | The actual request arriving at the Codex instance MUST remain Responses-native and retain tool/reasoning/session fields. | MUST |
| FR-018 | If Astra is absent from the installed catalog, the system MAY use an exact-version catalog exported from the installed Codex build, after schema validation. | SHOULD |
| FR-019 | The system MUST verify model-picker behavior end to end; setting config alone is insufficient. | MUST |
| FR-020 | The system MUST preserve session affinity and prove failover when the bound credential is disabled or exhausted. | MUST |
| FR-021 | Setup MUST patch only explicitly owned keys and record the prior values. | MUST |
| FR-022 | Configuration writes MUST be atomic and recoverable after interruption. | MUST |
| FR-023 | Rollback MUST restore owned keys and stop services without deleting OAuth credentials. | MUST |
| FR-024 | Doctor MUST report versions, ports, process identity, bind addresses, config ownership, pool counts and probe status without exposing secrets. | MUST |
| FR-025 | Upstream upgrades MUST be opt-in, pinned, verified, staged and reversible. | MUST |

## Non-functional requirements

| ID | Requirement | Acceptance threshold |
|---|---|---|
| NFR-001 | Local confidentiality | No prompts, outputs, tool args, source content, raw tokens or raw session IDs in logs/evidence. |
| NFR-002 | Pool isolation | Opposite-provider credential selection is structurally impossible because auth roots/processes are separate. |
| NFR-003 | Recovery | Integration rollback completes without network access and restores the last owned snapshot. |
| NFR-004 | Reliability | 20-turn Antigravity and 30-turn Codex torture runs complete without protocol corruption. |
| NFR-005 | Startup | Supervisor detects occupied ports, stale PIDs, wrong executables and partial state before launching. |
| NFR-006 | Performance | Added local median TTFT overhead is measured; release gate is baseline + max(100 ms, 10%) unless owner accepts otherwise. |
| NFR-007 | Determinism | The same config and pinned binaries produce the same process topology and generated files. |
| NFR-008 | Maintainability | Upstream internals are not imported; integration uses documented HTTP/config boundaries. |
| NFR-009 | Auditability | Every roadmap gate has machine-readable result plus human summary and environment fingerprint. |
| NFR-010 | Least privilege | No admin rights, service installation, firewall change, CA install or hosts-file edit in default path. |

## UX requirements

- `poolbridge setup` MUST show what will change and require one final apply confirmation after probes.
- Errors MUST identify component, operation, stable code, safe remediation and rollback availability.
- Account labels shown to the user MAY use a locally stored nickname; committed evidence MUST use opaque IDs only.
- Four accounts per pool is displayed as a recommendation, never as a blocker.
- Setup MUST be restartable and idempotent. Rerunning it must not duplicate config blocks, credentials or services.

## Constraints and assumptions

- The target project repository may already contain files; agents must inspect before applying this package.
- Upstream schemas and model availability change. Version-dependent facts belong in `upstream.lock` and evidence, not hardcoded prose.
- Subscription use remains subject to provider terms. The software must not claim compliance or guaranteed account safety.
- Antigravity bridge feasibility and Codex picker/provider retention are unresolved until Phase 0.
