# Phase 0B U-006 opaque authenticated Antigravity profile clone

## Header

- Starting HEAD: `04bd98535a4edcfd6edc07e8e39486f49e1379db`.
- Branch: `phase-0/reversible-compatibility`.
- Objective: close U-006 by cloning the owner's authenticated Antigravity profile opaquely into a disposable probe profile, then proving GPT-6 Astra retains the temporary Codex custom provider and exact Responses model.
- Scope: U-006 only. U-001..U-004, Phase 1, Cloud Code, catalog research, extension patching, OAuth implementation, and CLI `auth.json` reuse are out of scope.

## Existing status

- U-001: `BLOCKED`
- U-002: `BLOCKED`
- U-003: `BLOCKED`
- U-004: `BLOCKED`
- U-005: `PROBED`
- U-006: `BLOCKED: CLI_AUTH_NOT_CONSUMED_BY_EXTENSION`
- U-007: `PROBED`
- U-008: `PARTIAL / UNKNOWN`

## Worktree classification

- Historical probe residue is present under `evidence/phase-0b-codex-extension/`; it is preserved, not reset, stashed, cleaned, overwritten, or staged.
- Current run starts from the expected branch and HEAD. No current-run artifact is treated as historical residue.

## Owner authorization and security boundary

- The owner authorizes local opaque copying of the authenticated Antigravity user-data profile, including cookies, storage, session state, extension state, and encrypted credential blobs.
- The entire source profile is treated as opaque authenticated material. Its contents must not be read, parsed, printed, logged, hashed, committed, uploaded, or inspected.
- The source profile remains read-only. The clone stays under approved `%TEMP%`, outside the repository, and is deleted as a complete root before finalization.
- The temporary Codex home contains provider configuration only; CLI `auth.json` is not copied in this run.

## Strategy

1. `HOT_PROFILE_CLONE`: discover the real active profile, snapshot protected PIDs and real config controls, copy the complete profile opaquely, then launch an isolated Antigravity Probe.
2. If HOT cloning is incomplete or authentication is not recognized, use `COLD_PROFILE_CLONE_WATCHDOG` in the same run. The watchdog waits for the owner to close the original IDE, performs a quiescent opaque copy, relaunches the original profile, and then launches the clone Probe.

## Acceptance contract

- Authenticated clone is recognized by the owner-assisted Probe checkpoint.
- The owner selects GPT-6 Astra in the actual picker.
- Exactly one synthetic request reaches `127.0.0.1` at `POST /v1/responses`.
- Exact wire model is `gpt-6-astra`; the same temporary custom provider is retained; synthetic auth and prompt match; JSON and Responses lifecycle pass.
- No raw request/response, credential, profile inventory, or absolute local path is persisted.

## Cleanup contract

- Kill/close only Probe-owned processes; never terminate pre-existing Antigravity PIDs.
- Delete the complete cloned profile and all temporary Codex/extension/workspace/recorder state.
- Verify clone root existed before cleanup and is absent afterward; verify recorder/listener/probe absence, original IDE survival or controlled relaunch, real config/settings/extension integrity, and repository-only evidence.

## Run history and final outcome

- HOT clone attempt: `HOT_PROFILE_CLONE_INCOMPLETE`; no Probe was launched.
- COLD clone attempts reached the Probe after the owner manually reopened the original IDE. One exploratory request carried `gpt-5.6-luna`, not Astra; it is not accepted as U-006 proof because the owner confirmed Astra had not actually been selected for that request.
- The final COLD Probe displayed `GPT-6 Astra` in the dropdown, but the owner confirmed the Codex settings contained no active Codex Plus account and the UI remained reconnecting. The authenticated-clone checkpoint therefore failed: `CLONED_AUTH_STATE_NOT_RECOGNIZED`.
- U-006 remains **BLOCKED**. Dropdown visibility alone is non-diagnostic for authenticated entitlement or picker retention.
- Cleanup and integrity controls passed for the final run: clone and Probe roots were removed, the original IDE survived after manual relaunch, real configuration/settings and installed extension controls remained unchanged, and no profile contents were inspected.
