# Incident audit — primary watchdog recovery

## Header

- Run ID: `20260919T0905Z-primary-watchdog-incident-audit`
- Roadmap: Phase 0B, U-006
- Repository: `https://github.com/trungqwe/dual-pool`
- Authoritative branch: `phase-0/reversible-compatibility`
- Incident classification: `PRIMARY_REAL_CONFIG_MUTATION_SURVIVED_WATCHDOG_ABORT`

## Incident facts

The primary-profile watchdog temporarily replaced the real user-level Codex `config.toml` with a probe provider that required `DUALPOOL_CODEX_KEY`. The watchdog stopped while waiting for the authentication checkpoint. A Windows restart did not run the later restore stage. The next normal Codex launch reported `Missing environment variable: DUALPOOL_CODEX_KEY` and the normal UI did not show the expected prior chat state.

The owner/Gemini recovery report states that a verified backup existed with SHA-256 `1AE4E3BC2C1185EA4C9C863BA481D66F5C160B02B63F4C04F1D885754257470D`, that the real config was restored to the same hash, that probe keys were removed, and that the temporary watchdog session was deleted. It also reports that the normal Codex UI became usable again and that the history database and auth/session files remained present. Those recovery observations are `OWNER/GEMINI_REPORTED`; they are not independently re-verified by this commit.

The incident confirms that a `try/finally` restore is insufficient. It covers ordinary exceptions while the process remains alive, but it cannot run after process termination, console termination, OS reboot, power loss, or machine failure. The absence of a durable restore stage therefore left the normal installation requiring an ephemeral probe environment variable.

## Independent checks in this run

- Remote branch was read at `37d568a2b9006e628845bb60ee7ae2bf62023c9a` before this correction.
- No watchdog/config-probe process was started.
- No auth, session, history, or config contents were inspected by this repository correction.
- The user's current config hash was observed as `4164F9C936CD58624A50A3B2191CAA02BABF903887606729CD1364BEB3EDED62`, with no `dualpool_probe` or `DUALPOOL_CODEX_KEY` string. This differs from the historical baseline and is recorded as `UNKNOWN: REAL_CONFIG_BASELINE_DRIFT`; no automatic restore was attempted.
- The incident report was absent from remote HEAD and is added here in sanitized form.

## Policy correction

- `REAL_USER_CONFIG_LIVE_MUTATION_DISABLED` is now mandatory.
- The old primary watchdog entrypoint fails before starting a recorder, closing an IDE, creating a session, or mutating a config.
- The old transaction and fixture code remains historical analysis only and is not a live entrypoint.
- U-006 remains `BLOCKED`.

## Required replacement

The next design must use the real Antigravity profile with a complete opaque shadow `CODEX_HOME` under `%TEMP%`. Only the shadow config may be changed. The real `%USERPROFILE%\.codex\config.toml` must never be modified. A crash or reboot must leave the normal Codex installation independent of probe cleanup, so `real_config_mutation_capability=false` and `crash_requires_restore=false` are required safety fields.

After shadow cleanup the watchdog must print `SHADOW_CLEANUP_PASS` and wait for the owner to open Antigravity normally. It must never auto-launch the normal IDE. If the full shadow loses authentication, classify `FULL_CODEX_HOME_SHADOW_AUTH_NOT_RECOGNIZED`, delete only the shadow, and keep U-006 blocked.

## Acceptance

- Incident reconciled: PASS.
- Real-config mutation hard-disabled: PASS after the accompanying test.
- Shadow implementation: not included in this correction commit.
- Live U-006: BLOCKED.
