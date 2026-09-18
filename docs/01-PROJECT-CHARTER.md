# Project Charter

## Problem

The user needs two convenient, stable, multi-account model pools inside existing IDE experiences:

1. A Google/Antigravity pool that replaces exactly one user-selected native Antigravity model slot while leaving every other native model untouched.
2. A Codex pool that exposes the canonical `gpt-6-astra` choice in the Codex model selector and routes its native Responses traffic through multiple Codex OAuth accounts.

Account connection must use upstream OAuth flows, sessions must remain sticky where possible, failures must fail over predictably, and every machine-level configuration mutation must be reversible.

## Product outcome

A user runs one Windows CLI, `poolbridge.exe`, to install/pin upstream CLIProxyAPI, connect accounts through the browser, validate account eligibility, configure Antigravity and Codex, run/stop services, diagnose faults, and roll back integrations.

## V1 target environment

- Windows 11 x64.
- One interactive desktop user.
- Antigravity IDE installed locally.
- OpenAI Codex extension/CLI installed locally.
- Loopback-only networking.
- One to N Google accounts and one to N Codex accounts; four each is a target, not a hard requirement.
- Go implementation producing a single `poolbridge.exe`; two child CLIProxyAPI processes use the same pinned upstream executable with separate data roots.

## In scope

- Compatibility discovery without mutation.
- Signed/checksummed upstream artifact acquisition and pinning.
- Two isolated CLIProxyAPI instances.
- Browser-driven upstream OAuth initiation and status polling.
- Account inventory, enable/disable, re-auth and per-account model eligibility probes.
- Antigravity loopback bridge with native passthrough.
- Exact donor-model override into the Google pool.
- Codex user-level custom provider configuration using Responses.
- Exact-version model catalog fallback only when needed and supported.
- Service supervision, health, doctor, backup, rollback and evidence bundles.
- Tests for affinity, failover, stream cancellation, tool calls, restarts and provider isolation.
- Reproducible GitHub handoff after every implementation run.

## Explicit non-goals

- Supporting Claude, Grok, Kimi, API-key providers or generic OpenAI-compatible backends.
- A web dashboard, Electron/Tauri GUI, tray application, mobile app or remote control.
- macOS/Linux support in V1.
- Cloud/LAN exposure, team/multi-tenant operation or remote management.
- Billing, quota forecasting, usage analytics or prompt history.
- Account creation, credential sharing, token export or policy circumvention.
- Patching CLIProxyAPI, Antigravity, Codex extension, VSIX, JavaScript bundles or certificates.
- Creating synthetic model slugs such as `astra-pool`.
- Translating Codex Responses to Chat Completions.
- Adding new models to Antigravity's catalog. V1 reuses one exact existing slot.

## Success measures

- Setup requires no manual YAML/TOML/JSON editing.
- Google and Codex credentials can never be selected by the opposite service instance.
- A conversation uses one credential until it becomes unavailable or the affinity TTL expires.
- Non-donor Antigravity traffic behaves identically to the direct baseline.
- Codex retains its native model metadata and Responses item/tool semantics.
- `poolbridge rollback all` restores owned configuration keys without deleting upstream auth files.
- A clean restart and Windows login restart retain a working state.
- Every acceptance result is supported by sanitized, versioned evidence.

## Definition of done

V1 is done only after all MUST requirements in `02-REQUIREMENTS.md`, all mandatory rows in `15-MASTER-CHECKLIST.md`, and the end-to-end cases in `13-TEST-STRATEGY.md` pass on the target Windows machine. Unsupported compatibility behavior is a blocker, not an invitation to patch third-party binaries.
