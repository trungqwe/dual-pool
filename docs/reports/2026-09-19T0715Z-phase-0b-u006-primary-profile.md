# Phase 0B U-006 primary authenticated profile probe

## Header

- Starting HEAD: `4f57f8a402be282cbefff802de6473c167e6735a`.
- Branch: `phase-0/reversible-compatibility`.
- Objective: run one final U-006 probe against the real authenticated Antigravity profile, prove GPT-6 Astra retains the temporary Responses provider and emits exact wire model `gpt-6-astra`, then restore the real Codex configuration byte-for-byte.
- Non-goals: U-001..U-004, Phase 1, authentication inspection, profile cloning, catalog injection, extension patching, and further CLI/profile-auth investigation.

## Worktree classification

- Historical residue exists under `evidence/phase-0b-codex-extension/`; it is preserved, not reset, stashed, cleaned, overwritten, or staged.
- The current run starts from the expected branch and HEAD. No current-run artifact is classified as historical residue.

## Existing authoritative status

- U-001: `BLOCKED`
- U-002: `BLOCKED`
- U-003: `BLOCKED`
- U-004: `BLOCKED`
- U-005: `PROBED`
- U-006: `BLOCKED: CLONED_AUTH_STATE_NOT_RECOGNIZED`
- U-007: `PROBED`
- U-008: `PARTIAL / UNKNOWN`

## Authorization and boundaries

- The owner authorizes one temporary, reversible mutation of the real user-level Codex `config.toml` for this compatibility probe.
- Authentication/session files, cookies, webview storage, tokens, credential databases, and account contents remain untouched and uninspected.
- The external watchdog owns backup, mutation, recorder, checkpoints, rollback, normal relaunch, and final evidence so the tested IDE extension is not required to keep the control loop alive.
- No project-local provider override is used. The probe uses the real default profile and default `CODEX_HOME`.

## Official configuration basis

The official OpenAI configuration reference states that user configuration is `~/.codex/config.toml`; project-local `.codex/config.toml` cannot override provider-routing keys such as `model_provider` and `model_providers`. The temporary provider therefore belongs in the user-level file and must be restored byte-for-byte afterward.

## State machine and acceptance

`PRECHECK → WATCHDOG_READY → BACKUP_COMPLETE → WAIT_PRIMARY_CLOSE → PRIMARY_CLOSED → RECORDER_READY → PROBE_CONFIG_ACTIVE → PRIMARY_RELAUNCHED_FOR_PROBE → WAIT_AUTH_CONFIRMATION → WAIT_ASTRA_SELECTION → WAIT_ASTRA_SENTINEL → ASTRA_REQUEST_CAPTURED → WAIT_PROBE_CLOSE → RESTORE_CONFIG → RELAUNCH_NORMAL → VERIFY_RESTORE → CLEANUP → FINALIZE`.

U-006 passes only when the real authenticated extension sends exactly one sentinel-bearing `POST /v1/responses` to the loopback recorder with exact model `gpt-6-astra`, matching synthetic authorization and prompt, valid JSON, and completed Responses lifecycle. Any terminal failure must restore the original configuration before finalization.

## Observed outcome

- An initial watchdog attempt stopped before mutation because its atomic replacement call used an invalid null backup path. The real config remained unchanged; this is recorded as development correction, not a probe result.
- The authoritative retry created the required backup, activated the temporary user-level provider configuration, then restored the original config byte-for-byte. Before and after config hash: `1AE4E3BC2C1185EA4C9C863BA481D66F5C160B02B63F4C04F1D885754257470D`.
- The watchdog could not relaunch Antigravity within its bounded window and terminated as `NORMAL_RELAUNCH_FAILED` before the authentication, Astra, or wire checkpoints. No target request was captured.
- U-006 remains **BLOCKED**. Cleanup completed and no authentication material was inspected, logged, hashed, committed, or retained.
