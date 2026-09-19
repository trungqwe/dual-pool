# Decisions and Evidence Ledger

## Evidence classes

- `DOC`: current official/upstream documentation supports the claim.
- `SOURCE`: pinned source/release artifact supports the claim.
- `PROBE`: observed on the target machine with sanitized evidence.
- `TEST`: automated repeatable verification.
- `ASSUMPTION`: not yet proven; cannot support a release claim.

Retrieval baseline for web references in this document: 2026-09-18 UTC. URLs must be rechecked when pinning versions.

## Accepted architecture decisions

### ADR-001 — Do not fork CLIProxyAPI

Status: ACCEPTED.

Use an upstream binary behind documented HTTP/config interfaces. A patch is allowed only when all are proven: reproduced on pinned current release; minimal reproducer exists; config cannot solve it; provider-specific endpoints cannot solve it; Management API cannot solve it; poolbridge cannot solve it; upstream fix is unavailable. The preferred exception is a small rebasing patch file, not a divergent repository.

### ADR-002 — Two isolated CLIProxyAPI processes

Status: ACCEPTED.

Run the same pinned executable twice with different ports, data roots, auth directories, API keys and management keys. This changes the earlier one-process concept in favor of structural provider isolation. Under Poolbridge-managed state, each process receives a disjoint auth root and endpoint. Unexpected opposite-provider auth material is an integrity violation that must be detected and fail closed before provider traffic is allowed.

### ADR-003 — Windows-first CLI, no GUI

Status: ACCEPTED.

V1 produces one Go companion executable and uses user-scoped processes/startup integration. No Electron, Node or Python runtime is required for the product.

### ADR-004 — Native protocols end to end

Status: ACCEPTED.

Antigravity donor traffic remains Gemini/Cloud-Code native. Codex traffic remains Responses native. Protocol translation to Chat Completions is not an acceptable fallback.

### ADR-005 — Donor replacement, not model injection

Status: ACCEPTED.

Antigravity reuses one exact model slot chosen by the user. V1 does not mutate the native catalog or create a new picker item.

### ADR-006 — Canonical Codex model identity

Status: ACCEPTED.

Use `gpt-6-astra` and the installed Codex build's descriptor. Do not invent aliases that might lose model metadata.

### ADR-007 — Reversible key ownership

Status: ACCEPTED.

Poolbridge owns only named configuration keys. It records previous values, writes atomically and restores only its own changes.

## Verified upstream capabilities

| Claim | Class | Evidence | Consequence |
|---|---|---|---|
| CLIProxyAPI supports OpenAI Responses, Gemini-compatible clients and multi-account load balancing. | DOC | `https://github.com/router-for-me/CLIProxyAPI` | Suitable upstream engine, subject to pinned tests. |
| Management API has `/codex-auth-url`, `/antigravity-auth-url` and `/get-auth-status`. | DOC | `https://help.router-for.me/management/api` | Poolbridge can orchestrate OAuth without implementing it. |
| Management API can list credential-specific models and enable/disable auth records. | DOC | Same Management API reference. | Eligibility and failure-injection paths exist. |
| CLIProxyAPI supports session affinity, TTL and subagent inheritance fields in current example config. | DOC | `CLIProxyAPI/config.example.yaml` | Pin and schema-check before emitting fields. |
| Management API can be localhost-only and still requires a secret key. | DOC | Same example config. | Both instances must use non-empty independent management keys. |
| Codex supports custom `model_provider`, `base_url`, `env_key`, and Responses wire API in user-level config. | DOC | `https://learn.chatgpt.com/docs/config-file/config-reference` | No extension bundle patch is needed for transport configuration. |
| Codex supports `model_catalog_json`. | DOC | Same config reference. | Exact-version fallback may be possible. |
| Current upstream Codex source contains `gpt-6-astra` with list visibility and specialized tool/session metadata. | SOURCE | `openai/codex/codex-rs/models-manager/models.json` | Preserve canonical slug and matching descriptor; target install still requires probe. |

## Unresolved compatibility questions

