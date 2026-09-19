# Phase 1 — Windows Credential Manager secret store

## Header

- Run ID: `20260919T1821Z-phase-1-windows-secret-store`
- Date/time UTC: `2026-09-19T18:21:00Z`
- Local date: `2026-09-20` (`Asia/Saigon`)
- Agent/tool version: Codex desktop
- Roadmap phase: Phase 1
- Repository: `https://github.com/trungqwe/dual-pool`
- Worktree: `C:\Users\Admin\.codex\worktrees\shadow-watchdog-repair\dual-pool`
- Branch: `phase-1/state-foundation`
- Start HEAD: `6fbab30dc5ae3b3668cdbf330bfbc5037607bc5e`
- Toolchain: `go version go1.26.0 windows/amd64`
- End HEAD: PENDING
- Remote push result: PENDING

## Assigned objective

Implement a closed secret-store interface and a Windows Credential Manager backend for exactly four Poolbridge-owned local keys. Prove exact-target CRUD, replacement, process reuse, purpose isolation, cleanup and safe failure behavior using only runtime-generated synthetic secrets.

## Non-goals

- No provider OAuth, access or refresh token ownership, parsing or storage.
- No CLIProxyAPI auth-root access, real account credential, provider API token or auth JSON access.
- No config mutation, Codex/Antigravity integration, process lifecycle, listener, doctor or product-root initialization.
- No DPAPI implementation or fallback, credential enumeration, arbitrary target API, user-facing CLI operation or new `apperr.Code`.
- No real Poolbridge production-key initialization; the checklist item for four distinct strong product secrets remains open.

## Starting state

- Worktree is clean at the exact expected Phase 1 HEAD; `origin/phase-1/state-foundation` resolves to the same commit.
- `git ls-remote` confirms Phase 0 `6619034bc7edd79af3101ee7015a3de724b41542` and Phase 1 `6fbab30dc5ae3b3668cdbf330bfbc5037607bc5e`.
- `git fetch origin` fetched the expected Phase 0 update but could not update its local tracking ref; this unrelated ref issue does not alter the clean Phase 1 worktree or the server-side authoritative heads.
- Existing lock, atomic store, state codec, safelog, CLI and Phase 0 tests remain regression gates.

## Backend decision

V1 selects Windows Credential Manager with Unicode exact-target operations, `CRED_TYPE_GENERIC` and `CRED_PERSIST_LOCAL_MACHINE`.

- Credential Manager owns storage, so Poolbridge does not need a custom secret ciphertext file, file durability protocol, ACL lifecycle, backup/deletion semantics or ciphertext format/versioning.
- Generic credentials are application-defined opaque data. The four Poolbridge keys fit within a conservative product limit of 1..512 bytes, below Windows' 2560-byte generic credential blob maximum.
- `CRED_PERSIST_LOCAL_MACHINE` retains the credential for later logon sessions of the same user on the same computer without enterprise roaming. It does not mean machine-wide user access.
- Exact fixed targets support create/replace, read and idempotent delete without enumeration or wildcard operations and require no administrator rights.
- Current-user DPAPI is a viable user-bound alternative, but it would leave Poolbridge responsible for an encrypted blob file and its lifecycle.
- DPAPI machine scope is rejected: `CRYPTPROTECT_LOCAL_MACHINE` permits any user on the same computer to decrypt the data.
- Credential Manager is selected for storage ownership and lifecycle architecture; no claim is made that it is cryptographically stronger than DPAPI.

## Threat boundary

The Windows user account is a trust boundary. The backend keeps Poolbridge keys out of plaintext project/state/config files, committed artifacts and ordinary cross-user access. Generic credentials are readable by user processes in the owning context, so this backend does not defend against malicious code already running with equivalent access in that Windows user context. It does not claim hardware-backed protection. Target names are intentionally non-secret; only credential blobs are secret material.

## Secret purpose registry

The public typed registry contains exactly:

- Codex client key → `dualpool:v1:codex:client-key`
- Codex management key → `dualpool:v1:codex:management-key`
- Google client key → `dualpool:v1:google:client-key`
- Google management key → `dualpool:v1:google:management-key`

