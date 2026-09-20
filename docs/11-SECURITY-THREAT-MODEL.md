# Security and Privacy Threat Model

## Assets

- OAuth credentials managed by CLIProxyAPI.
- Local client and management keys.
- User IDE configuration and backups.
- Prompts, outputs, tools and source code transiting memory.
- Account identity and model entitlement metadata.
- Executable/update supply chain.
- Local process control and loopback ports.

## Trust boundaries

1. User shell to `poolbridge.exe`.
2. Poolbridge to two child CLIProxyAPI processes.
3. Codex client to Codex loopback API.
4. Antigravity to bridge to native upstream/Google instance.
5. CLIProxyAPI to provider OAuth/model services.
6. Update downloader to upstream release storage.
7. Local persistent data to other processes/users.

## Threats and controls

| ID | Threat | Control | Verification |
|---|---|---|---|
| T-01 | LAN access to proxy/management | Explicit `127.0.0.1`, post-start listener inspection | NET-001 |
| T-02 | Local unauthenticated management | Strong independent management keys | SEC-002 |
| T-03 | Cross-provider credential use | Two processes and disjoint auth roots | ISO-001/002 |
| T-04 | Token leakage in logs/errors | Structured allowlist logging, redaction tests | LOG-001/002 |
| T-05 | Prompt/source persistence | Metadata-only observation; bodies disabled | LOG-003 |
| T-06 | Malicious/tampered upstream binary | Pinned source, SHA-256, version check, staged smoke | SUP-001 |
| T-07 | Open proxy/SSRF through bridge | Fixed allowlisted upstream origins; reject request hosts | AG-SEC-001 |
| T-08 | Config clobber/concurrent edits | Hash compare-and-swap and owned-key patching | CFG-003 |
| T-09 | PID reuse kills unrelated process | Verify image, start time, args and instance token | PROC-002 |
| T-10 | OAuth CSRF/state confusion | Use upstream state, serialize flows, deadline/cancel | AUTH-003 |
| T-11 | Credential substitution after OAuth | Diff inventory and credential-specific probe | AUTH-004 |
| T-12 | Retry duplicates tool side effects | Bounded pre-first-byte retries only | REL-004 |
| T-13 | Catalog/schema downgrade | Exact-version catalog fingerprint and parser test | CX-003 |
| T-14 | Secret committed to Git/ZIP | pre-commit secret scan and artifact allowlist | DEL-003 |
| T-15 | Browser opens attacker-controlled OAuth URL | validate scheme and expected host policy | AUTH-002 |

## Secret-handling rules

- Generate with OS cryptographic RNG.
- Separate keys by instance and purpose.
- Prefer DPAPI/Credential Manager for poolbridge-owned secrets.
- Use environment variables or command-backed auth at process launch when supported.
- Avoid command-line arguments for secrets because other processes may inspect them.
- Zero temporary byte buffers where practical; never include values in formatted errors.
- Redaction operates on structured header/config fields and known secret types before serialization.
- `--debug` may add timing/schema metadata, never payloads or secret values.

### Poolbridge key storage

V1 stores only four Poolbridge-generated local keys in Windows Credential Manager: client and management keys for the Codex and Google instances. They use generic credentials, fixed non-secret versioned target names, local-machine persistence within the current user's credential set, and exact target CRUD. Poolbridge does not enumerate the credential set and does not own provider OAuth, access or refresh tokens.

The Windows user account is the trust boundary. Credential Manager prevents plaintext secret files and ordinary cross-user access, but generic credentials can be read by processes with equivalent access in the owning Windows user context. Target names are metadata and intentionally non-secret. Temporary Go and native read buffers are wiped where practical to bound plaintext lifetime; this is best effort and does not guarantee erasure of every historical compiler/runtime copy.

Credential Manager `CRED_PERSIST_LOCAL_MACHINE` is not DPAPI machine scope. The former persists for later sessions of the same user on the same computer. DPAPI `CRYPTPROTECT_LOCAL_MACHINE` permits any user on that computer to decrypt and is rejected for default Poolbridge protection.

## File permissions

On Windows, generated secret-containing files and backups must have ACL inheritance reviewed and access limited to the current user plus required system principals. The installer does not run elevated by default and must not broaden ACLs to `Users` or `Everyone`.

## Supply chain

- Permit downloads only from documented upstream release origins.
- Store artifact SHA-256 in `upstream.lock` after independent verification.
- Do not execute a file before hash and signature/version checks.
- Record Go module sums and use dependency scanning in CI.
- Build release binaries from a clean committed tree with reproducible flags where practical.
- Generate an SBOM and checksum file for releases.

## Abuse and policy boundaries

The project manages credentials the user legitimately authorizes. It must not create accounts, evade provider enforcement, falsify client identity beyond upstream default behavior, sell/share credentials, or claim unlimited access. Provider errors and limits are surfaced, not bypassed through hidden cross-provider routing.

## Security release gate

Concurrent stale reclaimers cannot remove a successor's canonical lock by pathname. Each owner or reclaimer must hold an exclusive handle to the exact canonical file object; stale and normal-release deletion use handle disposition only after record and process-identity checks.

Release is blocked if any listener is non-loopback, management works without a key, a secret sentinel appears in logs/evidence/ZIP, opposite-pool credentials are visible to an instance, updater accepts a checksum mismatch, or rollback can overwrite concurrent user edits.
# Phase 2 entry gates from Phase 1 reconciliation

`P2-ENTRY-ACL-001`: before the first real product-root or secret-containing CLIProxyAPI config is created, prove current-user-restricted ACLs and prove that Users/Everyone permissions are not broadened.

`P2-ENTRY-KEYS-001`: before two real CLIProxyAPI configs/processes are created, generate four independent `crypto/rand` product keys, persist them only through the four-purpose secret store, prove all four differ, and prove none enters config evidence or logs.

These are Phase 2 entry gates. Phase 1 used disposable fixtures and did not create the real product root or product keys.
# Rủi ro còn lại của cấu hình pinned v7.3.7

CLIProxyAPI v7.3.7 so khớp `api-keys` bằng chuỗi plaintext. Vì vậy, hai client wire key nằm trong đúng hai `config.yaml` thuộc instance, được bảo vệ bằng DACL current-user/SYSTEM; đây là rủi ro tồn dư đã chấp nhận cho adapter pinned. Không sao chép nội dung cấu hình vào repository, evidence, log hay chẩn đoán. Management key chỉ lưu bcrypt verifier. Không tuyên bố “không có plaintext secret trên đĩa”. Phạm vi chứng minh là không có plaintext secret ngoài hai config protected cần thiết cho upstream.
