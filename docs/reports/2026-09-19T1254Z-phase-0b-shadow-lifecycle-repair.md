# Phase 0B — live shadow lifecycle repair

## Scope

- Starting HEAD: `7db245469cbda20d9080e0924c49edd29757878b`.
- Branch: `phase-0/reversible-compatibility`.
- Policy: `REAL_USER_CONFIG_LIVE_MUTATION_DISABLED`.
- No live probe was run before this safety commit.

## Repairs

- `SHADOW-009`: watchdog now announces `WATCHDOG_READY` and waits for voluntary Antigravity closure before cold copy.
- `SHADOW-010`: PowerShell process observation fails closed on process errors, non-zero exit, or invalid output.
- `SHADOW-011`: probe-close timeout retains the owned shadow, skips cleanup, and skips normal reopen.
- `SHADOW-012`: final PASS requires cleanup, unchanged real config, manual normal reopen, observed normal process, and `NORMAL_IDE_OK`.
- `SHADOW-013`: session root ACL is restricted with the current Windows SID and SYSTEM SID, inheritance is removed, and a real create/read/delete fixture runs before copy.

Additional controls: executable discovery occurs before primary close; heartbeat is written in the external watchdog; launcher creates an independent PowerShell console; bounded cleanup helper deletes only an owned, process-free session root.

## Safety evidence

- Current config SHA-256: `C15720CC37B0670D4EC1D0294DA5E067DD7CF65437D2E5DF4C209A9411F5CD58`.
- TOML parse: `PASS`; all probe markers: `false`; historical equivalence: not claimed.
- Preflight matching Antigravity process count: `21`; no process was terminated.
- Real config mutation: `false`.
- Tests: `20/20 PASS`.
- Node syntax: `PASS`.
- PowerShell parser: `PASS`.
- `git diff --check`: `PASS`.
- Live eligibility after push: conditional; this commit must be pushed and remote-verified first.

No auth, token, session, chat history, raw prompt, synthetic secret, PID list, or absolute local path was persisted.
