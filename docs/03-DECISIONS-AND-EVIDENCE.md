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

Run the same pinned executable twice with different ports, data roots, auth directories, API keys and management keys. This changes the earlier one-process concept in favor of structural provider isolation. The small process/memory cost is accepted because it removes cross-provider credential selection from the failure domain.

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

| ID | Unknown | Required proof | If false |
|---|---|---|---|
| U-001 | Exact Antigravity setting path/key and whether loopback HTTP Cloud Code URL is honored. | Before/after settings capture plus loopback request trace. | Stop Antigravity integration; no binary patch fallback. |
| U-002 | Native upstream URL and full endpoint set needed for transparent passthrough. | Sanitized route inventory from a real session. | Stop; do not guess upstream routes. |
| U-003 | Cloud Code request/response envelope permits lossless donor unwrap/rewrap. | Golden fixtures and byte/semantic diff. | Stop donor override. |
| U-004 | Provider-specific Gemini endpoint and exact target model supported by pinned Google instance. | `/v1/models`, credential model endpoint and tiny generation. | Select only observed route/model or block. |
| U-005 | Installed Codex version contains Astra and extension shares the intended config layer. | Version, bundled catalog and outbound request capture. | Upgrade supported client or exact-version fallback; no main-branch catalog. |
| U-006 | Model picker retains `model_provider=dualpool_codex` when choosing Astra. | UI action plus request arrival on Codex loopback port. | Use a supported profile/default workflow or block dropdown acceptance; do not patch UI. |
| U-007 | Installed Codex exposes a safe bundled-catalog export command. | Help output and command result for exact installed version. | Extract only from matching pinned artifact/source or omit fallback. |
| U-008 | CLIProxyAPI health and provider-specific endpoint shapes for pinned release. | Generated compatibility manifest and contract tests. | Adapt through documented version surface or pin another tested release. |

## Decision review rule

Any ADR change must include: motivation, alternatives, security impact, migration, rollback, changed requirements/tests, and owner approval. An agent may propose but must not silently reverse an accepted ADR.
