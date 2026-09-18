# Current Handoff

Last updated: 2026-09-18 UTC.

## Current status

Phase 0A non-destructive compatibility inventory is complete and pushed on branch `phase-0/compatibility-inventory`. Runtime implementation has not started. No effective Antigravity/Codex configuration was changed and no provider account/OAuth flow was used.

## Repository and delivery

- Target remote: `https://github.com/trungqwe/dual-pool.git`.
- Start HEAD: `5e91df5f31531452a74444d171a7edb7db405573`.
- Branch: `phase-0/compatibility-inventory`.
- Phase 0A commit: `3f70ed84cbc52772ed1e448f3ee891c6f3343bbe`.
- Push result: PASS — new remote branch created normally with upstream tracking; no force push.
- Compare/PR URL: `https://github.com/trungqwe/dual-pool/compare/main...phase-0/compatibility-inventory`.
- Delivery receipt: this mutable handoff update records the pushed run commit; its own commit identity is intentionally left to Git history to avoid self-reference.
- Run report: `docs/reports/2026-09-18T1930Z-phase-0a-compatibility-inventory.md`.
- Evidence: `evidence/2026-09-18T1930Z-phase-0a-compatibility-inventory/`.

## Phase 0A results

- Governance now separates non-destructive Phase 0A from reversible mutation-dependent Phase 0B.
- Root `.gitignore` and `docs/06-COMPATIBILITY-PROBE.md` define raw → sanitize → scan → commit; sanitized evidence remains trackable.
- Repository risk R-13 is resolved by authenticated inventory.
- Observed platform is Microsoft Windows 10 Pro `10.0.19045` x64, not the Windows 11 target in the charter. Results are valid for this observed machine but do not satisfy a Windows 11 acceptance claim.
- Proposed ports `51074`, `8317`, `8318` and candidate callback ports `1455`, `51121` were free at capture time.

## Codex discovery

- CLI: `0.154.0`; exact command path/hash are sanitized in `versions.json`.
- VS Code extension: `openai.chatgpt` `26.311.21342`.
- Default Codex home: `%USERPROFILE%\.codex`; config candidate exists, but contents were not read or changed.
- `codex debug models --bundled` is supported. Exact bundled/effective catalogs contain canonical `gpt-6-astra` with list visibility, API support, unified shell, code-only tool mode, and multi-agent v2 metadata.
- Extension config sharing, outbound Responses transport, and picker/provider retention remain UNKNOWN for Phase 0B.

## Antigravity discovery

- Product version `2.5.5`, core `1.107.0`, bundled extension `0.2.0`.
- Existing settings candidate: `%APPDATA%\Antigravity IDE\User\settings.json`; contents were not read or changed.
- Installed static bundle contains `jetski.cloudCodeUrl`, but the contributed settings schema does not declare it. The key and loopback behavior remain unproven until Phase 0B.

## CLIProxyAPI candidate

- Latest official release rechecked: `v7.3.7`, commit `b773607e3e7756dc6020a291825e4eb08899595a`.
- Windows amd64 archive SHA-256: `da5466b81beb7c769b99e26a5f6f41d9999a07be7c36be170167f10a2a6ecfc7`; matches both published `checksums.txt` and GitHub asset digest.
- Executable SHA-256: `bb44c6fa6a30214a294bdf64cf7385aa7ff65982aebd501ae37dc6a62a227072`.
- Tagged config/source contains required isolation, management, routing/affinity, streaming, WebSocket, OAuth/inventory and Responses/Gemini route surfaces recorded in `cliproxy-schema.json`.
- Credential-free disposable probe bound only `127.0.0.1`, enforced management/client keys, created no auth files, and stopped cleanly.
- Provider-specific response shapes and dedicated health semantics remain UNKNOWN without later authorized credentialed probes.

## Unknown status

- U-001: BLOCKED.
- U-002: BLOCKED.
- U-003: BLOCKED.
- U-004: BLOCKED pending separately authorized credential-specific probe.
- U-005: PARTIAL / UNKNOWN.
- U-006: BLOCKED pending user-assisted picker action.
- U-007: PROBED.
- U-008: PARTIAL / UNKNOWN.

Exact evidence and required next proof are in `docs/03-DECISIONS-AND-EVIDENCE.md`.

## Cleanup and security

- Disposable CLIProxyAPI process and listeners are absent.
- `%TEMP%` probe root containing archive, binary, source checkout, full catalogs, synthetic keys/configs and logs was removed after sanitization.
- No application integration was enabled, no account connected, and no foreign process terminated.
- JSON/link/privacy/secret/whitespace and final diff checks are recorded in the run report and `probe-results.json`.

## Latest Phase 0B run

- Run report: `docs/reports/2026-09-18T1953Z-phase-0b-reversible-compatibility.md`.
- Evidence: `evidence/2026-09-18T1953Z-phase-0b-reversible-compatibility/`.
- Codex CLI transport: `PASS` — synthetic recorder observed `POST /v1/responses`, `gpt-6-astra`, `wire_api=responses`, loopback-only traffic, and metadata-only capture.
- Codex picker retention: `BLOCKED` — user-assisted picker action is required; automatic Codex CLI/extension UI automation is outside the active Windows automation safety boundary.
- Antigravity U-001/U-002/U-003: `BLOCKED` pending user-assisted settings/reload/discovery actions. U-004 is separately `BLOCKED` pending authorized credential-specific probing.
- Codex CLI transport v2: `PASS` — reproducible script and sanitized key/type tree are in the append-only evidence directory.
- Cleanup: synthetic process and listener were removed; no application integration was enabled.
- Phase 1 remains forbidden. Repeat final acceptance on Windows 11 because this machine is Windows 10.

## Exact next bounded objective

Phase 0B — run backed-up and reversible Codex custom-provider/picker and Antigravity redirect/passthrough/envelope experiments. Entry requires confirming whether this Windows 10 machine is the intended exploratory target or providing the Windows 11 target required by the charter. Continue to prohibit provider-account work except a separately authorized U-004 probe.