Production callers cannot provide target strings. Unknown purposes fail before any Windows API call. Metadata contains no user identity or secret-derived data; a fixed `poolbridge` username may be used and comments/attributes remain empty.

## Test namespace design

Real WinCred tests use a unique runtime-generated prefix shaped as `dualpool-test:<random-run-id>:v1:` and the same closed purpose suffix registry. The namespace constructor is unexported and rejects malformed input. Parent tests register exact deletion for all four test targets before the first write. Cleanup never enumerates credentials or touches production targets.

## Memory-handling rules

- `Put` copies caller bytes, never mutates the caller, wipes the temporary copy after `CredWriteW`, reads back the exact target and constant-time compares it before success.
- `Get` copies bytes out of WinCred-owned memory, wipes the native credential blob where practical, calls `CredFree` and returns a fresh caller-owned slice.
- Verification and internal test buffers are wiped with a narrow helper and `runtime.KeepAlive` where useful.
- This is best-effort bounded plaintext lifetime; Go cannot guarantee erasure of every historical compiler/runtime copy.
- Errors, test output, logs, evidence and reports must not include secret bytes.

## Windows integration plan

- Isolate the minimal `CREDENTIALW` declaration and `CredWriteW`, `CredReadW`, `CredDeleteW`, `CredFree` boundary behind an injectable API.
- Unit-test invalid input before API access, write/read/delete failures, not-found mapping, verification mismatch, replacement behavior, fresh returned copies and temporary-buffer wiping.
- Run real synthetic exact-target CRUD, four-purpose isolation, selective/idempotent delete, replacement and a child-process read proof that passes only a SHA-256 digest.
- Add source gates for no credential enumeration, no DPAPI machine scope, no provider-token access and no production targets in integration tests.
- Run full Go format/vet/test/race/build/module verification, Phase 0 53/53, upstream-lock validation and repository JSON/docs/privacy/secret/diff gates.

## Official Microsoft sources checked

Checked `2026-09-19` UTC (`2026-09-20` local):

- `CredWriteW`: https://learn.microsoft.com/en-us/windows/win32/api/wincred/nf-wincred-credwritew
- `CredReadW`: https://learn.microsoft.com/en-us/windows/win32/api/wincred/nf-wincred-credreadw
- `CredDeleteW`: https://learn.microsoft.com/en-us/windows/win32/api/wincred/nf-wincred-creddeletew
- `CredFree`: https://learn.microsoft.com/en-us/windows/win32/api/wincred/nf-wincred-credfree
- `CREDENTIALW`, `CRED_TYPE_GENERIC`, `CRED_PERSIST_LOCAL_MACHINE` and blob limits: https://learn.microsoft.com/en-us/windows/win32/api/wincred/ns-wincred-credentialw
- Generic credential trust characteristics: https://learn.microsoft.com/en-us/windows/win32/secauthn/kinds-of-credentials
- `CryptProtectData` and `CRYPTPROTECT_LOCAL_MACHINE`: https://learn.microsoft.com/en-us/windows/win32/api/dpapi/nf-dpapi-cryptprotectdata

## Stop conditions

- Stop on any failed phase gate, source/history mismatch, real WinCred integration unavailability, cleanup failure, secret leakage, provider credential access, production-target test access, Phase 0 regression or remote CI failure.
- Do not substitute DPAPI if WinCred is unavailable.
- Do not mark the Windows secret-store checklist item complete until real Windows integration and remote CI pass.

## Investigation and evidence

