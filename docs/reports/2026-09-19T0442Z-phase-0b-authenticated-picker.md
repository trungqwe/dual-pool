# Phase 0B authenticated picker probe

## Header

- Start HEAD: `0a5b44e5e76a95ac451071ca47be9aaac221a7e1`.
- Branch: `phase-0/reversible-compatibility`.
- Objective: correct the unauthenticated picker interpretation, close U-005 from existing config-layer evidence, and run the authenticated U-006 picker probe in a second isolated Antigravity/Codex instance.
- Non-goals: no Phase 1, no Antigravity Cloud Code, no real configuration mutation, no Astra picker action before user authentication, and no credential-material inspection.

## Worktree inventory

The prior historical probe residue is preserved and not staged: two tracked modifications and one untracked two-request evidence file under `evidence/phase-0b-codex-extension/`. They are classified as `historical-probe-residue`; no unknown current-run change is being overwritten.

## Existing evidence and audit correction

The installed matching Codex catalog already contains `gpt-6-astra`. The prior extension run used a temporary `CODEX_HOME`, temporary `model_provider`, random loopback `base_url`, Responses wire mode, and a synthetic child-process environment key; its request reached that loopback and matched the synthetic authorization and prompt. The prior unauthenticated picker observation is therefore `NON_DIAGNOSTIC_FOR_ASTRA_ENTITLEMENT`. U-005 is audited as `PROBED`, qualified to Windows 10, `openai.chatgpt 26.5730.61309`, and the observed installed Antigravity/Codex runtime. The separate model-resolution mismatch does not reopen config-layer proof.

## Installed versions

- Antigravity product: `UNKNOWN` from the current product CLI surface; core/CLI surface reports `1.107.0`.
- Antigravity core version: `1.107.0`.
- Antigravity CLI version: `1.107.0`, executable hash recorded in the new evidence.
- Codex extension: `openai.chatgpt 26.5730.61309`.
- Codex CLI: `0.154.0`; CLI hash recorded in the new evidence.

## Security boundary and plan

The disposable Probe may receive authentication only through the official UI. Agent scripts will observe only a user-assisted authenticated/not-authenticated state. They will not enumerate, open, parse, copy, hash, search, log, or commit authentication files, cookies, databases, tokens, account IDs, emails, or billing data. The Probe retains a temporary extension silo, user-data root, `CODEX_HOME`, empty workspace, and loopback recorder; all original Antigravity PIDs are protected.

The U-006 recorder will accept and persist an exact model slug only after a strict allowlist check, require `gpt-6-astra`, synthetic authorization, synthetic prompt, Responses JSON parsing, same loopback route, and completed response lifecycle. The Probe will be deleted only as an opaque root after the user closes it.

## Stop conditions

Stop on authentication failure/unavailability, protected PID loss, real-state hash drift, non-loopback recorder, missing isolation flags, picker absence after authenticated login, model allowlist failure, route/auth/prompt/JSON/lifecycle failure, absolute-path or secret scan failure, or cleanup failure. U-006 remains open unless the complete contract passes.

## Outcome

The Probe launched in the proven isolated topology, but its custom-provider Codex extension exposed no login UI. The user therefore could not authenticate through the disposable profile without reusing the primary session. The run stopped before picker or wire testing and classified U-006 as `AUTHENTICATED_PROBE_UNAVAILABLE`. U-005 is `PROBED`; the earlier unauthenticated picker observation is non-diagnostic for Astra entitlement. The opaque Probe root was deleted after user-assisted close; no authentication material was inspected.

Final evidence: `evidence/phase-0b-codex-extension-u006/`. Security gate and manifest both pass; the manifest was generated last and its referenced hashes were verified.