| ID | Unknown | Phase 0A status/evidence | Required proof | If false |
|---|---|---|---|---|
| U-001 | Exact Antigravity setting path/key and whether loopback HTTP Cloud Code URL is honored. | **BLOCKED** — required user-assisted reload/discovery action was not available; no mutation was made. | User-assisted Phase 0B settings capture plus loopback request trace. | Stop Antigravity integration; no binary patch fallback. |
| U-002 | Native upstream URL and full endpoint set needed for transparent passthrough. | **BLOCKED** — U-001 user-assisted redirect gate did not run. | Sanitized route inventory from a real Phase 0B session. | Stop; do not guess upstream routes. |
| U-003 | Cloud Code request/response envelope permits lossless donor unwrap/rewrap. | **BLOCKED** — U-001 and U-002 did not pass. | Phase 0B golden fixtures and byte/semantic diff. | Stop donor override. |
| U-004 | Provider-specific Gemini endpoint and exact target model supported by pinned Google instance. | **BLOCKED** — this run is not authorized for credential-specific probing. Tagged routes do not prove provider support. | `/v1/models`, credential model endpoint and tiny generation in a separately authorized run. | Select only observed route/model or block. |
| U-005 | Installed Codex version contains Astra and extension shares the intended config layer. | **PARTIAL / UNKNOWN** — parallel extension probe reached `POST /v1/responses` on loopback with `application/json` and Authorization present, but the 58,422-byte body was not parseable without persisting raw content; model and sentinel ingress remain unproven. [Two-request result](../evidence/phase-0b-codex-extension/codex-extension-two-request-20260919T031615744Z.json). | Capture a sanitized structural parse from the exact extension request, or record an explicit unsupported payload format. | Do not claim config-layer proof or add a catalog fallback. |
| U-006 | Model picker retains `model_provider=dualpool_codex` when choosing Astra. | **BLOCKED** — baseline request did not reach an accepted parsed capture, so the picker step was not started. Astra was previously observed not visible in the picker. | First prove baseline request parsing, then select Astra and capture request two on the same loopback provider. | Use a supported profile/default workflow or block dropdown acceptance; do not patch UI. |
| U-007 | Installed Codex exposes a safe bundled-catalog export command. | **PROBED** — CLI `0.154.0` exposes `codex debug models --bundled`; Astra metadata was whitelisted into `codex-catalog.json`. | Re-run after each Codex version change. | Extract only from matching pinned artifact/source or omit fallback. |
| U-008 | CLIProxyAPI health and provider-specific endpoint shapes for pinned release. | **PARTIAL / UNKNOWN** — `v7.3.7` integrity, config fields, registered routes, loopback bind and key enforcement were probed in `cliproxy-schema.json`; provider-specific response shapes and dedicated health semantics remain unproven. | Credentialed provider contract probes in a later authorized run. | Adapt through documented version surface or pin another tested release. |

## Decision review rule

Any ADR change must include: motivation, alternatives, security impact, migration, rollback, changed requirements/tests, and owner approval. An agent may propose but must not silently reverse an accepted ADR.
## Phase 0B parser-repair correction — 2026-09-19

The historical `EXTENSION_PAYLOAD_NOT_PARSEABLE` label is **NOT PROVEN**: the old recorder used one broad catch around decode, JSON parse, model extraction, and shape sanitization. The v2 recorder separates those stages. The authoritative v2 result is `BASELINE_MODEL_MISMATCH`: transport, auth, identity encoding, strict UTF-8, JSON parse, prompt sentinel, and sanitizer passed, while the observed request was another `gpt-5.6-*` family member than the isolated baseline. U-005 remains **PARTIAL / UNKNOWN**. See [audit correction](../evidence/phase-0b-codex-extension-v2/audit-correction.json) and [v2 result](../evidence/phase-0b-codex-extension-v2/extension-baseline-20260919T040521877Z.json).
## Phase 0B authenticated-picker correction — 2026-09-19

The earlier unauthenticated `ASTRA_NOT_VISIBLE` observation is **NON_DIAGNOSTIC_FOR_ASTRA_ENTITLEMENT**. Existing temporary-`CODEX_HOME` and synthetic-provider evidence independently proves the extension consumed the intended custom provider layer, so U-005 is now **PROBED**, qualified to Windows 10, `openai.chatgpt 26.5730.61309`, and the observed installed Antigravity/Codex runtime. U-006 remains **BLOCKED: AUTHENTICATED_PROBE_UNAVAILABLE** because the disposable custom-provider extension instance exposed no Codex login UI; no authentication material was inspected or reused. Evidence: [U-005 correction](../evidence/phase-0b-codex-extension-u006/u005-audit-correction.json), [U-006 result](../evidence/phase-0b-codex-extension-u006/u006-result.json).

## Phase 0B U-006 auth-reuse boundary — 2026-09-19

An owner-authorized opaque copy of the existing Codex auth cache authenticated a temporary `CODEX_HOME` as `CHATGPT`, but the isolated Codex extension did not expose a login UI and did not consume that CLI auth state. The owner confirmed `AUTH_NOT_RECOGNIZED`; U-006 is therefore **BLOCKED: CLI_AUTH_NOT_CONSUMED_BY_EXTENSION**. The picker and Astra wire stages were not run. Temporary roots were deleted and no auth contents were inspected, logged, hashed, committed, or retained. See [run report](../docs/reports/2026-09-19T0515Z-phase-0b-u006-auth-reuse.md), [bootstrap](../evidence/phase-0b-codex-u006-auth-reuse/auth-bootstrap-20260919T053402170Z.json), and [result](../evidence/phase-0b-codex-u006-auth-reuse/u006-20260919T053402492Z.json).

The subsequent opaque full Antigravity profile clone reached the Probe and displayed GPT-6 Astra, but the owner confirmed that Codex settings had no active Codex Plus account and the Probe remained in reconnecting state. The dropdown item is not sufficient evidence of authenticated entitlement. U-006 remains **BLOCKED: CLONED_AUTH_STATE_NOT_RECOGNIZED**; the one observed `gpt-5.6-luna` request is explicitly not accepted as an Astra result because the owner confirmed Astra had not actually been selected for that request. See [profile-clone report](../docs/reports/2026-09-19T0553Z-phase-0b-u006-profile-clone.md) and [classification correction](../evidence/phase-0b-u006-profile-clone/profile-clone-audit-correction.json).
