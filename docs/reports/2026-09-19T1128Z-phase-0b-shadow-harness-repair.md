# Phase 0B — shadow CODEX_HOME harness repair

## Scope

- Starting remote HEAD: `383c38b1b65da08ac4b3e87bfee841c1f411cd8b`.
- Branch: `phase-0/reversible-compatibility`.
- Policy: `REAL_USER_CONFIG_LIVE_MUTATION_DISABLED`.
- This run repaired the shadow harness only; no live IDE probe was run.

## Read-only config assessment

- Current config SHA-256: `C15720CC37B0670D4EC1D0294DA5E067DD7CF65437D2E5DF4C209A9411F5CD58`.
- Historical clean backup SHA-256: `1AE4E3BC2C1185EA4C9C863BA481D66F5C160B02B63F4C04F1D885754257470D`.
- Incident backup SHA-256: `E699E3262CB1051ADE1B74E491AEF315692127111928C152BEC304E0900DC2A2`.
- TOML parse: `PASS`; all probe contamination markers: `false`.
- Decision: `CURRENT_WORKING_CONFIG_CANDIDATE`, based on the owner-reported current Codex chat being open. Historical byte equivalence is **not claimed**.
- Real config mutated: `false`.

## SHADOW-001..008 repair status

- `SHADOW-001`: resolved — session, state, and recorder directories exist before the first state checkpoint.
- `SHADOW-002`: resolved in code — actual Antigravity launch uses `probeEnvironment()` and process-count liveness; live execution was not run.
- `SHADOW-003`: resolved — cold process-count gate precedes opaque copy; no forced IDE termination.
- `SHADOW-004`: resolved — recorder contract uses `capture.json`.
- `SHADOW-005`: resolved — the generated sentinel is rendered in the external console after arming.
- `SHADOW-006`: resolved — `armed.json` is awaited before prompt display.
- `SHADOW-007`: resolved — `ASTRA_NOT_VISIBLE` maps to `AUTHENTICATED_ASTRA_NOT_VISIBLE`.
- `SHADOW-008`: resolved — the historical hash is not an execution gate; each run captures and rechecks an immutable dynamic baseline.

## Safety and verification

- Shadow topology is `sessionRoot/codex-home`, `sessionRoot/state`, `sessionRoot/recorder`.
- Cleanup accepts only the owned TEMP session-root pattern and rejects unsafe roots/reparse points.
- Probe launch has no temporary `--user-data-dir` and never launches the normal IDE automatically.
- `node --check`: PASS; shadow fixture/orchestration tests: `15/15 PASS`; `git diff --check`: PASS.
- Live U-006: `NOT RUN`; U-006 remains `BLOCKED` until a separately authorized owner-assisted run satisfies every live gate.

Evidence: `evidence/phase-0b-u006-shadow-home/baseline-audit.json` and `safety-gate.json`.
