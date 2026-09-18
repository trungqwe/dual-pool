# Antigravity Integration

## Objective

When the user selects one exact native donor model in Antigravity, generation is executed by the isolated Google pool. Every other model and every non-generation request retains native behavior.

## Preconditions

- U-001 through U-004 in the evidence ledger are PROBED or VERIFIED.
- Original settings value and native upstream target are recorded.
- Passthrough-only bridge passes the full baseline suite.
- At least one Google credential is eligible for the observed target model.
- Emergency rollback has been tested while the bridge is stopped.

## Route decision

```text
if route is not on the proven generation allowlist:
    passthrough
else if request schema/version is unknown:
    fail closed with AG_SCHEMA_UNSUPPORTED
else if exact request model_id != configured donor_model_id:
    passthrough
else if Google instance is unhealthy:
    return stable POOL_UNAVAILABLE; never silently use another provider
else:
    unwrap only the proven outer envelope
    forward the native inner Gemini request
    stream/rewrap the proven response envelope
```

Matching must compare the decoded exact identifier after schema validation. Do not use display names, substrings, regular expressions, case folding, aliases or fuzzy matching.

## Donor selection

`poolbridge antigravity configure`:

1. enter observe/passthrough mode;
2. wait for one native model catalog response;
3. extract model records using the probed schema;
4. display stable ID plus safe display label;
5. user selects one existing entry;
6. persist catalog fingerprint, Antigravity version, schema adapter version and exact donor ID;
7. require a final route dry run before activation.

If the catalog fingerprint changes after an Antigravity update, automatically disable override and require revalidation. Do not continue using a stale donor identifier.

## Native upstream passthrough

Passthrough must preserve:

- HTTP method, path/query and status code;
- required headers excluding hop-by-hop corrections;
- streaming chunk order and cancellation propagation;
- timeouts and backpressure;
- request/response bodies byte-for-byte where transport framing permits, otherwise semantically equivalent with a recorded normalization list.

The bridge must set explicit connect/header/idle timeouts and prevent open-proxy behavior. It forwards only to the discovered allowlisted native origin and Google instance; request-controlled hosts are rejected.

## Donor transformation contract

Create golden fixtures from synthetic, sanitized structures for:

- simple text generation;
- system/developer instruction fields;
- multi-turn content;
- tool declarations and tool results;
- image/multimodal parts if observed and in scope;
- streaming deltas;
- finish/usage metadata;
- provider errors;
- cancellation before and after first byte.

For each field, document `pass`, `unwrap`, `rewrap`, `derive`, or `drop`. A required field may not be `drop`. Unknown fields should be preserved when safe; a schema-version mismatch blocks donor routing.

## Failure behavior

| Failure | Non-donor | Donor |
|---|---|---|
| Google CLIProxyAPI down | Native passthrough | 503 `AG_POOL_UNAVAILABLE` |
| Native upstream down | Native upstream error | Google route may work only if request is fully classified donor |
| Unknown route/schema | Passthrough if no mutation needed | Fail closed |
| Stream breaks before first byte | Native behavior | One bounded upstream retry if request is safe/idempotent and CPA owns retry policy |
| Stream breaks after first byte | Propagate termination | Never replay automatically |
| Bridge cannot bind | Antigravity may fail until rollback | `emergency-rollback` restores original setting |

## Commands

```text
poolbridge antigravity probe
poolbridge antigravity configure
poolbridge antigravity enable
poolbridge antigravity disable
poolbridge antigravity status
poolbridge antigravity test --suite smoke|full
poolbridge antigravity rollback
poolbridge antigravity emergency-rollback
```

`disable` changes the bridge to passthrough without altering account credentials. `rollback` restores the previous IDE setting. `emergency-rollback` must work when all child processes are stopped.

## Acceptance

- Catalog behavior matches direct baseline.
- Every non-donor model passes functional and metadata diff tests.
- Exact donor selection reaches only the Google port.
- Provider/account logs show only Google instance auth IDs.
- Tools, files, terminal, stream, cancel, resume and 20+ turn flow pass.
- Affinity remains stable, then fails over when the bound auth is disabled.
- Restart and rollback pass.
