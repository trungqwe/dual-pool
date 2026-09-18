# Phase 0B Gate A Harness Repair

## Starting state

- Run ID: `2026-09-18T2115Z-phase-0b-gate-a-harness-repair`
- Branch: `phase-0/reversible-compatibility`
- Start HEAD: `a6a04adbd468b9aeeab6e90ac358e9b4019377a4`
- Worktree: clean at start.

## Objective

Repair the synthetic Codex Responses harness only, prove the v3 SSE framing defect with a byte assertion, run a bounded v4 lifecycle/network/security gate, and deliver evidence. Codex picker and Antigravity probes are explicitly excluded from this run.

## Known audit findings

- v3 used literal backtick characters in a single-quoted SSE string.
- v3 used synchronous invocation and persisted/scrubbed temporary output.
- v3 did not independently prove listener ownership or natural process exit.

## Planned test

- Unit assertion that the old construction lacks `0A 0A` and v4 construction contains it.
- Recorder self-test for `response.created`, `response.completed`, `text/event-stream`, close, and LF/LF framing.
- `System.Diagnostics.Process` with bounded timeout, PID identity, in-memory stdout/stderr, loopback listener ownership, in-memory sentinel checks, and final sanitized evidence.

## Non-goals and stop conditions

No user configuration mutation, OAuth, token/auth-file access, desktop UI automation, picker probe, Antigravity probe, or Phase 1 work. Stop on self-test failure, transport failure, timeout, security failure, cleanup failure, or restoration risk.

## Result

- The unit construction assertion and recorder path reached `POST /v1/responses` with `gpt-6-astra`, Authorization presence, and both in-memory sentinels observed.
- V4 did not produce a complete sanitized result because its final result-persistence path raised a harness runtime failure after capture. Gate A is therefore `FAIL`, not `PASS`.
- No picker or Antigravity probe was started. Temporary v4 roots were removed; existing unrelated listeners were not touched.
- Exact next bounded repair: isolate and fix the v4 result-finalization exception, then rerun the same gate with an independently persisted lifecycle result.
