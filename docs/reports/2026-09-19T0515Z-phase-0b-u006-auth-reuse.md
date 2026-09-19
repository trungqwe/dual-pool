# Phase 0B U-006 authenticated auth-reuse probe

## Header

- Start HEAD: `0f0f99079f609c9de0e843890e8e17588f1df93b`.
- Branch: `phase-0/reversible-compatibility`.
- Objective: close U-006 by proving authenticated GPT-6 Astra picker retention and exact wire routing through the temporary custom provider.
- Non-goals: U-001..U-004, Phase 1, Cloud Code, catalog fallback, extension patching, real configuration mutation, and real-session logout.

## Worktree inventory

Two tracked modifications and one untracked file under `evidence/phase-0b-codex-extension/` are classified as `historical-probe-residue`. They are preserved, not reset, stashed, cleaned, overwritten, or staged. No unknown current-run change is being modified.

## Owner authorization and credential boundary

The owner explicitly authorizes local opaque reuse of the already authenticated Codex state for this compatibility probe. The agent may check `codex login status` and copy an opaque `auth.json` byte-for-byte into the approved temporary Probe root. It must not read, parse, print, log, hash, inspect, commit, or exfiltrate authentication material. Browser cookies, webview cookies, and the primary IDE profile remain out of scope. No real `codex logout` is permitted.

## Preflight authentication

The default Codex home has an opaque auth-file presence signal and `codex login status` classified the active method as `CHATGPT`; raw command output was held only in memory and not persisted. The temporary auth bootstrap will record only method, boolean status, and material-handling booleans.

## Planned acquisition order

1. Copy the opaque default `auth.json` only if present, without opening it.
2. Run `codex login status` with the temporary `CODEX_HOME`; persist only `AUTHENTICATED_CHATGPT` or `NOT_AUTHENTICATED`.
3. If file reuse does not authenticate, try the existing OS credential-store path through the same status command without enumeration.
4. If necessary, launch official `codex login` in a separate visible process scoped to the temporary `CODEX_HOME`; user completes browser login, then status is checked again.

## Picker and wire acceptance

After temporary ChatGPT authentication is established, the isolated extension must recognize the authenticated state. The user then selects GPT-6 Astra in the actual picker and sends one synthetic prompt. U-006 becomes PROBED only if the same loopback `dualpool_probe` receives `POST /v1/responses` with exact allowlisted model `gpt-6-astra`, matching synthetic authorization and prompt, parsed JSON, and completed Responses lifecycle.

## Cleanup and stop conditions

All pre-existing Antigravity PIDs are protected. The temporary auth state remains opaque and is deleted only as part of the complete temporary root after the user closes the Probe. Stop on auth not recognized, picker absence, route/auth/model/prompt/parser/lifecycle failure, real-state hash drift, protected PID loss, privacy scan failure, manifest failure, or cleanup failure. U-005 remains PROBED and is not reopened.

## Observed outcome

- The opaque auth bootstrap succeeded: temporary `codex login status` classified the copied state as `AUTHENTICATED_CHATGPT`.
- The isolated Codex extension did not expose a login UI and did not consume the authenticated CLI state. The owner confirmed `AUTH_NOT_RECOGNIZED`; this is classified as `CLI_AUTH_NOT_CONSUMED_BY_EXTENSION`.
- The picker and Astra wire stages were not started. U-006 remains **BLOCKED**; no claim is made about GPT-6 Astra visibility or model-provider retention.
- Cleanup passed: both temporary roots were removed, the recorder listener was absent, the probe process was absent, the original Antigravity instance survived, and real configuration/extension hashes were unchanged.
- Authentication material handling remained within the declared boundary: opaque copy only; contents were not read, parsed, printed, logged, hashed, committed, or stored in the repository.
