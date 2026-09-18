# Phase 0A Compatibility Inventory Summary

Run: `2026-09-18T1930Z-phase-0a-compatibility-inventory`

## Outcome

The target repository and Windows environment are available for Phase 0. The non-destructive inventory found installed Codex and Antigravity products, free proposed/callback ports, a safe Codex catalog export, and an independently hash-verified CLIProxyAPI candidate. No application configuration or provider account was changed.

## Key findings

- Codex CLI `0.154.0` exposes `codex debug models --bundled`. Canonical `gpt-6-astra` exists in both bundled and effective catalogs with list visibility, API support, unified shell, code-only tool mode and multi-agent v2 metadata.
- The Codex config candidate is `%USERPROFILE%\.codex\config.toml`; it exists, but Phase 0A did not read or modify its contents. Extension/CLI config sharing and provider transport remain Phase 0B work.
- Antigravity IDE product `2.5.5` (core `1.107.0`, bundled extension `0.2.0`) has `%APPDATA%\Antigravity IDE\User\settings.json` as an existing candidate. Its installed static bundle contains `jetski.cloudCodeUrl`, but its contributed settings schema does not declare that key and Phase 0A did not test whether loopback HTTP is honored.
- Proposed ports `51074`, `8317`, `8318` and CLIProxyAPI v7.3.7 callback ports `1455` (Codex) and `51121` (Antigravity) were free at capture time.
- Official CLIProxyAPI `v7.3.7` remains latest. The Windows amd64 archive matches both the published `checksums.txt` value and GitHub asset digest. Binary version is `7.3.7`, commit `b773607e`.
- A credential-free disposable instance with independent CSPRNG 256-bit keys bound only `127.0.0.1`. Missing/wrong management credentials and a missing client credential returned `401`; correct credentials returned `200`. No auth file was created, and the process/listener were gone after stop.

## Unknowns after Phase 0A

- U-001: partial evidence only; path/key candidates exist, but loopback behavior is UNKNOWN.
- U-002: UNKNOWN; native upstream URL/routes require Phase 0B observation.
- U-003: UNKNOWN; envelope round trip requires Phase 0B.
- U-004: UNKNOWN; no Google credential/model call was allowed.
- U-005: partial evidence; Astra is in the exact installed CLI catalog, but extension config sharing and outbound transport are UNKNOWN.
- U-006: UNKNOWN; picker/provider retention is Phase 0B.
- U-007: PROBED; exact installed CLI supports safe bundled/effective catalog export.
- U-008: partial evidence; candidate integrity, config fields, registered routes, loopback bind and authentication were probed. Provider-specific response shapes and dedicated health semantics remain UNKNOWN without later credentialed tests.

## Evidence boundary

Raw downloads, source checkout, full Codex catalogs, logs, temporary configs/keys and disposable data roots were kept under `%TEMP%\dual-pool-probe\2026-09-18T1930Z-phase-0a-compatibility-inventory\`. Only whitelisted derivatives in this directory are intended for commit after JSON, privacy and secret validation.

## Next permitted work

After cleanup and final security gates pass, the next bounded task is Phase 0B: backed-up, reversible Codex provider/picker and Antigravity redirect/passthrough/envelope experiments. U-004 remains account-dependent and must not be guessed.
