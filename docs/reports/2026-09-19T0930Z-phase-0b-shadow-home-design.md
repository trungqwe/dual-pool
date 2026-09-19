# Phase 0B — shadow CODEX_HOME U-006 design

## Header

- Run ID: `20260919T0930Z-shadow-home-design`
- Roadmap: Phase 0B, U-006
- Branch: `phase-0/reversible-compatibility`
- Parent remote: `a563d512be00e37a30b37a37dd387a7cb0cf003f`
- End HEAD / push: PENDING

## Objective

Replace the unsafe real-user-config mutation path with a shadow `CODEX_HOME` path. This commit adds the safe design and fixture tests; it does not run a live probe.

## Safety contract

`real_config_mutation_capability=false` and `crash_requires_restore=false` are hard-coded policy fields. The real `%USERPROFILE%\\.codex\\config.toml` is hashed read-only against the reported historical baseline before a run. A drift blocks the run; it is never repaired automatically. The full Codex home is copied opaquely into a TEMP root. Only the shadow `config.toml` is changed.

If the watchdog dies or Windows reboots, the real configuration remains untouched and therefore does not require a restore stage. The probe launcher receives `CODEX_HOME=<shadow>` and the synthetic key only in its child environment. The normal IDE is never auto-launched after cleanup.

## Changes

| Path | Change | Reason |
|---|---|---|
| `scripts/phase0b-u006-shadow-home-watchdog.cjs` | New shadow-only watchdog, checkpoints, opaque copy, cleanup and sanitized result | Remove real-config crash blast radius |
| `scripts/phase0b-u006-shadow-home-watchdog.test.cjs` | Five fixture tests | Prove policy, source immutability, environment isolation and no primary/normal launch path |
| `docs/18-HANDOFF.md` | Shadow design and incident boundary | Make next action explicit |
| `docs/15-MASTER-CHECKLIST.md` | Shadow gate open | Track live proof separately |

## Verification

- `node --test scripts/phase0b-u006-shadow-home-watchdog.test.cjs`: exit 0, 5/5 PASS.
- `node --check scripts/phase0b-u006-shadow-home-watchdog.cjs`: PASS.
- `git diff --check`: PASS.
- Real configuration mutation: not attempted.
- Auth/session/history: not inspected or copied into repository artifacts.

## Live status

U-006 remains `BLOCKED`. The current machine reports a real-config hash different from the historical baseline; the shadow watchdog will classify this as `REAL_CONFIG_BASELINE_DRIFT` and stop before copying or mutation until an owner-approved baseline is established by an independent read-only audit.

## Next run

First perform a read-only owner check of the current config baseline. If accepted, run one shadow-only U-006 probe. On `AUTH_LOST`, close the probe, delete only the shadow, record `FULL_CODEX_HOME_SHADOW_AUTH_NOT_RECOGNIZED`, and keep U-006 blocked.
