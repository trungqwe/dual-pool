# Codex Integration

## Objective

The existing Codex extension/CLI presents and runs canonical `gpt-6-astra` while the request uses the native Codex harness and reaches the isolated Codex CLIProxyAPI instance.

## Supported configuration approach

At user level, generate/patch the equivalent of:

```toml
model = "gpt-6-astra"
model_provider = "dualpool_codex"

[model_providers.dualpool_codex]
name = "Dual Pool Codex"
base_url = "http://127.0.0.1:8317/v1"
wire_api = "responses"
env_key = "DUALPOOL_CODEX_KEY"
```

The exact config path and supported keys must come from the installed version probe. Project-local config is not used for provider routing because current official documentation treats provider keys as user-level/machine-local concerns.

The secret is supplied to the Codex process environment through a documented launch/startup mechanism. If the extension host cannot safely receive that environment without global plaintext persistence, the implementation must evaluate the current command-backed auth option or stop for an ADR; hardcoding a key in `config.toml` is not accepted silently.

## Catalog behavior

Preferred path:

1. Use the installed Codex build's bundled/remote-effective catalog.
2. Confirm canonical Astra descriptor is visible and compatible.
3. Do not write `model_catalog_json`.

Fallback path, only when required:

1. Export/extract the catalog from the exact installed build using a supported mechanism proven in Phase 0.
2. Validate its schema using that same build.
3. Verify Astra's complete descriptor, not only slug/display name.
4. Write an immutable generated copy under the Dual Pool data root.
5. Patch `model_catalog_json` to that path.
6. Restart the relevant Codex process and rerun picker/transport tests.

Never download `main/models.json` for a different installed version. A catalog can replace normal behavior rather than merge; preserve the exact matching full catalog.

## Picker compatibility gate

Configuration support does not prove UI support. The mandatory test is:

1. Start a local metadata-only Responses recorder or the authenticated Codex instance.
2. Launch/restart the Codex extension through the intended user workflow.
3. Open the model selector and select `GPT-6 Astra`.
4. Start a new thread and send a synthetic prompt.
5. Prove the request reaches `127.0.0.1:8317`, uses `/v1/responses` or supported Responses WebSocket, includes model `gpt-6-astra`, and authenticates with the Dual Pool key.
6. Confirm no request went to the Google port.

If picker selection resets provider routing, supported remedies are limited to user-level profiles/defaults or current documented settings. Editing extension code/bundles is forbidden. If no supported workflow meets the stated dropdown goal, mark the feature BLOCKED and present the closest supported workflow to the owner.

## Harness preservation contract

Sanitized request-shape evidence must prove preservation of:

- Responses input item structure;
- developer/system instructions as emitted by Codex;
- reasoning configuration and summaries where applicable;
- tool definitions and parallel tool capability;
- `apply_patch` tool representation;
- shell/unified execution representation;
- session/conversation/cache identifiers;
- compaction-related metadata;
- image inputs if included in the target scenario;
- stream event framing;
- subagent/parent identifiers where available.

Do not persist bodies. Record only safe key paths, value types, model name, route, event names and hashes needed for comparisons.

## Transport

Start with HTTP/SSE Responses. Enable WebSockets only if both the installed Codex descriptor/client and pinned CLIProxyAPI explicitly support it and the benchmark shows correctness. Test transports separately; do not infer one from the other.

Codex retry settings and CLIProxyAPI retries must be coordinated to prevent multiplicative retries. After any response bytes/tool side effects, automatic replay is prohibited unless the protocol supplies a safe resume/idempotency mechanism.

## Account eligibility

For each Codex credential:

1. present in the correct instance inventory;
2. not disabled;
3. credential-specific model list includes exact Astra slug;
4. tiny Responses request succeeds;
5. streaming event sequence parses;
6. tool-capable synthetic request succeeds or reaches the expected tool event;
7. no sensitive output is stored.

Only passing credentials participate in the pool.

## Acceptance

- Astra appears in the intended selector or the owner explicitly accepts a documented supported alternative.
- Selection preserves `dualpool_codex` transport.
- All traffic reaches only the Codex instance.
- 30+ turns, tools, multi-file edits, shell, apply_patch, compaction, subagents, cancel/resume and restart pass.
- One conversation remains on one credential until injected disable/quota failure causes a controlled switch.
- Performance meets NFR-006 or has an accepted waiver with measurements.