- Microsoft documentation confirms `CredWriteW` creates or replaces an exact `(TargetName, Type)` credential in the current token's user credential set; `CredReadW` returns one allocated buffer that must be released by `CredFree`; and `CredDeleteW` deletes one exact target/type.
- `CREDENTIALW` documents generic credentials as application-defined, a 2560-byte maximum blob, case-insensitive target names and `CRED_PERSIST_LOCAL_MACHINE` visibility only to later logon sessions of the same user on the same computer.
- `CryptProtectData` documents current-user behavior by default and broader any-user-on-the-computer decryption when `CRYPTPROTECT_LOCAL_MACHINE` is set. No DPAPI code was added.
- The local WinCred integration executed successfully in the current Windows user credential set under a unique test namespace. Evidence records classifications and counts only; it contains no target, secret, digest, username, PID or local path.
- The implementation uses `golang.org/x/sys/windows`, already pinned by the state-store slice, only for secure DLL loading, UTF-16 conversion, Windows types and status constants. No dependency was added.

## Changes

| Path | Change | Reason |
|---|---|---|
| `internal/secretstore/` | Closed four-purpose interface, exact WinCred backend, safe errors, best-effort wipe helper, fault tests and real Windows integration | Implement the bounded secret-store objective |
| `docs/03-DECISIONS-AND-EVIDENCE.md` | ADR-008 selects Credential Manager and distinguishes both LOCAL_MACHINE concepts | Record the V1 architecture decision |
| `docs/11-SECURITY-THREAT-MODEL.md` | Same-user boundary, ownership and memory limits | Prevent overclaiming protection |
| `docs/23-TRACEABILITY-MATRIX.md` | Seven secret-store requirement rows | Connect implementation to proof |
| `docs/18-HANDOFF.md` | Implementation state and exact next task | Preserve session continuity |
| `evidence/phase-1-secret-store/` | Sanitized decision, test, security and manifest records | Machine-readable proof without credential data |

## Verification

| Test/command | Result |
|---|---|
| `go fmt ./...` | PASS |
| `go vet ./...` | PASS |
| `go test -count=1 ./...` | PASS: 72 Go test functions; 8 secret-store functions |
| Real WinCred synthetic integration | PASS: CRUD, replacement, missing, child process, four-purpose isolation, selective and exact cleanup |
| `go test -race -count=1 ./...` | PASS; lock stress completed under race instrumentation |
| `go build ./cmd/poolbridge` | PASS |
| `go mod verify` | PASS: all modules verified |
| `node --test scripts/*.test.cjs` | PASS: 53/53 |
| `node scripts/phase0-upstream-lock.cjs` | PASS: `UPSTREAM_LOCK_VALID` |
| JSON/docs/privacy/secret/diff checks | PASS across 15 scoped changed files; repeated after staging |
| GitHub-hosted Source CI | PENDING implementation push |

## Security/privacy review

- Production code has no credential enumeration/find-best API, wildcard delete, DPAPI implementation, provider token/auth-file access, logging or arbitrary target entry point.
- Tests use runtime-generated random bytes only. The cross-process test passes a namespace and SHA-256 digest, never the secret; the child returns only success/failure.
- Cleanup is registered before the first write, deletes only the four exact targets in the unique test namespace and is idempotent. No credential listing or dump was performed.
- Caller input is not mutated. Internal write and verification copies are wiped. Native WinCred blob memory is wiped where practical before `CredFree`; callers own and can wipe returned copies.
- Error classification includes only package categories and safe Win32 status text. Dynamic leak tests confirm input bytes do not appear.
- Real product root touched: false. Product targets used in integration tests: false. Provider credentials touched: false.

## Acceptance evaluation

- Interface, four closed purposes, fixed production mappings, exact Unicode WinCred CRUD, replacement, verification, missing/idempotent delete, process reuse, isolation, cleanup, fault injection, memory review and negative source gates: PASS locally.
- Remote CI: PENDING. The Windows secret-store checklist item remains open until the implementation workflow passes.
- Four distinct strong product secrets: OPEN. This slice proves four synthetic storage slots and does not initialize real product keys.
- Phase 1 remains open. Config transaction fixtures, full end-to-end sentinel/redaction exercise and exit reconciliation remain.

## Git delivery

- Implementation commit/push: PENDING.
- Implementation Source CI: PENDING.
- Delivery receipt: PENDING and will be a separate commit after observable implementation CI.
- PR: not requested; none created.

## Next run

Phase 1 — config backup/patch/rollback engine using synthetic fixtures only. Do not begin it in this run.
